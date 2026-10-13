package bridge

import (
	"context"
	"math/big"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	sdk "github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// Nullify returns v with the zero values the SDK v2 writes for Optional, non-Computed
// attributes that the configuration never set turned back into null, so the framework
// does not plan an update from "" to null. prior says which attributes were never set: a
// zero value is made null only where prior is null (or absent). It recurses into nested
// blocks; list elements are matched with the prior element at the same index, set elements
// against a null prior when the prior set is null.
func Nullify(v, prior tftypes.Value, m map[string]*sdk.Schema) tftypes.Value {
	if v.IsNull() || !v.IsKnown() {
		return v
	}
	var attrs map[string]tftypes.Value
	if err := v.As(&attrs); err != nil {
		return v
	}
	var priorAttrs map[string]tftypes.Value
	if known(prior) {
		_ = prior.As(&priorAttrs)
	}
	out := make(map[string]tftypes.Value, len(attrs))
	for name, val := range attrs {
		p, hasPrior := priorAttrs[name]
		priorNull := !hasPrior || !known(p)
		s := m[name]
		switch {
		case s == nil:
			out[name] = val
		case isNestedResource(s):
			out[name] = nullifyNested(val, p, priorNull, s)
		case s.Optional && !s.Computed && s.Default == nil && priorNull && isZero(val):
			out[name] = tftypes.NewValue(val.Type(), nil)
		default:
			out[name] = val
		}
	}
	return tftypes.NewValue(v.Type(), out)
}

func known(v tftypes.Value) bool { return v.Type() != nil && !v.IsNull() && v.IsKnown() }

func isNestedResource(s *sdk.Schema) bool {
	_, ok := s.Elem.(*sdk.Resource)
	return ok && s.Type != sdk.TypeMap
}

func nullifyNested(val, prior tftypes.Value, priorNull bool, s *sdk.Schema) tftypes.Value {
	if !known(val) {
		return val
	}
	res := s.Elem.(*sdk.Resource)
	var elems []tftypes.Value
	if err := val.As(&elems); err != nil {
		return val
	}
	// A block list is never made null: Terraform gives an empty list for absent blocks, so
	// the empty list is the unset value. Only the attributes inside are nullified.
	if len(elems) == 0 {
		return val
	}
	var priorElems []tftypes.Value
	if !priorNull {
		_ = prior.As(&priorElems)
	}
	out := make([]tftypes.Value, len(elems))
	for i, e := range elems {
		var pe tftypes.Value
		if s.Type == sdk.TypeList && i < len(priorElems) {
			pe = priorElems[i]
		}
		out[i] = Nullify(e, pe, res.Schema)
	}
	return tftypes.NewValue(val.Type(), out)
}

// isZero reports whether v is a known "", 0, false or empty collection.
func isZero(v tftypes.Value) bool {
	if !known(v) {
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

// StateUpgrader upgrades state written by the SDK v2 implementation of a resource whose
// schema did not change shape: every zero value of an Optional, non-Computed attribute,
// at any depth, becomes null. prior is the framework schema at version 0.
func StateUpgrader(prior schema.Schema, m map[string]*sdk.Schema) resource.StateUpgrader {
	return resource.StateUpgrader{
		PriorSchema: &prior,
		StateUpgrader: func(_ context.Context, req resource.UpgradeStateRequest, resp *resource.UpgradeStateResponse) {
			if req.State == nil {
				resp.Diagnostics.AddError("State upgrade failed", "the prior state is not available")
				return
			}
			resp.State.Raw = Nullify(req.State.Raw, tftypes.NewValue(req.State.Raw.Type(), nil), m)
		},
	}
}
