data "harness_platform_infra_provider_signing_key" "example" {
  id = "signing_key_id"
}

output "key_id" {
  value = data.harness_platform_infra_provider_signing_key.example.key_id
}

output "key_name" {
  value = data.harness_platform_infra_provider_signing_key.example.key_name
}
