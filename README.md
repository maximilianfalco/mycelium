<p align="center">
  <img src="frontend/public/icon.svg" width="80" height="80" alt="mycelium"/>
</p>

<h1 align="center">mycelium</h1>

<p align="center">
  Local-only code intelligence. Structural graph + hybrid search + AI chat — all on your machine.
</p>

<p align="center">
  <a href="https://github.com/maximilianfalco/mycelium/blob/main/LICENSE"><img src="https://img.shields.io/github/license/maximilianfalco/mycelium?style=flat-square" alt="License"/></a>
  <a href="https://github.com/maximilianfalco/mycelium"><img src="https://img.shields.io/github/stars/maximilianfalco/mycelium?style=flat-square" alt="Stars"/></a>
  <a href="https://github.com/maximilianfalco/mycelium/commits/main"><img src="https://img.shields.io/github/last-commit/maximilianfalco/mycelium?style=flat-square" alt="Last Commit"/></a>
  <img src="https://img.shields.io/badge/go-1.22+-00ADD8?style=flat-square&logo=go&logoColor=white" alt="Go"/>
  <img src="https://img.shields.io/badge/postgres-16-336791?style=flat-square&logo=postgresql&logoColor=white" alt="Postgres"/>
</p>

---

## What it does

Mycelium parses your local repos, builds a structural graph of every function, class, import, and call relationship, embeds the code for semantic search, and exposes everything through a chat UI and MCP server.

1. **Index** — crawls files with tree-sitter, detects workspaces, resolves imports, embeds code via OpenAI, stores everything in a Postgres graph (pgvector + full-text search).
2. **Search** — hybrid search fuses keyword matching and vector similarity via Reciprocal Rank Fusion. Structural queries traverse the code graph (callers, callees, dependencies, dependents, importers).
3. **Chat** — ask questions about your codebase and get answers grounded in the indexed graph, with source attribution and streamed responses.
4. **MCP** — expose search and graph queries as tools for AI coding agents (Claude Code, Cursor, etc.).

<details>
<summary><strong>Design Structure</strong></summary>

- **Postgres does everything.** No Redis, no Elasticsearch, no Milvus. pgvector handles embeddings, built-in full-text search handles keywords, recursive CTEs handle graph traversal. One database, one connection pool.
- **Hybrid search with RRF.** Every query runs two searches in parallel — keyword (tsvector/ts_rank over a GIN-indexed generated column) and semantic (pgvector cosine similarity) — then merges results via Reciprocal Rank Fusion (`score = 1/(60 + rank_vector) + 1/(60 + rank_keyword)`). Exact name matches rank at the top; conceptual matches still surface.
- **Incremental indexing.** Change detection via git diff (or mtime fallback). Body hash comparison skips re-embedding unchanged code. Only modified symbols hit the OpenAI API.
- **Generated tsvector column.** `search_vector` is `GENERATED ALWAYS AS ... STORED` — Postgres auto-maintains it on every insert/update. Zero application code for keyword indexing.
- **Tree-sitter for parsing.** Language-agnostic AST extraction. Each language gets a parser that produces the same `NodeInfo`/`EdgeInfo` interface. Adding a new language means implementing one interface.

</details>

## Quick start

**Prerequisites:** Go 1.22+, Node.js 22+, Docker

```bash
# 1. Clone and configure
git clone https://github.com/maximilianfalco/mycelium.git
cd mycelium
echo "OPENAI_API_KEY=sk-..." > .env

# 2. Start everything (Postgres + Go API + Next.js frontend)
make dev
```

This starts:

| Service | URL |
|---|---|
| Next.js frontend | [localhost:3773](http://localhost:3773) |
| Go API | [localhost:8080](http://localhost:8080) |
| Postgres | localhost:5433 |
| pgAdmin | [localhost:5050](http://localhost:5050) |

<details>
<summary><strong>MCP server setup (for Claude Code, Cursor, etc.)</strong></summary>

Add to your MCP client config:

```json
{
  "mcpServers": {
    "mycelium": {
      "command": "bash",
      "args": ["/path/to/mycelium/scripts/mcp.sh"],
      "env": {
        "DATABASE_URL": "postgresql://mycelium:mycelium@localhost:5433/mycelium",
        "OPENAI_API_KEY": "sk-..."
      }
    }
  }
}
```

Available MCP tools:
- **`search`** — hybrid search (keyword + semantic) across indexed code
- **`query_graph`** — structural traversal (callers, callees, importers, dependencies, dependents, file context)
- **`list_projects`** — enumerate available projects

</details>

<details>
<summary><strong>All make commands</strong></summary>

```bash
make dev        # start full stack (Postgres + API + frontend) with hot reload
make build      # compile Go binary
make test       # run all tests (unit + integration)
make lint       # go vet
make clean      # remove binary + test cache
make db         # start Postgres only
make api        # start Go API only
make frontend   # start Next.js frontend only
```

</details>

## Features

### Supported languages

| Language | Extensions | Parser | Workspace detection |
|---|---|---|---|
| TypeScript | `.ts`, `.tsx` | Tree-sitter | package.json, tsconfig.json, pnpm/yarn/npm workspaces |
| JavaScript | `.js`, `.jsx` | Tree-sitter | package.json, pnpm/yarn/npm workspaces |
| Go | `.go` | Tree-sitter | go.mod, go.work |

### 7-stage indexing pipeline

1. **Change detection** — git diff against last indexed commit (or mtime for non-git dirs). Threshold guard prevents accidental full re-indexes.
2. **Workspace detection** — finds package.json / go.mod / go.work, resolves monorepo structure, builds alias maps from tsconfig paths.
3. **File crawling** — walks the directory tree, respects .gitignore, filters by extension and file size.
4. **Parsing** — tree-sitter extracts functions, classes, interfaces, types, enums, methods, and all edges (imports, calls, extends, implements, contains, uses_type). Parallel across 8 workers.
5. **Import resolution** — resolves import specifiers against the alias map, tsconfig paths, and filesystem. Tracks unresolved refs separately.
6. **Embedding** — compares body hashes to skip unchanged nodes, batches the rest through OpenAI `text-embedding-3-small` (1536-dim).
7. **Graph storage** — upserts nodes/edges/embeddings into Postgres, cleans up stale nodes from deleted files.

### Hybrid search

Every query runs two searches in a single Postgres transaction:

| Signal | How it works |
|---|---|
| **Keyword** | Postgres full-text search (`tsvector`/`ts_rank`) over a GIN-indexed generated column. Weighted fields: symbol names (A), signatures (B), docstrings (C). |
| **Semantic** | pgvector cosine similarity against 1536-dim OpenAI embeddings. IVFFlat index with configurable probes. |
| **Fusion** | Reciprocal Rank Fusion: `score = 1/(60 + rank_vector) + 1/(60 + rank_keyword)`. Each source provides 3x candidates before fusion. |

### Structural graph queries

| Query | Returns |
|---|---|
| `callers` | Functions that call the target symbol |
| `callees` | Functions called by the target symbol |
| `importers` | Files that import the target |
| `dependencies` | Transitive dependencies (up to 5 hops via recursive CTE) |
| `dependents` | Transitive dependents (up to 5 hops via recursive CTE) |
| `file` | All symbols in the same file |

### Streamed AI chat

Context assembly pulls relevant code from the graph (hybrid search + graph expansion), packs it within a token budget, and streams responses via SSE with source attribution.

## Tech stack

| Component | Choice |
|---|---|
| Backend | Go (Chi router, pgx for Postgres) |
| Frontend | Next.js 16 (App Router, TypeScript, shadcn/ui) |
| Database | Postgres 16 + pgvector |
| Parsing | Tree-sitter (TypeScript, JavaScript, Go) |
| Embeddings | OpenAI `text-embedding-3-small` |
| Search | Hybrid: Postgres FTS + pgvector cosine, fused via RRF |
| Chat | OpenAI `gpt-4o` |
| MCP | `mcp-go` (stdio transport) |

## Database

7 tables: `projects`, `project_sources`, `workspaces`, `packages`, `nodes`, `edges`, `unresolved_refs`.

Schema auto-applied on first `docker compose up`. No ORM — raw SQL via pgx. Graph traversal uses recursive CTEs, vector search uses pgvector's `<=>` operator, keyword search uses `tsvector` with GIN indexes.

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

## Contributing

```bash
# Run tests
make test

# Lint
make lint

# Build
make build
```

## License

[Apache 2.0](LICENSE)
