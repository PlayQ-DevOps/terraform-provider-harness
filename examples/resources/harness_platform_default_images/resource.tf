# CI — override the lite-engine image
resource "harness_platform_default_images" "ci_lite_engine" {
  kind  = "ci"
  field = "liteEngineTag"
  value = "harness/ci-lite-engine:1.16.5"
}

data "harness_platform_default_images" "ci_customer" {
  kind       = "ci"
  type       = "customer"
  depends_on = [harness_platform_default_images.ci_lite_engine]
}

output "ci_lite_engine_tag" {
  value = data.harness_platform_default_images.ci_customer.images["liteEngineTag"]
}

# CI — reset the lite-engine image to the Harness default
resource "harness_platform_default_images" "ci_lite_engine" {
  kind  = "ci"
  field = "liteEngineTag"
}

# IACM — override the AWS CDK image
resource "harness_platform_default_images" "iacm_aws_cdk" {
  kind  = "iacm"
  field = "iacmAwsCdk"
  value = "plugins/harness_aws_cdk:1.2.0"
}

data "harness_platform_default_images" "iacm_defaults" {
  kind       = "iacm"
  depends_on = [harness_platform_default_images.iacm_aws_cdk]
}

output "iacm_aws_cdk_tag" {
  value = data.harness_platform_default_images.iacm_defaults.images["iacmAwsCdk"]
}

# IACM — reset to default
resource "harness_platform_default_images" "iacm_aws_cdk" {
  kind  = "iacm"
  field = "iacmAwsCdk"
}

# IDP — override the register-catalog image
resource "harness_platform_default_images" "idp_register_catalog" {
  kind  = "idp"
  field = "registerCatalog"
  value = "harness/registercatalog:1.5.0"
}

data "harness_platform_default_images" "idp_defaults" {
  kind       = "idp"
  depends_on = [harness_platform_default_images.idp_register_catalog]
}

output "idp_register_catalog_tag" {
  value = data.harness_platform_default_images.idp_defaults.images["registerCatalog"]
}

# IDP — reset to default
resource "harness_platform_default_images" "idp_register_catalog" {
  kind  = "idp"
  field = "registerCatalog"
}
