resource "ona_project_policy" "platform_editor" {
  project_id = ona_project.example.id
  group_id   = ona_group.platform.id
  role       = "PROJECT_ROLE_EDITOR"
}
