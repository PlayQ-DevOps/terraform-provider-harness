# Account level — implicit account from provider
data "harness_platform_delegate_default_version" "account" {
}

output "delegate_version" {
  value = data.harness_platform_delegate_default_version.account.version
}

output "delegate_minimal_version" {
  value = data.harness_platform_delegate_default_version.account.minimal_version
}

# Org level — implicit account from provider
data "harness_platform_delegate_default_version" "org_implicit" {
  org_id = "your_org_id"
}

output "delegate_version_org_implicit" {
  value = data.harness_platform_delegate_default_version.org_implicit.version
}

# Project level — implicit account from provider
data "harness_platform_delegate_default_version" "project" {
  org_id     = "your_org_id"
  project_id = "your_project_id"
}

output "delegate_version_project" {
  value = data.harness_platform_delegate_default_version.project.version
}
