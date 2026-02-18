---
name: commit-msg
description: Generate a conventional commit message from recent uncommitted changes. Use when the user says "commit message", "write a commit", "create a commit message", "what should I commit", or invokes /commit-msg. Analyzes staged/unstaged diffs and untracked files to produce a well-structured conventional commit message.
---

# Commit Message Generator

Generate a conventional commit message by analyzing the current uncommitted changes.

## Steps

1. Run `git status` to see what's staged, unstaged, and untracked.
2. Run `git diff --stat` for modified tracked files and `git ls-files --others --exclude-standard` for untracked files.
3. Run `git diff` (or `git diff --cached` if changes are staged) to read the actual diffs. For untracked files, read them directly.
4. Analyze the changes and determine:
   - The primary type: `feat`, `fix`, `refactor`, `chore`, `test`, `docs`, `style`, `perf`, `ci`, `build`
   - An optional scope in parentheses if changes are localized (e.g., `feat(search):`)
   - A concise subject line (imperative mood, no period, <=72 chars)
5. Write a commit body with bullet points summarizing each logical change. Group related changes. Be specific — mention file names, function names, endpoints.
6. Present the full commit message to the user. Do NOT commit automatically.

## Format

```
<type>[optional scope]: <subject>

- <change 1>
- <change 2>
- ...
```

## Rules

- Subject line: imperative mood ("add X", not "added X"), no period, <=72 chars
- Body bullets: start with a verb, be specific about what changed and where
- If changes span multiple unrelated concerns, suggest splitting into separate commits
- Prefer a single type that best represents the primary intent
- Do NOT include file counts, line counts, or mechanical stats — focus on what the changes do
