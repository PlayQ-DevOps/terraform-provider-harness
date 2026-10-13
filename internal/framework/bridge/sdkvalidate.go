package bridge

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	sdk "github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// sdkValidator runs an SDK v2 ValidateFunc as a framework validator for primitive values.
type sdkValidator struct {
	f func(interface{}, string) ([]string, []error)
}

func (v sdkValidator) Description(context.Context) string {
	return "value accepted by the SDK v2 validation function"
}

func (v sdkValidator) MarkdownDescription(ctx context.Context) string { return v.Description(ctx) }

func (v sdkValidator) run(value interface{}, p interface{ String() string }, add func(summary, detail string, warning bool)) {
	warnings, errs := v.f(value, p.String())
	for _, w := range warnings {
		add("Validation warning", w, true)
	}
	for _, e := range errs {
		add("Invalid value", e.Error(), false)
	}
}

func (v sdkValidator) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	v.run(req.ConfigValue.ValueString(), req.Path, func(summary, detail string, warning bool) {
		if warning {
			resp.Diagnostics.AddAttributeWarning(req.Path, summary, detail)
		} else {
			resp.Diagnostics.AddAttributeError(req.Path, summary, detail)
		}
	})
}

func (v sdkValidator) ValidateInt64(_ context.Context, req validator.Int64Request, resp *validator.Int64Response) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	v.run(int(req.ConfigValue.ValueInt64()), req.Path, func(summary, detail string, warning bool) {
		if warning {
			resp.Diagnostics.AddAttributeWarning(req.Path, summary, detail)
		} else {
			resp.Diagnostics.AddAttributeError(req.Path, summary, detail)
		}
	})
}

func (v sdkValidator) ValidateBool(_ context.Context, req validator.BoolRequest, resp *validator.BoolResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	v.run(req.ConfigValue.ValueBool(), req.Path, func(summary, detail string, warning bool) {
		if warning {
			resp.Diagnostics.AddAttributeWarning(req.Path, summary, detail)
		} else {
			resp.Diagnostics.AddAttributeError(req.Path, summary, detail)
		}
	})
}

func (v sdkValidator) ValidateFloat64(_ context.Context, req validator.Float64Request, resp *validator.Float64Response) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	v.run(req.ConfigValue.ValueFloat64(), req.Path, func(summary, detail string, warning bool) {
		if warning {
			resp.Diagnostics.AddAttributeWarning(req.Path, summary, detail)
		} else {
			resp.Diagnostics.AddAttributeError(req.Path, summary, detail)
		}
	})
}

// suppressModifier runs a string attribute's SDK v2 DiffSuppressFunc as a plan modifier:
// when the function says the prior and planned values are the same, the prior value is
// planned, which is what the SDK did. The function must not use its ResourceData argument
// (nil is passed).
type suppressModifier struct {
	f   sdk.SchemaDiffSuppressFunc
	key string
}

func (m suppressModifier) Description(context.Context) string {
	return "keeps the prior value when the SDK v2 diff suppression function treats the values as equal"
}

func (m suppressModifier) MarkdownDescription(ctx context.Context) string { return m.Description(ctx) }

func (m suppressModifier) PlanModifyString(_ context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse) {
	if req.StateValue.IsNull() || req.StateValue.IsUnknown() || req.PlanValue.IsNull() || req.PlanValue.IsUnknown() {
		return
	}
	if m.f(m.key, req.StateValue.ValueString(), req.PlanValue.ValueString(), nil) {
		resp.PlanValue = req.StateValue
	}
}
