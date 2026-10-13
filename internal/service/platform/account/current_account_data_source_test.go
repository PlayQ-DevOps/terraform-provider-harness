package account_test

import (
	"os"
	"testing"

	"github.com/harness/terraform-provider-harness/internal/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccDataSourceCurrentAccount(t *testing.T) {

	var (
		resourceName = "data.harness_platform_current_account.test"
		accountId    = os.Getenv("HARNESS_ACCOUNT_ID")
	)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.TestAccPreCheck(t) },
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccDataSourceCurrentAccount(),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "account_id", accountId),
					resource.TestCheckResourceAttr(resourceName, "endpoint", os.Getenv("HARNESS_ENDPOINT")),
				),
			},
		},
	})
}

func testAccDataSourceCurrentAccount() string {
	return `data "harness_platform_current_account" "test" {}`
}
