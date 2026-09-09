module "some_module" {
  source = "../terraform-module"
}

module "another_module" {
  source = "git::https://example.com/module.git?ref=v1.0.0"
}
