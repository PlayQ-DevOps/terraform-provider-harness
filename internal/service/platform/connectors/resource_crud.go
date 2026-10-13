package connectors

import (
	"context"
	"fmt"
	"net/http"

	"github.com/harness/harness-go-sdk/harness/nextgen"
	"github.com/harness/terraform-provider-harness/helpers"
	"github.com/harness/terraform-provider-harness/internal/framework"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	sdkschema "github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// write creates the connector when the prior state is null, else updates it.
func (r *connectorResource) write(ctx context.Context, plan, prior tftypes.Value, state *tfsdk.State, identity *tfsdk.ResourceIdentity, diags *diag.Diagnostics) {
	if !r.ready(diags) || !framework.RequireSession(r.session, diags) {
		return
	}
	d, err := r.shim.FromPlan(ctx, plan, prior)
	if err != nil {
		diags.AddError("Preparing the connector failed", err.Error())
		return
	}
	var conn *nextgen.ConnectorInfo
	if r.def.Build != nil {
		conn = r.def.Build(d)
	} else {
		conn, err = r.def.BuildErr(d)
		if err != nil {
			diags.AddError("Invalid connector configuration", err.Error())
			return
		}
	}
	buildCommon(d, conn)
	c, ctx := r.session.GetPlatformClientWithContext(ctx)
	var out nextgen.ResponseDtoConnectorResponse
	var httpResp *http.Response
	if d.Id() == "" {
		out, httpResp, err = c.ConnectorsApi.CreateConnector(ctx, nextgen.Connector{Connector: conn}, c.AccountId, &nextgen.ConnectorsApiCreateConnectorOpts{})
	} else {
		out, httpResp, err = c.ConnectorsApi.UpdateConnector(ctx, nextgen.Connector{Connector: conn}, c.AccountId, &nextgen.ConnectorsApiUpdateConnectorOpts{})
	}
	if framework.WriteError(ctx, diags, err, httpResp) {
		return
	}
	if out.Data == nil || out.Data.Connector == nil {
		var governance *nextgen.GovernanceMetadata
		if out.Data != nil {
			governance = out.Data.GovernanceMetadata
		}
		for _, dg := range helpers.HandleEmptyCreateUpdateResponse(governance, "connector") {
			diags.AddError(dg.Summary, dg.Detail)
		}
		return
	}
	r.finish(ctx, d, out.Data.Connector, plan, state, identity, diags)
}

func (r *connectorResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	r.write(ctx, req.Plan.Raw, tftypes.NewValue(req.Plan.Raw.Type(), nil), &resp.State, resp.Identity, &resp.Diagnostics)
}

func (r *connectorResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	r.write(ctx, req.Plan.Raw, req.State.Raw, &resp.State, resp.Identity, &resp.Diagnostics)
}

func (r *connectorResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	if !r.ready(&resp.Diagnostics) || !framework.RequireSession(r.session, &resp.Diagnostics) {
		return
	}
	d, err := r.shim.FromState(ctx, req.State.Raw)
	if err != nil {
		resp.Diagnostics.AddError("Reading the connector failed", err.Error())
		return
	}
	id := d.Id()
	if id == "" {
		id, _ = d.Get("identifier").(string)
	}
	c, ctx := r.session.GetPlatformClientWithContext(ctx)
	out, httpResp, err := c.ConnectorsApi.GetConnector(ctx, c.AccountId, id, readOpts(d))
	if framework.ReadError(ctx, resp, err, httpResp) {
		return
	}
	if out.Data == nil || out.Data.Connector == nil {
		framework.ReadMissing(ctx, resp, "connector "+id)
		return
	}
	if out.Data.Connector.Type_ != r.def.Type {
		resp.Diagnostics.AddError("Unexpected connector type", fmt.Sprintf("Expected connector %s to be of type %s, but Harness reports %s.", id, r.def.Type, out.Data.Connector.Type_))
		return
	}
	r.finish(ctx, d, out.Data.Connector, req.State.Raw, &resp.State, resp.Identity, &resp.Diagnostics)
}

func (r *connectorResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	if !r.ready(&resp.Diagnostics) || !framework.RequireSession(r.session, &resp.Diagnostics) {
		return
	}
	d, err := r.shim.FromState(ctx, req.State.Raw)
	if err != nil {
		resp.Diagnostics.AddError("Deleting the connector failed", err.Error())
		return
	}
	c, ctx := r.session.GetPlatformClientWithContext(ctx)
	_, httpResp, err := c.ConnectorsApi.DeleteConnector(ctx, c.AccountId, d.Id(), &nextgen.ConnectorsApiDeleteConnectorOpts{
		OrgIdentifier:     helpers.BuildField(d, "org_id"),
		ProjectIdentifier: helpers.BuildField(d, "project_id"),
		ForceDelete:       helpers.BuildFieldBool(d, "force_delete"),
	})
	framework.DeleteError(ctx, &resp.Diagnostics, err, httpResp)
}

// finish writes the connector into the state: the common attributes, the type-specific
// read, and the identity. prior is the plan or the previous state.
func (r *connectorResource) finish(ctx context.Context, d *sdkschema.ResourceData, conn *nextgen.ConnectorInfo, prior tftypes.Value, state *tfsdk.State, identity *tfsdk.ResourceIdentity, diags *diag.Diagnostics) {
	if err := readCommon(d, conn); err != nil {
		diags.AddError("Reading the connector failed", err.Error())
		return
	}
	if err := r.def.Read(d, conn); err != nil {
		diags.AddError("Reading the connector failed", err.Error())
		return
	}
	v, err := r.shim.Result(ctx, d, prior, state.Raw.Type())
	if err != nil {
		diags.AddError("Reading the connector failed", err.Error())
		return
	}
	state.Raw = v
	if identity != nil {
		diags.Append(identity.Set(ctx, &connectorIdentity{
			OrgID:      identityValue(conn.OrgIdentifier),
			ProjectID:  identityValue(conn.ProjectIdentifier),
			Identifier: identityValue(conn.Identifier),
		})...)
	}
}
