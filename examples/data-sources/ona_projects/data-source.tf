data "ona_projects" "by_repository" {
  remote_uris = ["https://github.com/combor/terraform-provider-ona"]
}

data "ona_projects" "most_popular" {
  sort = {
    field = "popularity"
    order = "SORT_ORDER_DESC"
  }
  limit         = 10
  include_count = true
}
