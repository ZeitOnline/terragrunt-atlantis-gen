include "shared" {
  path = find_in_parent_folders("common.hcl")
}

include "root" {
  path = find_in_parent_folders("root.hcl")
}

terraform {
  source = "git::https://example.com/module.git?ref=v1.0.0"
}
