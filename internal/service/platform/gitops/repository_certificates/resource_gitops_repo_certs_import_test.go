package repository_certificates_test

import (
	"testing"

	"github.com/harness/terraform-provider-harness/internal/acctest"
	"github.com/harness/terraform-provider-harness/internal/fakeharness"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// An agent holds one certificate list, so the agent (in its scope) is the import ID and the
// resource ID; an agent whose list is empty is removed from state.
func TestImportRoundTrip_GitopsRepoCert(t *testing.T) {
	acctest.RunImportRoundTrip(t, acctest.ImportRoundTrip{
		ResourceType: "harness_platform_gitops_repo_cert",
		Kind:         fakeharness.KindGitopsRepoCert,
		Key:          fakeharness.Key{Org: "lc_org", Project: "lc_project", ID: "lc_agent"},
		Fake:         func(f *fakeharness.Server) { f.Mount(fakeharness.GitopsRepoCertificates()) },
		ImportID:     "lc_org/lc_project/lc_agent",
		// The request block is an input the read does not refresh yet (FORK-2.8).
		ImportIgnore: []string{"request"},
		Config: func(*fakeharness.Server) string {
			return acctest.ScopeParents("lc_org", "lc_project") + `
resource "harness_platform_gitops_repo_cert" "test" {
  agent_id   = "lc_agent"
  org_id     = harness_platform_organization.org.id
  project_id = harness_platform_project.project.id
  request {
    upsert = true
    certificates {
      items {
        server_name = "git.example.com"
        cert_type   = "https"
        cert_data   = "LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0t"
      }
    }
  }
}
`
		},
		Checks: []resource.TestCheckFunc{
			resource.TestCheckResourceAttr("harness_platform_gitops_repo_cert.test", "id", "lc_agent"),
			resource.TestCheckResourceAttr("harness_platform_gitops_repo_cert.test", "agent_id", "lc_agent"),
		},
	})
}
