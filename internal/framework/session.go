package framework

import (
	"fmt"

	"github.com/harness/terraform-provider-harness/internal"
	"github.com/hashicorp/terraform-plugin-framework/diag"
)

// SessionFromProviderData returns the session the provider's Configure stored, or nil when
// the provider has not been configured yet (Terraform may configure a resource before the
// provider; a nil session is not an error then). Any other type is a provider bug and is
// reported.
func SessionFromProviderData(data any, diags *diag.Diagnostics) *internal.Session {
	if data == nil {
		return nil
	}
	s, ok := data.(*internal.Session)
	if !ok {
		diags.AddError("Unexpected provider data", fmt.Sprintf("Expected *internal.Session, got %T. This is a bug in the provider.", data))
		return nil
	}
	return s
}

// RequireSession reports an error when the provider has not been configured, which no
// CRUD call should ever see.
func RequireSession(s *internal.Session, diags *diag.Diagnostics) bool {
	if s == nil {
		diags.AddError("Provider not configured", "The Harness provider was not configured before this resource was used. This is a bug in the provider.")
		return false
	}
	return true
}
