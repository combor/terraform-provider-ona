resource "ona_runner_policy" "platform_admin" {
  runner_id = ona_runner.example.id
  group_id  = ona_group.platform.id
  role      = "RUNNER_ROLE_ADMIN"
}
