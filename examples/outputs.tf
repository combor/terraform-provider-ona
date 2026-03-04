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
