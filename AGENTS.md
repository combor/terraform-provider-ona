# AGENTS.md

This file is a quick guide for AI coding agents and human contributors working on this repo.

## Project overview

Terraform provider for managing Gitpod resources on ona.com. The provider uses the HashiCorp Terraform Plugin Framework and the Gitpod SDK Go client.

- Provider registry address: `registry.terraform.io/combor/ona`
- Provider type name: `ona`
- Provider configuration: `api_key`, `base_url`, `max_retries`, `request_timeout`
- Resources: `ona_project`, `ona_runner`, `ona_runner_scm_integration`, `ona_secret`
- Data sources: `ona_authenticated_identity`, `ona_group`, `ona_groups`, `ona_project`, `ona_runner`, `ona_runner_environment_classes`, `ona_runners`

## Build and test commands

```bash
# Run all tests
go test ./...

# Build the provider binary
go build -o terraform-provider-ona .

# Run the non-release CI jobs used in day-to-day development
act push -j govulncheck -j build -j test
```

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

## Generating documentation

After creating a new resource/data source or modifying an existing schema or example, regenerate the docs:

```bash
cd tools && go generate ./...
```

This runs `terraform fmt` on examples and `tfplugindocs` to regenerate `docs/`.

## Validation checklist

From the repo root, before finishing a change:

1. Run `gofmt -w` on changed Go files
2. Run tests: `go test ./...`
3. If schemas or examples changed: `cd tools && go generate ./...`
4. Run local CI checks: `act push -j govulncheck -j build -j test`

## Running integration tests

"Run integration tests" means running the `integration` job from the CI pipeline via `act`:

```bash
act push -j integration \
  -P ubuntu-latest=catthehacker/ubuntu:act-latest \
  --action-offline-mode \
  -s GITPOD_API_KEY="$GITPOD_API_KEY" \
  -s RUNNER_MANAGER_ID=01984227-2946-7e40-a982-2f427741f5da
```

This runs the local `integration` matrix for Terraform `1.7.*` and `1.14.*` against the real Gitpod API. It first applies and destroys `examples/cleanup`, then applies and destroys the main `examples/` configuration, which currently exercises `ona_runner`, `ona_project`, and `ona_secret`. (`ona_runner_scm_integration` is not included because SCM integrations cannot be added to Gitpod-managed runners.)

Requires `GITPOD_API_KEY` to be set. The integration job also requires `RUNNER_MANAGER_ID`.
