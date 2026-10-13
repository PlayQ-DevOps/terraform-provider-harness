# Create a complete provider with version, files, and publish
resource "harness_platform_infra_provider" "aws" {
  type        = "aws"
  description = "AWS provider for IaCM"
}

resource "harness_platform_infra_provider_signing_key" "hashicorp" {
  key_id      = "51852D87348FFC4C"
  key_name    = "HashiCorp Security"
  user        = "security@hashicorp.com"
  ascii_armor = file("${path.module}/hashicorp.asc")
}

resource "harness_platform_infra_provider_version" "aws_v5" {
  provider_id = harness_platform_infra_provider.aws.id
  version     = "5.0.0"
  gpg_key_id  = harness_platform_infra_provider_signing_key.hashicorp.key_id
  protocols   = ["5.0", "6.0"]
}

# Upload all required files
resource "harness_platform_infra_provider_version_file" "aws_linux_amd64" {
  provider_id = harness_platform_infra_provider.aws.id
  version     = harness_platform_infra_provider_version.aws_v5.version
  filename    = "terraform-provider-aws_5.0.0_linux_amd64.zip"
}

resource "harness_platform_infra_provider_version_file" "aws_darwin_amd64" {
  provider_id = harness_platform_infra_provider.aws.id
  version     = harness_platform_infra_provider_version.aws_v5.version
  filename    = "terraform-provider-aws_5.0.0_darwin_amd64.zip"
}

resource "harness_platform_infra_provider_version_file" "aws_shasums" {
  provider_id = harness_platform_infra_provider.aws.id
  version     = harness_platform_infra_provider_version.aws_v5.version
  filename    = "terraform-provider-aws_5.0.0_SHA256SUMS"
}

resource "harness_platform_infra_provider_version_file" "aws_shasums_sig" {
  provider_id = harness_platform_infra_provider.aws.id
  version     = harness_platform_infra_provider_version.aws_v5.version
  filename    = "terraform-provider-aws_5.0.0_SHA256SUMS.sig"
}

# Publish the version after all files are uploaded
resource "harness_platform_infra_provider_version_publish" "aws_v5" {
  provider_id = harness_platform_infra_provider.aws.id
  version     = harness_platform_infra_provider_version.aws_v5.version

  depends_on = [
    harness_platform_infra_provider_version_file.aws_linux_amd64,
    harness_platform_infra_provider_version_file.aws_darwin_amd64,
    harness_platform_infra_provider_version_file.aws_shasums,
    harness_platform_infra_provider_version_file.aws_shasums_sig,
  ]
}
