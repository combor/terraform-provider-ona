resource "ona_group_membership" "developer" {
  group_id = ona_group.platform.id
  user_id  = "<user-id>"
}

resource "ona_group_membership" "ci" {
  group_id           = ona_group.platform.id
  service_account_id = ona_service_account.ci.id
}
