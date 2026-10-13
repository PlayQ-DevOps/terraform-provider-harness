package environment_test

import (
	"fmt"
	"testing"

	"github.com/harness/terraform-provider-harness/internal/acctest"
	"github.com/harness/terraform-provider-harness/internal/fakeharness"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// The drift cell changes the description out of band: the Read must take it from the API
// (it used to be set only when the API value was non-empty), so the plan shows the change.
func TestLifecycle_Environment(t *testing.T) {
	acctest.RunLifecycle(t, acctest.Lifecycle{
		ResourceType: "harness_platform_environment",
		Kind:         fakeharness.KindEnvironment,
		Scope:        acctest.ScopeProject,
		Fake:         func(f *fakeharness.Server) { f.Mount(fakeharness.EnvironmentsV2()) },
		Config: func(p acctest.LifecycleParams) string {
			return acctest.ScopeParents(p.OrgID, p.ProjectID) + fmt.Sprintf(`
resource "harness_platform_environment" "test" {
  identifier  = %[1]q
  name        = %[2]q
  description = "lifecycle"
  org_id      = harness_platform_organization.org.id
  project_id  = harness_platform_project.project.id
  type        = "PreProduction"
}
`, p.Identifier, p.Name)
		},
		Checks: func(p acctest.LifecycleParams) []resource.TestCheckFunc {
			return []resource.TestCheckFunc{
				resource.TestCheckResourceAttr("harness_platform_environment.test", "name", p.Name),
				resource.TestCheckResourceAttr("harness_platform_environment.test", "description", "lifecycle"),
				resource.TestCheckResourceAttr("harness_platform_environment.test", "type", "PreProduction"),
			}
		},
		Drift: func(obj map[string]any) { obj["description"] = "changed outside Terraform" },
	})
}
