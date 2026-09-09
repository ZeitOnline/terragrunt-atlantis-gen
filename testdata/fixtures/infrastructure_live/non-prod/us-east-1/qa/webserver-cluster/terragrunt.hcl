terraform {
  source = "${include.envcommon.locals.base_source_url}?ref=v0.4.0"
}

include "root" {
  path = find_in_parent_folders("root.hcl")
}

include "envcommon" {
  path   = "${dirname(find_in_parent_folders("root.hcl"))}/_envcommon/webserver-cluster.hcl"
  expose = true
}
