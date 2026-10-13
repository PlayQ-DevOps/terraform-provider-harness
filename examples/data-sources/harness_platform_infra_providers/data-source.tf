data "harness_platform_infra_providers" "all" {}

output "provider_count" {
  value = length(data.harness_platform_infra_providers.all.providers)
}

output "provider_types" {
  value = [for p in data.harness_platform_infra_providers.all.providers : p.type]
}
