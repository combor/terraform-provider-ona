resource "ona_runner" "example" {
  name              = "my-runner"
  provider_type     = "RUNNER_PROVIDER_MANAGED"
  runner_manager_id = "<your-runner-manager-id>" # ona.com → Settings → Runners → ⋯ → Copy runner manager ID

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
