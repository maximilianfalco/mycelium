# ⚡ Quick Start Guide

## 1-Minute Setup

### Step 1: Clone and Configure

```bash
git clone https://github.com/maximilianfalco/mycelium.git
cd mycelium
echo "OPENAI_API_KEY=sk-..." > .env
```

### Step 2: Start Everything

```bash
make dev
```

This starts three services:

| Service | URL | What it does |
|---|---|---|
| Postgres 16 + pgvector | localhost:5433 | Graph storage, vector search, full-text search |
| Go API | [localhost:8080](http://localhost:8080) | REST API with all endpoints |
| Next.js frontend | [localhost:3773](http://localhost:3773) | Web UI for managing colonies |

The Go API uses [air](https://github.com/air-verse/air) for hot reload — saving any `.go` file auto-rebuilds.

### Step 3: Create a Colony and Index Code

1. Open [localhost:3773](http://localhost:3773)
2. Create a new colony (project)
3. Add a substrate (link a local repo/directory)
4. Click **Decompose** to start indexing

The indexing pipeline will crawl, parse, resolve imports, embed code, and store everything in the graph. Progress is shown in real-time.

### Step 4: Search and Chat

- **Forage tab** — ask questions about your codebase in natural language
- **Spore lab tab** — run individual pipeline stages for debugging

## Running Individual Services

```bash
make db         # start Postgres only
make api        # start Go API only (requires Postgres)
make frontend   # start Next.js only (requires Go API)
```

## Using the MCP Server

See the [MCP Server Setup](mcp-setup.md) guide for integrating with Claude Code, Cursor, and other AI tools.
