package provider_test

import (
	"fmt"
	"testing"

	"github.com/harness/harness-go-sdk/harness/utils"
	"github.com/harness/terraform-provider-harness/internal/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestDataSourceProvider(t *testing.T) {

	id := fmt.Sprintf("%s_%s", t.Name(), utils.RandStringBytes(6))
	name := id
	resourceName := "data.harness_platform_provider.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.TestAccPreCheck(t) },
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccDataSourceProvider(id, name),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "identifier", id),
				),
			},
		},
	})
}

func testAccDataSourceProvider(id string, name string) string {
	return fmt.Sprintf(`
		resource "harness_platform_provider" "test" {
			identifier = "%[1]s"
			name = "%[2]s"
			spec {
				type = "BITBUCKET_SERVER"
				domain              = "https://example.com"
				secret_manager_ref  = "secret-ref"
				delegate_selectors  = ["delegate-1", "delegate-2"]
				client_id           = "client-id"
				client_secret_ref   = "client-secret-ref"
			}
		}

		data "harness_platform_provider" "test" {
			identifier = harness_platform_provider.test.identifier
		}
`, id, name)
}
