terraform {
  source = "git::https://example.com/module.git?ref=v1.0.0"
}

exclude {
  if      = true
  actions = ["all"]
}
