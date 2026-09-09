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

    optional_var_files = [
      "${get_parent_terragrunt_dir()}/${get_env("TF_VAR_env", "dev")}.tfvars",
      "${get_parent_terragrunt_dir()}/${get_env("TF_VAR_region", "us-east-1")}.tfvars",
      "${get_terragrunt_dir()}/${get_env("TF_VAR_env", "dev")}.tfvars",
      "${get_terragrunt_dir()}/${get_env("TF_VAR_region", "us-east-1")}.tfvars"
    ]
  }
}

locals {
  # mirrors the var-files above, which find cannot see
  reads = [
    mark_as_read("${get_parent_terragrunt_dir()}/${get_env("TF_VAR_env", "dev")}.tfvars"),
    mark_as_read("${get_parent_terragrunt_dir()}/${get_env("TF_VAR_region", "us-east-1")}.tfvars"),
    mark_as_read("${get_terragrunt_dir()}/${get_env("TF_VAR_env", "dev")}.tfvars"),
    mark_as_read("${get_terragrunt_dir()}/${get_env("TF_VAR_region", "us-east-1")}.tfvars"),
  ]
}
