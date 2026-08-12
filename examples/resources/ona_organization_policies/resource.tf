resource "ona_organization_policies" "current" {
  maximum_environment_timeout           = "8h"
  maximum_running_environments_per_user = 4
  maximum_environments_per_user         = 10
  port_sharing_disabled                 = false

  members_require_projects = true
  members_create_projects  = false
}
