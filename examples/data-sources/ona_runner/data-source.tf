data "ona_runner" "example" {
  id = "runner-id-here"
}

output "runner_name" {
  value = data.ona_runner.example.name
}
