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

#### 1. Think Before Coding

**Don't assume. Don't hide confusion. Surface tradeoffs.**

Before implementing:
- State your assumptions explicitly. If uncertain, ask.
- If multiple interpretations exist, present them - don't pick silently.
- If a simpler approach exists, say so. Push back when warranted.
- If something is unclear, stop. Name what's confusing. Ask.

#### 2. Simplicity First

**Minimum code that solves the problem. Nothing speculative.**

- No features beyond what was asked.
- No abstractions for single-use code.
- No "flexibility" or "configurability" that wasn't requested.
- No error handling for impossible scenarios.
- If you write 200 lines and it could be 50, rewrite it.

Ask yourself: "Would a senior engineer say this is overcomplicated?" If yes, simplify.

#### 3. Surgical Changes

**Touch only what you must. Clean up only your own mess.**

When editing existing code:
- Don't "improve" adjacent code, comments, or formatting.
- Don't refactor things that aren't broken.
- Match existing style, even if you'd do it differently.
- If you notice unrelated dead code, mention it - don't delete it.

When your changes create orphans:
- Remove imports/variables/functions that YOUR changes made unused.
- Don't remove pre-existing dead code unless asked.

The test: Every changed line should trace directly to the user's request.

#### 4. Goal-Driven Execution

**Define success criteria. Loop until verified.**

Transform tasks into verifiable goals:
- "Add validation" → "Write tests for invalid inputs, then make them pass"
- "Fix the bug" → "Write a test that reproduces it, then make it pass"
- "Refactor X" → "Ensure tests pass before and after"

For multi-step tasks, state a brief plan:
```
1. [Step] → verify: [check]
2. [Step] → verify: [check]
3. [Step] → verify: [check]
```

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
