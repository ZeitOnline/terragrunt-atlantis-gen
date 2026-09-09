include "root" {
  path = find_in_parent_folders("root.hcl")
}

include "envcommon" {
  path = "${dirname(find_in_parent_folders("root.hcl"))}/_envcommon/mysql.hcl"
}

inputs = {
  instance_class    = "db.t2.medium"
  allocated_storage = 100
}
