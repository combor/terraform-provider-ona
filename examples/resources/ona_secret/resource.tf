resource "ona_secret" "database_url" {
  name       = "DATABASE_URL"
  value      = "postgres://user:pass@db.example.com/mydb"
  project_id = ona_project.example.id

  environment_variable = true
}

resource "ona_secret" "registry_auth" {
  name       = "REGISTRY_AUTH"
  value      = "dXNlcjpwYXNz"
  project_id = ona_project.example.id

  container_registry_basic_auth_host = "ghcr.io"
}
