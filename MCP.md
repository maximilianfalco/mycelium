# Mycelium MCP Server

An MCP (Model Context Protocol) server that exposes mycelium's code intelligence to tools like Claude Code. Runs over stdio — no extra Docker containers or services needed beyond the existing Postgres.

## Prerequisites

- Docker installed (for Postgres)
- At least one colony with indexed sources (via the web UI)
- `.env` with `OPENAI_API_KEY` and `DATABASE_URL` configured

## Setup

```bash
# Build the binary
make build
```

That's it. The `.mcp.json` in the project root is already configured:

```json
{
  "mcpServers": {
    "mycelium": {
      "command": "./scripts/mcp.sh"
    }
  }
}
```

Claude Code auto-discovers this file when you open the project directory. The startup script (`scripts/mcp.sh`) automatically starts the Postgres Docker container if it's not already running, then launches the MCP server. No need to run `make dev` or `make db` first.

The tools will appear as `mycelium_search`, `mycelium_query_graph`, and `mycelium_list_projects`.

## Tools

### `search`

Semantic search across indexed code using pgvector embeddings.

| Parameter    | Type   | Required | Description                                      |
|-------------|--------|----------|--------------------------------------------------|
| `query`      | string | yes      | Natural language search query                    |
| `project_id` | string | yes      | Colony ID (use `list_projects` to discover)      |
| `limit`      | number | no       | Max results (1-100, default 10)                  |
| `kinds`      | string | no       | Comma-separated filter (e.g. `function,class`)   |

Returns matching symbols with file paths, signatures, source code, docstrings, and similarity scores.

### `query_graph`

Structural graph traversal — find callers, callees, dependencies, etc.

| Parameter        | Type   | Required | Description                                              |
|-----------------|--------|----------|----------------------------------------------------------|
| `qualified_name` | string | yes      | Symbol name (use `search` to discover)                   |
| `project_id`     | string | yes      | Colony ID                                                |
| `query_type`     | string | yes      | `callers`, `callees`, `importers`, `dependencies`, `dependents`, or `file` |
| `limit`          | number | no       | Max results (1-100, default 10)                          |

For `file` queries, pass a file path as `qualified_name` to get all symbols in that file.

### `list_projects`

Lists all available colonies with their IDs, names, and descriptions. No parameters.

## Architecture

```
Claude Code  ──stdio──>  scripts/mcp.sh  ──>  ./myc mcp  ──pgx──>  Postgres (Docker, port 5433)
                           (starts DB                │
                            if needed)               └──openai──>  text-embedding-3-small
```

The MCP server connects directly to Postgres and OpenAI — it does not need the Go API server or the Next.js frontend running. The only dependency is the Postgres Docker container, which the startup script handles automatically.
