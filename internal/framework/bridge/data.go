package bridge

import (
	"context"
	"fmt"

	"github.com/hashicorp/go-cty/cty"
	ctymsgpack "github.com/hashicorp/go-cty/cty/msgpack"
	"github.com/hashicorp/terraform-plugin-go/tfprotov5"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	sdk "github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

// Shim turns framework plan and state values into the *schema.ResourceData that SDK v2
// style build and read functions take, and the ResourceData back into framework state.
// It uses the SDK's own value shims (the ones its gRPC server uses), so Get, GetOk and
// Set behave exactly as they did under the SDK.
type Shim struct {
	r  *sdk.Resource
	ty cty.Type
}

// NewShim prepares a shim for the SDK resource declaration r. Only its Schema is used:
// the CRUD functions, importer and CustomizeDiff play no part.
func NewShim(r *sdk.Resource) *Shim {
	decl := &sdk.Resource{Schema: r.Schema, SchemaVersion: r.SchemaVersion, Description: r.Description, DeprecationMessage: r.DeprecationMessage}
	return &Shim{r: decl, ty: decl.CoreConfigSchema().ImpliedType()}
}

// Schema is the SDK schema the shim was built from.
func (s *Shim) Schema() map[string]*sdk.Schema { return s.r.Schema }

func (s *Shim) toCty(v tftypes.Value) (cty.Value, error) {
	dv, err := tfprotov5.NewDynamicValue(v.Type(), v)
	if err != nil {
		return cty.NilVal, fmt.Errorf("encoding the value: %w", err)
	}
	out, err := ctymsgpack.Unmarshal(dv.MsgPack, s.ty)
	if err != nil {
		return cty.NilVal, fmt.Errorf("decoding the value with the SDK schema type: %w", err)
	}
	return out, nil
}

func (s *Shim) fromCty(v cty.Value, t tftypes.Type) (tftypes.Value, error) {
	b, err := ctymsgpack.Marshal(v, s.ty)
	if err != nil {
		return tftypes.Value{}, fmt.Errorf("encoding the SDK value: %w", err)
	}
	out, err := tfprotov5.DynamicValue{MsgPack: b}.Unmarshal(t)
	if err != nil {
		return tftypes.Value{}, fmt.Errorf("decoding the SDK value with the framework schema type: %w", err)
	}
	return out, nil
}

func (s *Shim) instanceState(v tftypes.Value) (*terraform.InstanceState, error) {
	if v.IsNull() {
		return nil, nil
	}
	c, err := s.toCty(v)
	if err != nil {
		return nil, err
	}
	return s.r.ShimInstanceStateFromValue(c)
}

// FromPlan returns the ResourceData a Create or Update sees: the planned values over the
// prior state (a null prior for Create). Id is the prior ID, empty for Create.
func (s *Shim) FromPlan(ctx context.Context, plan, prior tftypes.Value) (*sdk.ResourceData, error) {
	planCty, err := s.toCty(plan)
	if err != nil {
		return nil, err
	}
	state, err := s.instanceState(prior)
	if err != nil {
		return nil, err
	}
	cfg := terraform.NewResourceConfigShimmed(planCty, s.r.CoreConfigSchema())
	diff, err := s.r.SimpleDiff(ctx, state, cfg, nil)
	if err != nil {
		return nil, fmt.Errorf("diffing the plan: %w", err)
	}
	d, err := sdk.InternalMap(s.r.Schema).Data(state, diff)
	if err != nil {
		return nil, fmt.Errorf("building the resource data: %w", err)
	}
	return d, nil
}

// FromState returns the ResourceData a Read or Delete sees.
func (s *Shim) FromState(_ context.Context, state tftypes.Value) (*sdk.ResourceData, error) {
	is, err := s.instanceState(state)
	if err != nil {
		return nil, err
	}
	return s.r.Data(is), nil
}

// Result converts the ResourceData after the read function's Set calls into framework
// state of type t. prior is the plan (after Create or Update) or the previous state (after
// Read): the zero values the SDK writes for Optional attributes that were null in prior
// become null again. A ResourceData without an ID is a gone resource: the result is null.
func (s *Shim) Result(_ context.Context, d *sdk.ResourceData, prior tftypes.Value, t tftypes.Type) (tftypes.Value, error) {
	is := d.State()
	if is == nil {
		return tftypes.NewValue(t, nil), nil
	}
	obj, err := is.AttrsAsObjectValue(s.ty)
	if err != nil {
		return tftypes.Value{}, fmt.Errorf("reading the resource data: %w", err)
	}
	v, err := s.fromCty(obj, t)
	if err != nil {
		return tftypes.Value{}, err
	}
	return Nullify(v, prior, s.r.Schema), nil
}
