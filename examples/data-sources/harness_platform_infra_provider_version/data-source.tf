data "harness_platform_infra_provider_version" "aws_v5" {
  provider_id = "provider_id"
  version     = "5.0.0"
}

output "gpg_key_id" {
  value = data.harness_platform_infra_provider_version.aws_v5.gpg_key_id
}

output "protocols" {
  value = data.harness_platform_infra_provider_version.aws_v5.protocols
}
