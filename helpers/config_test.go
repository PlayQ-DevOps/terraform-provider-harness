package helpers

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	sdkterraform "github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

var configTestSchema = map[string]*schema.Schema{
	"flag":  {Type: schema.TypeBool, Optional: true},
	"count": {Type: schema.TypeInt, Optional: true},
	"name":  {Type: schema.TypeString, Optional: true},
	"block": {Type: schema.TypeList, Optional: true, MaxItems: 1, Elem: &schema.Resource{Schema: map[string]*schema.Schema{
		"inner": {Type: schema.TypeBool, Optional: true},
	}}},
}

// dataWithConfig builds a ResourceData the way apply does: with the raw configuration
// attached, so GetRawConfigAt can see what the configuration sets.
func dataWithConfig(t *testing.T, cfg map[string]interface{}) *schema.ResourceData {
	t.Helper()
	r := &schema.Resource{Schema: configTestSchema}
	diff, err := r.Diff(context.Background(), nil, sdkterraform.NewResourceConfigRaw(cfg), nil)
	if err != nil {
		t.Fatal(err)
	}
	rd, err := schema.InternalMap(configTestSchema).Data(nil, diff)
	if err != nil {
		t.Fatal(err)
	}
	return rd
}

func TestGetBoolAndGetInt(t *testing.T) {
	// Explicit false and zero are "set" from the configuration; omitted ones are not.
	d := dataWithConfig(t, map[string]interface{}{"flag": false, "count": 0, "name": "x"})
	if _, ok := GetBool(d, "flag"); !ok {
		t.Error("an explicit false is set")
	}
	if _, ok := GetInt(d, "count"); !ok {
		t.Error("an explicit zero is set")
	}
	if v, ok := GetBool(d, "flag"); v || !ok {
		t.Errorf("flag = %v, %v", v, ok)
	}
	if b := BuildFieldBool(d, "flag"); !b.IsSet() || b.Value() {
		t.Errorf("BuildFieldBool(false) = set %v value %v", b.IsSet(), b.Value())
	}
	d = dataWithConfig(t, map[string]interface{}{"name": "x"})
	if _, ok := GetBool(d, "flag"); ok {
		t.Error("an omitted flag is not set")
	}
	if _, ok := GetInt(d, "count"); ok {
		t.Error("an omitted count is not set")
	}
	if BuildFieldBool(d, "flag").IsSet() || BuildFieldInt32(d, "count").IsSet() {
		t.Error("omitted attributes must not be sent")
	}
	d = dataWithConfig(t, map[string]interface{}{"flag": true, "count": 7, "block": []interface{}{map[string]interface{}{"inner": false}}})
	if v, ok := GetBool(d, "flag"); !v || !ok {
		t.Errorf("flag = %v, %v", v, ok)
	}
	if v, ok := GetInt(d, "count"); v != 7 || !ok {
		t.Errorf("count = %v, %v", v, ok)
	}
	if i := BuildFieldInt32(d, "count"); !i.IsSet() || i.Value() != 7 {
		t.Errorf("BuildFieldInt32 = %v %v", i.IsSet(), i.Value())
	}
	if v, ok := GetBool(d, "block.0.inner"); v || !ok {
		t.Errorf("nested explicit false: %v, %v", v, ok)
	}
}

func TestGetBoolWithoutConfigFallsBackToState(t *testing.T) {
	// A Read has no configuration: the state value counts.
	d := schema.TestResourceDataRaw(t, configTestSchema, map[string]interface{}{})
	_ = d.Set("flag", false)
	_ = d.Set("count", 0)
	if v, ok := GetBool(d, "flag"); v || !ok {
		t.Errorf("state false: %v, %v", v, ok)
	}
	if v, ok := GetInt(d, "count"); v != 0 || !ok {
		t.Errorf("state zero: %v, %v", v, ok)
	}
	if _, ok := GetBool(d, "name"); ok {
		t.Error("a non-boolean attribute is never a set boolean")
	}
}

func TestSetAll(t *testing.T) {
	d := schema.TestResourceDataRaw(t, configTestSchema, map[string]interface{}{})
	if err := SetAll(d, map[string]interface{}{"name": "n", "flag": true, "count": 3}); err != nil {
		t.Fatal(err)
	}
	if d.Get("name") != "n" || d.Get("flag") != true || d.Get("count") != 3 {
		t.Errorf("values not set: %v %v %v", d.Get("name"), d.Get("flag"), d.Get("count"))
	}
	if err := SetAll(d, map[string]interface{}{"name": "n", "missing": 1}); err == nil {
		t.Error("an unknown attribute must be reported")
	}
}
