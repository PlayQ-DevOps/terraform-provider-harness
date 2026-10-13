package bridge

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-go/tftypes"
	sdk "github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func TestShimRoundTrip(t *testing.T) {
	r := sample()
	fw, err := ResourceSchema(r)
	if err != nil {
		t.Fatal(err)
	}
	shim := NewShim(r)
	typ := fw.Type().TerraformType(ctx).(tftypes.Object)
	str := func(v string) tftypes.Value { return tftypes.NewValue(tftypes.String, v) }
	null := func(t tftypes.Type) tftypes.Value { return tftypes.NewValue(t, nil) }
	credType := typ.AttributeTypes["credentials"].(tftypes.List)
	credObj := credType.ElementType.(tftypes.Object)
	httpType := credObj.AttributeTypes["http"].(tftypes.List)
	httpObj := httpType.ElementType.(tftypes.Object)
	plan := tftypes.NewValue(typ, map[string]tftypes.Value{
		"id":         tftypes.NewValue(tftypes.String, tftypes.UnknownValue),
		"identifier": str("x"),
		"url":        str("https://example.com/"),
		"kind":       str("Account"),
		"enabled":    tftypes.NewValue(tftypes.Bool, true),
		"retries":    null(tftypes.Number),
		"selectors":  tftypes.NewValue(typ.AttributeTypes["selectors"], []tftypes.Value{str("a"), str("b")}),
		"labels":     null(typ.AttributeTypes["labels"]),
		"token":      null(tftypes.String),
		"credentials": tftypes.NewValue(credType, []tftypes.Value{tftypes.NewValue(credObj, map[string]tftypes.Value{
			"http": tftypes.NewValue(httpType, []tftypes.Value{tftypes.NewValue(httpObj, map[string]tftypes.Value{
				"username": str("u"), "username_ref": null(tftypes.String), "token_ref": str("account.tok"),
			})}),
			"ssh": null(credObj.AttributeTypes["ssh"]),
		})}),
		"status": tftypes.NewValue(typ.AttributeTypes["status"], tftypes.UnknownValue),
	})
	d, err := shim.FromPlan(ctx, plan, null(typ))
	if err != nil {
		t.Fatal(err)
	}
	if d.Id() != "" || d.Get("identifier") != "x" || d.Get("kind") != "Account" || d.Get("enabled") != true {
		t.Errorf("plan values: id %q identifier %v kind %v enabled %v", d.Id(), d.Get("identifier"), d.Get("kind"), d.Get("enabled"))
	}
	if got := d.Get("selectors").(*sdk.Set).List(); len(got) != 2 {
		t.Errorf("sets come back as *schema.Set: %v", got)
	}
	if u := d.Get("credentials.0.http.0.username"); u != "u" {
		t.Errorf("nested value: %v", u)
	}
	if _, ok := d.GetOk("retries"); ok {
		t.Error("a null attribute is not ok")
	}

	// The read side: what an SDK read function writes, including zero values.
	d.SetId("x")
	_ = d.Set("url", "https://example.com/")
	_ = d.Set("retries", 0)
	_ = d.Set("token", "")
	_ = d.Set("labels", map[string]interface{}{})
	_ = d.Set("credentials", []interface{}{map[string]interface{}{
		"http": []interface{}{map[string]interface{}{"username": "u", "username_ref": "", "token_ref": "account.tok"}},
		"ssh":  []interface{}{},
	}})
	_ = d.Set("status", []interface{}{map[string]interface{}{"state": "ok"}})
	out, err := shim.Result(ctx, d, plan, typ)
	if err != nil {
		t.Fatal(err)
	}
	var attrs map[string]tftypes.Value
	if err := out.As(&attrs); err != nil {
		t.Fatal(err)
	}
	for name, wantNull := range map[string]bool{"retries": true, "token": true, "labels": true, "url": false, "selectors": false, "status": false, "id": false} {
		if attrs[name].IsNull() != wantNull {
			t.Errorf("%s null = %v, want %v (%v)", name, attrs[name].IsNull(), wantNull, attrs[name])
		}
	}
	var creds []tftypes.Value
	_ = attrs["credentials"].As(&creds)
	var cred map[string]tftypes.Value
	_ = creds[0].As(&cred)
	if cred["ssh"].IsNull() {
		t.Errorf("an empty block list stays an empty list (never null): %v", cred["ssh"])
	}
	var https []tftypes.Value
	_ = cred["http"].As(&https)
	var h map[string]tftypes.Value
	_ = https[0].As(&h)
	if !h["username_ref"].IsNull() || h["username"].IsNull() {
		t.Errorf("nested zero values follow the plan: %v", h)
	}

	// The state side, as a Read sees it, and the upgrade of SDK-written state.
	ds, err := shim.FromState(ctx, out)
	if err != nil {
		t.Fatal(err)
	}
	if ds.Id() != "x" || ds.Get("credentials.0.http.0.token_ref") != "account.tok" {
		t.Errorf("state values: id %q token_ref %v", ds.Id(), ds.Get("credentials.0.http.0.token_ref"))
	}
}
