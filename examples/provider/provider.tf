provider "ona" {
  api_key  = var.ona_api_key   # or set GITPOD_API_KEY env var
  base_url = var.ona_base_url  # optional, defaults to https://app.gitpod.io/api
}
