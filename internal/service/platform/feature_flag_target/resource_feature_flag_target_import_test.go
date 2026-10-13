package feature_flag_target_test

import (
	"fmt"
	"testing"

	"github.com/harness/terraform-provider-harness/internal/acctest"
	"github.com/harness/terraform-provider-harness/internal/fakeharness"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// A target lives in an environment, so the import ID carries it; a deleted target is
// removed from state whatever the shape of the 404.
func TestImportRoundTrip_FeatureFlagTarget(t *testing.T) {
	acctest.RunImportRoundTrip(t, acctest.ImportRoundTrip{
		ResourceType:   "harness_platform_feature_flag_target",
		Kind:           fakeharness.KindFeatureFlagTarget,
		Key:            fakeharness.CFKey("lc_org", "lc_project", "lc_env", "lc_target"),
		Fake:           func(f *fakeharness.Server) { f.Mount(fakeharness.FeatureFlagTargets()) },
		ImportID:       "lc_org/lc_project/lc_env/lc_target",
		NotFoundStyles: true,
		Config: func(f *fakeharness.Server) string {
			return acctest.ScopeParents("lc_org", "lc_project") + fmt.Sprintf(`
resource "harness_platform_feature_flag_target" "test" {
  identifier  = "lc_target"
  name        = "Lifecycle target"
  org_id      = harness_platform_organization.org.id
  project_id  = harness_platform_project.project.id
  environment = "lc_env"
  account_id  = %q
}
`, f.Account())
		},
		Checks: []resource.TestCheckFunc{
			resource.TestCheckResourceAttr("harness_platform_feature_flag_target.test", "name", "Lifecycle target"),
			resource.TestCheckResourceAttr("harness_platform_feature_flag_target.test", "environment", "lc_env"),
		},
	})
}
