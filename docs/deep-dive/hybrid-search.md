# 🔍 Hybrid Search

Mycelium's search combines two ranking signals — keyword matching and vector similarity — using Reciprocal Rank Fusion (RRF) to produce a single ranked result set.

## Why Hybrid?

| Search type | Good at | Bad at |
|---|---|---|
| **Keyword** (full-text) | Exact name matches: `BuildGraph`, `handleLogin` | Conceptual queries: "authentication middleware" |
| **Semantic** (vector) | Conceptual similarity: "auth logic" → `verifyJWT` | Exact names: `BuildGraph` might rank below `ConstructDAG` |
| **Hybrid** (both) | Both — exact matches rank first, conceptual matches still surface | — |

## How It Works

Every search query runs in a single Postgres transaction:

### 1. Keyword Search

Uses Postgres built-in full-text search:

```sql
SELECT n.id, ROW_NUMBER() OVER (ORDER BY ts_rank(n.search_vector, query) DESC) AS rank_k
FROM nodes n
JOIN workspaces ws ON n.workspace_id = ws.id,
     plainto_tsquery('english', $query) query
WHERE ws.project_id = $project_id
  AND n.search_vector @@ plainto_tsquery('english', $query)
ORDER BY ts_rank(n.search_vector, query) DESC
LIMIT $candidate_limit
```

The `search_vector` column is a Postgres generated column with weighted fields:

| Weight | Fields | Priority |
|---|---|---|
| **A** (highest) | `name`, `qualified_name` | Symbol names rank first |
| **B** | `signature` | Parameter names, return types |
| **C** | `docstring` | Natural language descriptions |

Source code is excluded — too noisy for keyword matching.

### 2. Semantic Search

Uses pgvector cosine similarity:

```sql
SELECT n.id, ROW_NUMBER() OVER (ORDER BY n.embedding <=> $query_vector) AS rank_v
FROM nodes n
JOIN workspaces ws ON n.workspace_id = ws.id
WHERE ws.project_id = $project_id
  AND n.embedding IS NOT NULL
ORDER BY n.embedding <=> $query_vector
LIMIT $candidate_limit
```

The query text is embedded via OpenAI `text-embedding-3-small` (1536 dimensions) before running this query.

### 3. Reciprocal Rank Fusion

Both result sets are merged via `FULL OUTER JOIN` and scored:

```sql
SELECT
    COALESCE(v.id, k.id) AS id,
    COALESCE(1.0 / (60 + v.rank_v), 0) + COALESCE(1.0 / (60 + k.rank_k), 0) AS rrf_score
FROM vector_results v
FULL OUTER JOIN keyword_results k ON v.id = k.id
ORDER BY rrf_score DESC
```

**k=60** is the standard RRF constant (same as Elasticsearch, Pinecone, etc.).

**Candidate oversampling:** Each search returns `3x` the requested limit before fusion, giving RRF enough data to merge effectively.

## Indexing

### Keyword Index

The `search_vector` column is `GENERATED ALWAYS AS ... STORED` — Postgres auto-maintains it. A GIN index enables fast full-text lookup:

```sql
CREATE INDEX idx_nodes_search_vector ON nodes USING GIN (search_vector);
```

Zero application code needed. Any node insert/update automatically refreshes the keyword index.

### Vector Index

Embeddings use IVFFlat indexing for approximate nearest neighbor search:

```sql
CREATE INDEX idx_nodes_embedding ON nodes
    USING ivfflat (embedding vector_cosine_ops)
    WITH (lists = 100);
```

The search runs with `SET LOCAL ivfflat.probes = 10` for better recall.

## API

### Go Functions

```go
// Full hybrid search — embeds query, then runs both searches
func HybridSearch(ctx, pool, oaiClient, query, projectID string, limit int, kinds []string) ([]SearchResult, error)

// Pre-computed vector variant (used in tests)
func HybridSearchWithVector(ctx, pool, queryVec []float32, query, projectID string, limit int, kinds []string) ([]SearchResult, error)

// Pure semantic search (no keyword component)
func SemanticSearch(ctx, pool, oaiClient, query, projectID string, limit int, kinds []string) ([]SearchResult, error)
```

### SearchResult

```go
type SearchResult struct {
    NodeID        string  `json:"nodeId"`
    QualifiedName string  `json:"qualifiedName"`
    FilePath      string  `json:"filePath"`
    Kind          string  `json:"kind"`
    Similarity    float64 `json:"similarity"`     // RRF score for hybrid, cosine for semantic
    Signature     string  `json:"signature"`
    SourceCode    string  `json:"sourceCode"`
    Docstring     string  `json:"docstring"`
}
```

## Filtering

Both keyword and semantic searches support optional `kinds` filtering (e.g., `["function", "class"]`). The filter is applied inside both CTEs, so it doesn't waste candidate slots on unwanted node types.
