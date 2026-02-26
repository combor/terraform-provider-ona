# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Terraform provider for managing runners on ona.com (Gitpod runners). Uses the HashiCorp Terraform Plugin Framework and the Gitpod SDK Go client.

- Provider registry address: `registry.terraform.io/combor/ona`
- Provider type name: `ona`
- Single resource: `ona_runner` (CRUD for Gitpod runners)
- Single data source: `ona_runner` (read-only runner lookup by ID)

## Build & Test Commands

```bash
# Run all tests
go test ./...

# Build the provider binary
go build -o terraform-provider-ona

# Release builds (cross-platform via goreleaser)
goreleaser release
```

## Architecture

The provider follows standard Terraform Plugin Framework patterns:

- **`main.go`** — Entry point; serves the provider via `providerserver.Serve`
- **`internal/provider/provider.go`** — Provider configuration (api_key via `GITPOD_API_KEY`, base_url via `GITPOD_BASE_URL`), registers resources and data sources, creates the Gitpod SDK client
- **`internal/resource_runner/`** — Runner resource implementation
  - `runner_resource.go` — CRUD operations using `r.client.Runners.{New,Get,Update,Delete}`
  - `runner_model.go` — Terraform state models (`RunnerModel`, `RunnerSpecModel`, etc.) and mappers between Gitpod API types and Terraform state
- **`internal/datasource_runner/`** — Runner data source (read-only, uses same Gitpod client)

Key patterns:
- 404 HTTP responses remove the resource from Terraform state (detected via `apierror` status code)
- Update performs a write then re-reads from API since the update response is `interface{}`
- `RequiresReplace` on immutable fields: `kind`, `provider_type`, `spec.variant`, `spec.configuration.region`

## Development Guidelines (from AGENTS.md)

- Prefer small, focused changes with matching test updates
- Don't hand-edit `dist/` artifacts unless release-related
- Keep scope tight; avoid broad refactors
- Always run `go test ./...` before finishing
- Prefer small, reliable tests that fail before and pass after
- Avoid overconfident root-cause claims
- Do NOT invent bugs; if evidence is weak, say so and skip.
- Prefer the smallest safe fix; avoid refactors and unrelated cleanup.
- Anchor each suggestion to concrete evidence
- Avoid generic advice; make each recommendation actionable and specific
- in the commit messages provide explanation why the chage was made
