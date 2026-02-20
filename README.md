<p align="center">
  <img src="frontend/public/icon.svg" width="80" height="80" alt="mycelium"/>
</p>

<h1 align="center">mycelium</h1>

<p align="center">
  Local-only code intelligence. Structural graph + hybrid search + AI chat — all on your machine.
</p>

<p align="center">
  <img src="https://img.shields.io/badge/license-Apache_2.0-blue?style=flat-square" alt="License"/>
  <img src="https://img.shields.io/badge/go-1.22+-00ADD8?style=flat-square&logo=go&logoColor=white" alt="Go"/>
  <img src="https://img.shields.io/badge/typescript-5-3178C6?style=flat-square&logo=typescript&logoColor=white" alt="TypeScript"/>
  <img src="https://img.shields.io/badge/next.js-16-000000?style=flat-square&logo=next.js&logoColor=white" alt="Next.js"/>
  <img src="https://img.shields.io/badge/postgres-16-336791?style=flat-square&logo=postgresql&logoColor=white" alt="Postgres"/>
  <a href="docs/README.md"><img src="https://img.shields.io/badge/docs-browse-8A2BE2?style=flat-square&logo=readthedocs&logoColor=white" alt="Docs"/></a>
</p>

<p align="center">
  <a href="docs/getting-started/quick-start.md">Quick Start</a> ·
  <a href="docs/README.md">Documentation</a> ·
  <a href="docs/getting-started/mcp-setup.md">MCP Setup</a> ·
  <a href="docs/troubleshooting/faq.md">FAQ</a>
</p>

---

## 🧬 What it does

Mycelium parses your local repos, builds a structural graph of every function, class, import, and call relationship, embeds the code for semantic search, and exposes everything through a chat UI and MCP server.

1. **Index** — crawls files with tree-sitter, detects workspaces, resolves imports, embeds code via OpenAI, stores everything in a Postgres graph (pgvector + full-text search).
2. **Search** — hybrid search fuses keyword matching and vector similarity via [Reciprocal Rank Fusion](docs/deep-dive/hybrid-search.md). Structural queries traverse the code graph (callers, callees, dependencies, dependents, importers).
3. **Chat** — ask questions about your codebase and get answers grounded in the indexed graph, with source attribution and streamed responses.
4. **MCP** — expose search and graph queries as [tools for AI coding agents](docs/getting-started/mcp-setup.md) (Claude Code, Cursor, etc.).

## 🏗️ Architecture

<details>
<summary><strong>Key design decisions</strong></summary>

- **Postgres does everything.** No Redis, no Elasticsearch, no Milvus. pgvector handles embeddings, built-in full-text search handles keywords, recursive CTEs handle graph traversal. One database, one connection pool.
- **Hybrid search with RRF.** Two parallel searches (keyword + semantic) merged via Reciprocal Rank Fusion. Exact name matches rank first; conceptual matches still surface.
- **Incremental indexing.** `git diff` + body hash comparison. Only modified symbols hit the OpenAI API.
- **Generated tsvector column.** Postgres auto-maintains the keyword index on every insert/update. Zero application code.
- **Tree-sitter for parsing.** Language-agnostic AST extraction. Adding a new language = implementing one interface.

[Full design rationale →](docs/deep-dive/design-decisions.md)

</details>

## ⚡ Quick Start

**Prerequisites:** Docker ([full list](docs/getting-started/prerequisites.md))

```bash
# 1. Clone and configure
git clone https://github.com/maximilianfalco/mycelium.git
cd mycelium
cat > .env <<EOF
OPENAI_API_KEY=sk-...
REPOS_PATH=/path/to/your/code
EOF

# 2. Start everything (runs in background — no terminal needed)
make docker-up
```

> **`REPOS_PATH`** is the root directory containing the repos you want to index (e.g., `~/Desktop/Code`). It's bind-mounted into the API container so the indexer can read your source files.

This starts:

| Service | URL |
|---|---|
| Next.js frontend | [localhost:3773](http://localhost:3773) |
| Go API | [localhost:8080](http://localhost:8080) |
| Postgres | localhost:5433 |
| pgAdmin | [localhost:5050](http://localhost:5050) |

All services run in the background. Use `make docker-logs` to tail output, `make docker-down` to stop.

<details>
<summary><strong>🔌 MCP server setup (for Claude Code, Cursor, etc.)</strong></summary>

The `.mcp.json` in the project root auto-configures Claude Code. For other clients, add to your MCP config:

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

Available tools: `search`, `query_graph`, `list_projects`

[Full MCP setup guide →](docs/getting-started/mcp-setup.md)

</details>

<details>
<summary><strong>📋 All make commands</strong></summary>

```bash
# Docker (recommended — runs in background)
make docker-up      # build and start all services
make docker-down    # stop all services
make docker-logs    # tail logs from all services
make docker-build   # build images without starting
make docker-rebuild # full rebuild from scratch

# Local development (requires Go 1.22+, Node.js 22+)
make dev        # start full stack with hot reload (requires open terminal)
make build      # compile Go binary
make test       # run all tests (unit + integration)
make lint       # go vet
make clean      # remove binary + test cache
make db         # start Postgres only
make api        # start Go API only
make frontend   # start Next.js frontend only
```

</details>

## 🔍 Features

### Supported languages

| Language | Extensions | Parser | Workspace detection |
|---|---|---|---|
| TypeScript | `.ts`, `.tsx` | Tree-sitter | package.json, tsconfig.json, pnpm/yarn/npm workspaces |
| JavaScript | `.js`, `.jsx` | Tree-sitter | package.json, pnpm/yarn/npm workspaces |
| Go | `.go` | Tree-sitter | go.mod, go.work |

### 7-stage indexing pipeline

1. **Change detection** — git diff against last indexed commit. Threshold guard prevents accidental full re-indexes.
2. **Workspace detection** — finds package.json / go.mod / go.work, resolves monorepo structure.
3. **File crawling** — walks the directory tree, respects .gitignore.
4. **Parsing** — tree-sitter extracts functions, classes, types, and all edges. 8 parallel workers.
5. **Import resolution** — resolves specifiers against alias maps, tsconfig paths, and filesystem.
6. **Embedding** — body hash comparison skips unchanged nodes. Batched OpenAI API calls.
7. **Graph storage** — upserts to Postgres, cleans up stale nodes.

[Full pipeline documentation →](docs/deep-dive/pipeline.md)

### Hybrid search

Every query runs two searches in a single Postgres transaction:

| Signal | How it works |
|---|---|
| **Keyword** | Postgres full-text search over GIN-indexed generated column. Weighted: names (A), signatures (B), docstrings (C). |
| **Semantic** | pgvector cosine similarity against 1536-dim embeddings. IVFFlat index. |
| **Fusion** | RRF: `score = 1/(60 + rank_vector) + 1/(60 + rank_keyword)`. 3x candidate oversampling. |

[How hybrid search works →](docs/deep-dive/hybrid-search.md)

### Structural graph queries

| Query | Returns |
|---|---|
| `callers` | Functions that call the target symbol |
| `callees` | Functions called by the target symbol |
| `importers` | Files that import the target |
| `dependencies` | Transitive dependencies (up to 5 hops) |
| `dependents` | Transitive dependents (up to 5 hops) |
| `file` | All symbols in the same file |

[Graph query documentation →](docs/deep-dive/graph-queries.md)

### Streamed AI chat

Context assembly pulls relevant code from the graph (hybrid search + graph expansion), packs it within a token budget, and streams responses via SSE with source attribution.

## 🛠️ Tech Stack

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

## 🍄 Frontend

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

## 📖 Documentation

| Section | Description |
|---|---|
| [Quick Start](docs/getting-started/quick-start.md) | Get up and running in 1 minute |
| [Prerequisites](docs/getting-started/prerequisites.md) | Required and optional dependencies |
| [MCP Setup](docs/getting-started/mcp-setup.md) | Configure for Claude Code, Cursor, etc. |
| [Environment Variables](docs/getting-started/environment-variables.md) | All configuration options |
| [Design Decisions](docs/deep-dive/design-decisions.md) | Why Postgres-only, why RRF, why tree-sitter |
| [Pipeline Orchestrator](docs/deep-dive/pipeline.md) | 7-stage indexing pipeline |
| [Change Detector](docs/deep-dive/change-detector.md) | Git diff + mtime change detection |
| [Workspace Detection](docs/deep-dive/detectors.md) | Monorepo and package discovery |
| [Parsers & Crawling](docs/deep-dive/parsers.md) | Tree-sitter + file crawling |
| [Chunker](docs/deep-dive/chunker.md) | Embedding input preparation + tokenization |
| [Embedder](docs/deep-dive/embedder.md) | OpenAI API wrapper with batching + retry |
| [Graph Builder](docs/deep-dive/graph-builder.md) | Postgres upsert, stale cleanup |
| [Hybrid Search](docs/deep-dive/hybrid-search.md) | Keyword + semantic fusion via RRF |
| [Graph Queries](docs/deep-dive/graph-queries.md) | Structural traversal (callers, deps, etc.) |
| [FAQ](docs/troubleshooting/faq.md) | Common questions |

## 📄 License

[Apache 2.0](LICENSE)
