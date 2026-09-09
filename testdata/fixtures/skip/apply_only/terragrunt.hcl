terraform {
  source = "git::https://example.com/module.git?ref=v1.0.0"
}

# Leaves plan alone, so Atlantis keeps the project.
exclude {
  if      = true
  actions = ["apply"]
}
