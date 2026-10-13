package user_test

import (
	"testing"

	"github.com/harness/terraform-provider-harness/internal/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccDataSourceCurrentUser(t *testing.T) {
	t.Skip("Only works with personal access tokens.")

	resourceName := "data.harness_platform_current_user.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.TestAccPreCheck(t) },
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccDataSourceCurrentUser(),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "name", "micah.martin@harness.io"),
				),
			},
		},
	})
}

func testAccDataSourceCurrentUser() string {
	return `
		data "harness_platform_current_user" "test" {}
	`
}
