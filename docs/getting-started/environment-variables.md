# ⚙️ Environment Variables

All configuration is loaded from a `.env` file in the project root (or from actual environment variables).

## 🎯 Required

| Variable | Description | Example |
|---|---|---|
| `OPENAI_API_KEY` | OpenAI API key for embeddings and chat | `sk-proj-...` |

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
DATABASE_URL=postgresql://mycelium:mycelium@localhost:5433/mycelium
```

> **Note:** The database URL default works out of the box with `docker compose up`. Only override it if you're running Postgres on a custom host/port.

## Behavior Without an API Key

If `OPENAI_API_KEY` is empty:
- **Indexing** works but skips embedding — nodes are stored without vectors
- **Semantic search** returns 503 Service Unavailable
- **Structural queries** work normally (no embeddings needed)
- **Chat** returns 503 Service Unavailable
- **MCP tools** — `query_graph` and `list_projects` work; `search` returns an error
