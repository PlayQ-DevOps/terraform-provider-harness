package codeRepositories_test

import (
	"fmt"
	"testing"

	"github.com/harness/terraform-provider-harness/internal/acctest"
	"github.com/harness/terraform-provider-harness/internal/fakeharness"
)

// The framework implementation (through the bridge) takes over connector state written by
// the SDK v2 one: the zero values the SDK stored for unset attributes, at every depth,
// become null and the plan stays empty.
func TestMigration_ConnectorGithub(t *testing.T) {
	acctest.RunMigration(t, acctest.Migration{
		ResourceType: "harness_platform_connector_github",
		Kind:         fakeharness.KindConnector,
		Scope:        acctest.ScopeProject,
		Config: func(p acctest.LifecycleParams) string {
			return fmt.Sprintf(`
resource "harness_platform_organization" "org" {
  identifier = %[3]q
  name       = %[3]q
}

resource "harness_platform_project" "project" {
  identifier = %[4]q
  name       = %[4]q
  org_id     = harness_platform_organization.org.id
}

resource "harness_platform_connector_github" "test" {
  identifier      = %[1]q
  name            = %[2]q
  org_id          = harness_platform_organization.org.id
  project_id      = harness_platform_project.project.id
  url             = "https://github.com/acme"
  connection_type = "Account"

  credentials {
    http {
      username  = "octocat"
      token_ref = "account.gh_token"
    }
  }
}
`, p.Identifier, p.Name, p.OrgID, p.ProjectID)
		},
		V0State: map[string]string{
			"description": "", "tags.#": "0", "validation_repo": "", "delegate_selectors.#": "0",
			"execute_on_delegate": "true", "api_authentication.#": "0",
			"credentials.0.http.0.username_ref": "", "credentials.0.ssh.#": "0",
		},
		// Block lists stay empty lists (Terraform gives [] for absent blocks); the attributes
		// inside blocks and the collection attributes become null.
		NullAfterUpgrade: []string{
			"description", "tags.#", "validation_repo", "delegate_selectors.#",
			"credentials.0.http.0.username_ref",
		},
	})
}
