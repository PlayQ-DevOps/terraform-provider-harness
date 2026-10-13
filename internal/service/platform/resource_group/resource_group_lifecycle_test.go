package resource_group_test

import (
	"fmt"
	"testing"

	"github.com/harness/terraform-provider-harness/internal/acctest"
	"github.com/harness/terraform-provider-harness/internal/fakeharness"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// The configuration leaves included_scopes out: the API defaults it to the group's own
// scope, and the second plan after create must be empty (#989).
func TestLifecycle_ResourceGroup(t *testing.T) {
	acctest.RunLifecycle(t, acctest.Lifecycle{
		ResourceType: "harness_platform_resource_group",
		Kind:         fakeharness.KindResourceGroup,
		Scope:        acctest.ScopeProject,
		Fake:         func(f *fakeharness.Server) { f.Mount(fakeharness.ResourceGroups()) },
		Config: func(p acctest.LifecycleParams) string {
			return acctest.ScopeParents(p.OrgID, p.ProjectID) + fmt.Sprintf(`
resource "harness_platform_resource_group" "test" {
  identifier  = %[1]q
  name        = %[2]q
  description = "lifecycle"
  account_id  = "acct"
  org_id      = harness_platform_organization.org.id
  project_id  = harness_platform_project.project.id
  resource_filter {
    include_all_resources = true
  }
}
`, p.Identifier, p.Name)
		},
		Checks: func(p acctest.LifecycleParams) []resource.TestCheckFunc {
			return []resource.TestCheckFunc{
				resource.TestCheckResourceAttr("harness_platform_resource_group.test", "name", p.Name),
				resource.TestCheckResourceAttr("harness_platform_resource_group.test", "included_scopes.#", "1"),
				resource.TestCheckResourceAttr("harness_platform_resource_group.test", "included_scopes.0.filter", "EXCLUDING_CHILD_SCOPES"),
				resource.TestCheckResourceAttr("harness_platform_resource_group.test", "included_scopes.0.project_id", p.ProjectID),
				resource.TestCheckResourceAttr("harness_platform_resource_group.test", "resource_filter.0.include_all_resources", "true"),
			}
		},
		Drift: func(obj map[string]any) { obj["description"] = "changed outside Terraform" },
	})
}
