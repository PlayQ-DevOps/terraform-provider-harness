package cluster_test

import (
	"testing"

	"github.com/harness/terraform-provider-harness/internal/acctest"
	"github.com/harness/terraform-provider-harness/internal/fakeharness"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// The mapping is identified by its environment: the ID and import ID are
// "<org>/<project>/<env>", and an environment with no linked cluster left is removed from
// state. The identifier attribute has no meaning for the API and cannot be imported.
func TestImportRoundTrip_EnvironmentClustersMapping(t *testing.T) {
	acctest.RunImportRoundTrip(t, acctest.ImportRoundTrip{
		ResourceType: "harness_platform_environment_clusters_mapping",
		Kind:         fakeharness.KindEnvironmentClusters,
		Key:          fakeharness.Key{Org: "lc_org", Project: "lc_project", ID: "lc_env"},
		Fake:         func(f *fakeharness.Server) { f.Mount(fakeharness.EnvironmentClusters()) },
		ImportID:     "lc_org/lc_project/lc_env",
		ImportIgnore: []string{"identifier"},
		Config: func(*fakeharness.Server) string {
			return acctest.ScopeParents("lc_org", "lc_project") + `
resource "harness_platform_environment_clusters_mapping" "test" {
  identifier = "lc_mapping"
  org_id     = harness_platform_organization.org.id
  project_id = harness_platform_project.project.id
  env_id     = "lc_env"
  clusters {
    identifier       = "lc_cluster"
    name             = "lc_cluster"
    agent_identifier = "lc_agent"
    scope            = "PROJECT"
  }
}
`
		},
		Checks: []resource.TestCheckFunc{
			resource.TestCheckResourceAttr("harness_platform_environment_clusters_mapping.test", "id", "lc_org/lc_project/lc_env"),
			resource.TestCheckResourceAttr("harness_platform_environment_clusters_mapping.test", "clusters.#", "1"),
		},
	})
}
