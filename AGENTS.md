# AGENTS.md

## Project Overview

Terraform provider for Harness that enables infrastructure-as-code management of Harness CD and NextGen platform resources: applications, pipelines, environments, services, connectors, feature flags, GitOps configurations, AutoStopping rules and more.

The provider is served as a **mux of two halves** (`internal/provider/mux.go`): the terraform-plugin-sdk/v2 provider with the existing resources, and a terraform-plugin-framework provider with the new and migrated ones. Both halves share one provider schema and one `SessionConfig`/`NewSession` (`internal/provider/session.go`). **New resources and data sources are written on the framework**; the SDK v2 maps are frozen (see "Writing a resource").

## Build System

- **Language**: Go; the version is pinned by the `go` and `toolchain` directives in `go.mod` (single source of truth; `.tool-versions` mirrors it and a test keeps them in sync)
- **Build tool**: Make
- **Build command**: `make build`
- **Install locally**: `make install`
- **Clean**: `make clean`

## Testing

Three tiers, all driven by `go test`:

- **`make test-unit`**: offline unit, contract and lint-ratchet tests (`go test -short ./...`); needs no Terraform CLI and no credentials.
- **`make test`**: everything offline, including the fake-backed suites that drive real Terraform core against the in-process fake Harness API (`internal/fakeharness`); needs a Terraform CLI (`make tools` installs the pinned version) but no credentials. This is what CI runs.
- **`make testacc`**: acceptance tests against a live Harness account (`TF_ACC=1`, requires HARNESS_* environment variables). Acceptance tests use `resource.Test`, which skips when `TF_ACC` is unset, and `acctest.ProtoV5ProviderFactories` (the mux); never use `resource.UnitTest` for tests that reach the real API.

Fake-backed suites (`internal/acctest`):

- `acctest.RunLifecycle` runs the standard matrix for a resource: create, update, import with verify, out-of-band delete for every not-found body shape, scope and identifier changes plan a replacement, a transient 503 is retried, retry exhaustion is an error and not a crash, optional eventual-consistency and drift cells.
- `acctest.RunImportRoundTrip` proves an import ID format.
- `acctest.RunMigration` proves a resource moved from SDK v2 to the framework takes over the old state with an empty plan.
- The fake (`fakeharness.New(t)`) serves the nextgen DTO, OpenAPI, chaos and code dialects from one store; add a `Resource` definition or `Handle` a custom route when a resource needs one.

Guardrails:

- `make test-contract` runs `internal/provider/contract_test.go`: the provider-wide contract (importers, identity attributes replace, intent flags, Sensitive secrets, data-source shape, description and Optional+Computed ratchets, golden type lists, the frozen SDK v2 type list). It covers both halves. After fixing violations regenerate with `go test ./internal/provider -run TestContract -args -update`; deliberate exceptions go in `contract_allowlist_test.go` with a reason.
- `internal/contract` holds go/ast ratchets over `internal/service`: no deprecated error helpers, no manual not-found checks, reads refresh from the API, tflog only, unchecked `d.Set`, bool/int assertions on `d.Get`, and panics. Baselines may only shrink; regenerate with `CONTRACT_PRINT_BASELINE=1 go test ./internal/contract -run TestRatchets`.

Other targets:

- **Run FME (Split) acceptance tests only**: `make testacc-fme`
- **Run tests with coverage**: `make test-coverage`
- **Run one package**: `go test -v ./internal/service/platform/connector/... -run TestAccResourceConnector`
- **Run the fake-backed suites of one package**: `go test -count=1 -run 'TestLifecycle_|TestImportRoundTrip_|TestMigration_' ./internal/service/platform/project/`

### Acceptance Tests

Acceptance tests create real resources in Harness. Required environment variables: `HARNESS_ACCOUNT_ID`, `HARNESS_API_KEY`, `HARNESS_PLATFORM_API_KEY`.

## Linting & Formatting

- **Format code**: `make fmt`
- **Check formatting**: `make fmt-check`
- **Run vet**: `make vet`
- **Run linter**: `make lint` (golangci-lint; CI gates new issues only)
- **Run all checks**: `make check` (fmt-check + vet + contract)

## Git Workflow

- **Commit format**: `type: [JIRA-TICKET]: Description`
  - Types: `feat`, `fix`, `chore`, `refactor`, `docs`, `test`
  - Jira prefixes: DBOPS, CDS, CCM, IAC, PL, AH, and others; on this fork, refactor steps use `[FORK-<step>]`
  - Example: `feat: [DBOPS-2019]: Add usePercona field support in schema resource`
- **Default branch**: `main`

## Writing a resource

New resources and data sources use terraform-plugin-framework and the `internal/framework` package. `TestContract` rejects any SDK v2 type that is not in `internal/provider/testdata/sdk_v2_types.txt`, so a new type cannot be added to `provider.go`'s maps.

1. **Package**: `internal/service/<area>/<name>/`, files `<name>_resource.go` and `<name>_data_source.go`. `harness_platform_organization` and `harness_platform_project` are the reference implementations.
2. **Schema**: start from `framework.CommonResourceAttributes(level)` (id, identifier, name, description, tags, and the scope attributes for `AccountLevel`, `OrgLevel`, `ProjectLevel` or `MultiLevel`); identity attributes get `framework.RequiresReplace()`; Optional+Computed attributes get `UseStateForUnknown`; secret values are `Sensitive`; every attribute has a `MarkdownDescription`. The provider is served over protocol 5: use list/set/map attributes of object types or nested blocks, never nested attributes.
3. **Model**: embed `framework.CommonModel` by value and add the scope and type-specific fields.
4. **CRUD**: `framework.SessionFromProviderData` in Configure, `framework.RequireSession` at the top of each call, then the SDK client from the session. Interpret API errors only through `framework.ReadError` (a missing object removes the resource from state), `framework.ReadMissing`, `framework.WriteError`, `framework.DeleteError` (already gone is success) and `framework.Error` (data sources). Never read `httpResp.StatusCode` before checking `err`, never compare a status code by hand.
5. **Reads refresh from the API**: every attribute comes from the response; `framework.StringFromAPI` and `framework.TagsFromAPI` keep a never-set attribute null. An input the API does not return is documented as such and listed in the lifecycle test's `ImportIgnore`.
6. **Import and identity**: `framework.ImportState` with the matching spec (`ImportPassthrough`, `ImportOrgLevel`, `ImportProjectLevel`, `ImportMultiLevel`, or a `helpers.ImportSpec`) and `framework.IdentityAttributes` for the identity schema; set the identity in Create, Read and Import.
7. **Register** the constructor in `internal/provider/framework_provider.go` (`Resources` or `DataSources`).
8. **Tests**: a `RunLifecycle` suite against the fake (add a fake `Resource` or route if needed) plus the usual `resource.Test` acceptance test with `acctest.ProtoV5ProviderFactories`.
9. **Docs and changelog**: an example under `examples/`, `make docs`, and a `.changelog/` entry (`new-resource`, `new-data-source`, ...).

Migrating an SDK v2 resource: rename the SDK files to `legacy_*.go` (`LegacyResourceX`), register them only in `provider.LegacySDKProvider`, write the framework implementation with schema `Version: 1` and `framework.ZeroValueToNullUpgrader` (the SDK stored `""`/`0`/`false`/`[]` for unset attributes), remove the type from the SDK maps (the golden type lists do not change), and add a `RunMigration` test.

Rules for the SDK v2 code that remains (fixes only): error handling goes through `helpers.HandleRead`, `HandleDelete`, `HandleWrite`, `HandleWriteGit`, `HandleError` and `ReadMissing`; identity attributes are `ForceNew` (`helpers.MarkIdentity`); importers are built from `helpers.ImportSpec`; config-only attributes use `helpers.ConfigOnly`; booleans and integers are read with `helpers.GetBool`/`GetInt` (an explicit false or zero counts as set); logging uses `tflog`; no panics.

## DOs

- Always run `make check` before committing to verify formatting, static analysis and the provider contract
- Add changelog entries in `.changelog/` for user-facing changes (see README for format)
- Run `make test` before pushing (offline, needs a Terraform CLI)
- Follow the reference implementations and the helper packages instead of copying code
- Use descriptive commit messages with Jira ticket references
- Run `make docs` after modifying resource schemas, examples or templates to regenerate documentation; hand-written page prose lives in `templates/<kind>/<name>.md.tmpl`, examples in `examples/`
- Use `make dev-setup` when setting up the development environment for the first time

## DON'Ts

- Never force push to main/master
- Never commit secrets, .env files, or credentials
- Never run `make sweep` without explicit confirmation - it destroys test resources and can be dangerous if misconfigured
- Never skip code quality checks before committing
- Never modify generated documentation files in `docs/` directly - regenerate with `make docs`
- Never put prose into a generated page; put it in its template under `templates/` and regenerate
- Never add a resource or data source to the SDK v2 maps in `provider.go`; write it on the framework
- Never use `resource.UnitTest` for a test that reaches the real API

## Commands to Never Run

- `git push --force origin main`
- `git push --force origin master`
- `git commit --no-verify` or `git push --no-verify`
- `make sweep` (without explicit user confirmation)
- `rm -rf /` or any destructive recursive deletes

## Project Structure

```
terraform-provider-harness/
├── main.go                 # Serves the muxed provider
├── Makefile                # Build, test, and development commands
├── go.mod                  # Go module definition
├── internal/
│   ├── provider/           # SDK v2 provider (provider.go), framework provider, mux, shared session, contract tests
│   ├── framework/          # Helpers for framework resources (attributes, model, errors, import, state upgrade)
│   ├── apierr/             # Client-agnostic classification of Harness API errors
│   ├── httpclient/         # Shared HTTP client policy (retries, timeouts, redacting logs)
│   ├── fakeharness/        # In-process fake Harness API for offline tests
│   ├── acctest/            # Test kits: lifecycle, import round trip, migration, acceptance utilities
│   ├── contract/           # go/ast guardrail tests over internal/service
│   ├── service/            # Resource implementations by service
│   │   ├── cd/             # First Generation CD resources (guardrails only)
│   │   ├── cd_nextgen/     # NextGen CD resources
│   │   ├── chaos/          # Chaos Engineering resources
│   │   ├── pipeline/       # Pipeline resources
│   │   ├── platform/       # Platform resources (connectors, secrets, organization, project, ...)
│   │   └── service_discovery/  # Service discovery resources
│   ├── sweep/              # Resource sweepers for test cleanup
│   ├── test/               # Repository-wide test guards
│   └── utils/              # Utility functions
├── helpers/                # Helpers for the SDK v2 resources (schema, lifecycle, importer, tags, config)
├── docs/                   # Generated Terraform documentation
├── examples/               # Example Terraform configurations
├── templates/              # Documentation templates: index, guides, and per-page templates that keep hand-written prose
├── scripts/                # Build and utility scripts
└── .changelog/             # Changelog entries for releases
```

## Dependencies

Key dependencies (from go.mod):
- `github.com/harness/harness-go-sdk` - Harness Go SDK
- `github.com/harness/harness-openapi-go-client` - Harness OpenAPI client
- `github.com/hashicorp/terraform-plugin-framework` (+ `-validators`) - new and migrated resources
- `github.com/hashicorp/terraform-plugin-sdk/v2` - the existing resources
- `github.com/hashicorp/terraform-plugin-mux` - serves both halves as one provider
- `github.com/hashicorp/terraform-plugin-testing` - acceptance and fake-backed tests
- `github.com/stretchr/testify` - Testing utilities

## Language-Specific Guidelines

This is a Go codebase. Follow Go conventions:
- Use `gofmt` for formatting (handled by `make fmt`)
- Follow effective Go guidelines
- Use meaningful variable and function names
- Handle errors explicitly
- Write table-driven tests where appropriate
