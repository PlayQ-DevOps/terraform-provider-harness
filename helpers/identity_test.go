package helpers

import (
	"context"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	sdkterraform "github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func TestMarkIdentityAndBuilders(t *testing.T) {
	s := map[string]*schema.Schema{}
	SetMultiLevelResourceSchema(s)
	for _, k := range []string{"identifier", "org_id", "project_id"} {
		if !s[k].ForceNew {
			t.Errorf("SetMultiLevelResourceSchema: %s must be ForceNew", k)
		}
	}
	if s["name"].ForceNew || s["description"].ForceNew {
		t.Error("name and description must not be ForceNew")
	}
	ds := map[string]*schema.Schema{}
	SetMultiLevelDatasourceSchema(ds)
	for k, attr := range ds {
		if attr.ForceNew {
			t.Errorf("data source schema: %s must not be ForceNew", k)
		}
	}
	if GetIdentifierSchema(SchemaFlagTypes.Required).ForceNew || GetOrgIdSchema(SchemaFlagTypes.Required).ForceNew {
		t.Error("the builders must leave ForceNew to MarkIdentity")
	}
	if !Identity(GetOrgIdSchema(SchemaFlagTypes.Optional)).ForceNew {
		t.Error("Identity must set ForceNew")
	}
	got := WithIdentity(map[string]*schema.Schema{"a": {Type: schema.TypeString, Optional: true}, "b": {Type: schema.TypeString, Optional: true}}, "a", "missing")
	if !got["a"].ForceNew || got["b"].ForceNew {
		t.Errorf("WithIdentity: a=%v b=%v", got["a"].ForceNew, got["b"].ForceNew)
	}
}

// A scope change on a resource built with the shared setters plans a replacement.
func TestScopeChangePlansReplacement(t *testing.T) {
	s := map[string]*schema.Schema{"spec": {Type: schema.TypeString, Optional: true}}
	SetMultiLevelResourceSchema(s)
	r := &schema.Resource{Schema: s}
	state := &sdkterraform.InstanceState{ID: "obj", Attributes: map[string]string{
		"id": "obj", "identifier": "obj", "name": "Obj", "org_id": "o", "project_id": "p", "spec": "x",
	}}
	cases := map[string]map[string]interface{}{
		"identifier": {"identifier": "obj2", "name": "Obj", "org_id": "o", "project_id": "p", "spec": "x"},
		"org_id":     {"identifier": "obj", "name": "Obj", "org_id": "o2", "project_id": "p", "spec": "x"},
		"project_id": {"identifier": "obj", "name": "Obj", "org_id": "o", "project_id": "p2", "spec": "x"},
		"spec":       {"identifier": "obj", "name": "Obj", "org_id": "o", "project_id": "p", "spec": "y"},
	}
	for attr, cfg := range cases {
		diff, err := r.Diff(context.Background(), state, sdkterraform.NewResourceConfigRaw(cfg), nil)
		if err != nil {
			t.Fatalf("%s: %v", attr, err)
		}
		if diff == nil {
			t.Fatalf("%s: expected a diff", attr)
		}
		if replace := diff.RequiresNew(); replace != (attr != "spec") {
			t.Errorf("changing %s: RequiresNew = %v", attr, replace)
		}
	}
}

func TestCreateOnly(t *testing.T) {
	r := &schema.Resource{
		Schema: map[string]*schema.Schema{
			"seed": {Type: schema.TypeString, Optional: true},
			"name": {Type: schema.TypeString, Optional: true},
		},
		CustomizeDiff: CreateOnly("seed"),
	}
	state := &sdkterraform.InstanceState{ID: "obj", Attributes: map[string]string{"id": "obj", "seed": "a", "name": "n"}}
	if _, err := r.Diff(context.Background(), state, sdkterraform.NewResourceConfigRaw(map[string]interface{}{"seed": "b", "name": "n"}), nil); err == nil || !strings.Contains(err.Error(), "seed can only be set when the resource is created") {
		t.Errorf("changing a create-only input must be a plan-time error, got %v", err)
	}
	if _, err := r.Diff(context.Background(), state, sdkterraform.NewResourceConfigRaw(map[string]interface{}{"seed": "a", "name": "m"}), nil); err != nil {
		t.Errorf("changing another input must plan: %v", err)
	}
	if _, err := r.Diff(context.Background(), nil, sdkterraform.NewResourceConfigRaw(map[string]interface{}{"seed": "b"}), nil); err != nil {
		t.Errorf("create must not be affected: %v", err)
	}
}
