package connectors

import (
	"context"
	"fmt"
	"sync"

	"github.com/harness/terraform-provider-harness/helpers"
	"github.com/harness/terraform-provider-harness/internal"
	"github.com/harness/terraform-provider-harness/internal/framework"
	"github.com/harness/terraform-provider-harness/internal/framework/bridge"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/identityschema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                     = (*connectorResource)(nil)
	_ resource.ResourceWithConfigure        = (*connectorResource)(nil)
	_ resource.ResourceWithImportState      = (*connectorResource)(nil)
	_ resource.ResourceWithIdentity         = (*connectorResource)(nil)
	_ resource.ResourceWithUpgradeState     = (*connectorResource)(nil)
	_ resource.ResourceWithConfigValidators = (*connectorResource)(nil)
)

// preparedResource is a definition converted for the framework. A definition the bridge
// cannot convert keeps the error, which every request then reports; the package tests
// convert every definition, so this never reaches a user.
type preparedResource struct {
	fw   schema.Schema
	shim *bridge.Shim
	spec helpers.ImportSpec
	err  error
}

var preparedResources sync.Map // suffix -> *preparedResource

func prepareResource(def Definition) *preparedResource {
	if p, ok := preparedResources.Load(def.Suffix); ok {
		return p.(*preparedResource)
	}
	p := &preparedResource{}
	sdkRes := def.Resource()
	p.fw, p.err = bridge.ResourceSchema(sdkRes)
	if p.err == nil {
		p.fw.Version = 1
		p.spec, p.err = def.importSpec()
	}
	if p.err == nil {
		p.shim = bridge.NewShim(sdkRes)
	}
	if p.err != nil {
		p.err = fmt.Errorf("connector %s: %w", def.Suffix, p.err)
	}
	actual, _ := preparedResources.LoadOrStore(def.Suffix, p)
	return actual.(*preparedResource)
}

type connectorResource struct {
	def     Definition
	fw      schema.Schema
	shim    *bridge.Shim
	spec    helpers.ImportSpec
	err     error
	session *internal.Session
}

type connectorIdentity struct {
	OrgID      types.String `tfsdk:"org_id"`
	ProjectID  types.String `tfsdk:"project_id"`
	Identifier types.String `tfsdk:"identifier"`
}

func newResource(def Definition) func() resource.Resource {
	return func() resource.Resource {
		p := prepareResource(def)
		return &connectorResource{def: def, fw: p.fw, shim: p.shim, spec: p.spec, err: p.err}
	}
}

// ready reports the definition error, if any, and whether the request may proceed.
func (r *connectorResource) ready(diags *diag.Diagnostics) bool {
	if r.err != nil {
		diags.AddError("Connector definition error", r.err.Error())
		return false
	}
	return true
}

func (r *connectorResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_platform_connector_" + r.def.Suffix
}

func (r *connectorResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	if r.ready(&resp.Diagnostics) {
		resp.Schema = r.fw
	}
}

func (r *connectorResource) IdentitySchema(_ context.Context, _ resource.IdentitySchemaRequest, resp *resource.IdentitySchemaResponse) {
	if r.ready(&resp.Diagnostics) {
		resp.IdentitySchema = identityschema.Schema{Attributes: framework.IdentityAttributes(r.spec)}
	}
}

// UpgradeState migrates state written by the SDK v2 implementation (schema version 0).
func (r *connectorResource) UpgradeState(_ context.Context) map[int64]resource.StateUpgrader {
	if r.err != nil {
		return nil
	}
	v0 := r.fw
	v0.Version = 0
	return map[int64]resource.StateUpgrader{0: bridge.StateUpgrader(v0, r.shim.Schema())}
}

func (r *connectorResource) ConfigValidators(context.Context) []resource.ConfigValidator {
	if r.err != nil || r.def.Validate == nil {
		return nil
	}
	return []resource.ConfigValidator{sdkConfigValidator{shim: r.shim, check: r.def.Validate}}
}

func (r *connectorResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.session = framework.SessionFromProviderData(req.ProviderData, &resp.Diagnostics)
}

func (r *connectorResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	if r.ready(&resp.Diagnostics) {
		framework.ImportState(ctx, r.spec, req, resp)
	}
}

func identityValue(s string) types.String {
	if s == "" {
		return types.StringNull()
	}
	return types.StringValue(s)
}
