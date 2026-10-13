resource "harness_platform_secret_file" "example" {
  identifier                = "identifier"
  name                      = "name"
  description               = "test"
  tags                      = ["foo:bar"]
  file_path                 = "file_path"
  secret_manager_identifier = "harnessSecretManager"
}

# With AWS Secret Manager KMS Key
resource "harness_platform_secret_file" "aws_secret_manager" {
  identifier                = "identifier"
  name                      = "name"
  description               = "example"
  tags                      = ["foo:bar"]
  file_path                 = "file_path"
  secret_manager_identifier = "awsSecretManager"

  additional_metadata {
    values {
      kms_key_id = "kmsKeyId"
    }
  }
}

# With GCP Secret Manager project ID and region
resource "harness_platform_secret_file" "gcp_secret_manager" {
  identifier                = "identifier"
  name                      = "name"
  description               = "example"
  tags                      = ["foo:bar"]
  file_path                 = "file_path"
  secret_manager_identifier = "gcpSecretManager"

  additional_metadata {
    values {
      regions        = "us-east1"
      gcp_project_id = "my-gcp-project-id"
    }
  }
}
