resource "harness_platform_ip_allowlist" "example" {
  identifier = "identifier"
  name       = "name"
  ip_address = "0.0.0.0/0"

  allowed_source_type = ["API"]
  enabled             = true

  description = "allowlist example"
  tags        = ["foo:bar"]
}
