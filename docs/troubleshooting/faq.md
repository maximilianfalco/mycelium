# ❓ Frequently Asked Questions

## Q: What languages are supported?

**A:** TypeScript (`.ts`, `.tsx`), JavaScript (`.js`, `.jsx`), and Go (`.go`). The parser interface is extensible — adding a new language means implementing one Go interface.

## Q: How much does indexing cost?

**A:** The embedding model (`text-embedding-3-small`) costs ~$0.02 per 1M tokens. A typical 10K-node codebase costs about $0.05 for the first full index. Incremental re-indexes (after code changes) are near-zero cost because unchanged nodes are skipped via body hash comparison.

## Q: Can I use it without an OpenAI API key?

**A:** Partially. Without a key:
- Indexing works but skips embedding (nodes stored without vectors)
- Structural graph queries work normally (`callers`, `callees`, `dependencies`, etc.)
- Semantic/hybrid search and chat are unavailable (return 503)
- MCP tools `query_graph` and `list_projects` work; `search` does not

## Q: How does hybrid search differ from pure vector search?

**A:** Pure vector search ranks by semantic similarity — searching for `BuildGraph` might return `ConstructDAG` first because they're conceptually similar. Hybrid search adds keyword matching, so exact name matches like `BuildGraph` rank at the top while conceptual results still appear. Results are merged via Reciprocal Rank Fusion (RRF).

See [Hybrid Search](../deep-dive/hybrid-search.md) for the full explanation.

## Q: What files get indexed?

**A:** Only code files with supported extensions (`.ts`, `.tsx`, `.js`, `.jsx`, `.go`) under 100KB. The crawler skips:
- `node_modules`, `dist`, `build`, `.next`, `vendor`, `testdata`
- Hidden directories (starting with `.`)
- Lockfiles (`package-lock.json`, `yarn.lock`, `pnpm-lock.yaml`, `go.sum`)
- `.log` files and symlinks
- Files excluded by `.gitignore`

## Q: What happens when I re-index after a small code change?

**A:** The pipeline detects changes via `git diff` and only processes modified files:

1. **Change detection** — identifies 3 changed files out of 5,000
2. **Parsing** — only parses those 3 files
3. **Embedding** — compares body hashes, only embeds functions whose code actually changed
4. **Graph storage** — upserts changed nodes, deletes stale ones

A typical 3-file commit re-indexes in under a minute.

## Q: Why does my index take a long time the first time?

**A:** The first index is a full index — every file is parsed, every symbol is embedded. For a large codebase (50K+ nodes), the bottleneck is the OpenAI embedding API. Subsequent indexes are incremental and much faster.

## Q: Can I index multiple repos in one colony?

**A:** Yes. A colony (project) can have multiple substrates (sources), each pointing to a different directory. Cross-source import resolution links symbols between them.

## Q: Where is the data stored?

**A:** Everything lives in Postgres (running in Docker on port 5433). Seven tables: `projects`, `project_sources`, `workspaces`, `packages`, `nodes`, `edges`, `unresolved_refs`. You can inspect them directly via pgAdmin at [localhost:5050](http://localhost:5050).

## Q: Does it work offline?

**A:** Structural features (indexing, graph queries) work offline since they only need the local filesystem and Postgres. Search and chat require the OpenAI API for embedding queries and generating responses.

## Q: How do I reset the index for a project?

**A:** Use the "force reindex" option in the settings panel. This bypasses the file change threshold and re-processes all files from scratch. Existing nodes and edges are upserted (updated in place), and stale nodes from deleted files are cleaned up.
