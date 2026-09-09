locals {
  environment_vars = read_terragrunt_config(find_in_parent_folders("env.hcl"))

  env = local.environment_vars.locals.environment

  base_source_url = "git::git@github.com:gruntwork-io/terragrunt-infrastructure-modules-example.git//asg-elb-service"
}

inputs = {
  name          = "webserver-example-${local.env}"
  instance_type = "t2.micro"

  min_size = 2
  max_size = 2

  server_port = 8080
  elb_port    = 80
}
