# AGENTS.md - Connector Resources

## Purpose

The `harness_platform_connector_*` resources and data sources: integrations with cloud
providers, code repositories, artifact repositories, secret managers, monitoring and
ticketing systems. Every connector is served by the terraform-plugin-framework half of the
provider through one generic implementation; a connector type contributes only its schema
declaration and its build and read functions.

## How a connector is served

- `internal/service/platform/connectors` (plural) is the generic framework resource and
  data source. It implements the connector lifecycle once: scope-aware read, create or
  update (with the governance denial message), delete with `force_delete`, the common
  attributes, import and resource identity from the multi-level import spec, state version
  1 with a recursive zero-to-null upgrader, and a configuration validator hook.
- A connector type is a `connectors.Definition`: `Suffix` (the type name after
  `harness_platform_connector_`), `Type` (the `nextgen.ConnectorType` Harness reports),
  `Resource` and `DataSource` (the SDK v2 `*schema.Resource` declarations, used for their
  Schema, Description and Importer), `Build` (or `BuildErr` when building can fail), `Read`,
  and an optional `Validate` (a whole-configuration check, what used to be CustomizeDiff).
- `internal/framework/bridge` converts the SDK v2 schema into the framework schema with the
  same wire shape and runs the build and read functions over a real `*schema.ResourceData`
  built from the framework plan or state. The SDK v2 CRUD wrappers (`resourceConnectorXRead`,
  `...CreateOrUpdate`) and `connectorbase` are not used by the served provider; they exist so
  `provider.LegacySDKProvider` can apply SDK v2 state in `acctest.RunMigration`.
- Each package exports `Definitions()`; `internal/provider/connectors.go` lists the packages.

```
connector/                      # this package: platform connectors (monitoring, ticketing, cost, models, JDBC)
├── definitions.go              # Definitions(): one entry per connector type
├── <type>.go                   # SDK v2 declaration: Resource<Type>(), build<Type>(), read<Type>()
├── <type>_data_source.go       # SDK v2 declaration of the data source schema
├── secretManagers/             # secret manager connectors, same layout
├── connectorbase/              # legacy SDK v2 CRUD, used only by the migration tests
└── ../../cd_nextgen/connector/{codeRepositories,cloudProviders,artifactRepositories}/  # same layout
```

## Adding a connector type

1. Declare the schema the SDK v2 way in `<type>.go`: `Resource<Type>()` returning a
   `*schema.Resource` with `Schema` (use `helpers.SetMultiLevelResourceSchema`), a
   `Description`, `Importer: helpers.MultiLevelResourceImporter`, and a `force_delete` plain
   Optional boolean. Keep every constraint declarative (`ExactlyOneOf`, `ConflictsWith`,
   `AtLeastOneOf`, `RequiredWith`, `ValidateFunc`, `MaxItems`, `Default`): the bridge turns
   them into framework validators, defaults and plan modifiers. A string `DiffSuppressFunc`
   becomes a plan modifier; one on a list is ignored (unset lists are null anyway). Do not
   use `CustomizeDiff`: put whole-configuration checks in a function that reads the
   configuration through `GetOk` and wire it as `Validate`.
2. Write `build<Type>(d *schema.ResourceData) *nextgen.ConnectorInfo` and
   `read<Type>(d *schema.ResourceData, conn *nextgen.ConnectorInfo) error`. Reads set every
   attribute from the API object; `helpers.SetAll` reports failed sets.
3. Declare the data source schema in `<type>_data_source.go` (`Datasource<Type>()`), all
   type-specific attributes Computed, with `helpers.SetMultiLevelDatasourceSchema`.
4. Add the `connectors.Definition` to this package's `Definitions()`. Nothing is added to
   `provider.go`: `TestContract` rejects SDK v2 connector types.
5. Tests: a `RunLifecycle` suite against the fake (see `codeRepositories/github_lifecycle_test.go`),
   the `resource.Test` acceptance test with `acctest.ProtoV5ProviderFactories`, and
   `TestConnectorDefinitionsConvert` in `internal/provider` already checks the conversion.
6. An example under `examples/resources/harness_platform_connector_<type>/`, `make docs`,
   and a `.changelog/` entry.

Secret reference descriptions use `helpers.Descriptions.SecretRefText.String()`.

## Testing

```sh
go test ./internal/provider -run 'TestConnectorDefinitionsConvert|TestContract'
go test -count=1 -run 'TestLifecycle_|TestMigration_' ./internal/service/cd_nextgen/connector/codeRepositories/
TF_ACC=1 go test -v ./internal/service/platform/connector/... -run TestAccResourceConnector<Type>
```

On Windows, fake-backed tests must live in a directory without subdirectories
(plugin-testing symlinks subdirectories into its working directory, which needs a privilege);
`internal/service/platform/connectors` is used for this package's migration test.

## DOs

- Keep the SDK v2 declaration purely declarative; the bridge does the rest
- Read every attribute from the API; keep import, identity and scope handling in the generic resource
- Use `helpers.Descriptions.SecretRefText` for secret reference descriptions
- Add both resource and data source declarations

## DON'Ts

- Never register a connector in `provider.go`'s SDK v2 maps
- Never add CRUD wrappers, `connectorbase` calls or `CustomizeDiff` to a new connector
- Don't hardcode account/org/project IDs - use schema values
- Don't add local .terraform or terraform.tfstate files to git
