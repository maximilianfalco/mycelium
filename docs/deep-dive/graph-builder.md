# internal/indexer — Graph Builder

Stage 6 of the indexing pipeline. Takes all the in-memory output from previous stages (workspace info, parsed nodes/edges, resolved imports, unresolved refs, embeddings) and writes it to Postgres in a single transaction. Handles upserts, deduplication, stale node cleanup, and vector storage.

## Why this exists

Stages 1–5 produce data in memory: workspace structure, parsed AST nodes, resolved edges, embedding vectors. None of that is persisted until the graph builder runs. It's the boundary between "analysis" and "storage" — everything upstream is stateless, everything downstream (search, chat, query engine) reads from the database.

## API

### BuildGraph

```go
func BuildGraph(ctx context.Context, pool *pgxpool.Pool, input *BuildInput) (*BuildResult, error)
```

Writes all pipeline output in a single transaction:

1. Upserts the workspace row
2. Upserts all packages
3. Upserts all nodes (with optional embedding vectors)
4. Upserts all edges (resolved imports, structural contains, depends_on)
5. Inserts unresolved refs (clears old ones first)
6. Deletes stale nodes from files no longer on disk

Returns a `BuildResult` with counts and timing. Rolls back on any error.

### CleanupStale

```go
func CleanupStale(ctx context.Context, pool *pgxpool.Pool, workspaceID string, currentFilePaths []string) (int, error)
```

Standalone version of stale cleanup for use outside the full pipeline. Deletes all nodes in the workspace whose `file_path` is not in the provided list. If the list is empty, deletes all nodes in the workspace.

## Types

### BuildInput

```go
type BuildInput struct {
    ProjectID  string
    SourceID   string
    SourcePath string
    Workspace  *detectors.WorkspaceInfo
    Nodes      []parsers.NodeInfo
    Edges      []parsers.EdgeInfo
    Resolved   []ResolvedEdge
    Unresolved []UnresolvedRef
    DependsOn  []ResolvedEdge
    Embeddings map[string][]float32  // qualifiedName -> vector
    FilePaths  []string              // all current file paths
}
```

Aggregates output from every prior stage. The orchestrator (not yet built) will assemble this struct and pass it to `BuildGraph`.

### BuildResult

```go
type BuildResult struct {
    WorkspaceID    string
    NodesUpserted  int
    EdgesUpserted  int
    UnresolvedRefs int
    NodesDeleted   int
    Duration       time.Duration
}
```

## ID generation

All IDs are deterministic strings derived from the data hierarchy. Running the pipeline twice on the same code produces the same IDs, which is what makes upserts work.

| Entity | ID format | Example |
|---|---|---|
| Workspace | `{projectID}/{sourceID}` | `proj-1/src-1` |
| Package | `{workspaceID}/{packageName}` | `proj-1/src-1/@mycelium/core` |
| Node | `{prefix}/{filePath}::{qualifiedName}` | `proj-1/src-1/@mycelium/core/src/auth.ts::validateToken` |

For nodes, `prefix` is the package ID if the node belongs to a package, otherwise the workspace ID.

## Upsert strategy

All writes use `INSERT ... ON CONFLICT DO UPDATE`. This means:
- First run inserts everything
- Subsequent runs update only what changed
- Existing data (like manually added metadata) is preserved for unchanged rows
- The vector index doesn't need to be rebuilt from scratch

Nodes, edges, and unresolved refs are batched in groups of 1000 using `pgx.Batch` to avoid holding large locks.

## Edge handling

Edges come from three sources, all merged and deduplicated before writing:

| Source | Edge kinds | Weight |
|---|---|---|
| Resolved imports (`input.Resolved`) | imports, calls, extends, implements, uses_type, embeds | varies |
| Structural edges (`input.Edges`) | contains | 1.0 |
| Package dependencies (`input.DependsOn`) | depends_on | 1.0 |

**Deduplication**: if the same `(source, target, kind)` tuple appears multiple times, the one with the highest weight wins.

**Edge weights**:
- `contains`, `extends`, `implements`, `embeds` → 1.0 (structural, always relevant)
- Everything else (`imports`, `calls`, `depends_on`, `uses_type`) → 0.5

## Embedding storage

Nodes with an entry in `input.Embeddings` get their vector stored via `pgvector.NewVector()`. Nodes without embeddings get `NULL` in the embedding column. The pool's `AfterConnect` callback registers pgvector types so pgx knows how to serialize vectors.

## Stale cleanup

After upserting, the builder deletes nodes whose `file_path` doesn't appear in `input.FilePaths`. This handles:
- Deleted files (removed since last index)
- Renamed files (old path no longer exists)

Edges cascade via foreign keys — deleting a node automatically deletes its edges and unresolved refs.

## Language detection

`detectLanguage` counts file extensions across all input files and picks the majority language. Used to tag nodes with a language field for filtering in search.

| Extensions | Language |
|---|---|
| `.ts`, `.tsx` | `typescript` |
| `.js`, `.jsx` | `javascript` |
| `.go` | `go` |

## No spore lab endpoint

Unlike stages 0–5, there's no debug endpoint for the graph builder. Storage is a write operation, not an inspection step — there's nothing meaningful to visualize in the UI that isn't already visible by querying the database directly (via pgAdmin at `localhost:5050`). The integration tests cover correctness.

## Files

| File | Purpose |
|---|---|
| `graph_builder.go` | `BuildGraph()`, `CleanupStale()`, upsert functions, ID generation, helpers |
| `../tests/integration/graph_builder_test.go` | Integration tests: basic write, idempotency, update detection, stale cleanup, cascade delete, embedding storage |
