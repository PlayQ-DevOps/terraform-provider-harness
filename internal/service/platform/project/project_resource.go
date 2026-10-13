package project

import (
	"context"

	"github.com/harness/terraform-provider-harness/internal"
	"github.com/harness/terraform-provider-harness/internal/framework"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/identityschema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                 = (*projectResource)(nil)
	_ resource.ResourceWithConfigure    = (*projectResource)(nil)
	_ resource.ResourceWithImportState  = (*projectResource)(nil)
	_ resource.ResourceWithIdentity     = (*projectResource)(nil)
	_ resource.ResourceWithUpgradeState = (*projectResource)(nil)
)

// NewProjectResource returns harness_platform_project.
func NewProjectResource() resource.Resource { return &projectResource{} }

type projectResource struct {
	session *internal.Session
}

type projectModel struct {
	framework.CommonModel
	OrgID   types.String `tfsdk:"org_id"`
	Color   types.String `tfsdk:"color"`
	Modules types.Set    `tfsdk:"modules"`
}

type projectIdentity struct {
	OrgID      types.String `tfsdk:"org_id"`
	Identifier types.String `tfsdk:"identifier"`
}

func (r *projectResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_platform_project"
}

func projectSchema() schema.Schema {
	attrs := framework.CommonResourceAttributes(framework.OrgLevel)
	attrs["color"] = schema.StringAttribute{
		MarkdownDescription: "Color of the project.",
		Optional:            true,
		Computed:            true,
		PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
	}
	attrs["modules"] = schema.SetAttribute{
		MarkdownDescription: "Modules in the project.",
		ElementType:         types.StringType,
		Computed:            true,
		PlanModifiers:       []planmodifier.Set{setplanmodifier.UseStateForUnknown()},
	}
	return schema.Schema{MarkdownDescription: "Resource for creating a Harness project.", Version: 1, Attributes: attrs}
}

func (r *projectResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = projectSchema()
}

func (r *projectResource) IdentitySchema(_ context.Context, _ resource.IdentitySchemaRequest, resp *resource.IdentitySchemaResponse) {
	resp.IdentitySchema = identityschema.Schema{Attributes: framework.IdentityAttributes(framework.ImportOrgLevel)}
}

// UpgradeState migrates state written by the SDK v2 implementation (schema version 0).
func (r *projectResource) UpgradeState(_ context.Context) map[int64]resource.StateUpgrader {
	v0 := projectSchema()
	v0.Version = 0
	return map[int64]resource.StateUpgrader{0: framework.ZeroValueToNullUpgrader(v0)}
}

func (r *projectResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.session = framework.SessionFromProviderData(req.ProviderData, &resp.Diagnostics)
}

func (r *projectResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	framework.ImportState(ctx, framework.ImportOrgLevel, req, resp)
}

func setProjectIdentity(ctx context.Context, identity *tfsdk.ResourceIdentity, m *projectModel, diags *diag.Diagnostics) {
	if identity != nil {
		diags.Append(identity.Set(ctx, &projectIdentity{OrgID: m.OrgID, Identifier: m.Identifier})...)
	}
}
