data "harness_platform_infra_provider_versions" "aws_versions" {
  account = "your-account-id"
  type    = "aws"
}

output "all_aws_versions" {
  value = data.harness_platform_infra_provider_versions.aws_versions.versions
}

# Filter to get only versions supporting protocol 6.0
locals {
  v6_compatible_versions = [
    for v in data.harness_platform_infra_provider_versions.aws_versions.versions : v
    if contains(v.protocols, "6.0")
  ]
}
