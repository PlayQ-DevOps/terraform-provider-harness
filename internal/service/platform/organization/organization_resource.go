package organization

import (
	"context"

	"github.com/harness/harness-go-sdk/harness/nextgen"
	"github.com/harness/terraform-provider-harness/internal"
	"github.com/harness/terraform-provider-harness/internal/framework"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/identityschema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                 = (*organizationResource)(nil)
	_ resource.ResourceWithConfigure    = (*organizationResource)(nil)
	_ resource.ResourceWithImportState  = (*organizationResource)(nil)
	_ resource.ResourceWithIdentity     = (*organizationResource)(nil)
	_ resource.ResourceWithUpgradeState = (*organizationResource)(nil)
)

// NewOrganizationResource returns harness_platform_organization.
func NewOrganizationResource() resource.Resource { return &organizationResource{} }

type organizationResource struct {
	session *internal.Session
}

type organizationModel struct {
	framework.CommonModel
}

type organizationIdentity struct {
	Identifier types.String `tfsdk:"identifier"`
}

func (r *organizationResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_platform_organization"
}

func organizationSchema() schema.Schema {
	return schema.Schema{
		MarkdownDescription: "Resource for creating a Harness organization.",
		Version:             1,
		Attributes:          framework.CommonResourceAttributes(framework.AccountLevel),
	}
}

func (r *organizationResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = organizationSchema()
}

func (r *organizationResource) IdentitySchema(_ context.Context, _ resource.IdentitySchemaRequest, resp *resource.IdentitySchemaResponse) {
	resp.IdentitySchema = identityschema.Schema{Attributes: framework.IdentityAttributes(framework.ImportPassthrough)}
}

// UpgradeState migrates state written by the SDK v2 implementation (schema version 0).
func (r *organizationResource) UpgradeState(_ context.Context) map[int64]resource.StateUpgrader {
	v0 := organizationSchema()
	v0.Version = 0
	return map[int64]resource.StateUpgrader{0: framework.ZeroValueToNullUpgrader(v0)}
}

func (r *organizationResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.session = framework.SessionFromProviderData(req.ProviderData, &resp.Diagnostics)
}

func (r *organizationResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	framework.ImportState(ctx, framework.ImportPassthrough, req, resp)
}

func setOrganizationIdentity(ctx context.Context, identity *tfsdk.ResourceIdentity, m *organizationModel, diags *diag.Diagnostics) {
	if identity != nil {
		diags.Append(identity.Set(ctx, &organizationIdentity{Identifier: m.Identifier})...)
	}
}

func (r *organizationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan organizationModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() || !framework.RequireSession(r.session, &resp.Diagnostics) {
		return
	}
	c, ctx := r.session.GetPlatformClientWithContext(ctx)
	out, httpResp, err := c.OrganizationApi.PostOrganization(ctx, nextgen.OrganizationRequest{Organization: buildOrganization(ctx, &plan, &resp.Diagnostics)}, c.AccountId)
	if framework.WriteError(ctx, &resp.Diagnostics, err, httpResp) {
		return
	}
	if out.Data == nil || out.Data.Organization == nil {
		resp.Diagnostics.AddError("Creating the organization failed", "Harness returned no organization.")
		return
	}
	readOrganization(&plan, out.Data.Organization)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	setOrganizationIdentity(ctx, resp.Identity, &plan, &resp.Diagnostics)
}

func (r *organizationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state organizationModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() || !framework.RequireSession(r.session, &resp.Diagnostics) {
		return
	}
	c, ctx := r.session.GetPlatformClientWithContext(ctx)
	out, httpResp, err := c.OrganizationApi.GetOrganization(ctx, state.ID.ValueString(), c.AccountId)
	if framework.ReadError(ctx, resp, err, httpResp) {
		return
	}
	if out.Data == nil || out.Data.Organization == nil {
		framework.ReadMissing(ctx, resp, "organization "+state.ID.ValueString())
		return
	}
	readOrganization(&state, out.Data.Organization)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
	setOrganizationIdentity(ctx, resp.Identity, &state, &resp.Diagnostics)
}

func (r *organizationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan organizationModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() || !framework.RequireSession(r.session, &resp.Diagnostics) {
		return
	}
	c, ctx := r.session.GetPlatformClientWithContext(ctx)
	org := buildOrganization(ctx, &plan, &resp.Diagnostics)
	out, httpResp, err := c.OrganizationApi.PutOrganization(ctx, nextgen.OrganizationRequest{Organization: org}, c.AccountId, org.Identifier, nil)
	if framework.WriteError(ctx, &resp.Diagnostics, err, httpResp) {
		return
	}
	if out.Data == nil || out.Data.Organization == nil {
		resp.Diagnostics.AddError("Updating the organization failed", "Harness returned no organization.")
		return
	}
	readOrganization(&plan, out.Data.Organization)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	setOrganizationIdentity(ctx, resp.Identity, &plan, &resp.Diagnostics)
}

func (r *organizationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state organizationModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() || !framework.RequireSession(r.session, &resp.Diagnostics) {
		return
	}
	c, ctx := r.session.GetPlatformClientWithContext(ctx)
	_, httpResp, err := c.OrganizationApi.DeleteOrganization(ctx, state.ID.ValueString(), c.AccountId, nil)
	framework.DeleteError(ctx, &resp.Diagnostics, err, httpResp)
}

func buildOrganization(ctx context.Context, m *organizationModel, diags *diag.Diagnostics) *nextgen.Organization {
	return &nextgen.Organization{
		Identifier:  m.Identifier.ValueString(),
		Name:        m.Name.ValueString(),
		Description: framework.ValueString(m.Description),
		Tags:        framework.TagsToAPI(ctx, m.Tags, diags),
	}
}

// readOrganization refreshes m from the API object; Optional attributes keep their
// null-ness when the API value is empty.
func readOrganization(m *organizationModel, org *nextgen.Organization) {
	m.ID = types.StringValue(org.Identifier)
	m.Identifier = types.StringValue(org.Identifier)
	m.Name = types.StringValue(org.Name)
	m.Description = framework.StringFromAPI(org.Description, m.Description)
	m.Tags = framework.TagsFromAPI(org.Tags, m.Tags)
}
