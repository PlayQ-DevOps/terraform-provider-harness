package project_test

import (
	"fmt"
	"testing"

	"github.com/harness/terraform-provider-harness/internal/acctest"
	"github.com/harness/terraform-provider-harness/internal/fakeharness"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestLifecycle_Project(t *testing.T) {
	acctest.RunLifecycle(t, acctest.Lifecycle{
		ResourceType: "harness_platform_project",
		Kind:         fakeharness.KindProject,
		Scope:        acctest.ScopeOrg,
		Config: func(p acctest.LifecycleParams) string {
			return fmt.Sprintf(`
resource "harness_platform_organization" "org" {
  identifier = %[3]q
  name       = %[3]q
}

resource "harness_platform_project" "test" {
  identifier  = %[1]q
  name        = %[2]q
  org_id      = harness_platform_organization.org.id
  color       = "#0063F7"
  description = "lifecycle test"
}
`, p.Identifier, p.Name, p.OrgID)
		},
		Checks: func(p acctest.LifecycleParams) []resource.TestCheckFunc {
			return []resource.TestCheckFunc{
				resource.TestCheckResourceAttr("harness_platform_project.test", "name", p.Name),
				resource.TestCheckResourceAttr("harness_platform_project.test", "org_id", p.OrgID),
			}
		},
	})
}
