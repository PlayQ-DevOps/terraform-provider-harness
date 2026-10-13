package framework

import (
	"errors"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/identityschema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestReadError(t *testing.T) {
	s := testSchema()
	state := func() tfsdk.State {
		return tfsdk.State{Schema: s, Raw: nullObject(s, map[string]tftypes.Value{"id": tftypes.NewValue(tftypes.String, "x")})}
	}
	resp := &resource.ReadResponse{State: state()}
	if ReadError(ctx, resp, nil, nil) {
		t.Error("no error, nothing handled")
	}
	httpResp, err := httpStatus(404)
	if !ReadError(ctx, resp, err, httpResp) || resp.Diagnostics.HasError() || !resp.State.Raw.IsNull() {
		t.Errorf("a 404 removes the resource from state without an error: diags %v, state null %v", resp.Diagnostics, resp.State.Raw.IsNull())
	}
	resp = &resource.ReadResponse{State: state()}
	httpResp, err = httpStatus(500)
	if !ReadError(ctx, resp, err, httpResp) || !resp.Diagnostics.HasError() || resp.State.Raw.IsNull() {
		t.Error("a 500 is an error and keeps the state")
	}
	resp = &resource.ReadResponse{State: state()}
	ReadMissing(ctx, resp, "thing x")
	if !resp.State.Raw.IsNull() {
		t.Error("ReadMissing removes the resource")
	}
}

func TestDeleteWriteAndError(t *testing.T) {
	var diags diag.Diagnostics
	httpResp, err := httpStatus(404)
	if DeleteError(ctx, &diags, err, httpResp) || diags.HasError() {
		t.Error("a 404 on delete is success")
	}
	if !DeleteError(ctx, &diags, errors.New("dial tcp: connection refused"), nil) || !diags.HasError() {
		t.Error("a transport failure on delete is an error")
	}
	diags = nil
	httpResp, err = httpStatus(409)
	if !WriteError(ctx, &diags, err, httpResp) || !strings.Contains(diags.Errors()[0].Detail(), "terraform import") {
		t.Errorf("a conflict names the import, got %v", diags)
	}
	diags = nil
	httpResp, err = httpStatus(403)
	if !Error(ctx, &diags, "Reading the thing failed", err, httpResp) || diags.Errors()[0].Summary() != "Reading the thing failed" {
		t.Errorf("Error uses the summary, got %v", diags)
	}
}

func TestImportState(t *testing.T) {
	s := testSchema()
	run := func(id string, identity *tfsdk.ResourceIdentity) *resource.ImportStateResponse {
		resp := &resource.ImportStateResponse{State: tfsdk.State{Schema: s, Raw: nullObject(s, nil)}}
		ImportState(ctx, ImportMultiLevel, resource.ImportStateRequest{ID: id, Identity: identity}, resp)
		return resp
	}
	get := func(resp *resource.ImportStateResponse, name string) string {
		var v types.String
		resp.Diagnostics.Append(resp.State.GetAttribute(ctx, path.Root(name), &v)...)
		return v.ValueString()
	}
	resp := run("o/p/x", nil)
	if resp.Diagnostics.HasError() || get(resp, "org_id") != "o" || get(resp, "project_id") != "p" || get(resp, "identifier") != "x" || get(resp, "id") != "x" {
		t.Errorf("full ID: %v", resp.Diagnostics)
	}
	resp = run("x", nil)
	if resp.Diagnostics.HasError() || get(resp, "org_id") != "" || get(resp, "id") != "x" {
		t.Errorf("account-level ID: %v", resp.Diagnostics)
	}
	for _, bad := range []string{"", "a//b", "a/b/c/d"} {
		if resp := run(bad, nil); !resp.Diagnostics.HasError() {
			t.Errorf("%q must be rejected", bad)
		}
	}

	idSchema := identityschema.Schema{Attributes: IdentityAttributes(ImportMultiLevel)}
	if !idSchema.Attributes["identifier"].IsRequiredForImport() || !idSchema.Attributes["org_id"].IsOptionalForImport() {
		t.Error("identity: identifier required, org_id optional")
	}
	idType := idSchema.Type().TerraformType(ctx).(tftypes.Object)
	identity := &tfsdk.ResourceIdentity{Schema: idSchema, Raw: tftypes.NewValue(idType, map[string]tftypes.Value{
		"org_id":     tftypes.NewValue(tftypes.String, "o"),
		"project_id": tftypes.NewValue(tftypes.String, nil),
		"identifier": tftypes.NewValue(tftypes.String, "x"),
	})}
	resp = run("", identity)
	if resp.Diagnostics.HasError() || get(resp, "org_id") != "o" || get(resp, "project_id") != "" || get(resp, "id") != "x" {
		t.Errorf("identity import: %v", resp.Diagnostics)
	}
}

func TestZeroValueToNullUpgrader(t *testing.T) {
	s := testSchema()
	str := func(v string) tftypes.Value { return tftypes.NewValue(tftypes.String, v) }
	emptySet := tftypes.NewValue(tftypes.Set{ElementType: tftypes.String}, []tftypes.Value{})
	raw := nullObject(s, map[string]tftypes.Value{
		"id": str("x"), "identifier": str("x"), "name": str("n"), "description": str(""), "org_id": str(""),
		"tags": emptySet, "color": str(""), "modules": emptySet,
	})
	up := ZeroValueToNullUpgrader(s)
	resp := &resource.UpgradeStateResponse{State: tfsdk.State{Schema: s, Raw: tftypes.NewValue(raw.Type(), nil)}}
	up.StateUpgrader(ctx, resource.UpgradeStateRequest{State: &tfsdk.State{Schema: s, Raw: raw}}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}
	var attrs map[string]tftypes.Value
	if err := resp.State.Raw.As(&attrs); err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]bool{"description": true, "org_id": true, "tags": true, "name": false, "color": false, "modules": false, "id": false} {
		if attrs[name].IsNull() != want {
			t.Errorf("%s null = %v, want %v", name, attrs[name].IsNull(), want)
		}
	}
}
