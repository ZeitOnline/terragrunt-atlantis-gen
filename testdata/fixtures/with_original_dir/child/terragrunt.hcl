include "root" {
  path   = find_in_parent_folders()
  expose = true
}

include "common_configs" {
  path   = "${dirname(find_in_parent_folders())}/common/terragrunt.hcl"
  expose = true
}

terraform {
  source = "git::https://example.com/module.git?ref=v1.0.0"
}

inputs = {
  foo = "bar"
}
