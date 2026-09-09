terraform {
  source = "git::https://example.com/module.git?ref=v1.0.0"
}

locals {
  reads = [
    mark_as_read("some_extra_dep"),
    mark_as_read(find_in_parent_folders("test_file.json"))
  ]
}
