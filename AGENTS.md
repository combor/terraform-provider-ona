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
- When a request is ambiguous, ask for clarification instead of guessing. Do not change your answer based on reactions — either stand by your reasoning or honestly say you are unsure.

## Validation checklist

From the repo root, before finishing a change:

1. Run `gofmt -w` on changed Go files
2. Run tests: `go test ./...`
3. Run CI pipeline locally: `act push -j build -j test`

## Running integration tests

"Run integration tests" means running the `integration` job from the CI pipeline via `act`:

```bash
act push -j integration --container-architecture linux/amd64 \
  -s GITPOD_API_KEY="$GITPOD_API_KEY" \
  -s RUNNER_MANAGER_ID=01984227-2946-7e40-a982-2f427741f5da
```

This runs `terraform apply` and `terraform destroy` against the real Gitpod API using the examples/ directory. Requires `GITPOD_API_KEY` to be set.
