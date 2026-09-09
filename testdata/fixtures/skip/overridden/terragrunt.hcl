include "root" {
  path = find_in_parent_folders("root.hcl")
}

terraform {
  source = "git::https://example.com/module.git?ref=v1.0.0"
}

# Re-enables a unit under an excluded parent.
exclude {
  if      = false
  actions = ["all"]
}
