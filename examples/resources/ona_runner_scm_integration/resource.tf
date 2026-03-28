resource "ona_runner_scm_integration" "github" {
  runner_id                     = ona_runner.example.id
  scm_id                        = "github"
  host                          = "github.com"
  oauth_client_id               = var.github_oauth_client_id
  oauth_plaintext_client_secret = var.github_oauth_client_secret
  pat                           = true
}
