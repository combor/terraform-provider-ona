terraform {
  required_providers {
    ona = {
      source = "combor/ona"
    }
  }
}

provider "ona" {
  # api_key  = "your-api-key"   # Or set GITPOD_API_KEY env var
  # base_url = "https://app.gitpod.io/api"  # Or set GITPOD_BASE_URL env var
}
