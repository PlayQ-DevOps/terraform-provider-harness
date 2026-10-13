package project_test

import (
	"fmt"
	"testing"

	"github.com/harness/terraform-provider-harness/internal/acctest"
	"github.com/harness/terraform-provider-harness/internal/fakeharness"
)

// The framework implementation takes over state written by the SDK v2 one: the "" and
// empty tags the SDK stored for unset attributes become null, the computed color and
// modules stay, and the plan stays empty.
func TestMigration_Project(t *testing.T) {
	acctest.RunMigration(t, acctest.Migration{
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
  identifier = %[1]q
  name       = %[2]q
  org_id     = harness_platform_organization.org.id
}
`, p.Identifier, p.Name, p.OrgID)
		},
		V0State:          map[string]string{"description": "", "tags.#": "0", "color": "", "modules.#": "0"},
		NullAfterUpgrade: []string{"description", "tags.#"},
	})
}
