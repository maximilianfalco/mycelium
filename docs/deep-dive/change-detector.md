# internal/indexer — Change Detector

Stage 0 of the indexing pipeline. Compares the current state of a source directory against its last indexed state to determine which files were added, modified, or deleted. Uses `git diff` for git repos and file `mtime` for plain directories. Output scopes all downstream stages so they only process what actually changed.

## Why this exists

Re-parsing and re-embedding an entire codebase on every index run is wasteful. The change detector narrows the work to only the files that changed. For a typical commit touching 3 files in a 3M-line monorepo, this turns a 20-minute full index into a sub-minute incremental update. The expensive part (embedding via OpenAI API) is entirely skipped for unchanged files.

## API

### DetectChanges

```go
func DetectChanges(ctx context.Context, sourcePath string, lastIndexedCommit *string, lastIndexedAt *time.Time, maxAutoReindexFiles int, force bool) (*ChangeSet, error)
```

Entry point. Detects whether `sourcePath` is a git repo and dispatches to the appropriate strategy.

**Parameters:**
- `sourcePath` — absolute path to the source directory
- `lastIndexedCommit` — previous commit hash from `project_sources.last_indexed_commit` (nil for first index)
- `lastIndexedAt` — previous index timestamp from `project_sources.last_indexed_at` (used for mtime fallback)
- `maxAutoReindexFiles` — threshold for auto-reindex skip (0 disables)
- `force` — when true, forces a full reindex regardless of change detection

## Types

### ChangeSet

```go
type ChangeSet struct {
    IsGitRepo         bool     `json:"isGitRepo"`
    CurrentCommit     string   `json:"currentCommit"`
    CurrentBranch     string   `json:"currentBranch"`
    LastIndexedCommit string   `json:"lastIndexedCommit"`
    IsFullIndex       bool     `json:"isFullIndex"`
    AddedFiles        []string `json:"addedFiles"`
    ModifiedFiles     []string `json:"modifiedFiles"`
    DeletedFiles      []string `json:"deletedFiles"`
    ThresholdExceeded bool     `json:"thresholdExceeded"`
}
```

File paths are relative to the source root. Only code files are included (`.ts`, `.tsx`, `.js`, `.jsx`, `.go`) — non-code files, lockfiles, and files in skip directories are filtered out.

## Git strategy

Uses `git diff --name-status --diff-filter=ACDMR` between the last indexed commit and current HEAD. This is content-aware — switching branches and back produces no changes if the code is identical.

| Scenario | Behavior |
|---|---|
| First index (`lastIndexedCommit` is nil/empty) | Full index — crawl all files, mark as added |
| Same commit | Empty change set, no work |
| Normal diff | Categorize files as A/C (added), M (modified), D (deleted), R (rename → delete old + add new) |
| Force push / missing commit | `git diff` fails → log warning, fall back to full index |
| Empty repo (no commits) | `git rev-parse HEAD` fails → log warning, return empty change set |
| Detached HEAD | `git symbolic-ref` fails → `CurrentBranch` is empty string, diff still works |

### Branch detection

`CurrentBranch` is populated via `git symbolic-ref --short HEAD`. Returns empty string for detached HEAD. This is informational — stored in `project_sources.last_indexed_branch` for visibility but doesn't affect indexing logic.

## Mtime strategy

For non-git directories where `git diff` isn't available. Walks the filesystem via `CrawlDirectory` and compares each file's `mtime` against `lastIndexedAt`.

| Scenario | Behavior |
|---|---|
| First index (`lastIndexedAt` is nil) | Full index — all files marked as added |
| Subsequent index | Files with `mtime > lastIndexedAt` marked as modified |

**Known limitation:** mtime can't detect deleted files — that requires comparing the current file list against what's in the database. The pipeline orchestrator handles this via the graph builder's stale cleanup.

**Why mtime is less reliable than git diff:**
- `git checkout` can update mtime without changing content
- Build tools and editors can touch files without modifying them
- The `body_hash` check in the embedding stage catches false positives — if content produces the same hash, the OpenAI API call is skipped

## File filtering

Changed files from `git diff` are filtered through the same rules as the crawler:

- Only code extensions: `.ts`, `.tsx`, `.js`, `.jsx`, `.go`
- Skip known directories: `node_modules`, `dist`, `build`, `vendor`, `.git`, hidden dirs
- Skip lockfiles: `package-lock.json`, `pnpm-lock.yaml`, `yarn.lock`, `go.sum`

Filtering happens after the diff, on the raw file list from git. The threshold check counts filtered files (only code files matter for the limit).

## Threshold guard

When the number of changed code files exceeds `maxAutoReindexFiles` (default 100, configurable via `MAX_AUTO_REINDEX_FILES` in `.env`):

- `ThresholdExceeded` is set to `true`
- File lists are still populated (the caller decides whether to proceed)
- A warning is logged

This prevents the background watcher from triggering expensive re-indexes on large branch switches. The user can always trigger a manual index to override.

Setting `maxAutoReindexFiles` to 0 disables the threshold check entirely.

## Debug endpoint

`POST /debug/changes` calls `DetectChanges` with real git operations. Accepts an optional `lastIndexedCommit` in the request body to simulate incremental detection.

```
Request:  { path: string, lastIndexedCommit?: string }
Response: { isGitRepo, currentCommit, lastIndexedCommit, isFullIndex, addedFiles, modifiedFiles, deletedFiles, thresholdExceeded }
```

When `lastIndexedCommit` is omitted, behaves as a first index (full crawl).

## Files

| File | Purpose |
|---|---|
| `change_detector.go` | `DetectChanges()`, git diff parsing, mtime comparison, file filtering |
| `change_detector_test.go` | 19 tests: first index, no changes, add/modify/delete/rename, filtering, threshold, force push fallback, empty repo, mtime, branch info, mixed changes |
