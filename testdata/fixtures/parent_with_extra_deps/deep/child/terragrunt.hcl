include {
  path = "${find_in_parent_folders("parent")}/terragrunt.hcl"
}

terraform {
  source = "git::https://example.com/module.git?ref=v1.0.0"
}

locals {
  extra_atlantis_dependencies = [
    mark_as_read("some_child_dep"),
  ]
}

inputs = {
  foo = "bar"
}
