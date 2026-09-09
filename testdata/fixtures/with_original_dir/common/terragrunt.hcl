locals {
  extra_atlantis_dependencies = [
    mark_as_read("${get_parent_terragrunt_dir()}/terragrunt.hcl")
  ]
}

dependency "dependency" {
  config_path = "${get_original_terragrunt_dir()}/../dependency"
}

inputs = {
  foo = "bar"
}
