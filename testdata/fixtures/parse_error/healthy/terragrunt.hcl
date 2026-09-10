include "root" {
  path = find_in_parent_folders("root.hcl")
}

locals {
  reads = [mark_as_read("${get_parent_terragrunt_dir()}/data/values.yaml")]
}
