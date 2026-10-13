package framework

import (
	"context"
	"fmt"
	"math/big"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// ZeroValueToNullUpgrader upgrades state written by the SDK v2 implementation of a
// resource. The SDK stored "", 0, false or an empty collection for Optional attributes the
// configuration never set; the framework reads those as values and would plan an update to
// null. The upgrader turns such zero values of Optional, non-Computed attributes into null
// and copies everything else, so the first plan after the migration is empty. It is for
// resources whose attributes did not change shape; a resource that adds or retypes
// attributes writes its own upgrader.
func ZeroValueToNullUpgrader(prior schema.Schema) resource.StateUpgrader {
	return resource.StateUpgrader{
		PriorSchema: &prior,
		StateUpgrader: func(ctx context.Context, req resource.UpgradeStateRequest, resp *resource.UpgradeStateResponse) {
			if req.State == nil {
				resp.Diagnostics.AddError("State upgrade failed", "the prior state is not available")
				return
			}
			raw, err := nullZeroValues(req.State.Raw, prior)
			if err != nil {
				resp.Diagnostics.AddError("State upgrade failed", err.Error())
				return
			}
			resp.State.Raw = raw
		},
	}
}

// nullZeroValues returns state with the zero values of Optional, non-Computed top-level
// attributes replaced by null.
func nullZeroValues(state tftypes.Value, prior schema.Schema) (tftypes.Value, error) {
	if state.IsNull() || !state.IsKnown() {
		return state, nil
	}
	var attrs map[string]tftypes.Value
	if err := state.As(&attrs); err != nil {
		return state, fmt.Errorf("decoding the prior state: %w", err)
	}
	out := make(map[string]tftypes.Value, len(attrs))
	for name, v := range attrs {
		a, ok := prior.Attributes[name]
		if ok && a.IsOptional() && !a.IsComputed() && isZero(v) {
			out[name] = tftypes.NewValue(v.Type(), nil)
			continue
		}
		out[name] = v
	}
	return tftypes.NewValue(state.Type(), out), nil
}

// isZero reports whether v is a known "", 0, false or empty collection.
func isZero(v tftypes.Value) bool {
	if v.IsNull() || !v.IsKnown() {
		return false
	}
	switch {
	case v.Type().Is(tftypes.String):
		var s string
		return v.As(&s) == nil && s == ""
	case v.Type().Is(tftypes.Bool):
		var b bool
		return v.As(&b) == nil && !b
	case v.Type().Is(tftypes.Number):
		var n *big.Float
		return v.As(&n) == nil && n != nil && n.Sign() == 0
	case v.Type().Is(tftypes.List{}) || v.Type().Is(tftypes.Set{}):
		var elems []tftypes.Value
		return v.As(&elems) == nil && len(elems) == 0
	case v.Type().Is(tftypes.Map{}):
		var elems map[string]tftypes.Value
		return v.As(&elems) == nil && len(elems) == 0
	}
	return false
}
