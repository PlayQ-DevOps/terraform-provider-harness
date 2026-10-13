package project

import (
	"context"
	"net/http"

	"github.com/antihax/optional"
	"github.com/harness/harness-go-sdk/harness/nextgen"
	"github.com/harness/terraform-provider-harness/helpers"
	"github.com/harness/terraform-provider-harness/internal"
	"github.com/harness/terraform-provider-harness/internal/framework"
	"github.com/hashicorp/terraform-plugin-framework-validators/datasourcevalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ datasource.DataSource                     = (*projectDataSource)(nil)
	_ datasource.DataSourceWithConfigure        = (*projectDataSource)(nil)
	_ datasource.DataSourceWithConfigValidators = (*projectDataSource)(nil)
)

// NewProjectDataSource returns data.harness_platform_project.
func NewProjectDataSource() datasource.DataSource { return &projectDataSource{} }

type projectDataSource struct {
	session *internal.Session
}

type projectDataSourceModel struct {
	framework.CommonModel
	OrgID   types.String `tfsdk:"org_id"`
	Color   types.String `tfsdk:"color"`
	Modules types.Set    `tfsdk:"modules"`
}

func (d *projectDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_platform_project"
}

func (d *projectDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	attrs := framework.CommonDataSourceAttributes(framework.OrgLevel, false)
	attrs["color"] = schema.StringAttribute{MarkdownDescription: "Color of the project.", Computed: true}
	attrs["modules"] = schema.SetAttribute{MarkdownDescription: "Modules in the project.", ElementType: types.StringType, Computed: true}
	resp.Schema = schema.Schema{MarkdownDescription: "Data source for retrieving a Harness project.", Attributes: attrs}
}

func (d *projectDataSource) ConfigValidators(context.Context) []datasource.ConfigValidator {
	return []datasource.ConfigValidator{datasourcevalidator.AtLeastOneOf(path.MatchRoot("identifier"), path.MatchRoot("name"))}
}

func (d *projectDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.session = framework.SessionFromProviderData(req.ProviderData, &resp.Diagnostics)
}

func (d *projectDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var cfg projectDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() || !framework.RequireSession(d.session, &resp.Diagnostics) {
		return
	}
	c, ctx := d.session.GetPlatformClientWithContext(ctx)
	project, httpResp, err := lookupProject(ctx, c, cfg.OrgID.ValueString(), framework.ValueString(cfg.Identifier), framework.ValueString(cfg.Name))
	if framework.Error(ctx, &resp.Diagnostics, "Reading the project failed", err, httpResp) {
		return
	}
	if project == nil {
		resp.Diagnostics.AddError("Project not found", "No project matched the given identifier or name.")
		return
	}
	cfg.ID = types.StringValue(project.Identifier)
	cfg.Identifier = types.StringValue(project.Identifier)
	cfg.OrgID = types.StringValue(project.OrgIdentifier)
	cfg.Name = types.StringValue(project.Name)
	cfg.Color = types.StringValue(project.Color)
	cfg.Description = types.StringValue(project.Description)
	cfg.Modules = framework.StringSet(project.Modules)
	cfg.Tags = framework.StringSet(helpers.FlattenTags(project.Tags))
	resp.Diagnostics.Append(resp.State.Set(ctx, &cfg)...)
}

// lookupProject finds a project by identifier, else by name.
func lookupProject(ctx context.Context, c *nextgen.APIClient, orgID, identifier, name string) (*nextgen.Project, *http.Response, error) {
	if identifier != "" {
		resp, httpResp, err := c.ProjectApi.GetProject(ctx, identifier, c.AccountId, &nextgen.ProjectApiGetProjectOpts{OrgIdentifier: optional.NewString(orgID)})
		if err != nil || resp.Data == nil {
			return nil, httpResp, err
		}
		return resp.Data.Project, httpResp, nil
	}
	resp, httpResp, err := c.ProjectApi.GetProjectByName(ctx, c.AccountId, orgID, name)
	if err != nil || resp == nil {
		return nil, httpResp, err
	}
	return resp.Project, httpResp, nil
}
