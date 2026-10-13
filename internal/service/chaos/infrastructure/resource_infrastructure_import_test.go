package infrastructure_test

import (
	"testing"

	"github.com/harness/terraform-provider-harness/internal/acctest"
	"github.com/harness/terraform-provider-harness/internal/fakeharness"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// An infrastructure is read in its environment, so the import ID carries the environment;
// a deleted infrastructure is removed from state whatever the shape of the 404.
func TestImportRoundTrip_ChaosInfrastructure(t *testing.T) {
	acctest.RunImportRoundTrip(t, acctest.ImportRoundTrip{
		ResourceType:   "harness_chaos_infrastructure",
		Kind:           fakeharness.KindChaosInfrastructure,
		Key:            fakeharness.Key{Org: "lc_org", Project: "lc_project", ID: "lc_infra"},
		Fake:           func(f *fakeharness.Server) { f.Mount(fakeharness.ChaosInfrastructures()) },
		ImportID:       "lc_org/lc_project/lc_env/lc_infra",
		NotFoundStyles: true,
		// namespace and service_account are not refreshed from the API yet (FORK-2.8).
		ImportIgnore: []string{"namespace", "service_account"},
		Config: func(*fakeharness.Server) string {
			return acctest.ScopeParents("lc_org", "lc_project") + `
resource "harness_chaos_infrastructure" "test" {
  org_id          = harness_platform_organization.org.id
  project_id      = harness_platform_project.project.id
  identifier      = "lc_infra"
  name            = "Lifecycle infrastructure"
  description     = "lifecycle"
  environment_id  = "lc_env"
  namespace       = "litmus"
  service_account = "litmus"
}
`
		},
		Checks: []resource.TestCheckFunc{
			resource.TestCheckResourceAttr("harness_chaos_infrastructure.test", "id", "lc_infra"),
			resource.TestCheckResourceAttr("harness_chaos_infrastructure.test", "environment_id", "lc_env"),
		},
	})
}
