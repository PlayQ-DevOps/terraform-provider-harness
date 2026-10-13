package idp_test

import (
	"fmt"
	"testing"

	"github.com/harness/harness-go-sdk/harness/utils"
	"github.com/harness/terraform-provider-harness/internal/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccDataSourceIdpCatalogEntity(t *testing.T) {
	id := fmt.Sprintf("%s_%s", t.Name(), utils.RandStringBytes(6))
	resourceName := "data.harness_platform_idp_catalog_entity.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.TestAccPreCheck(t) },
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccDataSourceCatalogEntity(id),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "identifier", id),
					resource.TestCheckResourceAttr(resourceName, "kind", "component"),
					resource.TestCheckResourceAttr(resourceName, "project_id", ""),
				),
			},
		},
	})
}

func testAccDataSourceCatalogEntity(id string) string {
	return fmt.Sprintf(`
		resource "harness_platform_idp_catalog_entity" "test" {
			identifier = "%[1]s"
			kind = "component"
			yaml = <<-EOT
	        apiVersion: harness.io/v1
	        kind: Component
	        name: Example Catalog
	        identifier: "%[1]s"
	        type: service
	        owner: user:account/admin@harness.io
	        spec:
	            lifecycle: prod
	        metadata:
	            tags:
		            - test
	        EOT
		}


		data "harness_platform_idp_catalog_entity" "test" {
			identifier = harness_platform_idp_catalog_entity.test.identifier
			kind       = harness_platform_idp_catalog_entity.test.kind
		}
	`, id)
}
