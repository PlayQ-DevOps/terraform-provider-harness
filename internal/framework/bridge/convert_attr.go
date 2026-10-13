package bridge

import (
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/float64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/mapplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	sdk "github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// flags are the attribute flags shared by every converted attribute. An attribute with a
// Default is Computed, as the framework requires for defaults.
func flags(s *sdk.Schema) (required, optional, computed bool) {
	return s.Required, s.Optional, s.Computed || s.Default != nil
}

// keepState says whether a Computed attribute keeps its state value through a plan
// (UseStateForUnknown): every Computed attribute without a default.
func keepState(s *sdk.Schema) bool { return s.Computed && s.Default == nil }

func convertAttribute(key string, s *sdk.Schema, root map[string]*sdk.Schema) (schema.Attribute, error) {
	pv, err := constraintsOf(s, key, root)
	if err != nil {
		return nil, err
	}
	required, optional, computed := flags(s)
	switch s.Type {
	case sdk.TypeString:
		a := schema.StringAttribute{Required: required, Optional: optional, Computed: computed, Sensitive: s.Sensitive, MarkdownDescription: s.Description, DeprecationMessage: s.Deprecated}
		if s.Default != nil {
			a.Default = stringdefault.StaticString(fmt.Sprint(s.Default))
		}
		if s.ValidateFunc != nil {
			a.Validators = append(a.Validators, sdkValidator{s.ValidateFunc})
		}
		a.Validators = append(a.Validators, stringConstraints(pv)...)
		if s.ForceNew {
			a.PlanModifiers = append(a.PlanModifiers, stringplanmodifier.RequiresReplace())
		}
		if keepState(s) {
			a.PlanModifiers = append(a.PlanModifiers, stringplanmodifier.UseStateForUnknown())
		}
		if s.DiffSuppressFunc != nil {
			a.PlanModifiers = append(a.PlanModifiers, suppressModifier{s.DiffSuppressFunc, key})
		}
		return a, nil
	case sdk.TypeBool:
		a := schema.BoolAttribute{Required: required, Optional: optional, Computed: computed, Sensitive: s.Sensitive, MarkdownDescription: s.Description, DeprecationMessage: s.Deprecated}
		if b, ok := s.Default.(bool); ok {
			a.Default = booldefault.StaticBool(b)
		}
		if s.ValidateFunc != nil {
			a.Validators = append(a.Validators, sdkValidator{s.ValidateFunc})
		}
		a.Validators = append(a.Validators, boolConstraints(pv)...)
		if s.ForceNew {
			a.PlanModifiers = append(a.PlanModifiers, boolplanmodifier.RequiresReplace())
		}
		if keepState(s) {
			a.PlanModifiers = append(a.PlanModifiers, boolplanmodifier.UseStateForUnknown())
		}
		return a, nil
	case sdk.TypeInt:
		a := schema.Int64Attribute{Required: required, Optional: optional, Computed: computed, Sensitive: s.Sensitive, MarkdownDescription: s.Description, DeprecationMessage: s.Deprecated}
		if i, ok := s.Default.(int); ok {
			a.Default = int64default.StaticInt64(int64(i))
		}
		if s.ValidateFunc != nil {
			a.Validators = append(a.Validators, sdkValidator{s.ValidateFunc})
		}
		a.Validators = append(a.Validators, int64Constraints(pv)...)
		if s.ForceNew {
			a.PlanModifiers = append(a.PlanModifiers, int64planmodifier.RequiresReplace())
		}
		if keepState(s) {
			a.PlanModifiers = append(a.PlanModifiers, int64planmodifier.UseStateForUnknown())
		}
		return a, nil
	case sdk.TypeFloat:
		a := schema.Float64Attribute{Required: required, Optional: optional, Computed: computed, Sensitive: s.Sensitive, MarkdownDescription: s.Description, DeprecationMessage: s.Deprecated}
		if s.ValidateFunc != nil {
			a.Validators = append(a.Validators, sdkValidator{s.ValidateFunc})
		}
		a.Validators = append(a.Validators, float64Constraints(pv)...)
		if s.ForceNew {
			a.PlanModifiers = append(a.PlanModifiers, float64planmodifier.RequiresReplace())
		}
		if keepState(s) {
			a.PlanModifiers = append(a.PlanModifiers, float64planmodifier.UseStateForUnknown())
		}
		return a, nil
	}
	elem, err := elemType(s)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", key, err)
	}
	switch s.Type {
	case sdk.TypeList:
		a := schema.ListAttribute{ElementType: elem, Required: required, Optional: optional, Computed: computed, Sensitive: s.Sensitive, MarkdownDescription: s.Description, DeprecationMessage: s.Deprecated}
		if s.MaxItems > 0 {
			a.Validators = append(a.Validators, listvalidator.SizeAtMost(s.MaxItems))
		}
		if s.MinItems > 0 {
			a.Validators = append(a.Validators, listvalidator.SizeAtLeast(s.MinItems))
		}
		a.Validators = append(a.Validators, listConstraints(pv)...)
		if s.ForceNew {
			a.PlanModifiers = append(a.PlanModifiers, listplanmodifier.RequiresReplace())
		}
		if keepState(s) {
			a.PlanModifiers = append(a.PlanModifiers, listplanmodifier.UseStateForUnknown())
		}
		return a, nil
	case sdk.TypeSet:
		a := schema.SetAttribute{ElementType: elem, Required: required, Optional: optional, Computed: computed, Sensitive: s.Sensitive, MarkdownDescription: s.Description, DeprecationMessage: s.Deprecated}
		if s.MaxItems > 0 {
			a.Validators = append(a.Validators, setvalidator.SizeAtMost(s.MaxItems))
		}
		if s.MinItems > 0 {
			a.Validators = append(a.Validators, setvalidator.SizeAtLeast(s.MinItems))
		}
		a.Validators = append(a.Validators, setConstraints(pv)...)
		if s.ForceNew {
			a.PlanModifiers = append(a.PlanModifiers, setplanmodifier.RequiresReplace())
		}
		if keepState(s) {
			a.PlanModifiers = append(a.PlanModifiers, setplanmodifier.UseStateForUnknown())
		}
		return a, nil
	case sdk.TypeMap:
		a := schema.MapAttribute{ElementType: elem, Required: required, Optional: optional, Computed: computed, Sensitive: s.Sensitive, MarkdownDescription: s.Description, DeprecationMessage: s.Deprecated}
		a.Validators = append(a.Validators, mapConstraints(pv)...)
		if s.ForceNew {
			a.PlanModifiers = append(a.PlanModifiers, mapplanmodifier.RequiresReplace())
		}
		if keepState(s) {
			a.PlanModifiers = append(a.PlanModifiers, mapplanmodifier.UseStateForUnknown())
		}
		return a, nil
	}
	return nil, fmt.Errorf("%s: unsupported SDK type %s", key, s.Type)
}
