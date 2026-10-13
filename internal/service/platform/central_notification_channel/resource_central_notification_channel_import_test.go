package central_notification_channel_test

import (
	"testing"

	"github.com/harness/terraform-provider-harness/internal/acctest"
	"github.com/harness/terraform-provider-harness/internal/fakeharness"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// The import ID carries the scope ("org/project/identifier"); a deleted channel is removed
// from state whatever the shape of the 404.
func TestImportRoundTrip_CentralNotificationChannel(t *testing.T) {
	acctest.RunImportRoundTrip(t, acctest.ImportRoundTrip{
		ResourceType:   "harness_platform_central_notification_channel",
		Kind:           fakeharness.KindNotificationChannel,
		Key:            fakeharness.Key{Org: "lc_org", Project: "lc_project", ID: "lc_channel"},
		Fake:           func(f *fakeharness.Server) { f.Mount(fakeharness.NotificationChannels()) },
		ImportID:       "lc_org/lc_project/lc_channel",
		NotFoundStyles: true,
		Config: func(*fakeharness.Server) string {
			return acctest.ScopeParents("lc_org", "lc_project") + `
resource "harness_platform_central_notification_channel" "test" {
  identifier                = "lc_channel"
  name                      = "Lifecycle channel"
  org                       = harness_platform_organization.org.id
  project                   = harness_platform_project.project.id
  notification_channel_type = "EMAIL"
  channel {
    email_ids = ["lifecycle@example.com"]
  }
}
`
		},
		Checks: []resource.TestCheckFunc{
			resource.TestCheckResourceAttr("harness_platform_central_notification_channel.test", "id", "lc_channel"),
			resource.TestCheckResourceAttr("harness_platform_central_notification_channel.test", "channel.0.email_ids.0", "lifecycle@example.com"),
		},
	})
}
