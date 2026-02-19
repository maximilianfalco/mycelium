# internal/indexer

Handles the first two stages of the indexing pipeline: figuring out what kind of project we're looking at (workspace detection) and collecting the files to parse (crawling).

## Workspace detection (`detectors/`)

`detectors.DetectWorkspace(path)` probes a directory and returns a `WorkspaceInfo` describing the project structure. It tries language-specific detectors in order (Node first, then Go) and falls back to `"standalone"` if nothing matches.

### Detectors

| Detector | Trigger files | What it finds |
|---|---|---|
| `NodeDetector` | `package.json` | npm/yarn/pnpm/lerna monorepos, standalone Node projects, TypeScript path aliases |
| `GoDetector` | `go.work`, `go.mod` | Go workspaces (multi-module) and standalone Go modules |

### WorkspaceInfo fields

| Field | Description |
|---|---|
| `workspaceType` | `monorepo`, `standalone`, or `go-workspace` |
| `packageManager` | `npm`, `yarn`, `pnpm`, `lerna`, or `go` |
| `packages` | List of discovered packages with name, path, version, and entry point |
| `aliasMap` | Maps package names to filesystem paths (e.g. `@mycelium/core` -> `packages/core`) |
| `tsconfigPaths` | TypeScript path aliases from `tsconfig.json` (follows `extends` chains) |

### Node detector details

- **Monorepo detection**: checks `pnpm-workspace.yaml`, `package.json` `workspaces` field (array and object forms), and `lerna.json`
- **Package manager**: identified by lockfile presence (`pnpm-lock.yaml` -> pnpm, `yarn.lock` -> yarn, `package-lock.json` -> npm)
- **Package discovery**: expands workspace globs, supports negation patterns (e.g. `!packages/deprecated-*`)
- **Entry points**: heuristic search — tries `src/index.ts`, `src/index.tsx`, `src/index.js`, `index.ts`, `index.js`, then falls back to `main`/`source`/`module` fields in `package.json`
- **TSConfig parsing**: strips JSON comments, follows `extends` chains to collect all path aliases

### Go detector details

- **go.work**: parses `use` directives to find all modules in a multi-module workspace
- **go.mod**: extracts module path and Go version for standalone modules
- **Package discovery**: walks module directories for any dir containing `.go` files, skips `vendor/`, `testdata/`, and hidden dirs
- **Entry points**: looks for `main.go`

### Mixed repos

If a directory has both `package.json` and `go.mod`, the Node detector takes priority since JS/TS projects are more likely to have complex workspace configurations that matter for alias resolution.

## File crawling (`crawler.go`)

`CrawlDirectory(path, codeOnly)` walks the filesystem and returns a list of files to parse.

### Filtering

| Filter | Behavior |
|---|---|
| `.gitignore` | Respected at root and nested levels, scoped to their directory |
| Hardcoded skip dirs | `node_modules`, `.git`, `dist`, `build`, `.next`, `__pycache__`, `vendor`, `testdata` |
| Hidden dirs | Any directory starting with `.` |
| Symlinks | Skipped |
| Lockfiles | `package-lock.json`, `pnpm-lock.yaml`, `yarn.lock`, `go.sum` |
| `.log` files | Skipped |
| File size | >100KB skipped |
| Code-only mode | When `codeOnly=true`, only `.ts`, `.tsx`, `.js`, `.jsx`, `.go` files are included |

### CrawlResult

Returns a list of `FileInfo` (absolute path, relative path, extension, size) plus stats broken down by extension (total count, skipped count, per-extension counts).

## Files

| File | Purpose |
|---|---|
| `crawler.go` | `CrawlDirectory()` — filesystem walker with gitignore + filtering |
| `crawler_test.go` | Crawler tests (gitignore, size limits, symlinks, hidden dirs, lockfiles) |
| `detectors/detectors.go` | `WorkspaceInfo`/`PackageInfo` types, `LanguageDetector` interface, `DetectWorkspace()` orchestrator |
| `detectors/node.go` | `NodeDetector` — npm/yarn/pnpm/lerna monorepo detection, tsconfig path extraction |
| `detectors/go_detect.go` | `GoDetector` — go.work/go.mod workspace detection |
| `detectors/detectors_test.go` | Comprehensive tests for both detectors |
