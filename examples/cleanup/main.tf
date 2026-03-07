# CI helper: discover and destroy stale runners for a given runner manager.
#
# Usage:
#   terraform apply  -var="runner_manager_id=..." -auto-approve
#   terraform destroy -var="runner_manager_id=..." -auto-approve

terraform {
  required_version = ">= 1.7.0"

  required_providers {
    ona = {
      source = "combor/ona"
    }
  }
}

provider "ona" {}

variable "runner_manager_id" {
  description = "Runner manager ID to clean up."
  type        = string
}

data "ona_runners" "stale" {
  filter {
    name   = "runner_manager_id"
    values = [var.runner_manager_id]
  }
}

locals {
  stale_runners = { for r in data.ona_runners.stale.runners : r.id => r }
}

import {
  for_each = local.stale_runners
  to       = ona_runner.stale[each.key]
  id       = each.key
}

resource "ona_runner" "stale" {
  for_each          = local.stale_runners
  name              = each.value.name
  provider_type     = each.value.provider_type
  runner_manager_id = each.value.runner_manager_id
}
