package pipeline_test

import (
	"fmt"
	"testing"

	"github.com/harness/terraform-provider-harness/internal/acctest"
	"github.com/harness/terraform-provider-harness/internal/fakeharness"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestLifecycle_Pipeline(t *testing.T) {
	acctest.RunLifecycle(t, acctest.Lifecycle{
		ResourceType: "harness_platform_pipeline",
		Kind:         fakeharness.KindPipeline,
		Drift:        func(obj map[string]any) { obj["description"] = "changed outside Terraform" },
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

resource "harness_platform_pipeline" "test" {
  identifier = %[1]q
  name       = %[2]q
  org_id     = harness_platform_organization.org.id
  project_id = harness_platform_project.project.id
  yaml       = <<-EOT
    pipeline:
      name: %[2]s
      identifier: %[1]s
      orgIdentifier: %[3]s
      projectIdentifier: %[4]s
      stages: []
  EOT
}
`, p.Identifier, p.Name, p.OrgID, p.ProjectID)
		},
		Checks: func(p acctest.LifecycleParams) []resource.TestCheckFunc {
			return []resource.TestCheckFunc{
				resource.TestCheckResourceAttr("harness_platform_pipeline.test", "name", p.Name),
				resource.TestCheckResourceAttrSet("harness_platform_pipeline.test", "yaml"),
			}
		},
	})
}
