resource "harness_platform_infra_provider" "aws" {
  type        = "aws"
  description = "AWS provider for IaCM"
}

resource "harness_platform_infra_provider" "azurerm" {
  type        = "azurerm"
  description = "Azure provider for IaCM"
}

resource "harness_platform_infra_provider" "google" {
  type        = "google"
  description = "Google Cloud provider for IaCM"
}
