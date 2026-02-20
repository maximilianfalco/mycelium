# internal/indexer — Pipeline Orchestrator

The main entry point for indexing a project. Chains all 7 pipeline stages (change detection → workspace detection → crawling → parsing → import resolution → embedding → graph storage) into a single `IndexProject()` call. Manages background execution, concurrent indexing guards, body hash skip-embed optimization, and live status tracking.

## Why this exists

Each pipeline stage (crawler, parser, resolver, embedder, graph builder) operates independently. The orchestrator wires them together in the correct order, feeds the output of each stage into the next, handles errors, and tracks progress. Without it, calling stages manually would require ~50 lines of glue code per indexing run, plus manual DB lookups for change detection and metadata updates.

## API

### IndexProject

```go
func IndexProject(ctx context.Context, pool *pgxpool.Pool, cfg *config.Config, oaiClient *openai.Client, projectID string, status *IndexStatus, force bool) *IndexResult
```

Runs the full pipeline for a project. Called from the `POST /projects/:id/index` handler in a background goroutine.

**Per-source execution order:**

| # | Stage | Function | What it does |
|---|-------|----------|-------------|
| 0 | Change detection | `DetectChanges()` | Git diff or mtime comparison. Determines added/modified/deleted files. |
| 1 | Workspace detection | `detectors.DetectWorkspace()` | Discovers packages, alias maps, tsconfig paths. |
| 2 | File crawling | `CrawlDirectory()` | Walks directories respecting .gitignore. |
| 3 | Parsing | `parseFiles()` | Parallel AST parsing via errgroup (8 workers). |
| 4 | Import resolution | `ResolveImports()` | Resolves raw imports to concrete files. |
| 5 | Embedding | `embedChangedNodes()` | Body hash compare + OpenAI API for changed nodes only. |
| 6 | Graph storage | `BuildGraph()` | Upserts workspace/packages/nodes/edges to Postgres. |
| 7 | Metadata | `updateSourceMetadata()` | Writes `last_indexed_commit`, `last_indexed_branch`, `last_indexed_at`. |

After all sources are processed, a project-level cross-source resolution step runs `ResolveCrossSources()` to resolve imports between workspaces in different sources.

**Returns** `*IndexResult` with aggregate counts across all sources.

**Concurrent indexing guard:** Uses `sync.Map` to prevent two jobs for the same project from running simultaneously. Returns an error in `IndexResult.Errors` if a job is already active. When `force=true`, the guard is bypassed.

### StatusStore

```go
type StatusStore struct { ... }

func NewStatusStore() *StatusStore
func (s *StatusStore) Set(jobID string, status *IndexStatus)
func (s *StatusStore) Get(jobID string) *IndexStatus
func (s *StatusStore) GetByProject(projectID string) *IndexStatus
```

Thread-safe in-memory store for tracking indexing job progress. Uses `sync.RWMutex` for concurrent access. `GetByProject` returns the most recent job by `StartedAt` timestamp, skipping nil entries.

A single global instance lives in `routes/indexing.go` and is shared between the trigger and status endpoints.

### IndexStatus

```go
type IndexStatus struct {
    JobID     string       // unique per run, e.g. "idx-myproject-1708300000000"
    ProjectID string
    Status    string       // "running" | "completed" | "failed"
    Stage     string       // current stage name (e.g. "parsing", "embedding")
    Progress  string       // human-readable progress (e.g. "source 2/3: auth")
    Result    *IndexResult // populated when done
    Error     string       // first error message if failed
    StartedAt time.Time
    DoneAt    *time.Time
}
```

Updated in real-time by the `updateStatus` callback passed through the pipeline. The status endpoint reads this to report live progress.

## Internal functions

### buildFilesToParse

```go
func buildFilesToParse(crawlResult *CrawlResult, changeSet *ChangeSet) []FileInfo
```

Filters the crawl result to only files that appear in the change set's `AddedFiles` or `ModifiedFiles`. On a full index (`IsFullIndex == true`), returns all crawled files. Deleted files are not included — they're handled by `BuildGraph`'s stale cleanup.

### parseFiles

```go
func parseFiles(ctx context.Context, files []FileInfo, rootPath string) ([]parsers.NodeInfo, []parsers.EdgeInfo, []string)
```

Parses files in parallel using `errgroup.Group` with `SetLimit(8)`. Each goroutine reads the file, calls `parsers.ParseFile`, and rewrites absolute paths in `contains`/`imports` edges to relative paths. Parse errors are collected (not fatal) — a single broken file doesn't abort the pipeline.

### embedChangedNodes

```go
func embedChangedNodes(ctx, pool, oaiClient, cfg, projectID, sourceID string, allNodes []parsers.NodeInfo, updateStatus func(stage, progress string)) (map[string][]float32, int, error)
```

The skip-embed optimization. Loads existing `(qualified_name, body_hash)` and `(qualified_name, embedding)` from the DB for the workspace. For each parsed node:

- If the body hash matches AND an existing embedding exists → **reuse** (no API call)
- Otherwise → add to the embed queue

Only the changed nodes get sent to `EmbedBatched()`. Returns a map of `qualifiedName → vector` (mix of reused and freshly embedded).

**Graceful degradation:** If `oaiClient` is nil (no API key configured), returns an empty map with a warning. Nodes will be stored without embeddings — semantic search won't work, but structural queries and the graph will.

### updateSourceMetadata

```go
func updateSourceMetadata(ctx context.Context, pool *pgxpool.Pool, sourceID string, cs *ChangeSet) error
```

After successful indexing, writes back to `project_sources`:
- `last_indexed_commit` — from `ChangeSet.CurrentCommit` (nil for non-git)
- `last_indexed_branch` — from `ChangeSet.CurrentBranch` (nil for detached HEAD)
- `last_indexed_at` — `time.Now()`

This is the commit pointer that stage 0 (change detection) reads on the next run. It must happen last — after all data is committed — so a failed index doesn't advance the pointer.

## Data flow diagram

```
project_sources (DB)
       │
       ▼
  DetectChanges ──▶ ChangeSet { added, modified, deleted }
       │
       ▼
  DetectWorkspace ──▶ WorkspaceInfo { packages, aliasMap, tsconfigPaths }
       │
       ▼
  CrawlDirectory ──▶ []FileInfo
       │
       ├── buildFilesToParse (scope to change set)
       ▼
  parseFiles (parallel) ──▶ []NodeInfo, []EdgeInfo
       │
       ▼
  ResolveImports ──▶ ResolveResult { resolved, unresolved, dependsOn }
       │
       ├── loadExistingHashes (DB) ──▶ skip unchanged
       ▼
  EmbedBatched (OpenAI) ──▶ map[qualifiedName][]float32
       │
       ▼
  BuildGraph (DB) ──▶ BuildResult { nodes, edges, deleted }
       │
       ▼
  updateSourceMetadata (DB) ──▶ last_indexed_commit, last_indexed_at
```

## HTTP integration

The pipeline is triggered and monitored through two endpoints in `routes/indexing.go`:

| Endpoint | Method | What it does |
|----------|--------|-------------|
| `/projects/:id/index` | POST | Creates a job, launches `IndexProject` in a goroutine, returns 202 with `{ jobId }` |
| `/projects/:id/index/status` | GET | Returns live job status + DB node/edge counts + `lastIndexedAt` |

The trigger endpoint returns 409 Conflict if a job is already running for the project.

## Configuration

| Config field | Used by | Default |
|---|---|---|
| `MaxAutoReindexFiles` | Change detection threshold | 100 |
| `MaxEmbeddingBatch` | OpenAI batch size | 1000 |
| `OpenAIAPIKey` | Embedding (nil client if empty) | — |

## Constants

| Name | Value | Purpose |
|------|-------|---------|
| `parseWorkers` | 8 | Max concurrent file parsing goroutines |
