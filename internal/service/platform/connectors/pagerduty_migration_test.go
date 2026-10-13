package connectors_test

import (
	"fmt"
	"testing"

	"github.com/harness/terraform-provider-harness/internal/acctest"
	"github.com/harness/terraform-provider-harness/internal/fakeharness"
)

// The framework implementation (through the bridge) takes over PagerDuty connector state
// written by the SDK v2 one with an empty plan. The test lives here rather than in the
// platform connector package because plugin-testing symlinks a test directory's
// subdirectories into its working directory, which needs a privilege on Windows.
func TestMigration_ConnectorPagerDuty(t *testing.T) {
	acctest.RunMigration(t, acctest.Migration{
		ResourceType: "harness_platform_connector_pagerduty",
		Kind:         fakeharness.KindConnector,
		Scope:        acctest.ScopeAccount,
		Config: func(p acctest.LifecycleParams) string {
			return fmt.Sprintf(`
resource "harness_platform_connector_pagerduty" "test" {
  identifier    = %[1]q
  name          = %[2]q
  api_token_ref = "account.pd_token"
}
`, p.Identifier, p.Name)
		},
		V0State:          map[string]string{"description": "", "tags.#": "0", "delegate_selectors.#": "0"},
		NullAfterUpgrade: []string{"description", "tags.#", "delegate_selectors.#"},
	})
}
