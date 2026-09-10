include "root" {
  path = find_in_parent_folders("root.hcl")
}

locals {
  settings = read_terragrunt_config("${get_parent_terragrunt_dir()}/settings/missing.hcl")
}
