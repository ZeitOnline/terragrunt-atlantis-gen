terraform {
  source = "git::https://example.com/module.git?ref=v1.0.0"
}

dependency "some_dep" {
  config_path = "../depender"
}

dependency "nested" {
  config_path = "./nested"
}

inputs = {
  foo = dependency.some_dep.outputs.some_output
}
