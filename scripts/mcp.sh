#!/bin/bash
# Starts the mycelium MCP server, ensuring Postgres is running first.
# Used by .mcp.json as the MCP server command.

set -e

DIR="$(cd "$(dirname "$0")/.." && pwd)"
cd "$DIR"

# Ensure Postgres is running
if ! docker compose ps --status running db --quiet 2>/dev/null | grep -q .; then
  docker compose up -d db >/dev/null 2>&1
  # Wait for Postgres to be ready
  until docker compose exec -T db pg_isready -U mycelium -q 2>/dev/null; do
    sleep 0.5
  done
fi

exec ./myc mcp
