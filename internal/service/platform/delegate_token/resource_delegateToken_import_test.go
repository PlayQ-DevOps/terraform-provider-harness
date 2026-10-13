package delegatetoken_test

import (
	"fmt"
	"testing"

	"github.com/harness/terraform-provider-harness/internal/acctest"
	"github.com/harness/terraform-provider-harness/internal/fakeharness"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// The token name is the resource ID: the importer must set it (there is no identifier
// attribute) and the read must look the token up by that name; a token that is no longer
// listed is removed from state.
func TestImportRoundTrip_DelegateToken(t *testing.T) {
	acctest.RunImportRoundTrip(t, acctest.ImportRoundTrip{
		ResourceType: "harness_platform_delegatetoken",
		Kind:         fakeharness.KindDelegateToken,
		Key:          fakeharness.Key{Org: "lc_org", Project: "lc_project", ID: "lc_token"},
		Fake:         func(f *fakeharness.Server) { f.Mount(fakeharness.DelegateTokens()) },
		ImportID:     "lc_org/lc_project/lc_token",
		Config: func(f *fakeharness.Server) string {
			return acctest.ScopeParents("lc_org", "lc_project") + fmt.Sprintf(`
resource "harness_platform_delegatetoken" "test" {
  name       = "lc_token"
  account_id = %q
  org_id     = harness_platform_organization.org.id
  project_id = harness_platform_project.project.id
}
`, f.Account())
		},
		Checks: []resource.TestCheckFunc{
			resource.TestCheckResourceAttr("harness_platform_delegatetoken.test", "id", "lc_token"),
			resource.TestCheckResourceAttr("harness_platform_delegatetoken.test", "token_status", "ACTIVE"),
			resource.TestCheckResourceAttrSet("harness_platform_delegatetoken.test", "value"),
		},
	})
}
