include "root" {
  path = find_in_parent_folders("root.hcl")
}

dependency "dep" {
  config_path = "../dep"
}
