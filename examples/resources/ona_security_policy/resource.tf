resource "ona_security_policy" "restricted" {
  name = "restricted"

  ports = {
    max_admission_level = "ADMISSION_LEVEL_ORGANIZATION"
  }

  executables = {
    rules = [
      {
        path   = "/usr/bin/curl"
        effect = "EFFECT_AUDIT"
      },
      {
        path   = "npx"
        effect = "EFFECT_BLOCK"
      },
    ]
  }
}
