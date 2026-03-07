# Terraform Provider Ona

[![CI](https://github.com/combor/terraform-provider-ona/actions/workflows/ci.yml/badge.svg)](https://github.com/combor/terraform-provider-ona/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/combor/terraform-provider-ona?display_name=tag)](https://github.com/combor/terraform-provider-ona/releases)

Terraform provider for managing [Gitpod](https://gitpod.io) resources on [ona.com](https://ona.com).

## Requirements

- [Terraform](https://developer.hashicorp.com/terraform/downloads) >= 1.6.0
- [Go](https://golang.org/doc/install) >= 1.26 (for building from source)

## Quick Links

- [Terraform Registry](https://registry.terraform.io/providers/combor/ona/latest)
- [Project Resource Docs](https://github.com/combor/terraform-provider-ona/blob/main/docs/resources/project.md)
- [Runner Resource Docs](https://github.com/combor/terraform-provider-ona/blob/main/docs/resources/runner.md)
- [Authenticated Identity Data Source Docs](https://github.com/combor/terraform-provider-ona/blob/main/docs/data-sources/authenticated_identity.md)
- [Project Data Source Docs](https://github.com/combor/terraform-provider-ona/blob/main/docs/data-sources/project.md)
- [Runner Data Source Docs](https://github.com/combor/terraform-provider-ona/blob/main/docs/data-sources/runner.md)
- [Group Data Source Docs](https://github.com/combor/terraform-provider-ona/blob/main/docs/data-sources/group.md)
- [Groups Data Source Docs](https://github.com/combor/terraform-provider-ona/blob/main/docs/data-sources/groups.md)
- [Runner Environment Classes Data Source Docs](https://github.com/combor/terraform-provider-ona/blob/main/docs/data-sources/runner_environment_classes.md)
- [Integration Example](https://github.com/combor/terraform-provider-ona/blob/main/examples/main.tf)

## Supported Types

Resources:

- `ona_project`
- `ona_runner`

Data sources:

- `ona_authenticated_identity`
- `ona_group`
- `ona_groups`
- `ona_project`
- `ona_runner`
- `ona_runner_environment_classes`

## Using the Provider

```hcl
terraform {
  required_providers {
    ona = {
      source  = "combor/ona"
      version = ">= 0.1.0"
    }
  }
}

provider "ona" {
  api_key = var.ona_api_key
}
```

## Example

To find your runner manager ID, go to [ona.com](https://ona.com) → **Settings** → **Runners**, click the **⋯** menu on any managed runner, and select **Copy runner manager ID**.

```hcl
resource "ona_runner" "example" {
  name              = "my-runner"
  provider_type     = "RUNNER_PROVIDER_MANAGED"
  runner_manager_id = "<your-runner-manager-id>" # see above for how to find this

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

data "ona_runner_environment_classes" "example" {
  runner_id = ona_runner.example.id
}

data "ona_authenticated_identity" "current" {}

resource "ona_project" "example" {
  name = "terraform-provider-ona"

  initializer = {
    specs = [
      {
        git = {
          remote_uri = "https://github.com/combor/terraform-provider-ona"
        }
      }
    ]
  }

  prebuild_configuration = {
    enabled = true
    environment_class_ids = [
      for environment_class in data.ona_runner_environment_classes.example.environment_classes :
      environment_class.id
    ]
    executor = {
      id        = data.ona_authenticated_identity.current.id
      principal = data.ona_authenticated_identity.current.principal
    }
  }
}

data "ona_project" "example" {
  id = ona_project.example.id
}

data "ona_runner" "example" {
  id = ona_runner.example.id
}
```

See [examples/main.tf](https://github.com/combor/terraform-provider-ona/blob/main/examples/main.tf) for the integration-test configuration and [docs/resources/project.md](https://github.com/combor/terraform-provider-ona/blob/main/docs/resources/project.md), [docs/resources/runner.md](https://github.com/combor/terraform-provider-ona/blob/main/docs/resources/runner.md), [docs/data-sources/authenticated_identity.md](https://github.com/combor/terraform-provider-ona/blob/main/docs/data-sources/authenticated_identity.md), [docs/data-sources/group.md](https://github.com/combor/terraform-provider-ona/blob/main/docs/data-sources/group.md), [docs/data-sources/groups.md](https://github.com/combor/terraform-provider-ona/blob/main/docs/data-sources/groups.md), [docs/data-sources/project.md](https://github.com/combor/terraform-provider-ona/blob/main/docs/data-sources/project.md), [docs/data-sources/runner.md](https://github.com/combor/terraform-provider-ona/blob/main/docs/data-sources/runner.md), and [docs/data-sources/runner_environment_classes.md](https://github.com/combor/terraform-provider-ona/blob/main/docs/data-sources/runner_environment_classes.md) for the generated schema docs.

## Development

```bash
# Build
go build -o terraform-provider-ona .

# Run tests
go test ./...

# Run the local CI checks used in day-to-day development
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

## Contributing

Bug reports and feature requests should go to [GitHub Issues](https://github.com/combor/terraform-provider-ona/issues). Code changes should be proposed through [pull requests](https://github.com/combor/terraform-provider-ona/pulls).

Before opening a pull request, run:

- `gofmt -w` on changed Go files
- `go test ./...`
- `act push -j build -j test -s GITPOD_API_KEY=<your-key>`
