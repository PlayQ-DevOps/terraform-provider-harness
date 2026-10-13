package secretManagers_test

import (
	"fmt"
	"testing"

	"github.com/harness/terraform-provider-harness/internal/acctest"
	"github.com/harness/terraform-provider-harness/internal/fakeharness"
)

// The framework implementation (through the bridge) takes over GCP Secret Manager connector
// state written by the SDK v2 one with an empty plan.
func TestMigration_ConnectorGcpSecretManager(t *testing.T) {
	acctest.RunMigration(t, acctest.Migration{
		ResourceType: "harness_platform_connector_gcp_secret_manager",
		Kind:         fakeharness.KindConnector,
		Scope:        acctest.ScopeAccount,
		Config: func(p acctest.LifecycleParams) string {
			return fmt.Sprintf(`
resource "harness_platform_connector_gcp_secret_manager" "test" {
  identifier      = %[1]q
  name            = %[2]q
  credentials_ref = "account.gcp_sa"
}
`, p.Identifier, p.Name)
		},
		V0State:          map[string]string{"description": "", "tags.#": "0", "delegate_selectors.#": "0"},
		NullAfterUpgrade: []string{"description", "tags.#", "delegate_selectors.#"},
	})
}
