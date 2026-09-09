include {
  path = find_in_parent_folders()
}

terraform {
  source = "git::https://example.com/module.git?ref=v1.0.0"
}

locals {
  atlantis_skip = true
}

inputs = {
  foo = "bar"
}
