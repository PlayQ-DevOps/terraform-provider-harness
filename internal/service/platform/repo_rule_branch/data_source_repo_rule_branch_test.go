package repo_rule_branch_test

import (
	"testing"

	"github.com/harness/terraform-provider-harness/internal/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccDataSourceProjectRepoRule(t *testing.T) {
	identifier := identifier(t.Name())

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.TestAccPreCheck(t) },
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testProjResourceRepoRule(identifier, description),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "identifier", "rule_"+identifier),
					resource.TestCheckResourceAttr(resourceName, "description", description),
				),
			},
		},
	})
}
