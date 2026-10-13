package helpers

import (
	"github.com/antihax/optional"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// ConfigOnly returns the state value of an input the API never reports back (a commit
// message, the branch a new branch was created from, a store type), so a Read that rewrites
// the block holding it can keep it. A Read must take everything else from the API; every
// ConfigOnly use documents an attribute that is not refreshed, so list it in the resource's
// ImportStateVerifyIgnore and say so in its description. internal/contract/refresh_test.go
// enforces that reads do not echo the configuration any other way.
func ConfigOnly(d *schema.ResourceData, key string) optional.String {
	return BuildField(d, key)
}

// ConfigOnlyBool is ConfigOnly for a boolean input; false when unset.
func ConfigOnlyBool(d *schema.ResourceData, key string) bool {
	v, ok := d.GetOk(key)
	if !ok {
		return false
	}
	b, _ := v.(bool)
	return b
}
