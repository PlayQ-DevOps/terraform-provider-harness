package cloudProviders_test

import (
	"fmt"
	"testing"

	"github.com/harness/terraform-provider-harness/internal/acctest"
	"github.com/harness/terraform-provider-harness/internal/fakeharness"
)

// The framework implementation (through the bridge) takes over AWS connector state written
// by the SDK v2 one with an empty plan; zero values inside the manual block become null.
func TestMigration_ConnectorAws(t *testing.T) {
	acctest.RunMigration(t, acctest.Migration{
		ResourceType: "harness_platform_connector_aws",
		Kind:         fakeharness.KindConnector,
		Scope:        acctest.ScopeAccount,
		Config: func(p acctest.LifecycleParams) string {
			return fmt.Sprintf(`
resource "harness_platform_connector_aws" "test" {
  identifier = %[1]q
  name       = %[2]q

  manual {
    access_key_ref = "account.access_id"
    secret_key_ref = "account.secret_id"
  }
}
`, p.Identifier, p.Name)
		},
		V0State:          map[string]string{"description": "", "tags.#": "0", "manual.0.access_key": "", "manual.0.delegate_selectors.#": "0"},
		NullAfterUpgrade: []string{"description", "tags.#", "manual.0.access_key", "manual.0.delegate_selectors.#"},
	})
}
