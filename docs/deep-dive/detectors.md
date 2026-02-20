# internal/indexer/detectors

Workspace detection — figures out what kind of project lives in a directory (Node monorepo, Go workspace, standalone) and discovers all packages, aliases, and entry points.

## API

Single entry point:

```go
func DetectWorkspace(sourcePath string) (*WorkspaceInfo, error)
```

Tries detectors in order (Node → Go). First non-nil result wins. If nothing matches, returns a fallback standalone `WorkspaceInfo` using the directory name.

## Types

### WorkspaceInfo

```go
type WorkspaceInfo struct {
    WorkspaceType  string            // "monorepo", "standalone", or "go-workspace"
    PackageManager string            // "npm", "yarn", "pnpm", "lerna", "go", or ""
    Packages       []PackageInfo
    AliasMap       map[string]string // package name → relative path to entry point
    TSConfigPaths  map[string]string // tsconfig alias → relative path
}
```

### PackageInfo

```go
type PackageInfo struct {
    Name       string // JS package name or Go import path
    Path       string // relative path from workspace root
    Version    string // semver (JS) or Go version
    EntryPoint string // relative path to entry file within the package
}
```

### LanguageDetector

```go
type LanguageDetector interface {
    Detect(sourcePath string) (*WorkspaceInfo, error)
}
```

Returns `nil, nil` when the directory isn't recognized by this detector — signals "skip me, try the next one."

<details>
<summary><strong>NodeDetector</strong> (<code>node.go</code>) — npm/yarn/pnpm/lerna monorepos and standalone Node/TypeScript projects</summary>

### Detection flow

1. Check for monorepo config (workspace globs)
2. Check for `package.json` — if neither exists, return `nil, nil`
3. Detect package manager from lockfiles
4. Discover packages (expand globs for monorepos, read root `package.json` for standalone)
5. Resolve entry points and build alias map
6. Parse tsconfig paths (root + per-package, root takes precedence)

### Monorepo detection

Checks these files in order, first match wins:

| File | Parser |
|---|---|
| `pnpm-workspace.yaml` | Line-by-line YAML (no external lib) — reads `packages:` block items |
| `package.json` | `workspaces` field — tries `[]string` array form, falls back to `{packages: []}` object form (Yarn classic) |
| `lerna.json` | `packages` array |

### Package manager detection

Identified by lockfile presence:

| Lockfile | Manager |
|---|---|
| `pnpm-lock.yaml` | pnpm |
| `yarn.lock` | yarn |
| `package-lock.json` | npm |

### Package discovery

1. Separate negation patterns (`!`-prefixed) from positive globs
2. Expand positive globs via `filepath.Glob`, filter to directories
3. Skip duplicates and negated matches (e.g. `!packages/deprecated-*`)
4. Read each directory's `package.json` for name, version — skip dirs without one

### Entry point resolution

Probes files in order, first existing file wins:

```
src/index.ts → src/index.tsx → src/index.js → src/index.jsx
index.ts → index.tsx → index.js → index.jsx
```

Falls back to `package.json` fields: `source` → `module` → `main`.

### TSConfig path extraction

- Strips JSON comments (single-line `//` and multi-line `/* */`) via a byte-by-byte state machine that respects string literals
- Follows `extends` chains (resolves relative paths, appends `.json` if needed)
- Depth limit: 10 levels
- Reads `compilerOptions.baseUrl` (defaults to `"."`) and `compilerOptions.paths`
- Resolves each path alias target relative to `baseUrl`, then makes it relative to the workspace root
- Per-package tsconfig paths are merged with root-level paths (root wins on conflicts)

</details>

<details>
<summary><strong>GoDetector</strong> (<code>go_detect.go</code>) — <code>go.work</code> multi-module workspaces and standalone <code>go.mod</code> projects</summary>

### Detection flow

1. If `go.work` exists → parse it for module directories and Go version, then parse each module's `go.mod` for import path. Returns `WorkspaceType: "monorepo"`
2. Else if `go.mod` exists → parse it for module path and Go version. Returns `WorkspaceType: "standalone"`
3. Else → return `nil, nil`

### go.work parsing

- Extracts `go <version>` directive
- Handles both block form (`use ( ... )`) and single-line form (`use ./path`)
- Strips `./` prefix from directory paths
- Skips blank lines and `//` comments

### go.mod parsing

- Extracts `module <path>` and `go <version>` directives
- Does NOT parse `require`, `replace`, or other directives
- Returns error if no `module` directive found

### Go package discovery

Walks the module directory tree:

- **Included**: any directory containing `.go` files
- **Skipped**: `vendor/`, `testdata/`, hidden dirs (starting with `.`)
- Import path = module path + relative subdir path (forward slashes)
- Entry point: `main.go` if present, otherwise empty

</details>

## Detector ordering

Node runs first, Go second. In mixed repos (both `package.json` and `go.mod`), Node wins — JS/TS projects are more likely to have complex workspace configs that matter for alias resolution.

Fallback when no detector matches: `WorkspaceType: "standalone"`, package name = directory basename.

## Files

| File | Purpose |
|---|---|
| `detectors.go` | Types, `LanguageDetector` interface, `DetectWorkspace()` orchestrator, fallback logic |
| `node.go` | `NodeDetector` — monorepo detection, package manager, package discovery, tsconfig paths, entry points |
| `go_detect.go` | `GoDetector` — `go.work`/`go.mod` parsing, Go package discovery |
| `detectors_test.go` | Integration tests (fixture-based) + unit tests (tmpdir-based) |

## Test fixtures

Located at `tests/fixtures/`:

| Fixture | What it tests |
|---|---|
| `monorepo-pnpm` | pnpm workspace with 3 packages, tsconfig extends chains |
| `monorepo-yarn` | Yarn workspace with 2 packages |
| `monorepo-npm` | npm workspace with 2 packages |
| `standalone-repo` | Single `package.json` project with tsconfig paths |
| `no-package-json` | Empty dir — tests fallback to anonymous standalone |
| `go-standalone` | Single `go.mod` project with sub-packages |
| `go-workspace` | `go.work` with 2 modules |
| `cross-repo-a` | Cross-source import resolution (source A) |
| `cross-repo-b` | Cross-source import resolution (source B) |
| `parser` | Parser test fixtures |

Additional in-memory tests cover: JSON comment stripping, negation patterns, Yarn object-form workspaces, single-line `use` in `go.work`, `go.mod` with `require` blocks, mixed repos (Node wins), and `vendor/` exclusion.
