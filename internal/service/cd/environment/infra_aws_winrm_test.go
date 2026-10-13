package environment_test

import (
	"fmt"
	"testing"

	"github.com/harness/harness-go-sdk/harness/utils"
	"github.com/harness/terraform-provider-harness/internal/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccResourceInfrastructureDefinition_AwsWinrm(t *testing.T) {
	expectedName := fmt.Sprintf("%s_%s", t.Name(), utils.RandStringBytes(12))
	resourceName := "harness_infrastructure_definition.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.TestAccPreCheck(t) },
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccInfraDefDestroy(resourceName),
		Steps: []resource.TestStep{
			{
				Config: testAccInfraDefAws_WinRM(expectedName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "name", expectedName),
					testAccInfraDefCreation(t, resourceName, expectedName),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateIdFunc: infraDefImportStateIdFunc(resourceName),
				ImportStateVerify: true,
			},
		},
	})
}

func testAccInfraDefAws_WinRM(name string) string {
	aws := acctest.TestAccResourceAwsCloudProvider(name)
	env := acctest.TestAccResourceInfraDefEnvironment(name)

	return fmt.Sprintf(`
		%[1]s

		%[2]s

		resource "harness_infrastructure_definition" "test" {
			name = "%[3]s"
			app_id = harness_application.test.id
			env_id = harness_environment.test.id
			cloud_provider_type = "AWS"
			deployment_type = "WINRM"

			aws_winrm {
				autoscaling_group_name = "test-autoscaling-group"
				cloud_provider_name = harness_cloudprovider_aws.test.name
				desired_capacity = 1 
				host_connection_attrs_name = "winrm-test"
				host_connection_type = "PRIVATE_DNS"
				loadbalancer_name = "lb-test"
				region = "us-west-2"
			}
		}
`, aws, env, name)
}
