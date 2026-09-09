include "root" {
  path = find_in_parent_folders("root.hcl")
}

terraform {
  source = "git::https://example.com/module.git?ref=v1.0.0"
}

locals {
  reads = [
    mark_as_read("some_child_dep"),
  ]
}
