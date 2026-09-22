# Integration test configuration used by CI (terraform apply/destroy).
# Documentation examples live in examples/provider/ and examples/resources/.

terraform {
  required_version = ">= 1.7.0"

  required_providers {
    ona = {
      source = "combor/ona"
    }
  }
}

provider "ona" {
  api_key  = var.ona_api_key
  base_url = var.ona_base_url
}

resource "ona_runner" "example" {
  name              = var.runner_name
  provider_type     = var.runner_provider_type
  runner_manager_id = var.runner_manager_id

  spec = {
    variant = "RUNNER_VARIANT_STANDARD"
    configuration = {
      region                           = var.runner_region
      auto_update                      = true
      devcontainer_image_cache_enabled = true
      release_channel                  = "RUNNER_RELEASE_CHANNEL_STABLE"
      log_level                        = "LOG_LEVEL_INFO"
    }
  }
}

data "ona_runner" "example" {
  id = ona_runner.example.id
}

# Disabled in CI: CreateRunnerToken returns 401 unauthenticated for
# user-principal API keys. Re-enable once the required auth scope is
# confirmed. See https://github.com/combor/terraform-provider-ona/actions/runs/25887549490
# data "ona_runner_token" "example" {
#   runner_id = ona_runner.example.id
# }

data "ona_runner_environment_classes" "example" {
  runner_id = ona_runner.example.id
}

data "ona_authenticated_identity" "current" {}

resource "ona_project" "example" {
  name = var.project_name

  initializer = {
    specs = [
      {
        git = {
          remote_uri   = var.project_git_remote_uri
          clone_target = "main"
          target_mode  = "CLONE_TARGET_MODE_REMOTE_BRANCH"
        }
      }
    ]
  }

  # Leave the API defaults in place when the runner reports no classes.
  environment_classes = length(data.ona_runner_environment_classes.example.environment_classes) > 0 ? [
    { environment_class_id = data.ona_runner_environment_classes.example.environment_classes[0].id },
  ] : null

  prebuild_configuration = {
    enabled               = true
    environment_class_ids = [for environment_class in data.ona_runner_environment_classes.example.environment_classes : environment_class.id]
    executor = {
      id        = data.ona_authenticated_identity.current.id
      principal = data.ona_authenticated_identity.current.principal
    }
  }

  recommended_editors = {
    vscode = {
      versions = []
    }
  }
}

data "ona_project" "example" {
  id = ona_project.example.id
}

data "ona_projects" "example" {
  project_ids   = [ona_project.example.id]
  include_count = true
}

resource "ona_secret" "example" {
  name       = "TF_CI_SECRET"
  value      = var.secret_value
  project_id = ona_project.example.id

  environment_variable = true
}
