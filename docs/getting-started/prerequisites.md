# Prerequisites

## Docker Setup (recommended)

### Docker

The only requirement for running Mycelium. Install [Docker Desktop](https://www.docker.com/products/docker-desktop/) or via your package manager.

```bash
docker --version
docker compose version
```

### OpenAI API Key

Required for code embeddings and chat. Get one at [platform.openai.com/api-keys](https://platform.openai.com/api-keys).

The embedding model (`text-embedding-3-small`) costs ~$0.02 per 1M tokens. A typical 10K-node codebase costs about $0.05 to index.

## Local Development Setup

These are only needed if you're developing Mycelium itself and want hot reload via `make dev`.

### Go 1.22+

Install via [golang.org/dl](https://go.dev/dl/) or your package manager.

```bash
go version  # should output go1.22 or higher
```

### Node.js 22+

Install via [nvm](https://github.com/nvm-sh/nvm) (recommended) or [nodejs.org](https://nodejs.org/).

```bash
node --version  # should output v22.x.x or higher
```

### Air (Go hot reload)

Installed automatically by `make dev`, but you can install it manually:

```bash
go install github.com/air-verse/air@latest
```

## Included in Docker Compose

### pgAdmin

Available at [localhost:5050](http://localhost:5050) with credentials:
- Email: `admin@mycelium.dev`
- Password: `admin`

Useful for inspecting the database directly — browse nodes, edges, embeddings, and run ad-hoc queries.
