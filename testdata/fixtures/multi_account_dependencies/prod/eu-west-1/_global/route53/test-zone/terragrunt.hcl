locals {
  environment_vars = read_terragrunt_config(find_in_parent_folders("env.hcl"))

  env = local.environment_vars.locals.env
}

terraform {
  source = "git::git@github.com:gruntwork-io/terragrunt-infrastructure-modules-example.git//zone?ref=v0.3.0"
}

include "root" {
  path = find_in_parent_folders("root.hcl")
}

dependency "vpc" {
  config_path = "../../../env-a/network/vpc/"
}

inputs = {
  name = "test_zone_${local.env}"
}
