data "ona_groups" "all" {}

data "ona_groups" "engineering" {
  filter {
    name   = "name"
    values = ["Engineering"]
  }
}
