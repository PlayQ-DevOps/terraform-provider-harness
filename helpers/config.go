package helpers

import (
	"sort"

	"github.com/antihax/optional"
	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// GetBool returns the value of a boolean attribute and whether it is set. Unlike d.GetOk,
// which reports an explicit false as "not set", it reads the configuration so that
// `flag = false` can be sent to the API. Where there is no configuration (a Read, an
// import) it falls back to the state through GetOkExists.
func GetBool(d *schema.ResourceData, key string) (bool, bool) {
	if v, ok := rawConfigValue(d, key); ok {
		if v.Type() == cty.Bool {
			return v.True(), true
		}
		return false, false
	}
	v, ok := d.GetOkExists(key) //nolint:staticcheck // deprecated, but the only state-side "is it set"; reached when there is no configuration (read, delete, import) or the attribute is omitted
	if !ok {
		return false, false
	}
	b, _ := v.(bool)
	return b, true
}

// GetInt is GetBool for integer attributes: an explicit zero counts as set.
func GetInt(d *schema.ResourceData, key string) (int, bool) {
	if v, ok := rawConfigValue(d, key); ok {
		if v.Type() == cty.Number {
			i, _ := v.AsBigFloat().Int64()
			return int(i), true
		}
		return 0, false
	}
	v, ok := d.GetOkExists(key) //nolint:staticcheck // deprecated, but the only state-side "is it set"; reached when there is no configuration (read, delete, import) or the attribute is omitted
	if !ok {
		return 0, false
	}
	i, _ := v.(int)
	return i, true
}

// rawConfigValue returns the known, non-null configuration value at key, if the
// configuration is available and sets it.
func rawConfigValue(d *schema.ResourceData, key string) (cty.Value, bool) {
	v, diags := d.GetRawConfigAt(parseCtyPath(key))
	if diags.HasError() || v == cty.NilVal || !v.IsKnown() || v.IsNull() {
		return cty.NilVal, false
	}
	return v, true
}

// BuildFieldBool returns the attribute as an optional.Bool that is set when the
// configuration (or, outside plan and apply, the state) sets it, including to false.
func BuildFieldBool(d *schema.ResourceData, field string) optional.Bool {
	if b, ok := GetBool(d, field); ok {
		return optional.NewBool(b)
	}
	return optional.EmptyBool()
}

// BuildFieldInt32 is BuildFieldBool for integer attributes.
func BuildFieldInt32(d *schema.ResourceData, field string) optional.Int32 {
	if i, ok := GetInt(d, field); ok {
		return optional.NewInt32(int32(i))
	}
	return optional.EmptyInt32()
}

// SetAll sets every attribute in values and returns the first error, in key order, so a
// Read reports a state write it could not make instead of ignoring it:
//
//	if err := helpers.SetAll(d, map[string]interface{}{"name": obj.Name, "tags": tags}); err != nil {
//		return diag.FromErr(err)
//	}
func SetAll(d *schema.ResourceData, values map[string]interface{}) error {
	keys := make([]string, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if err := d.Set(k, values[k]); err != nil {
			return err
		}
	}
	return nil
}
