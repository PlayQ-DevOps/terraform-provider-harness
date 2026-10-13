resource "harness_platform_resource_group" "example" {
  identifier  = "identifier"
  name        = "name"
  description = "test"
  tags        = ["foo:bar"]

  # Account where this resource group is created
  # - For Account-level resource groups: specify only account_id
  # - For Organization-level resource groups: specify account_id + org_id
  # organization_id = "organization_id"
  # - For Project-level resource groups: specify account_id + org_id + project_id
  # project_id = "project_id"

  account_id = "account_id"

  # Defines at which scope levels this resource group can be referenced in role bindings.
  # This must match the scope at which the resource group is created. If not specified, it defaults to the creation scope.
  allowed_scope_levels = ["account"]

  included_scopes {
    # EXCLUDING_CHILD_SCOPES: Only the specified scope is included
    # INCLUDING_CHILD_SCOPES: Includes the specified scope and all child scopes
    filter     = "EXCLUDING_CHILD_SCOPES"
    account_id = "account_id"
  }

  resource_filter {
    # If true, all resources within the included scopes are selected
    # If false, you must explicitly define resources below
    include_all_resources = false

    resources {
      # Must be a valid Harness resource type (see Supported resource_type values)
      resource_type = "CONNECTOR"

      # Optional: further narrow the selection using attribute-based filtering
      attribute_filter {
        attribute_name   = "category"
        attribute_values = ["CLOUD_COST"]
      }
    }
  }
}
