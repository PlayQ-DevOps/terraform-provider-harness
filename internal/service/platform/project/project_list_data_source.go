package project

import (
	"context"

	"github.com/antihax/optional"
	"github.com/harness/harness-go-sdk/harness/nextgen"
	"github.com/harness/terraform-provider-harness/internal"
	"github.com/harness/terraform-provider-harness/internal/framework"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ datasource.DataSource              = (*projectListDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*projectListDataSource)(nil)
)

// NewProjectListDataSource returns data.harness_platform_project_list.
func NewProjectListDataSource() datasource.DataSource { return &projectListDataSource{} }

type projectListDataSource struct {
	session *internal.Session
}

// projectListModel keeps the SDK v2 shape, including the common lookup attributes the
// SDK helper added even though the list ignores them.
type projectListModel struct {
	framework.CommonModel
	OrgID    types.String `tfsdk:"org_id"`
	Page     types.Int64  `tfsdk:"page"`
	Limit    types.Int64  `tfsdk:"limit"`
	Projects types.List   `tfsdk:"projects"`
}

type projectListItem struct {
	Identifier types.String `tfsdk:"identifier"`
	Name       types.String `tfsdk:"name"`
}

var projectListItemType = types.ObjectType{AttrTypes: map[string]attr.Type{"identifier": types.StringType, "name": types.StringType}}

func (d *projectListDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_platform_project_list"
}

func (d *projectListDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	attrs := framework.CommonDataSourceAttributes(framework.OrgLevel, false)
	attrs["page"] = schema.Int64Attribute{MarkdownDescription: "Page index of the project list to return (zero based).", Optional: true}
	attrs["limit"] = schema.Int64Attribute{MarkdownDescription: "Number of projects per page.", Optional: true}
	// A list of objects, not a nested attribute: the provider is served over protocol 5, which
	// has no nested attributes, and this is the wire shape the SDK v2 implementation had.
	attrs["projects"] = schema.ListAttribute{
		MarkdownDescription: "Projects in the organization: objects with the project identifier and name.",
		ElementType:         projectListItemType,
		Computed:            true,
	}
	resp.Schema = schema.Schema{MarkdownDescription: "Data source for retrieving the projects of a Harness organization.", Attributes: attrs}
}

func (d *projectListDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.session = framework.SessionFromProviderData(req.ProviderData, &resp.Diagnostics)
}

func (d *projectListDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var cfg projectListModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() || !framework.RequireSession(d.session, &resp.Diagnostics) {
		return
	}
	c, ctx := d.session.GetPlatformClientWithContext(ctx)
	opts := &nextgen.ProjectApiGetProjectListOpts{
		OrgIdentifier: optional.NewString(cfg.OrgID.ValueString()),
		PageIndex:     optional.NewInt32(int32(cfg.Page.ValueInt64())),
	}
	if !cfg.Limit.IsNull() && cfg.Limit.ValueInt64() != 0 {
		opts.PageSize = optional.NewInt32(int32(cfg.Limit.ValueInt64()))
	}
	out, httpResp, err := c.ProjectApi.GetProjectList(ctx, c.AccountId, opts)
	if framework.Error(ctx, &resp.Diagnostics, "Listing the projects failed", err, httpResp) {
		return
	}
	if out.Data == nil || len(out.Data.Content) == 0 {
		resp.Diagnostics.AddError("Project list not found", "No project matched the given arguments.")
		return
	}
	items := make([]projectListItem, 0, len(out.Data.Content))
	for _, v := range out.Data.Content {
		if v.Project == nil {
			continue
		}
		items = append(items, projectListItem{Identifier: types.StringValue(v.Project.Identifier), Name: types.StringValue(v.Project.Name)})
	}
	list, diags := types.ListValueFrom(ctx, projectListItemType, items)
	resp.Diagnostics.Append(diags...)
	cfg.Projects = list
	cfg.ID = types.StringValue(out.CorrelationId)
	for _, v := range []*types.String{&cfg.Identifier, &cfg.Name, &cfg.Description} {
		if v.IsUnknown() {
			*v = types.StringNull()
		}
	}
	if cfg.Tags.IsUnknown() {
		cfg.Tags = types.SetNull(types.StringType)
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &cfg)...)
}
