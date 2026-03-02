resource "ona_project" "example" {
  name = "terraform-provider-ona"

  automations_file_path  = ".gitpod/automations.yaml"
  devcontainer_file_path = ".devcontainer/devcontainer.json"

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
    enabled                 = true
    enable_jetbrains_warmup = false
    environment_class_ids   = ["<environment-class-id>"]
    timeout                 = "3600s"
    trigger = {
      daily_schedule = {
        hour_utc = 2
      }
    }
  }

  recommended_editors = {
    vscode = {
      versions = []
    }
    goland = {
      versions = ["2025.1"]
    }
  }

  technical_description = "Terraform provider for managing Gitpod resources on Ona."
}
