package featureflagtargetgroup_test

import (
	"fmt"
	"testing"

	"github.com/harness/terraform-provider-harness/internal/acctest"
	"github.com/harness/terraform-provider-harness/internal/fakeharness"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// A target group lives in an environment, so the import ID carries it; a deleted group is
// removed from state whatever the shape of the 404.
func TestImportRoundTrip_FeatureFlagTargetGroup(t *testing.T) {
	acctest.RunImportRoundTrip(t, acctest.ImportRoundTrip{
		ResourceType:   "harness_platform_feature_flag_target_group",
		Kind:           fakeharness.KindFeatureFlagTargetGroup,
		Key:            fakeharness.CFKey("lc_org", "lc_project", "lc_env", "lc_group"),
		Fake:           func(f *fakeharness.Server) { f.Mount(fakeharness.FeatureFlagTargetGroups()) },
		ImportID:       "lc_org/lc_project/lc_env/lc_group",
		NotFoundStyles: true,
		Config: func(f *fakeharness.Server) string {
			return acctest.ScopeParents("lc_org", "lc_project") + fmt.Sprintf(`
resource "harness_platform_feature_flag_target_group" "test" {
  identifier  = "lc_group"
  name        = "Lifecycle group"
  org_id      = harness_platform_organization.org.id
  project_id  = harness_platform_project.project.id
  environment = "lc_env"
  account_id  = %q
  included    = ["lc_target"]
}
`, f.Account())
		},
		Checks: []resource.TestCheckFunc{
			resource.TestCheckResourceAttr("harness_platform_feature_flag_target_group.test", "name", "Lifecycle group"),
			resource.TestCheckResourceAttr("harness_platform_feature_flag_target_group.test", "included.0", "lc_target"),
		},
	})
}
