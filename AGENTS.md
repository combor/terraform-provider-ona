# AGENTS.md

This file is a quick guide for AI coding agents and human contributors working on this repo.

## Editing guidance

- Prefer small, focused changes with matching test updates
- Don't hand-edit `dist/` artifacts unless release-related
- Keep scope tight; avoid broad refactors
- Prefer small, reliable tests that fail before and pass after
- Avoid overconfident root-cause claims
- Do NOT invent bugs; if evidence is weak, say so and skip.
- Prefer the smallest safe fix; avoid refactors and unrelated cleanup.
- Anchor each suggestion to concrete evidence
- Avoid generic advice; make each recommendation actionable and specific
- In commit messages, explain why the change was made.

## Validation checklist

From the repo root, before finishing a change:

1. Run tests: `go test ./...`
