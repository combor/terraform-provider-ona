# Terraform Provider Ona

Terraform provider for managing [Gitpod](https://gitpod.io) runners on [ona.com](https://ona.com).

## Requirements

- [Terraform](https://developer.hashicorp.com/terraform/downloads) >= 1.6.0
- [Go](https://golang.org/doc/install) >= 1.26 (for building from source)

## Example

```hcl
resource "ona_runner" "example" {
  name              = "my-runner"
  provider_type     = "RUNNER_PROVIDER_MANAGED"
  runner_manager_id = "<your-runner-manager-id>"

  spec = {
    variant = "RUNNER_VARIANT_STANDARD"
    configuration = {
      region                           = "eu-central-1"
      auto_update                      = true
      devcontainer_image_cache_enabled = true
      release_channel                  = "RUNNER_RELEASE_CHANNEL_STABLE"
      log_level                        = "LOG_LEVEL_INFO"
    }
  }
}
```

## Development

```bash
# Build
go build -o terraform-provider-ona .

# Run tests
go test ./...

# Run full CI pipeline locally (requires docker and act)
act push -j build -j test -s GITPOD_API_KEY=<your-key>
```

To test the provider locally, create a `~/.terraformrc` with dev overrides:

```hcl
provider_installation {
  dev_overrides {
    "combor/ona" = "/path/to/terraform-provider-ona"
  }
  direct {}
}
```
