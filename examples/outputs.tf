output "runner_id" {
  value = ona_runner.example.id
}

output "runner_name" {
  value = data.ona_runner.example.name
}

output "runner_phase" {
  value = data.ona_runner.example.status.phase
}
