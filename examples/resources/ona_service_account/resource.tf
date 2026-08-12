resource "ona_service_account" "ci" {
  name        = "ci"
  description = "Runs Terraform from CI"
  valid_until = "2027-01-31T00:00:00Z"
}
