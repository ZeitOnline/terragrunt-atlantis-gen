terraform {
  source = "git::git@github.com:gruntwork-io/terragrunt-infrastructure-modules-example.git//tgw?ref=v0.3.0"
}

include "root" {
  path = find_in_parent_folders("root.hcl")
}

locals {
  env_vars = read_terragrunt_config(find_in_parent_folders("env.hcl"))
  env_name = local.env_vars.locals.env
}

inputs = {
  name = "tgw-${local.env_name}"
}
