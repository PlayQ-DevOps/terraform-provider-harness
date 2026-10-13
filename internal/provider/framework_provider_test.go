package provider

import (
	"context"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/harness/terraform-provider-harness/internal"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tfprotov5"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// The mux only serves when both halves agree on the provider schema and no type is
// served twice; GetProviderSchema reports either problem as a diagnostic.
func TestMuxServer_SchemaParity(t *testing.T) {
	ctx := context.Background()
	server, err := NewMuxServer(ctx, "test")
	if err != nil {
		t.Fatalf("NewMuxServer: %s", err)
	}
	resp, err := server().GetProviderSchema(ctx, &tfprotov5.GetProviderSchemaRequest{})
	if err != nil {
		t.Fatalf("GetProviderSchema: %s", err)
	}
	for _, d := range resp.Diagnostics {
		t.Errorf("schema diagnostic: %s: %s", d.Summary, d.Detail)
	}

	sdk := Provider("test")()
	fw := NewFrameworkProvider("test")()
	fwResources := map[string]bool{}
	for _, newResource := range fw.Resources(ctx) {
		mr := &resource.MetadataResponse{}
		newResource().Metadata(ctx, resource.MetadataRequest{ProviderTypeName: "harness"}, mr)
		fwResources[mr.TypeName] = true
		if _, dup := sdk.ResourcesMap[mr.TypeName]; dup {
			t.Errorf("resource %s is served by both halves; remove it from the SDK ResourcesMap", mr.TypeName)
		}
	}
	fwDataSources := map[string]bool{}
	for _, newDataSource := range fw.(interface {
		DataSources(context.Context) []func() datasource.DataSource
	}).DataSources(ctx) {
		mr := &datasource.MetadataResponse{}
		newDataSource().Metadata(ctx, datasource.MetadataRequest{ProviderTypeName: "harness"}, mr)
		fwDataSources[mr.TypeName] = true
		if _, dup := sdk.DataSourcesMap[mr.TypeName]; dup {
			t.Errorf("data source %s is served by both halves; remove it from the SDK DataSourcesMap", mr.TypeName)
		}
	}
	if got, want := len(resp.ResourceSchemas), len(sdk.ResourcesMap)+len(fwResources); got != want {
		t.Errorf("mux serves %d resources, the two halves define %d", got, want)
	}
	if got, want := len(resp.DataSourceSchemas), len(sdk.DataSourcesMap)+len(fwDataSources); got != want {
		t.Errorf("mux serves %d data sources, the two halves define %d", got, want)
	}
	if _, ok := resp.EphemeralResourceSchemas["harness_platform_gitops_agent_token"]; !ok {
		t.Error("ephemeral resource harness_platform_gitops_agent_token is not served")
	}
	if _, ok := resp.ResourceSchemas["harness_platform_organization"]; !ok {
		t.Error("SDK resource harness_platform_organization is not served through the mux")
	}
	if resp.Provider == nil || len(resp.Provider.Block.Attributes) != len(sdk.Schema) {
		t.Errorf("provider schema has %d attributes through the mux, the SDK schema %d", len(resp.Provider.Block.Attributes), len(sdk.Schema))
	}
}

// The framework configuration model must name every SDK provider attribute, or
// req.Config.Get fails at configure time.
func TestFrameworkProviderModelMatchesSchema(t *testing.T) {
	var tags []string
	rt := reflect.TypeOf(frameworkProviderModel{})
	for i := 0; i < rt.NumField(); i++ {
		tags = append(tags, rt.Field(i).Tag.Get("tfsdk"))
	}
	var attrs []string
	for name := range Provider("test")().Schema {
		attrs = append(attrs, name)
	}
	sort.Strings(tags)
	sort.Strings(attrs)
	if !reflect.DeepEqual(tags, attrs) {
		t.Errorf("model fields %v do not match the provider schema %v", tags, attrs)
	}
}

func TestSessionConfigValidate(t *testing.T) {
	if _, err := (SessionConfig{Endpoint: "app.harness.io"}).Validate(); err == nil || !strings.Contains(err.Error(), "Invalid Harness API endpoint") {
		t.Errorf("an endpoint without a scheme is an error, got %v", err)
	}
	warnings, err := (SessionConfig{Endpoint: "https://app.harness.io/gateway"}).Validate()
	if err != nil || len(warnings) != 2 {
		t.Errorf("missing account id and key are two warnings, got %v, %v", warnings, err)
	}
	warnings, err = (SessionConfig{Endpoint: "https://app.harness.io/gateway", AccountID: "a", PlatformAPIKey: "k"}).Validate()
	if err != nil || len(warnings) != 0 {
		t.Errorf("a complete configuration has no warnings, got %v, %v", warnings, err)
	}
}

func TestNewSession_FMEEndpoint(t *testing.T) {
	ctx := context.Background()
	s := NewSession(ctx, SessionConfig{Endpoint: "https://app.harness.io/gateway", AccountID: "a", APIKey: "fg", PlatformAPIKey: "k", MaxRetries: 1, RequestTimeout: time.Second})
	if s.SplitClient == nil || s.FMEAdminAPIEndpointError != nil || s.PLClient == nil || s.CDClient == nil || s.PolicyClient == nil {
		t.Errorf("session incomplete: split %v fmeErr %v", s.SplitClient != nil, s.FMEAdminAPIEndpointError)
	}
	s = NewSession(ctx, SessionConfig{Endpoint: "https://example.com", AccountID: "a", PlatformAPIKey: "k"})
	if s.SplitClient != nil || s.FMEAdminAPIEndpointError == nil {
		t.Error("an endpoint without /gateway leaves the FME client unset and records why")
	}
}

// nullConfig is a provider configuration with every attribute unset.
func nullConfig(t *testing.T, ctx context.Context, p provider.Provider) tfsdk.Config {
	t.Helper()
	sr := &provider.SchemaResponse{}
	p.Schema(ctx, provider.SchemaRequest{}, sr)
	if sr.Diagnostics.HasError() {
		t.Fatalf("schema: %v", sr.Diagnostics)
	}
	objType := sr.Schema.Type().TerraformType(ctx).(tftypes.Object)
	vals := map[string]tftypes.Value{}
	for name, at := range objType.AttributeTypes {
		vals[name] = tftypes.NewValue(at, nil)
	}
	return tfsdk.Config{Schema: sr.Schema, Raw: tftypes.NewValue(objType, vals)}
}

// The framework half applies the same defaults and environment variables as the SDK
// half's DefaultFuncs.
func TestFrameworkProviderConfigure_UsesEnvironment(t *testing.T) {
	t.Setenv("HARNESS_ENDPOINT", "https://env.example.com/gateway")
	t.Setenv("HARNESS_ACCOUNT_ID", "env-account")
	t.Setenv("HARNESS_PLATFORM_API_KEY", "env-key")
	t.Setenv("HARNESS_MAX_RETRIES", "2")
	t.Setenv("HARNESS_REQUEST_TIMEOUT", "7")
	ctx := context.Background()
	p := NewFrameworkProvider("test")()
	resp := &provider.ConfigureResponse{}
	p.Configure(ctx, provider.ConfigureRequest{Config: nullConfig(t, ctx, p)}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("configure: %v", resp.Diagnostics)
	}
	if len(resp.Diagnostics.Warnings()) != 0 {
		t.Errorf("no warnings expected with account and key set, got %v", resp.Diagnostics.Warnings())
	}
	s, ok := resp.ResourceData.(*internal.Session)
	if !ok || s.AccountId != "env-account" || s.Endpoint != "https://env.example.com/gateway" || s.SplitClient == nil {
		t.Errorf("session not built from the environment: %+v", resp.ResourceData)
	}
	if resp.DataSourceData != resp.ResourceData || resp.EphemeralResourceData != resp.ResourceData {
		t.Error("resources, data sources and ephemeral resources must share one session")
	}

	t.Setenv("HARNESS_MAX_RETRIES", "lots")
	resp = &provider.ConfigureResponse{}
	p.Configure(ctx, provider.ConfigureRequest{Config: nullConfig(t, ctx, p)}, resp)
	if !resp.Diagnostics.HasError() {
		t.Error("a non-integer HARNESS_MAX_RETRIES is a configure error")
	}
}
