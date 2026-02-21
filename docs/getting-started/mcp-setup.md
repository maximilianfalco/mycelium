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

The tools will appear as `mycelium_explore`, `mycelium_list_projects`, and `mycelium_detect_project`.

## Tools

### `explore`

Hybrid search (keyword + semantic) with automatic graph expansion. Searches for symbols, expands results via the code graph (callers, callees, dependencies), and returns token-budgeted context with source code and relationship annotations — all in one call.

| Parameter    | Type     | Required | Description                                      |
|-------------|----------|----------|--------------------------------------------------|
| `query`      | string   | no*      | Natural language search query                    |
| `queries`    | string[] | no*      | Multiple queries to batch in one call            |
| `project_id` | string   | no       | Colony ID (or use `path` for auto-detection)     |
| `path`       | string   | no       | Directory path for auto-detecting the project    |
| `max_tokens` | number   | no       | Token budget for the response (default 8000)     |

*Provide either `query` or `queries` (or both).

### `list_projects`

Lists all available colonies with their IDs, names, and descriptions. No parameters.

### `detect_project`

Auto-detects which project a directory belongs to. Usually not needed — `explore` accepts a `path` param directly.

| Parameter | Type   | Required | Description                    |
|-----------|--------|----------|--------------------------------|
| `path`    | string | yes      | Absolute directory path to match |

## Architecture

```
Claude Code  ──stdio──>  scripts/mcp.sh  ──>  ./myc mcp  ──pgx──>  Postgres (Docker, port 5433)
                           (starts DB                │
                            if needed)               └──openai──>  text-embedding-3-small
```

The MCP server connects directly to Postgres and OpenAI — it does not need the Go API server or the Next.js frontend running. The only dependency is the Postgres Docker container, which the startup script handles automatically.
