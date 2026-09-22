output "runner_id" {
  value = ona_runner.example.id
}

output "runner_environment_class_ids" {
  value = [for environment_class in data.ona_runner_environment_classes.example.environment_classes : environment_class.id]
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

output "project_list_ids" {
  value = [for project in data.ona_projects.example.projects : project.id]
}

output "project_list_total_count" {
  value = data.ona_projects.example.total_count
}

output "secret_id" {
  value = ona_secret.example.id
}
