# First, create a provider
resource "harness_platform_infra_provider" "aws" {
  type        = "aws"
  description = "AWS provider for IaCM"
}

# Then create a version for that provider
resource "harness_platform_infra_provider_version" "aws_v5" {
  provider_id = harness_platform_infra_provider.aws.id
  version     = "5.0.0"
  gpg_key_id  = "your-gpg-key-id"
  protocols   = ["5.0", "6.0"]
}

resource "harness_platform_infra_provider_version" "aws_v4" {
  provider_id = harness_platform_infra_provider.aws.id
  version     = "4.67.0"
  gpg_key_id  = "your-gpg-key-id"
  protocols   = ["5.0"]
}
