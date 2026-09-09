include {
  path = find_in_parent_folders()
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

inputs = {
  foo = "bar"
}

locals {
  # read-marks for the var-files above; terragrunt find reports them
  atlantis_var_file_reads = [
    mark_as_read("${get_parent_terragrunt_dir()}/terraform.tfvars"),
  ]
}
