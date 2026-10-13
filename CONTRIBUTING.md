# Contributing

This guide covers setting up a development environment, the test tiers, how to write the
offline tests that back a resource, how resources are migrated to the plugin framework, and
the changelog and documentation rules. `AGENTS.md` is the short version for automated agents.

## Setup

The Go version is pinned by the `go` and `toolchain` directives in `go.mod` (`.tool-versions`
mirrors it; a test keeps them in sync). Any Go of at least that version downloads the pinned
toolchain on first use.

```sh
make dev-setup   # go mod download, then make tools
make tools       # installs the pinned Terraform CLI into $(go env GOPATH)/bin via hc-install, and golangci-lint
```

`tfplugindocs` and `changelog-build` are `tool` dependencies in `go.mod`; run them with
`go tool tfplugindocs` and `go tool changelog-build`.

### Windows

- Use Git Bash for the `make` targets and scripts (`winget install ezwinports.make` provides GNU make).
- Point the test harness at the Terraform binary explicitly when the Makefile is not involved:
  `export TF_ACC_TERRAFORM_PATH="$HOME/go/bin/terraform.exe"`.
- Set `CHECKPOINT_DISABLE=1` so Terraform does not call home during tests.
- The test harness (terraform-plugin-testing) symlinks the subdirectories of a test's package
  directory into its working directory, which needs the symlink privilege on Windows. Put
  fake-backed tests in packages whose directory has no subdirectories (for example the
  `internal/service/platform/connectors` package rather than `internal/service/platform/connector`).
- OpenTofu works too: `TF_ACC_TERRAFORM_PATH=$(which tofu)`, `TF_ACC_PROVIDER_HOST=registry.opentofu.org`,
  `TF_ACC_PROVIDER_NAMESPACE=hashicorp`.

## Test tiers

| Target | What runs | Needs |
|---|---|---|
| `make test-unit` | `go test -short ./...`: unit tests, the provider contract, the go/ast ratchets | nothing |
| `make test` | everything offline, including the fake-backed suites that drive real Terraform core against the in-process fake Harness API | a Terraform CLI |
| `make test-contract` | `internal/provider` contract tests only | nothing |
| `make test-lifecycle` | the fake-backed lifecycle suites only | a Terraform CLI |
| `make testacc` | acceptance tests against a live account (`TF_ACC=1`) | `HARNESS_ACCOUNT_ID`, `HARNESS_API_KEY`, `HARNESS_PLATFORM_API_KEY` |
| `make check` | `fmt-check`, `vet`, `test-contract` | nothing |
| `make lint` | golangci-lint (CI gates new issues only, `--new-from-rev`) | golangci-lint |

Acceptance tests use `resource.Test`, which skips when `TF_ACC` is unset, and
`acctest.ProtoV5ProviderFactories`, which serves the muxed provider. `resource.UnitTest` is
reserved for fake-backed tests (`internal/test` enforces it).

## Guardrails

Two sets of tests keep the provider honest; both only ratchet down.

- `internal/provider/contract_test.go` checks every resource and data source of both halves of the
  provider: importers present and panic-free, identity attributes replace on change, intent flags
  are plain booleans, secret values are Sensitive, data sources have a data-source shape,
  descriptions and Optional+Computed counts, the golden type lists, and the frozen list of SDK v2
  types (`testdata/sdk_v2_types.txt`). Fix a violation, or add it to `contract_allowlist_test.go`
  with a reason. Regenerate the baseline and golden lists after fixing with
  `go test ./internal/provider -run TestContract -args -update`.
- `internal/contract` parses `internal/service` with go/ast: no deprecated error helpers, no manual
  not-found checks, reads refresh from the API, `tflog` only, and per-package counts of unchecked
  `d.Set`, bool/int assertions on `d.Get` and `panic` calls. Regenerate the counts with
  `CONTRACT_PRINT_BASELINE=1 go test ./internal/contract -run TestRatchets` and paste them into
  `ratchet_baseline_test.go`.

## Writing a fake-backed test

`internal/fakeharness` is an in-process Harness API: an httptest server under `/gateway` with a
store keyed by kind, account, org, project and id. `fakeharness.New(t)` starts one; its
`ProviderHCL()` renders the provider block that points the provider at it.

1. **Serve the API.** Most resources are served by a `fakeharness.Resource` definition
   (`internal/fakeharness/resource.go`): the `Kind`, the `Dialect` (`DialectNextgenDTO` for the
   harness-go-sdk nextgen style, `DialectOpenAPIV1` for the openapi client style), the `BasePath`,
   the `BodyKey`, where the scope travels (`ScopeSpec`), and knobs such as `UpdateHasID`,
   `RequireParents`, `Decorate` and `Validate`. Add a constructor next to `Organizations()` and
   mount it in the server's defaults. An API that does not follow the generic shape adds
   `Routes` (or uses `RoutesOnly`) or registers handlers with `Server.Handle`.
2. **Run the lifecycle matrix.** `acctest.RunLifecycle(t, acctest.Lifecycle{...})` takes the
   resource type, its kind and scope, a `Config` function and optional checks, and runs the
   standard cells: create, update and import with verify; out-of-band deletion for every
   not-found body shape (`disappears`, `not_found_styles/*`); scope and identifier changes plan a
   replacement and are a DELETE then a POST on the wire; a transient 503 is retried; retry
   exhaustion is an error, not a crash; plus `eventually_consistent` and `drift_detected` when
   enabled. `KnownFailing` skips a cell while a fix is tracked and fails once the cell passes.
3. **Prove an import ID format** with `acctest.RunImportRoundTrip`, and **prove a migration**
   with `acctest.RunMigration` (below).

Scenario knobs on the server: `FailNext`, `FailAlways`, `DropConnection`, `SetNotFoundStyle`,
`EventuallyConsistent`, `GoneAfter`, `Vanish`; the recorder exposes `Requests`, `Mark`, `Since`,
`Count`, `MutatingMethods`, `AssertNoRequest` and `AssertSequence`.

## Framework resources and migrations

The provider is a mux of the SDK v2 provider and a terraform-plugin-framework provider
(`internal/provider/mux.go`); both share one `SessionConfig` and `NewSession`. New resources and
data sources are written on the framework with the `internal/framework` helpers and registered in
`internal/provider/framework_provider.go` (see "Writing a resource" in `AGENTS.md`). The SDK v2
maps in `provider.go` are frozen: `TestContract` rejects a type that is not in
`internal/provider/testdata/sdk_v2_types.txt`.

Connectors are served by one generic framework resource (`internal/service/platform/connectors`)
from a `Definition` per type; the SDK v2 schema declaration stays as the source and
`internal/framework/bridge` converts it and runs the build and read functions. See the connector
package's `AGENTS.md`.

To migrate an SDK v2 resource:

1. Rename the SDK files to `legacy_*.go` and the constructors to `LegacyResourceX` /
   `LegacyDataSourceX`; register them in `provider.LegacySDKProvider` only.
2. Write the framework implementation with schema `Version: 1` and
   `framework.ZeroValueToNullUpgrader` (or `bridge.StateUpgrader` for nested schemas).
3. Remove the type from the SDK v2 maps; the golden type lists do not change.
4. Add a `RunMigration` test: it applies the configuration with the legacy provider, checks the v0
   state shape, plans and applies with the mux expecting an empty plan, and imports.

State compatibility between the two implementations:

| SDK v2 | Framework |
|---|---|
| `TypeList` MaxItems 1 of `*Resource` | `ListNestedBlock` with `SizeAtMost(1)` (never `SingleNested*`) |
| `TypeSet` of `*Resource` | `SetNestedBlock` |
| `TypeSet`/`TypeList` of string | `SetAttribute`/`ListAttribute` |
| `TypeMap` | `MapAttribute` |
| Optional + Default | Optional+Computed + static default |
| Optional+Computed | Optional+Computed + `UseStateForUnknown` |
| Optional primitive, no default | Optional, with the state upgrader (SDK state holds `""`/`0`/`false`) |
| implicit `id` | explicit `id` Computed + `UseStateForUnknown` |
| `ExactlyOneOf`/`ConflictsWith`/`StringInSlice` | framework validators |
| nested attributes | not available: the provider is served over protocol 5 |

Absent blocks are empty lists, never null; unset Optional attributes are null.

## Changelog

Every user-facing change needs a file in `.changelog/` named after the ticket, PR number or fork
step (`CDS-1234.txt`, `1532.txt`, `FORK-3.4.txt`). `make changelog-entry ENTRY=<name> TYPE=<type>`
scaffolds one. Each entry is one or more fenced blocks whose first line is
` ```release-note:<type> ` with a type from `scripts/check-changelog.sh` (`breaking-change`,
`note`, `feature`, `new-resource`, `new-data-source`, `new-guide`, `new-ephemeral-resource`,
`enhancement`, `bug`). `make changelog-check` validates the entries changed on your branch;
`make changelog-check BASE=--all` validates every entry. CI runs the check on pull requests unless
the `no-changelog` label is set.

## Documentation

`docs/` is generated: `make docs` regenerates the pages whose inputs changed and `make docs-all`
regenerates everything. The inputs are the schema descriptions, `examples/<kind>/<type>/`
(`resource.tf`, `data-source.tf`, `import.sh`) and `templates/`. Hand-written prose belongs in a
template, `templates/<kind>/<name>.md.tmpl`, around the generated description, example, schema
and import sections; never edit a page under `docs/resources` or `docs/data-sources` directly.
`make docs-validate` runs the Registry validation; it currently reports
`platform_gitops_applicationset.md`, whose nested schema renders above the Registry size limit.

## Commits and pull requests

- Commit format: `type: [TICKET]: Description` (`feat`, `fix`, `chore`, `refactor`, `docs`, `test`;
  fork steps use `[FORK-<step>]`).
- Run `make check` and `make test` before pushing. CI runs build, vet, formatting, lint on new
  issues, the unit tests on Linux and Windows, the offline suites on Linux, the changelog
  check and the docs job.
- Never force-push `main`, never commit credentials, never run `make sweep` without confirmation.
