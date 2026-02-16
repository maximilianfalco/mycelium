# Mycelium

A local-only code intelligence tool. Parses local repos, builds a structural graph of code relationships, embeds code for semantic search, and exposes it through a chat UI and (eventually) an MCP server.

## Tech Stack

| Component | Choice |
|---|---|
| Backend | Go (Chi router, pgx for Postgres) |
| Frontend | Next.js (App Router, TypeScript, shadcn/ui) |
| Database | Postgres 16 + pgvector (Docker) |
| CLI | Go (cobra) |
| Embeddings/Chat | OpenAI API (future) |

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
│   │       ├── indexing.go      # Stubbed — POST /index, GET /index/status
│   │       ├── search.go        # Stubbed — semantic + structural search
│   │       └── chat.go          # Stubbed — chat endpoint
│   ├── parsers/                 # (empty — Phase 1.2)
│   ├── indexer/                 # (empty — Phase 1.2)
│   └── engine/                  # (empty — Phase 1.2)
├── frontend/                    # Next.js app
│   └── src/
│       ├── app/
│       │   ├── layout.tsx       # Root layout (dark mode, JetBrains Mono)
│       │   ├── page.tsx         # Colony list (project manager)
│       │   └── projects/[id]/
│       │       └── page.tsx     # Project detail (sources + chat tabs)
│       ├── components/ui/       # shadcn components
│       └── lib/api.ts           # Typed API client for all endpoints
├── docker-compose.yml           # Postgres + pgvector (port 5433) + pgAdmin (port 5050)
├── Makefile                     # build, test, lint, dev, etc.
├── .env                         # Config (gitignored)
└── tests/integration/           # DB integration tests
```

## Running

```bash
make dev        # starts Postgres, Go API (8080), Next.js (3000) — Ctrl+C stops all
make build      # compile Go binary
make test       # run all tests
make lint       # go vet
```

Individual pieces: `make db`, `make api`, `make frontend`

pgAdmin available at http://localhost:5050 (email: admin@mycelium.dev, password: admin)

## Ports

| Service | Port |
|---|---|
| Go API | 8080 |
| Next.js frontend | 3000 |
| Postgres | 5433 (not 5432 — pendaki-postgres uses that) |
| pgAdmin | 5050 |

## Database

- 7 tables: `projects`, `project_sources`, `workspaces`, `packages`, `nodes`, `edges`, `unresolved_refs`
- Schema auto-applied on first `docker compose up` via init script
- No ORM — raw SQL via pgx (graph queries need recursive CTEs, pgvector operators)
- `internal/` is a special Go directory — compiler enforces it can't be imported by external code. Do NOT rename it.

## API Status

| Endpoint group | Status |
|---|---|
| Projects CRUD | Real (Postgres) |
| Sources CRUD | Real (Postgres) |
| POST /scan | Real (filesystem) |
| Indexing | Stubbed |
| Search | Stubbed |
| Chat | Stubbed |

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
- Fungi terminology (colony, substrate, decompose, forage) is UI/CLI only — backend code uses plain terms (project, source, index, search)
- Structured logging via `log/slog` (not `log` or `fmt.Println`)
- Error handling: `if err != nil { return ..., fmt.Errorf("context: %w", err) }`
- All DB functions take `context.Context` and `*pgxpool.Pool` as first two params
- Route handlers use closure pattern: `func handler(pool) http.HandlerFunc`
- Frontend uses custom shadcn theme (zinc/monochrome, 0 border-radius, no shadows, JetBrains Mono)

## Design Docs

Full design documentation lives in Obsidian: `~/Documents/Obsidian/obsidian-vault/Mycelium/`

Key files:
- `Mycelium Design - Overview.md` — architecture, tech stack, glossary
- `Implementation Steps.md` — ordered build steps (currently on Step 0 done, Step 1 in progress)
- `Design/Data Model.md` — hierarchy, node/edge kinds, full DB schema
- `Design/Phase Plan.md` — phases 1-4 with rationale
- `Design/Project Structure & CLI.md` — file layout, CLI commands, config
- `Design/Query Engine.md` — query types, context assembly
- `Design/Indexing Pipeline.md` — 7-stage pipeline
- `Design/Monorepo & Workspace Support.md` — detection logic, alias maps
