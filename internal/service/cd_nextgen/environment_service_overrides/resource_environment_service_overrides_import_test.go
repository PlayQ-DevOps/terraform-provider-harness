package environment_service_overrides_test

import (
	"testing"

	"github.com/harness/terraform-provider-harness/internal/acctest"
	"github.com/harness/terraform-provider-harness/internal/fakeharness"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// An override is identified by its service and environment; the import ID carries both,
// and a read whose list no longer contains the override removes it from state.
func TestImportRoundTrip_EnvironmentServiceOverrides(t *testing.T) {
	acctest.RunImportRoundTrip(t, acctest.ImportRoundTrip{
		ResourceType: "harness_platform_environment_service_overrides",
		Kind:         fakeharness.KindServiceOverride,
		Key:          fakeharness.ServiceOverrideKey("lc_org", "lc_project", "lc_env", "lc_service"),
		Fake:         func(f *fakeharness.Server) { f.Mount(fakeharness.ServiceOverrides()) },
		ImportID:     "lc_org/lc_project/lc_env/lc_service",
		Config: func(*fakeharness.Server) string {
			return acctest.ScopeParents("lc_org", "lc_project") + `
resource "harness_platform_environment_service_overrides" "test" {
  org_id     = harness_platform_organization.org.id
  project_id = harness_platform_project.project.id
  env_id     = "lc_env"
  service_id = "lc_service"
  yaml       = <<-EOT
    serviceOverrides:
      environmentRef: lc_env
      serviceRef: lc_service
      variables:
        - name: region
          type: String
          value: eu-west-1
  EOT
}
`
		},
		Checks: []resource.TestCheckFunc{
			resource.TestCheckResourceAttr("harness_platform_environment_service_overrides.test", "id", "lc_service-lc_env"),
			resource.TestCheckResourceAttr("harness_platform_environment_service_overrides.test", "identifier", "lc_service-lc_env"),
		},
	})
}
