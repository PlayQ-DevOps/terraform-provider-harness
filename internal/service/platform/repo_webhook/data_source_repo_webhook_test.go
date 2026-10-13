package repo_webhook_test

import (
	"testing"

	"github.com/harness/terraform-provider-harness/internal/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccDataSourceProjectRepoWebhook(t *testing.T) {
	identifier := identifier(t.Name())

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.TestAccPreCheck(t) },
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testProjResourceRepoWebhook(identifier, description),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "identifier", identifier),
					resource.TestCheckResourceAttr(resourceName, "description", description),
				),
			},
		},
	})
}
