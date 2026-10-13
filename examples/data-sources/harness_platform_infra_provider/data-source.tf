data "harness_platform_infra_provider" "aws" {
  id = "123"
}

output "provider_type" {
  value = data.harness_platform_infra_provider.aws.type
}

output "provider_versions" {
  value = data.harness_platform_infra_provider.aws.versions
}
