package organization_test

import (
	"fmt"
	"testing"

	"github.com/harness/terraform-provider-harness/internal/acctest"
	"github.com/harness/terraform-provider-harness/internal/fakeharness"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestLifecycle_Organization(t *testing.T) {
	acctest.RunLifecycle(t, acctest.Lifecycle{
		ResourceType: "harness_platform_organization",
		Kind:         fakeharness.KindOrganization,
		Drift:        func(obj map[string]any) { obj["description"] = "changed outside Terraform" },
		Scope:        acctest.ScopeAccount,
		Config: func(p acctest.LifecycleParams) string {
			return fmt.Sprintf(`
resource "harness_platform_organization" "test" {
  identifier  = %q
  name        = %q
  description = "lifecycle test"
  tags        = ["team:platform", "url:https://example.com/x"]
}
`, p.Identifier, p.Name)
		},
		Checks: func(p acctest.LifecycleParams) []resource.TestCheckFunc {
			return []resource.TestCheckFunc{
				resource.TestCheckResourceAttr("harness_platform_organization.test", "name", p.Name),
				resource.TestCheckResourceAttr("harness_platform_organization.test", "tags.#", "2"),
			}
		},
	})
}
