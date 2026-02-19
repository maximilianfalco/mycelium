# 🤖 Using Mycelium with Claude Code

A practical guide to getting the most out of Mycelium's MCP tools inside Claude Code — when to use them, when to skip them, and what to put in your `CLAUDE.md`.

## The Two Modes

Claude Code has two ways to read your codebase:

| Mode | How it works | Best for |
|---|---|---|
| **File reads** (Grep, Read, Glob) | Text search + direct file access | Known files, exact symbol names, quick lookups |
| **Mycelium MCP** (search, query_graph) | Semantic embeddings + structural graph DB | Discovery, relationships, "who calls X?", conceptual queries |

They're complementary — not competing. The best results come from knowing when to reach for each one.

## When to Use What

### Use file reads when you...

- **Know the file path** — `Read internal/engine/search.go` is faster than any search
- **Need exact text matches** — Grep for `HybridSearch(` finds every call site instantly
- **Want line-level precision** — file:line output is directly actionable
- **Are making edits** — you need to Read before you Edit anyway

### Use Mycelium MCP when you...

- **Don't know where something lives** — `search("authentication middleware")` finds it by concept
- **Need structural relationships** — `query_graph("AssembleContext", callers)` returns only real callers, not text matches in docs or comments
- **Want cross-file traversal** — one `callees` query returns the full call tree with source code, across multiple files
- **Are exploring unfamiliar code** — semantic search finds related code even when you don't know the right keywords

### Side-by-side: Same question, different tools

**"Who calls HybridSearch?"**

| Grep | Mycelium `query_graph(callers)` |
|---|---|
| 5 text matches (includes the definition, a markdown doc, and 3 real call sites) | 3 results — only the actual callers, with full source code |
| Must filter noise manually | Zero noise |

**"What does AssembleContext depend on?"**

| Read file | Mycelium `query_graph(callees)` |
|---|---|
| Read 60 lines, visually trace function calls | 2 results: `HybridSearch` + `assembleFromResults`, both with full source |
| Need follow-up reads to see cross-file dependencies | Cross-file results included automatically |

**"How does the indexing pipeline work?"**

| Grep for `pipeline\|stage` | Mycelium `search("indexing pipeline stages")` |
|---|---|
| Requires knowing the file to search | Finds `IndexProject`, `indexSource`, `triggerIndex` across 3 files |
| Returns keyword matches in file order | Returns ranked results by conceptual relevance |

## CLAUDE.md Configuration

Add a section like this to your project's `CLAUDE.md`:

```markdown
## Mycelium MCP

This project is indexed by Mycelium (project ID: `your-project-id`).
Use the Mycelium MCP tools for **exploration** — discovering callers/callees
(`query_graph`), finding code you don't know the location of (`search`),
and understanding relationships across the codebase. For targeted questions
where you already know the file paths, prefer direct file reads instead —
they're faster and less noisy. Use `detect_project` with the cwd to resolve
the project ID if needed.
```

### For multi-project setups

If you index multiple repos, mention which project ID maps to which codebase:

```markdown
## Mycelium MCP

Indexed projects:
- `my-backend` — the Go API (`~/Code/backend`)
- `my-frontend` — the Next.js app (`~/Code/frontend`)

Use `detect_project` to auto-resolve, or pass the ID directly if you know
which codebase the question is about.
```

## Tool Reference (Quick)

### `search`

Hybrid semantic + keyword search. Use for discovery.

```
search("error handling middleware", project_id="my-project")
search("database connection pool", project_id="my-project", kinds="function,class")
```

### `query_graph`

Structural traversal. Use for relationships.

```
query_graph("HandleLogin", project_id="my-project", query_type="callers")
query_graph("HandleLogin", project_id="my-project", query_type="callees")
query_graph("src/auth/login.ts", project_id="my-project", query_type="file")
```

| Query type | Returns |
|---|---|
| `callers` | Functions that call this symbol |
| `callees` | Functions this symbol calls |
| `importers` | Files/modules that import this symbol |
| `dependencies` | Outgoing edges (calls, imports, type usage) |
| `dependents` | Incoming edges (called by, imported by) |
| `file` | All symbols defined in a file |

### `detect_project`

Auto-resolve project ID from a directory path. Useful when you don't want to hardcode the ID.

```
detect_project(path="/Users/you/Code/my-project")
```

### `list_projects`

List all indexed projects. No parameters.

## Tips

- **Search first, then graph query.** If `query_graph` says "symbol not found", the qualified name might not match exactly. Use `search` to find the correct name, then pass it to `query_graph`.
- **Keep the index fresh.** After significant code changes, reindex via the web UI or `POST /projects/{id}/index`. Stale indexes return stale results.
- **Use `kinds` to filter.** Searching for a function? Add `kinds="function"` to skip class definitions, variables, and type aliases.
- **Top 3 results include full source.** Mycelium returns complete source code for the top 3 results and signatures only for the rest. If you need full source of a lower-ranked result, use a file read.
