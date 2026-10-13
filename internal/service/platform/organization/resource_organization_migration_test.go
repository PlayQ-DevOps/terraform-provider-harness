package organization_test

import (
	"fmt"
	"testing"

	"github.com/harness/terraform-provider-harness/internal/acctest"
	"github.com/harness/terraform-provider-harness/internal/fakeharness"
)

// The framework implementation takes over state written by the SDK v2 one: the "" and
// empty tags the SDK stored for unset attributes become null, and the plan stays empty.
func TestMigration_Organization(t *testing.T) {
	acctest.RunMigration(t, acctest.Migration{
		ResourceType: "harness_platform_organization",
		Kind:         fakeharness.KindOrganization,
		Scope:        acctest.ScopeAccount,
		Config: func(p acctest.LifecycleParams) string {
			return fmt.Sprintf(`
resource "harness_platform_organization" "test" {
  identifier = %q
  name       = %q
}
`, p.Identifier, p.Name)
		},
		V0State:          map[string]string{"description": "", "tags.#": "0"},
		NullAfterUpgrade: []string{"description", "tags.#"},
	})
}
