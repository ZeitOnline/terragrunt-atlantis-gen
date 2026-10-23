# Pins that a relative source inherited through include resolves against the
# unit, as a run does: on Terragrunt below 1.2.0 the module stays unmarked and
# the golden fails.
terraform {
  source = "${path_relative_from_include()}/modules//root"
}
