data "harness_platform_infra_provider_signing_keys" "all" {}

output "all_signing_keys" {
  value = data.harness_platform_infra_provider_signing_keys.all.signing_keys
}

# Filter to find a specific key by key_id
locals {
  hashicorp_key = [
    for key in data.harness_platform_infra_provider_signing_keys.all.signing_keys : key
    if key.key_id == "51852D87348FFC4C"
  ]
}
