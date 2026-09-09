include "root" {
  path = find_in_parent_folders("root.hcl")
}

terraform {
  source = "git::https://example.com/module.git?ref=v1.0.0"
  extra_arguments "conditional_vars" {
    commands = [
      "apply",
      "plan",
      "import",
      "push",
      "refresh"
    ]

    required_var_files = [
      "${get_parent_terragrunt_dir()}/terraform.tfvars"
    ]
  }
}

locals {
  # mirrors the var-files above, which find cannot see
  reads = [
    mark_as_read("${get_parent_terragrunt_dir()}/terraform.tfvars"),
  ]
}
