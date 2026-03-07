# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Terraform provider for managing runners on ona.com (Gitpod runners). Uses the HashiCorp Terraform Plugin Framework and the Gitpod SDK Go client.

- Provider registry address: `registry.terraform.io/combor/ona`
- Provider type name: `ona`
- Single resource: `ona_runner` (CRUD for Gitpod runners)
- Single data source: `ona_runner` (read-only lookup by ID)

## Build & Test Commands

```bash
# Run all tests
go test ./...

# Build the provider binary
go build -o terraform-provider-ona

# Run CI pipeline locally (requires docker and act)
act push -j build -j test
```

## Development Guidelines (from AGENTS.md)

- Prefer small, focused changes with matching test updates
- Don't hand-edit `dist/` artifacts unless release-related
- Keep scope tight; avoid broad refactors
- Always run `gofmt -w` on changed Go files before committing
- Always run `go test ./...` before finishing
- Prefer small, reliable tests that fail before and pass after
- Avoid overconfident root-cause claims
- Do NOT invent bugs; if evidence is weak, say so and skip.
- Prefer the smallest safe fix; avoid refactors and unrelated cleanup.
- Anchor each suggestion to concrete evidence
- Avoid generic advice; make each recommendation actionable and specific
- in the commit messages provide explanation why the chage was made
- When a request is ambiguous, ask for clarification instead of guessing. Do not change your answer based on reactions — either stand by your reasoning or honestly say you are unsure.
