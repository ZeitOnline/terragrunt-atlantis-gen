include "root" {
  path = find_in_parent_folders("root.hcl")
}

include "shared" {
  path = find_in_parent_folders("common.hcl")
}

terraform {
  source = "git::https://example.com/module.git?ref=v1.0.0"
}
