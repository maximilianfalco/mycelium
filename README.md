# mycelium

Local-only code intelligence tool. Parses local repos, builds a structural graph of code relationships, embeds code for semantic search, and exposes it through a chat UI.

## What it does

1. **Index** local repos — crawls files, parses them with tree-sitter, detects workspaces and resolves imports, embeds code with OpenAI, and stores everything in a structural graph (Postgres + pgvector).
2. **Search** the graph — semantic search (vector similarity) and structural queries (callers, callees, dependencies, dependents, importers, file context).
3. **Chat** about your code — ask questions and get answers grounded in the indexed codebase, with source attribution and streamed responses.

## Supported languages

| Language | Extensions | Parsing | Workspace detection |
|---|---|---|---|
| TypeScript | `.ts`, `.tsx` | Tree-sitter | package.json, tsconfig.json, pnpm/yarn/npm workspaces |
| JavaScript | `.js`, `.jsx` | Tree-sitter | package.json, pnpm/yarn/npm workspaces |
| Go | `.go` | Tree-sitter | go.mod, go.work |

## Tech stack

| Component | Choice |
|---|---|
| Backend | Go (Chi router, pgx for Postgres) |
| Frontend | Next.js 16 (App Router, TypeScript, shadcn/ui) |
| Database | Postgres 16 + pgvector (Docker) |
| Parsing | Tree-sitter (TypeScript, JavaScript, Go) |
| Embeddings | OpenAI `text-embedding-3-small` |
| Chat | OpenAI `gpt-4o` |

## Quick start

```bash
# Prerequisites: Go 1.22+, Node.js 22+, Docker

# 1. Configure
echo "OPENAI_API_KEY=sk-..." > .env

# 2. Start everything
make dev
```

This starts Postgres (port 5433), the Go API (port 8080) with live reload, and the Next.js frontend (port 3773).

## Commands

```bash
make dev        # start full stack (Postgres + API + frontend)
make build      # compile Go binary
make test       # run all tests
make lint       # go vet
make clean      # remove binary + test cache
```

Individual pieces: `make db`, `make api`, `make frontend`

## Ports

| Service | Port |
|---|---|
| Go API | 8080 |
| Next.js frontend | 3773 |
| Postgres | 5433 |
| pgAdmin | 5050 |

## Indexing pipeline

The indexing pipeline runs in 7 stages:

1. **Change detection** — git diff against last indexed commit (or mtime for non-git dirs). Skips unchanged files. Threshold guard prevents accidental full re-indexes (bypassed with force reindex).
2. **Workspace detection** — finds package.json / go.mod / go.work, resolves monorepo structure, builds alias maps from tsconfig paths.
3. **File crawling** — walks the directory tree, respects .gitignore, filters by extension and file size.
4. **Parsing** — tree-sitter extracts functions, classes, interfaces, types, enums, methods, edges (imports, calls, extends, implements, contains, uses_type), signatures, docstrings, and body hashes. Parallel (8 workers).
5. **Import resolution** — resolves import specifiers against the alias map, tsconfig paths, and filesystem. Produces resolved edges and tracks unresolved refs.
6. **Embedding** — compares body hashes to skip unchanged nodes, batches the rest through OpenAI `text-embedding-3-small`.
7. **Graph storage** — upserts nodes/edges/embeddings into Postgres, cleans up stale nodes from deleted files.

## Database

7 tables: `projects`, `project_sources`, `workspaces`, `packages`, `nodes`, `edges`, `unresolved_refs`.

Schema auto-applied on first `docker compose up`. No ORM — raw SQL via pgx (graph queries use recursive CTEs, pgvector cosine distance).

## Frontend

The UI uses fungi terminology:

| UI term | Backend term |
|---|---|
| Colony | Project |
| Substrate | Source (linked repo/directory) |
| Decompose | Index |
| Forage | Chat/Search |
| Spore lab | Debug mode |

Four tabs per project:
- **Substrates** — manage linked source directories, trigger indexing
- **Forage** — chat with your codebase (streamed responses, source attribution)
- **Spore lab** — run individual pipeline stages interactively for debugging
- **Mycelial map** — graph visualization (coming soon)

Settings panel includes force reindex to bypass the file change threshold.
