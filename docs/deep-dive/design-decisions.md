# 🧠 Design Decisions

Core architectural choices and the reasoning behind them.

## Postgres Does Everything

No Redis, no Elasticsearch, no Milvus, no Pinecone. Mycelium uses a single Postgres 16 instance for:

- **Relational data** — projects, sources, workspaces, packages (standard tables)
- **Code graph** — nodes and edges with foreign keys and recursive CTEs for traversal
- **Vector search** — pgvector extension with IVFFlat indexing for 1536-dim embeddings
- **Keyword search** — built-in full-text search with tsvector, ts_rank, and GIN indexes

**Why:** One database means one connection pool, one backup strategy, one deployment, and zero inter-service latency. Postgres's full-text search is BM25-equivalent, pgvector's IVFFlat is fast enough for codebases up to millions of nodes, and recursive CTEs handle graph traversal without a dedicated graph database.

## Hybrid Search with Reciprocal Rank Fusion

Every search query runs two parallel searches and merges results:

```
score = 1/(60 + rank_vector) + 1/(60 + rank_keyword)
```

**Why:** Pure vector search misses exact name matches — searching for `BuildGraph` might rank `ConstructDAG` higher because it's semantically similar. Pure keyword search misses conceptual queries — "authentication middleware" won't find a function called `verifyJWT`. RRF fusion gives exact matches a keyword boost while preserving semantic understanding.

**Why RRF over linear combination:** RRF only uses ranks, not raw scores, so it doesn't require normalizing scores from two completely different systems (cosine distance vs. BM25). It's the same algorithm used by Elasticsearch, Pinecone, and other production hybrid search systems.

## Generated tsvector Column

The `search_vector` column is defined as:

```sql
GENERATED ALWAYS AS (
    setweight(to_tsvector('english', COALESCE(name, '')), 'A') ||
    setweight(to_tsvector('english', COALESCE(qualified_name, '')), 'A') ||
    setweight(to_tsvector('english', COALESCE(signature, '')), 'B') ||
    setweight(to_tsvector('english', COALESCE(docstring, '')), 'C')
) STORED;
```

**Why generated:** Postgres auto-maintains this column on every insert/update. Zero application code for keyword indexing — the indexing pipeline doesn't need to know about full-text search at all.

**Why these weights:** Symbol names (A) are the most important for exact matches. Signatures (B) contain parameter names and return types. Docstrings (C) provide natural language context. Source code is excluded — too large and noisy for keyword matching.

## Incremental Indexing

The pipeline only processes what changed:

1. **Stage 0** — `git diff` against last indexed commit (or mtime fallback for non-git dirs)
2. **Stage 5** — body hash comparison skips re-embedding unchanged code

**Why:** Re-embedding a 50K-node codebase costs ~$1 and takes several minutes. A typical commit touches 3-10 files. Incremental indexing turns this into a sub-minute, sub-cent operation.

**Threshold guard:** If >100 files changed (configurable), the pipeline pauses and asks for confirmation. This prevents accidental full re-indexes on large branch switches.

## Tree-sitter for Parsing

Each language gets a tree-sitter parser that produces the same `NodeInfo`/`EdgeInfo` interface.

**Why tree-sitter:** Fast (written in C), incremental, and produces concrete syntax trees that map directly to code structure. Unlike regex-based extraction, tree-sitter understands nesting, scoping, and language grammar.

**Why a common interface:** Adding a new language means implementing one interface — `Parse(filePath string, source []byte) (*ParseResult, error)`. The rest of the pipeline (import resolution, embedding, graph storage) works without changes.

## Deterministic IDs

All entity IDs are derived from the data hierarchy:

| Entity | ID format | Example |
|---|---|---|
| Workspace | `{projectID}/{sourceID}` | `proj-1/src-1` |
| Package | `{workspaceID}/{packageName}` | `proj-1/src-1/@mycelium/core` |
| Node | `{prefix}/{filePath}::{qualifiedName}` | `proj-1/src-1/src/auth.ts::validateToken` |

**Why:** Running the pipeline twice on the same code produces the same IDs, which makes upserts idempotent. No UUID generation, no collision handling — the hierarchy itself is the key.

## Stdio Transport for MCP

The MCP server communicates over stdin/stdout — no HTTP server, no port allocation.

**Why:** Claude Code, Cursor, and other MCP clients spawn the server as a child process and pipe JSON-RPC messages. Stdio is simpler, more reliable, and avoids port conflicts. The server connects directly to Postgres and OpenAI — it doesn't need the Go API or frontend running.
