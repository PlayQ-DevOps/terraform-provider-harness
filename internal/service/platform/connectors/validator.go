package connectors

import (
	"context"

	"github.com/harness/terraform-provider-harness/internal/framework/bridge"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	sdkschema "github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// sdkConfigValidator runs a Definition.Validate check (the body of what used to be an SDK
// v2 CustomizeDiff) over the configuration. A configuration with unknown values is not
// checked: the check would see the unknown values as unset.
type sdkConfigValidator struct {
	shim  *bridge.Shim
	check func(d *sdkschema.ResourceData) error
}

var _ resource.ConfigValidator = sdkConfigValidator{}

func (v sdkConfigValidator) Description(context.Context) string {
	return "configuration accepted by the connector's validation function"
}

func (v sdkConfigValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v sdkConfigValidator) ValidateResource(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	if !req.Config.Raw.IsFullyKnown() {
		return
	}
	d, err := v.shim.FromPlan(ctx, req.Config.Raw, tftypes.NewValue(req.Config.Raw.Type(), nil))
	if err != nil {
		resp.Diagnostics.AddError("Validating the configuration failed", err.Error())
		return
	}
	if err := v.check(d); err != nil {
		resp.Diagnostics.AddError("Invalid connector configuration", err.Error())
	}
}
