# Integration test configuration used by CI (terraform apply/destroy).
# Documentation examples live in examples/provider/ and examples/resources/.

terraform {
  required_version = ">= 1.6.0"

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