package codeRepositories_test

import (
	"fmt"
	"testing"

	"github.com/harness/terraform-provider-harness/internal/acctest"
	"github.com/harness/terraform-provider-harness/internal/fakeharness"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestLifecycle_ConnectorGithub(t *testing.T) {
	acctest.RunLifecycle(t, acctest.Lifecycle{
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
  validation_repo = "repo"

  credentials {
    http {
      username  = "octocat"
      token_ref = "account.gh_token"
    }
  }
}
`, p.Identifier, p.Name, p.OrgID, p.ProjectID)
		},
		Checks: func(p acctest.LifecycleParams) []resource.TestCheckFunc {
			return []resource.TestCheckFunc{
				resource.TestCheckResourceAttr("harness_platform_connector_github.test", "name", p.Name),
				resource.TestCheckResourceAttr("harness_platform_connector_github.test", "project_id", p.ProjectID),
				resource.TestCheckResourceAttr("harness_platform_connector_github.test", "credentials.0.http.0.username", "octocat"),
			}
		},
		KnownFailing: map[string]string{
			acctest.CellScopeChange: "#1347 / FORK-3.1: org_id and project_id are not ForceNew, so a scope change is planned as an in-place update that the API rejects",
		},
	})
}
