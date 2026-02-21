# 🤖 Using Mycelium with Claude Code

A practical guide to getting the most out of Mycelium's MCP tools inside Claude Code — when to use them, when to skip them, and what to put in your `CLAUDE.md`.

## The Two Modes

Claude Code has two ways to read your codebase:

| Mode | How it works | Best for |
|---|---|---|
| **File reads** (Grep, Read, Glob) | Text search + direct file access | Known files, exact symbol names, quick lookups |
| **Mycelium MCP** (explore) | Semantic embeddings + structural graph DB | Discovery, relationships, "who calls X?", conceptual queries |

They're complementary — not competing. The best results come from knowing when to reach for each one.

## When to Use What

### Use file reads when you...

- **Know the file path** — `Read internal/engine/search.go` is faster than any search
- **Need exact text matches** — Grep for `HybridSearch(` finds every call site instantly
- **Want line-level precision** — file:line output is directly actionable
- **Are making edits** — you need to Read before you Edit anyway

### Use Mycelium MCP when you...

- **Don't know where something lives** — `explore("authentication middleware")` finds it by concept
- **Need structural relationships** — `explore("callers of AssembleContext")` returns real callers with relationship annotations, not text matches in docs or comments
- **Want cross-file traversal** — explore automatically does 2-hop graph expansion, returning the call tree with source code across multiple files
- **Are exploring unfamiliar code** — semantic search finds related code even when you don't know the right keywords
- **Have multiple questions** — batch them with `queries: ["question 1", "question 2"]` to minimize round-trips

### Side-by-side: Same question, different tools

**"Who calls HybridSearch?"**

| Grep | Mycelium `explore("callers of HybridSearch")` |
|---|---|
| 5 text matches (includes the definition, a markdown doc, and 3 real call sites) | 3 results — only the actual callers, with full source code and relationship annotations |
| Must filter noise manually | Zero noise — graph expansion finds real callers |

**"What does AssembleContext depend on?"**

| Read file | Mycelium `explore("AssembleContext dependencies")` |
|---|---|
| Read 60 lines, visually trace function calls | Returns `HybridSearch` + `assembleFromResults` with full source, plus 2-hop graph expansion |
| Need follow-up reads to see cross-file dependencies | Cross-file results included automatically |

**"How does the indexing pipeline work?"**

| Grep for `pipeline\|stage` | Mycelium `explore("indexing pipeline stages")` |
|---|---|
| Requires knowing the file to search | Finds `IndexProject`, `indexSource`, `triggerIndex` across 3 files |
| Returns keyword matches in file order | Returns ranked results by conceptual relevance |

## CLAUDE.md Configuration

Add a section like this to your project's `CLAUDE.md`:

```markdown
## Mycelium MCP

This project is indexed by Mycelium (project ID: `your-project-id`).
Use the `explore` tool for code discovery — it runs semantic + keyword search,
expands results via the structural graph (callers, callees, dependencies),
and returns token-budgeted context with source code in one call. Batch
multiple questions with `queries: [...]` to minimize round-trips. For targeted
questions where you already know the file paths, prefer direct file reads
instead — they're faster and less noisy.
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

### `explore`

All-in-one code intelligence tool. Runs hybrid search (semantic + keyword), expands results via the structural graph (2-hop: callers, callees, dependencies), deduplicates, ranks, annotates relationships, and returns token-budgeted context with full source code.

```
explore(query="error handling middleware", project_id="my-project")
explore(query="authentication flow", path="/Users/you/Code/my-project")
explore(queries=["who calls HandleLogin", "database connection pool"], project_id="my-project")
```

| Parameter | Type | Required | Description |
|---|---|---|---|
| `query` | string | no* | Natural language search query |
| `queries` | string[] | no* | Multiple queries to batch in one call |
| `project_id` | string | no | Colony ID (or use `path` for auto-detection) |
| `path` | string | no | Directory path for auto-detecting the project |
| `max_tokens` | number | no | Token budget for the response (default 8000) |

*Provide either `query` or `queries` (or both).

### `detect_project`

Auto-detect which project a directory belongs to. Usually not needed — `explore` accepts a `path` param directly.

```
detect_project(path="/Users/you/Code/my-project")
```

### `list_projects`

List all indexed projects with IDs, names, and descriptions. No parameters.

## Tips

- **Batch your questions.** Use `queries: [...]` to ask multiple things in one call instead of making separate explore calls across multiple turns.
- **Keep the index fresh.** After significant code changes, reindex via the web UI or `POST /projects/{id}/index`. Stale indexes return stale results.
- **Use `path` for auto-detection.** Pass your cwd as `path` instead of hardcoding `project_id` — explore will auto-detect the project.
- **Top 5 results include full source.** Mycelium returns complete source code for the top 5 results and signatures only for the rest. If you need full source of a lower-ranked result, use a file read.
