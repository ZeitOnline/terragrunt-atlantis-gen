locals {
  reads = [
    # A relative file to the child should work
    mark_as_read("some_parent_dep"),

    # Relative files to the child should be conditionally includable
    mark_as_read(fileexists("${path_relative_to_include()}/local_tags.yaml") ? "local_tags.yaml" : ""),

    # Functions should run from the child dir, not the parent dir
    mark_as_read(find_in_parent_folders("file_in_parent_of_child.json")),
    mark_as_read("${get_parent_terragrunt_dir()}/shared/common_tags.hcl"),

    # Empty strings should be ignored completely
    mark_as_read(find_in_parent_folders("file_name_that_does_not_exist.jpg", ""))
  ]
}
