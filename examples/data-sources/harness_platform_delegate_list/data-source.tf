# Get all delegates in an account
data "harness_platform_delegate_list" "all" {
  account_id  = "your_account_id"
  filter_type = "Delegate"
  fetch_all   = true
}

# Get only connected delegates with specific tags
data "harness_platform_delegate_list" "connected_with_tags" {
  account_id    = "your_account_id"
  org_id        = "your_org_id"
  project_id    = "your_project_id"
  filter_type   = "Delegate"
  status        = "CONNECTED"
  delegate_tags = ["production", "kubernetes"]
}

# Get delegates by name pattern and group
data "harness_platform_delegate_list" "specific_group" {
  account_id                = "your_account_id"
  filter_type               = "Delegate"
  delegate_name             = "prod-delegate"
  delegate_group_identifier = "production-group"
  version_status            = "ACTIVE"
}
