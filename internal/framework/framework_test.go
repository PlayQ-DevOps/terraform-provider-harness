package framework

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/harness/terraform-provider-harness/internal"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

var ctx = context.Background()

func testSchema() schema.Schema {
	attrs := CommonResourceAttributes(MultiLevel)
	attrs["color"] = schema.StringAttribute{Optional: true, Computed: true}
	attrs["modules"] = schema.SetAttribute{ElementType: types.StringType, Computed: true}
	return schema.Schema{Attributes: attrs}
}

// nullObject is a state value with every attribute null unless overridden.
func nullObject(s schema.Schema, overrides map[string]tftypes.Value) tftypes.Value {
	typ := s.Type().TerraformType(ctx).(tftypes.Object)
	vals := map[string]tftypes.Value{}
	for name, at := range typ.AttributeTypes {
		vals[name] = tftypes.NewValue(at, nil)
	}
	for name, v := range overrides {
		vals[name] = v
	}
	return tftypes.NewValue(typ, vals)
}

func httpStatus(code int) (*http.Response, error) {
	return &http.Response{StatusCode: code, Status: http.StatusText(code), Body: io.NopCloser(strings.NewReader("{}"))}, errors.New(http.StatusText(code))
}

func TestCommonAttributes(t *testing.T) {
	attrs := CommonResourceAttributes(MultiLevel)
	for _, name := range []string{"id", "identifier", "name", "description", "tags", "org_id", "project_id"} {
		if _, ok := attrs[name]; !ok {
			t.Errorf("missing %s", name)
		}
	}
	if len(attrs["identifier"].(schema.StringAttribute).PlanModifiers) == 0 || len(attrs["org_id"].(schema.StringAttribute).PlanModifiers) == 0 {
		t.Error("identity attributes must require replacement")
	}
	if len(attrs["project_id"].(schema.StringAttribute).Validators) == 0 {
		t.Error("project_id must require org_id")
	}
	if _, ok := CommonResourceAttributes(AccountLevel)["org_id"]; ok {
		t.Error("account-level resources have no org_id")
	}
	ds := CommonDataSourceAttributes(OrgLevel, false)
	if !ds["org_id"].IsRequired() || !ds["description"].IsComputed() || !ds["identifier"].IsOptional() {
		t.Error("data source shape: org_id required, description computed, identifier optional")
	}
	if !CommonDataSourceAttributes(ProjectLevel, true)["identifier"].IsRequired() {
		t.Error("identifierRequired makes identifier required")
	}
}

func TestValues(t *testing.T) {
	if !StringFromAPI("", types.StringNull()).IsNull() {
		t.Error("an empty API value keeps a null prior null")
	}
	if v := StringFromAPI("", types.StringValue("")); v.IsNull() || v.ValueString() != "" {
		t.Error("an explicit empty string stays an empty string")
	}
	if v := StringFromAPI("x", types.StringNull()); v.ValueString() != "x" {
		t.Error("a non-empty API value is the value")
	}
	if !TagsFromAPI(nil, types.SetNull(types.StringType)).IsNull() {
		t.Error("no tags keeps a null prior null")
	}
	set := TagsFromAPI(map[string]string{"team": "sre", "flag": ""}, types.SetNull(types.StringType))
	var diags diag.Diagnostics
	if got := Strings(ctx, set, &diags); len(got) != 2 || got[0] != "flag" || got[1] != "team:sre" {
		t.Errorf("tags set = %v", got)
	}
	back := TagsToAPI(ctx, set, &diags)
	if diags.HasError() || back["team"] != "sre" || back["flag"] != "" {
		t.Errorf("TagsToAPI = %v, %v", back, diags)
	}
	if len(TagsToAPI(ctx, types.SetNull(types.StringType), &diags)) != 0 || len(TagsToAPI(ctx, types.SetUnknown(types.StringType), &diags)) != 0 {
		t.Error("null and unknown tags are empty maps")
	}
}

func TestSessionFromProviderData(t *testing.T) {
	var diags diag.Diagnostics
	if SessionFromProviderData(nil, &diags) != nil || diags.HasError() {
		t.Error("nil provider data is not an error")
	}
	if SessionFromProviderData("nope", &diags) != nil || !diags.HasError() {
		t.Error("the wrong type is a provider bug")
	}
	s := &internal.Session{}
	diags = nil
	if SessionFromProviderData(s, &diags) != s || diags.HasError() {
		t.Error("the session is returned as is")
	}
}
