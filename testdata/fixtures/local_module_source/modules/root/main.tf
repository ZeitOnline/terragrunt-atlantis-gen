module "shared" {
  source = "../shared"
}

module "registry" {
  source = "oci://europe-west3-docker.pkg.dev/example/terraform-modules/base?tag=1.0.0"
}
