include "root" {
  path   = find_in_parent_folders("root.hcl")
  expose = true
}

include "common_configs" {
  path   = find_in_parent_folders("common.hcl")
  expose = true
}

terraform {
  source = "git::https://example.com/module.git?ref=v1.0.0"
}
