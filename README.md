# mycelium

Local-only code intelligence tool. Parses local repos, builds a structural graph of code relationships, embeds code for semantic search, and exposes it through a chat UI.

## Supported Languages

| Language | Extensions | Status |
|---|---|---|
| TypeScript | `.ts`, `.tsx` | In progress |
| JavaScript | `.js`, `.jsx` | In progress |

Other languages (Go, Python, Rust, etc.) are not yet supported. The workspace detection, file crawling, and parsing pipeline currently targets the JavaScript/TypeScript ecosystem only (package.json, tsconfig.json, pnpm/yarn/npm workspaces).
