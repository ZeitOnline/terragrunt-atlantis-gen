terraform {
  source = "git::git@github.com:transcend-io/terraform-aws-fargate-container?ref=v0.0.4"
}

locals {
  extra_atlantis_dependencies = [
    mark_as_read("some_extra_dep"),
    mark_as_read(find_in_parent_folders("test_file.json"))
  ]
}
