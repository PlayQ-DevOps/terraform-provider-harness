package idp_test

import (
	"fmt"
	"testing"

	"github.com/harness/terraform-provider-harness/internal/acctest"
	"github.com/harness/terraform-provider-harness/internal/fakeharness"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// The IDP catalog indexes writes asynchronously, so an entity can answer 404 for a few
// seconds after its create returned (#1385). EventuallyConsistentReads proves the resource
// converges instead of failing with "inconsistent result after apply".
func TestLifecycle_IDPCatalogEntity(t *testing.T) {
	acctest.RunLifecycle(t, acctest.Lifecycle{
		ResourceType: "harness_platform_idp_catalog_entity",
		Kind:         fakeharness.KindIDPEntity,
		Scope:        acctest.ScopeProject,
		Fake:         func(f *fakeharness.Server) { f.Mount(fakeharness.IDPEntities()) },
		ImportID: func(p acctest.LifecycleParams) string {
			return p.OrgID + "/" + p.ProjectID + "/component/" + p.Identifier
		},
		EventuallyConsistentReads: 2,
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

resource "harness_platform_idp_catalog_entity" "test" {
  identifier = %[1]q
  kind       = "component"
  org_id     = harness_platform_organization.org.id
  project_id = harness_platform_project.project.id
  yaml       = <<-EOT
    apiVersion: harness.io/v1
    kind: Component
    name: %[2]s
    identifier: %[1]s
    orgIdentifier: %[3]s
    projectIdentifier: %[4]s
    type: service
    owner: user:account/admin@example.com
    spec:
      lifecycle: prod
  EOT
}
`, p.Identifier, p.Name, p.OrgID, p.ProjectID)
		},
		Checks: func(p acctest.LifecycleParams) []resource.TestCheckFunc {
			return []resource.TestCheckFunc{
				resource.TestCheckResourceAttr("harness_platform_idp_catalog_entity.test", "kind", "component"),
				resource.TestCheckResourceAttr("harness_platform_idp_catalog_entity.test", "org_id", p.OrgID),
				resource.TestCheckResourceAttr("harness_platform_idp_catalog_entity.test", "project_id", p.ProjectID),
			}
		},
		KnownFailing: map[string]string{
			acctest.CellScopeChange: "#1347: org_id and project_id are planned as in-place updates until FORK-3.1 marks them ForceNew",
		},
	})
}
