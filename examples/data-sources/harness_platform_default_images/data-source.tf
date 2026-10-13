# CI — read Harness default images
data "harness_platform_default_images" "ci_defaults" {
  kind = "ci"
}

output "ci_default_images" {
  value = data.harness_platform_default_images.ci_defaults.images
}

# CI — read customer-configured overrides
data "harness_platform_default_images" "ci_customer" {
  kind = "ci"
  type = "customer"
}

output "ci_lite_engine_override" {
  value = data.harness_platform_default_images.ci_customer.images["liteEngineTag"]
}

# IACM — read Harness default images
data "harness_platform_default_images" "iacm_defaults" {
  kind = "iacm"
}

output "iacm_default_images" {
  value = data.harness_platform_default_images.iacm_defaults.images
}

# IACM — read customer-configured overrides
data "harness_platform_default_images" "iacm_customer" {
  kind = "iacm"
  type = "customer"
}

output "iacm_aws_cdk_override" {
  value = data.harness_platform_default_images.iacm_customer.images["iacmAwsCdk"]
}

# IDP — read Harness default images
data "harness_platform_default_images" "idp_defaults" {
  kind = "idp"
}

output "idp_default_images" {
  value = data.harness_platform_default_images.idp_defaults.images
}

# IDP — read customer-configured overrides
data "harness_platform_default_images" "idp_customer" {
  kind = "idp"
  type = "customer"
}

output "idp_register_catalog_override" {
  value = data.harness_platform_default_images.idp_customer.images["registerCatalog"]
}
