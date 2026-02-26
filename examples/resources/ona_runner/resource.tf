resource "ona_runner" "example" {
  name          = "my-runner"
  kind          = "RUNNER_KIND_REMOTE"
  provider_type = "RUNNER_PROVIDER_AWS_EC2"

  spec {
    desired_phase = "RUNNER_PHASE_ACTIVE"

    configuration {
      region         = "us-west-2"
      auto_update    = true
      release_channel = "RUNNER_RELEASE_CHANNEL_STABLE"
      log_level      = "LOG_LEVEL_INFO"
    }
  }
}
