package provider

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/harness/harness-go-sdk/harness/helpers"
	"github.com/harness/harness-go-sdk/harness/utils"
	gitops_agent "github.com/harness/terraform-provider-harness/internal/service/platform/gitops/agent"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/ephemeral"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	sdkschema "github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// frameworkProvider is the terraform-plugin-framework half of the muxed provider. New
// resources and data sources are written against it, and the SDK v2 half keeps the
// existing ones until they are migrated. Both halves must expose the same provider
// schema (the mux refuses to serve otherwise), so this one is derived from the SDK v2
// schema attribute by attribute.
type frameworkProvider struct {
	version string
}

var (
	_ provider.Provider                       = (*frameworkProvider)(nil)
	_ provider.ProviderWithEphemeralResources = (*frameworkProvider)(nil)
)

// NewFrameworkProvider returns the plugin-framework half of the provider.
func NewFrameworkProvider(version string) func() provider.Provider {
	return func() provider.Provider {
		return &frameworkProvider{version: version}
	}
}

func (p *frameworkProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "harness"
	resp.Version = p.version
}

func (p *frameworkProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	s, err := frameworkProviderSchema(Provider(p.version)().Schema)
	if err != nil {
		resp.Diagnostics.AddError("Provider schema mismatch", err.Error())
		return
	}
	resp.Schema = s
}

// frameworkProviderSchema mirrors the SDK v2 provider schema. The SDK marks every
// description as Markdown, so the framework side sets MarkdownDescription; defaults and
// validation stay on the SDK side and are applied again in Configure.
func frameworkProviderSchema(sdk map[string]*sdkschema.Schema) (schema.Schema, error) {
	attrs := make(map[string]schema.Attribute, len(sdk))
	for name, s := range sdk {
		switch s.Type {
		case sdkschema.TypeString:
			attrs[name] = schema.StringAttribute{Optional: s.Optional, Required: s.Required, Sensitive: s.Sensitive, MarkdownDescription: s.Description, DeprecationMessage: s.Deprecated}
		case sdkschema.TypeInt:
			attrs[name] = schema.Int64Attribute{Optional: s.Optional, Required: s.Required, Sensitive: s.Sensitive, MarkdownDescription: s.Description, DeprecationMessage: s.Deprecated}
		case sdkschema.TypeBool:
			attrs[name] = schema.BoolAttribute{Optional: s.Optional, Required: s.Required, Sensitive: s.Sensitive, MarkdownDescription: s.Description, DeprecationMessage: s.Deprecated}
		default:
			return schema.Schema{}, fmt.Errorf("provider attribute %q has type %s, which frameworkProviderSchema does not mirror yet", name, s.Type)
		}
	}
	return schema.Schema{Attributes: attrs}, nil
}

// frameworkProviderModel holds the provider configuration. Its fields must match the SDK
// v2 provider schema; TestFrameworkProviderModelMatchesSchema checks that.
type frameworkProviderModel struct {
	Endpoint            types.String `tfsdk:"endpoint"`
	FMEAdminAPIEndpoint types.String `tfsdk:"fme_admin_api_endpoint"`
	AccountID           types.String `tfsdk:"account_id"`
	APIKey              types.String `tfsdk:"api_key"`
	PlatformAPIKey      types.String `tfsdk:"platform_api_key"`
	MaxRetries          types.Int64  `tfsdk:"max_retries"`
	RequestTimeout      types.Int64  `tfsdk:"request_timeout"`
}

// stringOr returns the configured value, else the environment variable, else def. An
// unknown value counts as unset, as it does on the SDK v2 side.
func stringOr(v types.String, env, def string) string {
	if !v.IsNull() && !v.IsUnknown() && v.ValueString() != "" {
		return v.ValueString()
	}
	if e := os.Getenv(env); e != "" {
		return e
	}
	return def
}

func intOr(v types.Int64, env string, def int) (int, error) {
	if !v.IsNull() && !v.IsUnknown() {
		return int(v.ValueInt64()), nil
	}
	return envInt(env, def)
}

// sessionConfig applies the same defaults and environment variables as the SDK v2
// schema's DefaultFuncs.
func (m frameworkProviderModel) sessionConfig(version string) (SessionConfig, error) {
	maxRetries, err := intOr(m.MaxRetries, envMaxRetries, defaultMaxRetries)
	if err != nil {
		return SessionConfig{}, err
	}
	timeout, err := intOr(m.RequestTimeout, envRequestTimeout, int(defaultRequestTimeout/time.Second))
	if err != nil {
		return SessionConfig{}, err
	}
	return SessionConfig{
		Endpoint:            stringOr(m.Endpoint, helpers.EnvVars.Endpoint.String(), utils.BaseUrl),
		FMEAdminAPIEndpoint: stringOr(m.FMEAdminAPIEndpoint, envFMEAdminAPIEndpoint, ""),
		AccountID:           stringOr(m.AccountID, helpers.EnvVars.AccountId.String(), ""),
		APIKey:              stringOr(m.APIKey, helpers.EnvVars.ApiKey.String(), ""),
		PlatformAPIKey:      stringOr(m.PlatformAPIKey, helpers.EnvVars.PlatformApiKey.String(), ""),
		MaxRetries:          maxRetries,
		RequestTimeout:      time.Duration(timeout) * time.Second,
		Version:             version,
	}, nil
}

// Configure builds the session every framework resource, data source and ephemeral
// resource receives through its own Configure.
func (p *frameworkProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var m frameworkProviderModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	cfg, err := m.sessionConfig(p.version)
	if err != nil {
		resp.Diagnostics.AddError("Invalid provider configuration", err.Error())
		return
	}
	warnings, err := cfg.Validate()
	if err != nil {
		resp.Diagnostics.AddError("Invalid provider configuration", err.Error())
		return
	}
	for _, w := range warnings {
		resp.Diagnostics.AddWarning(w.Summary, w.Detail)
	}
	session := NewSession(ctx, cfg)
	resp.ResourceData = session
	resp.DataSourceData = session
	resp.EphemeralResourceData = session
}

// Resources lists the framework resources. A type name served here must not also be in
// the SDK v2 ResourcesMap; the mux reports duplicates.
func (p *frameworkProvider) Resources(_ context.Context) []func() resource.Resource {
	return nil
}

func (p *frameworkProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return nil
}

func (p *frameworkProvider) EphemeralResources(_ context.Context) []func() ephemeral.EphemeralResource {
	return []func() ephemeral.EphemeralResource{
		gitops_agent.NewEphemeralGitopsAgentToken,
	}
}
