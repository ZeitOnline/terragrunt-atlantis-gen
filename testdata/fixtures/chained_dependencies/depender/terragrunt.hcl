terraform {
  source = "git::https://example.com/module.git?ref=v1.0.0"
}

dependency "some_dep" {
  config_path = "../dependency"
}

inputs = {
  foo = dependency.some_dep.outputs.some_output
}
