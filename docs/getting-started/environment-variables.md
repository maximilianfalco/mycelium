# ⚙️ Environment Variables

All configuration is loaded from a `.env` file in the project root (or from actual environment variables).

## 🎯 Required

| Variable | Description | Example |
|---|---|---|
| `OPENAI_API_KEY` | OpenAI API key for embeddings and chat | `sk-proj-...` |
| `REPOS_PATH` | Root directory containing repos to index (Docker only) | `/Users/you/Desktop/Code` |

> **`REPOS_PATH`** is bind-mounted into the API container at the same absolute path, so the indexer can read your source files. Only needed when running via Docker (`make docker-up`).

## 🔧 Optional

| Variable | Description | Default |
|---|---|---|
| `DATABASE_URL` | Postgres connection string | `postgresql://mycelium:mycelium@localhost:5433/mycelium` |
| `EMBEDDING_MODEL` | OpenAI embedding model | `text-embedding-3-small` |
| `CHAT_MODEL` | OpenAI chat model | `gpt-4o` |
| `MAX_EMBEDDING_BATCH` | Max texts per embedding API call | `1000` |
| `MAX_CONTEXT_TOKENS` | Token budget for chat context assembly | `8000` |
| `MAX_AUTO_REINDEX_FILES` | File count threshold before requiring force reindex | `100` |
| `SERVER_PORT` | Go API server port | `8080` |

## 📋 Example `.env`

```bash
OPENAI_API_KEY=sk-proj-abc123...
REPOS_PATH=/Users/you/Desktop/Code
DATABASE_URL=postgresql://mycelium:mycelium@localhost:5433/mycelium
```

> **Note:** `DATABASE_URL` defaults to `localhost:5433` for local development (`make dev`). When running via Docker (`make docker-up`), the compose file overrides it to use the internal Docker network (`db:5432`) — you don't need to change it.

## Behavior Without an API Key

If `OPENAI_API_KEY` is empty:
- **Indexing** works but skips embedding — nodes are stored without vectors
- **Semantic search** returns 503 Service Unavailable
- **Structural queries** work normally (no embeddings needed)
- **Chat** returns 503 Service Unavailable
- **MCP tools** — `list_projects` and `detect_project` work; `explore` returns an error (requires embeddings)
