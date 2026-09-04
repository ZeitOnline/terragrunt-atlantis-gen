include {
  path = "${find_in_parent_folders("parent")}/terragrunt.hcl"
}

terraform {
  source = "git::git@github.com:transcend-io/terraform-aws-fargate-container?ref=v0.0.4"
}

locals {
  extra_atlantis_dependencies = [
    mark_as_read("some_child_dep"),
  ]
}

inputs = {
  foo = "bar"
}
