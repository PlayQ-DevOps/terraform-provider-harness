package connectors

import (
	"context"
	"fmt"
	"sync"

	"github.com/harness/terraform-provider-harness/internal"
	"github.com/harness/terraform-provider-harness/internal/framework"
	"github.com/harness/terraform-provider-harness/internal/framework/bridge"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

var (
	_ datasource.DataSource              = (*connectorDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*connectorDataSource)(nil)
)

type connectorDataSource struct {
	def     Definition
	shim    *bridge.Shim
	fw      schema.Schema
	err     error
	session *internal.Session
}

type preparedDataSource struct {
	fw   schema.Schema
	shim *bridge.Shim
	err  error
}

var preparedDataSources sync.Map // suffix -> *preparedDataSource

func prepareDataSource(def Definition) *preparedDataSource {
	if p, ok := preparedDataSources.Load(def.Suffix); ok {
		return p.(*preparedDataSource)
	}
	p := &preparedDataSource{}
	sdkDS := def.DataSource()
	p.fw, p.err = bridge.DataSourceSchema(sdkDS)
	if p.err == nil {
		p.shim = bridge.NewShim(sdkDS)
	} else {
		p.err = fmt.Errorf("connector %s data source: %w", def.Suffix, p.err)
	}
	actual, _ := preparedDataSources.LoadOrStore(def.Suffix, p)
	return actual.(*preparedDataSource)
}

func newDataSource(def Definition) func() datasource.DataSource {
	return func() datasource.DataSource {
		p := prepareDataSource(def)
		return &connectorDataSource{def: def, shim: p.shim, fw: p.fw, err: p.err}
	}
}

func (d *connectorDataSource) ready(diags *diag.Diagnostics) bool {
	if d.err != nil {
		diags.AddError("Connector definition error", d.err.Error())
		return false
	}
	return true
}

func (d *connectorDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_platform_connector_" + d.def.Suffix
}

func (d *connectorDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	if d.ready(&resp.Diagnostics) {
		resp.Schema = d.fw
	}
}

func (d *connectorDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.session = framework.SessionFromProviderData(req.ProviderData, &resp.Diagnostics)
}

func (d *connectorDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	if !d.ready(&resp.Diagnostics) || !framework.RequireSession(d.session, &resp.Diagnostics) {
		return
	}
	rd, err := d.shim.FromPlan(ctx, req.Config.Raw, tftypes.NewValue(req.Config.Raw.Type(), nil))
	if err != nil {
		resp.Diagnostics.AddError("Reading the connector failed", err.Error())
		return
	}
	id, _ := rd.Get("identifier").(string)
	c, ctx := d.session.GetPlatformClientWithContext(ctx)
	out, httpResp, err := c.ConnectorsApi.GetConnector(ctx, c.AccountId, id, readOpts(rd))
	if framework.Error(ctx, &resp.Diagnostics, "Reading the connector failed", err, httpResp) {
		return
	}
	if out.Data == nil || out.Data.Connector == nil {
		resp.Diagnostics.AddError("Connector not found", fmt.Sprintf("No connector with identifier %q was found in the given scope.", id))
		return
	}
	if out.Data.Connector.Type_ != d.def.Type {
		resp.Diagnostics.AddError("Unexpected connector type", fmt.Sprintf("Expected connector %s to be of type %s, but Harness reports %s.", id, d.def.Type, out.Data.Connector.Type_))
		return
	}
	if err := readCommon(rd, out.Data.Connector); err != nil {
		resp.Diagnostics.AddError("Reading the connector failed", err.Error())
		return
	}
	if err := d.def.Read(rd, out.Data.Connector); err != nil {
		resp.Diagnostics.AddError("Reading the connector failed", err.Error())
		return
	}
	v, err := d.shim.Result(ctx, rd, req.Config.Raw, resp.State.Raw.Type())
	if err != nil {
		resp.Diagnostics.AddError("Reading the connector failed", err.Error())
		return
	}
	resp.State.Raw = v
}
