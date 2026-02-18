# Mycelium

A local-only code intelligence tool. Parses local repos, builds a structural graph of code relationships, embeds code for semantic search, and exposes it through a chat UI and (eventually) an MCP server.

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
├── cmd/myc/main.go              # CLI entrypoint (cobra)
├── internal/                    # Go convention — private packages
│   ├── config/config.go         # .env loading, typed Config struct
│   ├── db/
│   │   ├── pool.go              # pgxpool connection + health check
│   │   └── schema.sql           # DDL for all tables (auto-applied by Docker)
│   ├── projects/
│   │   ├── models.go            # Project, ProjectSource, ScanResult structs
│   │   ├── manager.go           # CRUD for projects and sources (real DB)
│   │   └── scanner.go           # Filesystem scanner (detects git repos, monorepos)
│   ├── api/
│   │   ├── server.go            # Chi router, middleware, graceful shutdown
│   │   └── routes/
│   │       ├── helpers.go       # writeJSON, writeError
│   │       ├── projects.go      # Project/source CRUD endpoints
│   │       ├── scan.go          # POST /scan (real filesystem scan)
│   │       ├── debug.go         # 8 POST /debug/* endpoints (spore lab) — all real
│   │       ├── indexing.go      # POST /index, GET /index/status — real
│   │       ├── search.go        # Semantic + structural search — real
│   │       └── chat.go          # Streamed chat with context assembly — real
│   ├── indexer/
│   │   ├── pipeline.go          # 7-stage indexing pipeline orchestrator
│   │   ├── change_detector.go   # Git diff / mtime change detection
│   │   ├── crawler.go           # Directory crawling with gitignore support
│   │   ├── import_resolver.go   # Import resolution against alias maps
│   │   ├── graph_builder.go     # Node/edge upsert into Postgres
│   │   ├── embedder.go          # OpenAI embedding with batching + retry
│   │   ├── chunker.go           # Embedding input preparation + tokenization
│   │   ├── parsers/             # Tree-sitter parsers (TS/JS, Go)
│   │   └── detectors/           # Workspace detection (Node.js, Go)
│   └── engine/
│       ├── chat.go              # Streamed chat with OpenAI + context assembly
│       ├── context_assembler.go # Scores and ranks nodes for LLM context
│       ├── graph_query.go       # Structural queries (callers, deps, etc.)
│       └── search.go            # Semantic search via pgvector
├── frontend/                    # Next.js app
│   └── src/
│       ├── app/
│       │   ├── layout.tsx       # Root layout (dark mode, JetBrains Mono)
│       │   ├── page.tsx         # Colony list (project manager)
│       │   └── projects/[id]/
│       │       └── page.tsx     # Project detail (sources + chat tabs)
│       ├── components/
│       │   ├── ui/              # shadcn components
│       │   ├── colony-list.tsx  # Home page colony list
│       │   ├── project-detail.tsx # Project detail (3 tabs: substrates, forage, spore lab)
│       │   └── debug/           # Spore lab (debug) components
│       │       ├── debug-tab.tsx             # Container: path inputs + stage cards
│       │       ├── stage-card.tsx            # Reusable collapsible card with run button
│       │       ├── crawl-output.tsx          # File list table + stats
│       │       ├── parse-output.tsx          # Node cards + edge list
│       │       ├── workspace-output.tsx      # Package tree + alias map
│       │       ├── changes-output.tsx        # File diff lists
│       │       └── embedding-playground.tsx  # Two textareas + compare + similarity
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
