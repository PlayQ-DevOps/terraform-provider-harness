package organization

import (
	"context"
	"net/http"

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
	_ datasource.DataSource                     = (*organizationDataSource)(nil)
	_ datasource.DataSourceWithConfigure        = (*organizationDataSource)(nil)
	_ datasource.DataSourceWithConfigValidators = (*organizationDataSource)(nil)
)

// NewOrganizationDataSource returns data.harness_platform_organization.
func NewOrganizationDataSource() datasource.DataSource { return &organizationDataSource{} }

type organizationDataSource struct {
	session *internal.Session
}

func (d *organizationDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_platform_organization"
}

func (d *organizationDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Data source for retrieving a Harness organization",
		Attributes:          framework.CommonDataSourceAttributes(framework.AccountLevel, false),
	}
}

func (d *organizationDataSource) ConfigValidators(context.Context) []datasource.ConfigValidator {
	return []datasource.ConfigValidator{datasourcevalidator.AtLeastOneOf(path.MatchRoot("identifier"), path.MatchRoot("name"))}
}

func (d *organizationDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.session = framework.SessionFromProviderData(req.ProviderData, &resp.Diagnostics)
}

func (d *organizationDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var cfg framework.CommonModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() || !framework.RequireSession(d.session, &resp.Diagnostics) {
		return
	}
	c, ctx := d.session.GetPlatformClientWithContext(ctx)
	org, httpResp, err := lookupOrganization(ctx, c, framework.ValueString(cfg.Identifier), framework.ValueString(cfg.Name))
	if framework.Error(ctx, &resp.Diagnostics, "Reading the organization failed", err, httpResp) {
		return
	}
	if org == nil {
		resp.Diagnostics.AddError("Organization not found", "No organization matched the given identifier or name.")
		return
	}
	cfg.ID = types.StringValue(org.Identifier)
	cfg.Identifier = types.StringValue(org.Identifier)
	cfg.Name = types.StringValue(org.Name)
	cfg.Description = types.StringValue(org.Description)
	cfg.Tags = framework.StringSet(helpers.FlattenTags(org.Tags))
	resp.Diagnostics.Append(resp.State.Set(ctx, &cfg)...)
}

// lookupOrganization finds an organization by identifier, else by name.
func lookupOrganization(ctx context.Context, c *nextgen.APIClient, identifier, name string) (*nextgen.Organization, *http.Response, error) {
	if identifier != "" {
		resp, httpResp, err := c.OrganizationApi.GetOrganization(ctx, identifier, c.AccountId)
		if err != nil || resp.Data == nil {
			return nil, httpResp, err
		}
		return resp.Data.Organization, httpResp, nil
	}
	resp, httpResp, err := c.OrganizationApi.GetOrganizationByName(ctx, c.AccountId, name)
	if err != nil || resp == nil {
		return nil, httpResp, err
	}
	return resp.Organization, httpResp, nil
}
