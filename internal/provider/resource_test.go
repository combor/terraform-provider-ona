package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccRunnerResource(t *testing.T) {
	testAccPreCheck(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read
			{
				Config: testAccRunnerConfig("tf-acc-test", "RUNNER_PROVIDER_AWS_EC2"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("ona_runner.test", "id"),
					resource.TestCheckResourceAttr("ona_runner.test", "name", "tf-acc-test"),
					resource.TestCheckResourceAttr("ona_runner.test", "provider_type", "RUNNER_PROVIDER_AWS_EC2"),
					resource.TestCheckResourceAttrSet("ona_runner.test", "status.phase"),
				),
			},
			// Import
			{
				ResourceName:      "ona_runner.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Update name
			{
				Config: testAccRunnerConfig("tf-acc-test-updated", "RUNNER_PROVIDER_AWS_EC2"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ona_runner.test", "name", "tf-acc-test-updated"),
				),
			},
		},
	})
}

func testAccRunnerConfig(name, providerType string) string {
	return fmt.Sprintf(`
resource "ona_runner" "test" {
  name          = %q
  provider_type = %q

  spec = {
    desired_phase = "RUNNER_PHASE_ACTIVE"
    configuration = {
      region          = "us-west-2"
      auto_update     = true
      release_channel = "RUNNER_RELEASE_CHANNEL_STABLE"
      log_level       = "LOG_LEVEL_INFO"
    }
  }
}
`, name, providerType)
}
