package framework

import (
	"context"

	"github.com/harness/terraform-provider-harness/helpers"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Optional attributes that are not Computed must come back from a Read in the shape the
// configuration gave them: an attribute that was never set is null, an attribute set to
// "" or [] is that. The API cannot tell the two apart (it returns "" either way), so the
// readers below keep the prior value's null-ness when the API value is empty.

// StringFromAPI returns v as a state value; an empty v keeps prior null.
func StringFromAPI(v string, prior types.String) types.String {
	if v == "" && prior.IsNull() {
		return types.StringNull()
	}
	return types.StringValue(v)
}

// TagsFromAPI returns tags as the set of "key:value" strings the schema holds; no tags keeps
// prior null.
func TagsFromAPI(tags map[string]string, prior types.Set) types.Set {
	if len(tags) == 0 && prior.IsNull() {
		return types.SetNull(types.StringType)
	}
	return StringSet(helpers.FlattenTags(tags))
}

// TagsToAPI converts the tags set to the map the API takes; null and unknown are empty.
func TagsToAPI(ctx context.Context, set types.Set, diags *diag.Diagnostics) map[string]string {
	elems := Strings(ctx, set, diags)
	ifaces := make([]interface{}, len(elems))
	for i, e := range elems {
		ifaces[i] = e
	}
	return helpers.ExpandTags(ifaces)
}

// StringSet returns values as a known set of strings (empty, not null, when there are none).
func StringSet(values []string) types.Set {
	elems := make([]attr.Value, len(values))
	for i, v := range values {
		elems[i] = types.StringValue(v)
	}
	return types.SetValueMust(types.StringType, elems)
}

// Strings returns the elements of a set of strings; null and unknown are empty.
func Strings(ctx context.Context, set types.Set, diags *diag.Diagnostics) []string {
	if set.IsNull() || set.IsUnknown() {
		return nil
	}
	var out []string
	diags.Append(set.ElementsAs(ctx, &out, false)...)
	return out
}

// ValueString returns the string of v, or "" when it is null or unknown.
func ValueString(v types.String) string {
	if v.IsNull() || v.IsUnknown() {
		return ""
	}
	return v.ValueString()
}
