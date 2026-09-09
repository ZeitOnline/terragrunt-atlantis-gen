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

    arguments = [
      "-var-file=${get_terragrunt_dir()}/../../../../common_vars/apps/consul/sg.tfvars",
      "-var-file=${get_terragrunt_dir()}/main.tfvars"
    ]
  }
}

inputs = {
  foo = "bar"
}

locals {
  # read-marks for the var-files above; terragrunt find reports them
  atlantis_var_file_reads = [
    mark_as_read("${get_terragrunt_dir()}/../../../../common_vars/apps/consul/sg.tfvars"),
    mark_as_read("${get_terragrunt_dir()}/main.tfvars"),
  ]
}
