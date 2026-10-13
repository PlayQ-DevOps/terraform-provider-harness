# First, create a provider
resource "harness_platform_infra_provider" "aws" {
  type        = "aws"
  description = "AWS provider for IaCM"
}

# Create a version
resource "harness_platform_infra_provider_version" "aws_v5" {
  provider_id = harness_platform_infra_provider.aws.id
  version     = "5.0.0"
  gpg_key_id  = "your-gpg-key-id"
  protocols   = ["5.0", "6.0"]
}

# Upload files for different platforms
resource "harness_platform_infra_provider_version_file" "aws_linux_amd64" {
  provider_id = harness_platform_infra_provider.aws.id
  version     = harness_platform_infra_provider_version.aws_v5.version
  file_path   = "./terraform-provider-aws_5.0.0_linux_amd64.zip"
}

resource "harness_platform_infra_provider_version_file" "aws_darwin_amd64" {
  provider_id = harness_platform_infra_provider.aws.id
  version     = harness_platform_infra_provider_version.aws_v5.version
  file_path   = "./terraform-provider-aws_5.0.0_darwin_amd64.zip"
}

resource "harness_platform_infra_provider_version_file" "aws_windows_amd64" {
  provider_id = harness_platform_infra_provider.aws.id
  version     = harness_platform_infra_provider_version.aws_v5.version
  file_path   = "./terraform-provider-aws_5.0.0_windows_amd64.zip"
}

# Upload SHA256SUMS and signature files
resource "harness_platform_infra_provider_version_file" "aws_shasums" {
  provider_id = harness_platform_infra_provider.aws.id
  version     = harness_platform_infra_provider_version.aws_v5.version
  file_path   = "./terraform-provider-aws_5.0.0_SHA256SUMS"
}

resource "harness_platform_infra_provider_version_file" "aws_shasums_sig" {
  provider_id = harness_platform_infra_provider.aws.id
  version     = harness_platform_infra_provider_version.aws_v5.version
  file_path   = "./terraform-provider-aws_5.0.0_SHA256SUMS.sig"
}

# Optional: Override the uploaded filename if different from local filename
resource "harness_platform_infra_provider_version_file" "custom_name" {
  provider_id = harness_platform_infra_provider.aws.id
  version     = harness_platform_infra_provider_version.aws_v5.version
  file_path   = "./local-file.zip"
  filename    = "terraform-provider-aws_5.0.0_custom.zip"
}
