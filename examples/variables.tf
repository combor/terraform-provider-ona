variable "ona_api_key" {
  description = "Ona API key. Leave empty to use GITPOD_API_KEY."
  type        = string
  sensitive   = true
  default     = ""
}

variable "ona_base_url" {
  description = "Ona API base URL."
  type        = string
  default     = "https://app.gitpod.io/api"
}

variable "runner_name" {
  description = "Runner name. Use a unique value in CI."
  type        = string
}

variable "runner_provider_type" {
  description = "Runner provider type."
  type        = string
}

variable "runner_manager_id" {
  description = "Runner manager ID required for managed runners."
  type        = string
  default     = ""
}

variable "runner_region" {
  description = "Runner region."
  type        = string
  default     = "eu-central-1"
}

variable "project_name" {
  description = "Project name. Use a unique value in CI."
  type        = string
}

variable "project_git_remote_uri" {
  description = "Git remote URI for the project initializer."
  type        = string
  default     = "https://github.com/combor/terraform-provider-ona"
}
