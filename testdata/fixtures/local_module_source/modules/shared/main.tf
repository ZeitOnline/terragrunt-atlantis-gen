resource "null_resource" "shared" {}

module "nested" {
  source = "./nested"
}

module "another" {
  source = "../another"
}
