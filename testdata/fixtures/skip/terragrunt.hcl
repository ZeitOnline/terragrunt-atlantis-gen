locals {
  atlantis_skip = true
}

# skipped: applied manually, never by Atlantis or run --all
exclude {
  if      = true
  actions = ["all"]
}
