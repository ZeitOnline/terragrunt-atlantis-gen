include "root" {
  path = find_in_parent_folders("root.hcl")
}

include "envcommon" {
  path = "${dirname(find_in_parent_folders("root.hcl"))}/_envcommon/webserver-cluster.hcl"
}

inputs = {
  instance_type = "t2.medium"

  min_size = 3
  max_size = 3
}
