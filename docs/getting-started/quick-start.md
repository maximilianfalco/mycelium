# ⚡ Quick Start Guide

## Docker Setup (recommended)

Everything runs in Docker — no Go, Node.js, or other toolchains needed on your machine.

### Step 1: Clone and Configure

```bash
git clone https://github.com/maximilianfalco/mycelium.git
cd mycelium
cat > .env <<EOF
OPENAI_API_KEY=sk-...
REPOS_PATH=/path/to/your/code
EOF
```

Set `REPOS_PATH` to the root directory containing the repos you want to index (e.g., `~/Desktop/Code`).

### Step 2: Start Everything

```bash
make docker-up
```

This builds and starts all services in the background:

| Service | URL | What it does |
|---|---|---|
| Postgres 16 + pgvector | localhost:5433 | Graph storage, vector search, full-text search |
| Go API | [localhost:8080](http://localhost:8080) | REST API with all endpoints |
| Next.js frontend | [localhost:3773](http://localhost:3773) | Web UI for managing colonies |
| pgAdmin | [localhost:5050](http://localhost:5050) | Database admin panel |

No terminal needed — services restart automatically.

### Step 3: Create a Colony and Index Code

1. Open [localhost:3773](http://localhost:3773)
2. Create a new colony (project)
3. Add a substrate (link a local repo/directory under your `REPOS_PATH`)
4. Click **Decompose** to start indexing

The indexing pipeline will crawl, parse, resolve imports, embed code, and store everything in the graph. Progress is shown in real-time.

### Step 4: Search and Chat

- **Forage tab** — ask questions about your codebase in natural language
- **Spore lab tab** — run individual pipeline stages for debugging

### Managing the Stack

```bash
make docker-logs    # tail logs from all services
make docker-down    # stop everything
make docker-rebuild # full rebuild from scratch (after code changes)
```

## Local Development Setup

If you're developing Mycelium itself and want hot reload:

**Prerequisites:** Go 1.22+, Node.js 22+, Docker

```bash
make dev
```

This starts Postgres in Docker, the Go API with [air](https://github.com/air-verse/air) for hot reload, and the Next.js dev server. Requires keeping the terminal open.

```bash
make db         # start Postgres only
make api        # start Go API only (requires Postgres)
make frontend   # start Next.js only (requires Go API)
```

## Using the MCP Server

See the [MCP Server Setup](mcp-setup.md) guide for integrating with Claude Code, Cursor, and other AI tools.
