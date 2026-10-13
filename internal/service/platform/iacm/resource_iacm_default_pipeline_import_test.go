package iacm_test

import (
	"testing"

	"github.com/harness/terraform-provider-harness/internal/acctest"
	"github.com/harness/terraform-provider-harness/internal/fakeharness"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// A default pipeline is identified by its provisioner and operation; the import ID carries
// both, and a default that is no longer listed is removed from state.
func TestImportRoundTrip_IacmDefaultPipeline(t *testing.T) {
	acctest.RunImportRoundTrip(t, acctest.ImportRoundTrip{
		ResourceType: "harness_platform_iacm_default_pipeline",
		Kind:         fakeharness.KindIacmDefaultPipeline,
		Key:          fakeharness.Key{Org: "lc_org", Project: "lc_project", ID: "terraform-plan"},
		Fake:         func(f *fakeharness.Server) { f.Mount(fakeharness.IacmDefaultPipelines()) },
		ImportID:     "lc_org/lc_project/terraform/plan",
		Config: func(*fakeharness.Server) string {
			return acctest.ScopeParents("lc_org", "lc_project") + `
resource "harness_platform_iacm_default_pipeline" "test" {
  org_id           = harness_platform_organization.org.id
  project_id       = harness_platform_project.project.id
  provisioner_type = "terraform"
  operation        = "plan"
  pipeline         = "lc_pipeline"
}
`
		},
		Checks: []resource.TestCheckFunc{
			resource.TestCheckResourceAttr("harness_platform_iacm_default_pipeline.test", "id", "terraform-plan"),
			resource.TestCheckResourceAttr("harness_platform_iacm_default_pipeline.test", "pipeline", "lc_pipeline"),
		},
	})
}
