# Mycelium Documentation

## 🚀 Quick Navigation

### Getting Started
- [🔧 Prerequisites](getting-started/prerequisites.md)
- [⚡ Quick Start Guide](getting-started/quick-start.md)
- [🔌 MCP Server Setup](getting-started/mcp-setup.md)
- [🤖 Using Mycelium with Claude Code](getting-started/claude-code-guide.md)
- [⚙️ Environment Variables](getting-started/environment-variables.md)

### Architecture
- [🧠 Design Decisions](deep-dive/design-decisions.md)

### Deep Dive
- [📦 Pipeline Orchestrator](deep-dive/pipeline.md) — 7-stage indexing pipeline
- [🔍 Hybrid Search](deep-dive/hybrid-search.md) — keyword + semantic fusion via RRF
- [🗺️ Structural Graph Queries](deep-dive/graph-queries.md) — callers, deps, etc.
- [🔄 Change Detector](deep-dive/change-detector.md) — git diff + mtime change detection
- [📐 Workspace Detection](deep-dive/detectors.md) — monorepo and package discovery
- [🌳 Parsers & Crawling](deep-dive/parsers.md) — tree-sitter + file crawling
- [✂️ Chunker](deep-dive/chunker.md) — embedding input preparation + tokenization
- [🧲 Embedder](deep-dive/embedder.md) — OpenAI API wrapper with batching + retry
- [🏗️ Graph Builder](deep-dive/graph-builder.md) — Postgres upsert, stale cleanup

### Troubleshooting
- [❓ FAQ](troubleshooting/faq.md)

## 🔗 External Resources

- [GitHub Repository](https://github.com/maximilianfalco/mycelium)
- [OpenAI Embeddings Docs](https://platform.openai.com/docs/guides/embeddings)
- [pgvector](https://github.com/pgvector/pgvector)
- [Tree-sitter](https://tree-sitter.github.io/)
- [MCP Specification](https://modelcontextprotocol.io/)

## 💬 Support

- **Issues**: [GitHub Issues](https://github.com/maximilianfalco/mycelium/issues)
