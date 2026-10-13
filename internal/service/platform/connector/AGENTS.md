# AGENTS.md - Connector Resources

## Purpose

This package implements Terraform resources and data sources for Harness connectors.

**Status (fork):** this package is SDK v2 and frozen. No new SDK v2 connector may be added
(`TestContract` rejects any type missing from `internal/provider/testdata/sdk_v2_types.txt`).
New connectors are written on terraform-plugin-framework with the `internal/framework` helpers
and registered in `internal/provider/framework_provider.go`; see the root `AGENTS.md`. The
existing connectors move to the framework one source package at a time (plan Phase 5); until a
package has moved, fixes to its SDK v2 code still go here and keep using `connectorbase`. Connectors provide integration with external systems like cloud providers, monitoring tools, ticketing systems, and secret managers.

## Package Structure

```
connector/
├── connectorbase/base.go     # CRUD logic shared by every connector (also used by cd_nextgen/connector/*)
├── (connector data sources live next to their resources as *_data_source.go)
├── (secret reference description text: helpers.Descriptions.SecretRefText)
├── secretManagers/           # Secret manager connector implementations
│   ├── aws_kms.go
│   ├── aws_secrets_manager.go
│   ├── azure_key_vault.go
│   ├── gcp_kms.go
│   ├── gcp_secret_manager.go
│   ├── hashicorp_vault.go
│   └── ...
├── appdynamics.go            # AppDynamics connector
├── aws_cc.go                 # AWS Cloud Cost connector
├── azure_cloud_cost.go       # Azure Cloud Cost connector
├── datadog.go                # Datadog connector
├── dynatrace.go              # Dynatrace connector
├── elasticsearch.go          # Elasticsearch connector
├── gcp_cloud_cost.go         # GCP Cloud Cost connector
├── jdbc.go                   # JDBC database connector
├── jira.go                   # Jira connector
├── kubernetes_cloud_cost.go  # Kubernetes Cloud Cost connector
├── newrelic.go               # New Relic connector
├── pagerduty.go              # PagerDuty connector
├── prometheus.go             # Prometheus connector
├── service_now.go            # ServiceNow connector
├── splunk.go                 # Splunk connector
├── sumologic.go              # Sumo Logic connector
└── *_test.go                 # Test files for each connector
```

## Base Functions (connectorbase package)

All connector resources, in this package and in internal/service/cd_nextgen/connector/*, use the single shared base package `connectorbase` (never copy it):

```go
// For resource Read operations
connectorbase.Read(ctx, d, meta, connType)   // returns (nil, nil) when the connector is gone and state was cleared

// For data source Read operations
connectorbase.DataRead(ctx, d, meta, connType)

// For Create/Update operations
connectorbase.CreateOrUpdate(ctx, d, meta, connector)

// For Delete operations
connectorbase.Delete(ctx, d, meta)
```

## Creating a New Connector Resource

1. Create `<connector_type>.go` with:
   - `Resource<ConnectorType>()` function returning `*schema.Resource`
   - Schema definition using `helpers.MergeSchemas`
   - CRUD context functions that call base functions

2. Create `<connector_type>_data_source.go` for the data source

3. Create `<connector_type>_test.go` and `<connector_type>_data_source_test.go` for tests

4. Register in `internal/provider/provider.go`:
   - Add import alias (e.g., `pl_connector_newtype "github.com/harness/terraform-provider-harness/internal/service/platform/connector"`)
   - Add to `ResourcesMap` and `DataSourcesMap`

## Common Patterns

### Schema Definition
```go
func Resource<ConnectorType>() *schema.Resource {
    resource := &schema.Resource{
        Description:   "Resource for creating a <Type> connector.",
        ReadContext:   resource<ConnectorType>Read,
        CreateContext: resource<ConnectorType>CreateOrUpdate,
        UpdateContext: resource<ConnectorType>CreateOrUpdate,
        DeleteContext: connectorbase.Delete,  // Uses shared delete
        Importer:      helpers.MultiLevelResourceImporter,
        Schema: helpers.MergeSchemas(
            commonConnectorSchema(),       // Common fields
            connectorTypeSpecificSchema(), // Type-specific fields
        ),
    }
    return resource
}
```

### Secret References
Use `helpers.Descriptions.SecretRefText.String()` for secret reference field descriptions:
```go
Description: "Reference to the Harness secret containing the token." + helpers.Descriptions.SecretRefText.String(),
```

## Testing

Run tests for a specific connector:
```sh
go test -v ./internal/service/platform/connector/... -run TestAccResourceConnector<Type>
```

Run all connector tests:
```sh
go test -v ./internal/service/platform/connector/... -timeout 120m
```

## API Client

Uses the Platform client from the session:
```go
c, ctx := meta.(*internal.Session).GetPlatformClientWithContext(ctx)
```

Connector API calls:
- `c.ConnectorsApi.GetConnector()`
- `c.ConnectorsApi.CreateConnector()`
- `c.ConnectorsApi.UpdateConnector()`
- `c.ConnectorsApi.DeleteConnector()`

## DOs

- Use the `connectorbase` package for CRUD operations
- Use `helpers.MergeSchemas` to combine common and specific schemas
- Use `helpers.Descriptions.SecretRefText` for secret reference field descriptions
- Add both resource and data source implementations
- Add comprehensive acceptance tests

## DON'Ts

- Never copy connectorbase into another package; import it
- Don't hardcode account/org/project IDs - use schema values
- Don't forget to register new connectors in provider.go
- Don't add local .terraform or terraform.tfstate files to git
