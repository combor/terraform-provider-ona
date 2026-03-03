output "runner_id" {
  value = ona_runner.example.id
}

output "project_id" {
  value = ona_project.example.id
}

output "project_lookup_id" {
  value = data.ona_project.example.id
}

output "project_lookup_name" {
  value = data.ona_project.example.name
}
