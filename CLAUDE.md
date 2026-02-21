# Mycelium

A local-only code intelligence tool. Parses local repos, builds a structural graph of code relationships, embeds code for semantic search, and exposes it through a chat UI and an MCP server.

## Tech Stack

| Component | Choice |
|---|---|
| Backend | Go (Chi router, pgx for Postgres) |
| Frontend | Next.js (App Router, TypeScript, shadcn/ui) |
| Database | Postgres 16 + pgvector (Docker) |
| CLI | Go (cobra) |
| Embeddings | OpenAI `text-embedding-3-small` |
| Chat | OpenAI `gpt-4o` |

## Project Structure

```
mycelium/
├── cmd/myc/
│   ├── main.go                  # CLI entrypoint
│   ├── root.go                  # Root cobra command (registers subcommands)
│   ├── serve.go                 # `myc serve` — starts the API server
│   ├── mcp.go                   # `myc mcp` — starts the MCP server (stdio)
│   └── colonies.go              # `myc colonies list` — list projects
├── internal/
│   ├── config/config.go         # .env loading, typed Config struct
│   ├── db/
│   │   ├── pool.go              # pgxpool connection + health check
│   │   ├── schema.sql           # DDL for all tables (auto-applied by Docker)
│   │   └── migrations/          # Incremental schema migrations
│   ├── projects/
│   │   ├── models.go            # Project, ProjectSource, ScanResult structs
│   │   ├── manager.go           # CRUD for projects and sources
│   │   └── scanner.go           # Filesystem scanner (detects git repos, monorepos)
│   ├── api/
│   │   ├── server.go            # Chi router, middleware, graceful shutdown
│   │   └── routes/
│   │       ├── helpers.go       # writeJSON, writeError
│   │       ├── projects.go      # Project/source CRUD endpoints
│   │       ├── scan.go          # POST /scan
│   │       ├── debug.go         # 8 POST /debug/* endpoints (spore lab)
│   │       ├── indexing.go      # POST /index, GET /index/status
│   │       ├── search.go        # Semantic + structural search
│   │       └── chat.go          # Streamed chat with context assembly
│   ├── indexer/
│   │   ├── pipeline.go          # 7-stage indexing pipeline orchestrator
│   │   ├── change_detector.go   # Git diff / mtime change detection
│   │   ├── crawler.go           # Directory crawling with gitignore support
│   │   ├── import_resolver.go   # Import resolution against alias maps
│   │   ├── cross_resolver.go    # Cross-source import resolution between workspaces
│   │   ├── graph_builder.go     # Node/edge upsert into Postgres
│   │   ├── embedder.go          # OpenAI embedding with batching + retry
│   │   ├── chunker.go           # Embedding input preparation + tokenization
│   │   ├── parsers/             # Tree-sitter parsers (TS/JS, Go)
│   │   └── detectors/           # Workspace detection (Node.js, Go)
│   ├── engine/
│   │   ├── chat.go              # Streamed chat with OpenAI + context assembly
│   │   ├── context_assembler.go # Search + graph expansion, ranking, token-budgeted context assembly
│   │   ├── graph_query.go       # Structural queries (callers, deps, etc.)
│   │   └── search.go            # Hybrid search (semantic + keyword via RRF)
│   └── mcp/
│       └── server.go            # MCP server (explore, list/detect projects)
├── frontend/
│   └── src/
│       ├── app/
│       │   ├── layout.tsx       # Root layout (dark mode, JetBrains Mono)
│       │   ├── page.tsx         # Colony list (project manager)
│       │   └── projects/[id]/
│       │       └── page.tsx     # Project detail page
│       ├── components/
│       │   ├── ui/              # shadcn components
│       │   ├── colony-list.tsx  # Home page colony list
│       │   ├── project-detail.tsx # Project detail (4 tabs: substrates, forage, spore lab, mycelial map)
│       │   ├── settings-panel.tsx # Colony settings (max file size, root path, reindex)
│       │   └── debug/           # Spore lab (debug) components
│       │       ├── debug-tab.tsx             # Container: path inputs + stage cards
│       │       ├── stage-card.tsx            # Reusable collapsible card with run button
│       │       ├── crawl-output.tsx          # File list table + stats
│       │       ├── parse-output.tsx          # Node cards + edge list
│       │       ├── workspace-output.tsx      # Package tree + alias map
│       │       ├── changes-output.tsx        # File diff lists
│       │       ├── embedding-playground.tsx  # Two textareas + compare + similarity
│       │       └── code-viewer.tsx           # Side panel file viewer with line highlighting
│       └── lib/api.ts           # Typed API client for all endpoints
├── docker-compose.yml           # Postgres + pgvector (port 5433) + pgAdmin (port 5050)
├── Makefile                     # build, test, lint, dev, etc.
├── .env                         # Config (gitignored)
└── tests/integration/           # DB integration tests
```

## Running

```bash
make dev        # starts Postgres, Go API (8080) with air live reload, Next.js (3773) — Ctrl+C stops all
make build      # compile Go binary
make test       # run all tests
make lint       # go vet
```

Individual pieces: `make db`, `make api`, `make frontend`

`make dev` uses [air](https://github.com/air-verse/air) for Go live reload — saving any `.go` file auto-rebuilds and restarts the API. Config in `.air.toml`. Air binary resolved via `$(go env GOPATH)/bin/air`.

pgAdmin available at http://localhost:5050 (email: admin@mycelium.dev, password: admin)

## Ports

| Service | Port |
|---|---|
| Go API | 8080 |
| Next.js frontend | 3773 |
| Postgres | 5433 |
| pgAdmin | 5050 |

## Database

- 7 tables: `projects`, `project_sources`, `workspaces`, `packages`, `nodes`, `edges`, `unresolved_refs`
- Schema auto-applied on first `docker compose up` via init script
- No ORM — raw SQL via pgx (graph queries need recursive CTEs, pgvector operators)
- Hybrid search: `nodes.search_vector` is a generated tsvector column (weighted: A = name/qualified_name, B = signature, C = docstring) with a GIN index, used alongside pgvector for Reciprocal Rank Fusion
- `internal/` is a special Go directory — compiler enforces it can't be imported by external code. Do NOT rename it.

## API Status

All endpoints are fully implemented (no stubs or mocks remain).

| Endpoint group | Status |
|---|---|
| Projects CRUD | Real (Postgres) |
| Sources CRUD | Real (Postgres) |
| POST /scan | Real (filesystem) |
| Debug (spore lab) | Real (8 endpoints: crawl, parse, resolve, read-file, embed-text, compare, workspace, changes) |
| Indexing | Real (7-stage pipeline, background jobs, status polling) |
| Search | Real (semantic via pgvector, structural via graph queries) |
| Chat | Real (streamed via OpenAI, context assembly from graph + embeddings) |
| MCP | Real (stdio transport, 3 tools: explore, list_projects, detect_project) |

## Git

- Commits should be small and focused — one feature, fix, or change per commit
- Do NOT bundle multiple unrelated changes into a single commit
- Example of good commits: `feat: add project CRUD endpoints`, `feat: scaffold Next.js frontend`, `fix: handle null alias in source linking`
- Example of bad commits: `feat: add entire backend and frontend` (too large, impossible to review)

## Testing

- Every feature or fix MUST be accompanied by tests before it's considered done
- Backend (Go): write integration tests in `tests/integration/` for DB-touching code, unit tests alongside the package for pure logic
- Frontend (Next.js): run `npm run build` after every change to catch type errors and build failures
- Run `make test` after every feature to ensure nothing is broken
- Test the happy path AND edge cases (empty inputs, missing data, duplicates, error responses)
- Do NOT skip tests just because the feature "works manually" — if it's not tested, it's not done

## Key Conventions

- `internal/` is a Go-enforced private package boundary — do not rename
- Fungi terminology (colony, substrate, decompose, forage, spore lab) is UI/CLI only — backend code uses plain terms (project, source, index, search, debug)
- Request logging uses colored ANSI output (`fmt.Fprintf` with color codes), not slog — method/status colored by type (green GET, cyan POST, yellow PUT, red DELETE; green 2xx, yellow 4xx, red 5xx)
- All other logging via `log/slog` (not `log` or `fmt.Println`)
- Error handling: `if err != nil { return ..., fmt.Errorf("context: %w", err) }`
- All DB functions take `context.Context` and `*pgxpool.Pool` as first two params
- Route handlers use closure pattern: `func handler(pool) http.HandlerFunc`
- Frontend uses custom shadcn theme (zinc/monochrome, 0 border-radius, no shadows, JetBrains Mono)

## Spore Lab (Debug Mode)

The "spore lab" tab in the project detail view runs individual indexing pipeline stages interactively. All 8 debug endpoints (`/debug/*`) are fully implemented — crawl, parse, resolve, read-file, embed-text, compare, workspace, and changes.

## Mycelium MCP

This project is indexed by its own MCP server (project ID: `initial-test-colony`). Use `explore` for code questions — it does hybrid search + graph expansion in one call, and accepts multiple queries via the `queries` array param to batch questions. All tools accept a `path` param for auto-detection so you don't need to call `detect_project` first. For targeted questions where you already know the file paths, prefer direct file reads instead — they're faster and less noisy.
