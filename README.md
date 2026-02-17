# mycelium

Local-only code intelligence tool. Parses local repos, builds a structural graph of code relationships, embeds code for semantic search, and exposes it through a chat UI.

## Supported Languages

| Language | Extensions | Status |
|---|---|---|
| TypeScript | `.ts`, `.tsx` | In progress |
| JavaScript | `.js`, `.jsx` | In progress |
| Go | `.go` | In progress |

Other languages (Python, Rust, etc.) are not yet supported. Workspace detection supports both the JavaScript/TypeScript ecosystem (package.json, tsconfig.json, pnpm/yarn/npm workspaces) and Go (go.work, go.mod). Tree-sitter parsers for all three languages are planned for step 2.3.
