# AWS EC2 Runner Environment Class
# For runners with provider_type = "RUNNER_PROVIDER_AWS_EC2"
resource "ona_runner_environment_class" "aws_ec2_example" {
  runner_id    = "<aws-ec2-runner-id>"
  display_name = "Small"
  description  = "2 vCPU / 8 GiB / 50 GiB disk"

  configuration = {
    instance_type = "m6i.large"
    disk_size_gb  = 50
    spot          = false
  }

  enabled = true
}

# AWS EC2 Spot Instance Example
resource "ona_runner_environment_class" "aws_ec2_spot_example" {
  runner_id    = "<aws-ec2-runner-id>"
  display_name = "Large Spot"
  description  = "8 vCPU / 32 GiB / 200 GiB disk (Spot)"

  configuration = {
    instance_type = "m6i.2xlarge"
    disk_size_gb  = 200
    spot          = true
  }

  enabled = true
}
