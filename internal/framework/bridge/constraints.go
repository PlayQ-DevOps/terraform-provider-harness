package bridge

import (
	"github.com/hashicorp/terraform-plugin-framework-validators/boolvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/float64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/mapvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

// The cross-attribute constraints as typed framework validators.

func stringConstraints(pv pathValidators) []validator.String {
	var out []validator.String
	if len(pv.exactlyOneOf) > 0 {
		out = append(out, stringvalidator.ExactlyOneOf(pv.exactlyOneOf...))
	}
	if len(pv.conflictsWith) > 0 {
		out = append(out, stringvalidator.ConflictsWith(pv.conflictsWith...))
	}
	if len(pv.atLeastOneOf) > 0 {
		out = append(out, stringvalidator.AtLeastOneOf(pv.atLeastOneOf...))
	}
	if len(pv.alsoRequires) > 0 {
		out = append(out, stringvalidator.AlsoRequires(pv.alsoRequires...))
	}
	return out
}

func boolConstraints(pv pathValidators) []validator.Bool {
	var out []validator.Bool
	if len(pv.exactlyOneOf) > 0 {
		out = append(out, boolvalidator.ExactlyOneOf(pv.exactlyOneOf...))
	}
	if len(pv.conflictsWith) > 0 {
		out = append(out, boolvalidator.ConflictsWith(pv.conflictsWith...))
	}
	if len(pv.atLeastOneOf) > 0 {
		out = append(out, boolvalidator.AtLeastOneOf(pv.atLeastOneOf...))
	}
	if len(pv.alsoRequires) > 0 {
		out = append(out, boolvalidator.AlsoRequires(pv.alsoRequires...))
	}
	return out
}

func int64Constraints(pv pathValidators) []validator.Int64 {
	var out []validator.Int64
	if len(pv.exactlyOneOf) > 0 {
		out = append(out, int64validator.ExactlyOneOf(pv.exactlyOneOf...))
	}
	if len(pv.conflictsWith) > 0 {
		out = append(out, int64validator.ConflictsWith(pv.conflictsWith...))
	}
	if len(pv.atLeastOneOf) > 0 {
		out = append(out, int64validator.AtLeastOneOf(pv.atLeastOneOf...))
	}
	if len(pv.alsoRequires) > 0 {
		out = append(out, int64validator.AlsoRequires(pv.alsoRequires...))
	}
	return out
}

func float64Constraints(pv pathValidators) []validator.Float64 {
	var out []validator.Float64
	if len(pv.exactlyOneOf) > 0 {
		out = append(out, float64validator.ExactlyOneOf(pv.exactlyOneOf...))
	}
	if len(pv.conflictsWith) > 0 {
		out = append(out, float64validator.ConflictsWith(pv.conflictsWith...))
	}
	if len(pv.atLeastOneOf) > 0 {
		out = append(out, float64validator.AtLeastOneOf(pv.atLeastOneOf...))
	}
	if len(pv.alsoRequires) > 0 {
		out = append(out, float64validator.AlsoRequires(pv.alsoRequires...))
	}
	return out
}

func listConstraints(pv pathValidators) []validator.List {
	var out []validator.List
	if len(pv.exactlyOneOf) > 0 {
		out = append(out, listvalidator.ExactlyOneOf(pv.exactlyOneOf...))
	}
	if len(pv.conflictsWith) > 0 {
		out = append(out, listvalidator.ConflictsWith(pv.conflictsWith...))
	}
	if len(pv.atLeastOneOf) > 0 {
		out = append(out, listvalidator.AtLeastOneOf(pv.atLeastOneOf...))
	}
	if len(pv.alsoRequires) > 0 {
		out = append(out, listvalidator.AlsoRequires(pv.alsoRequires...))
	}
	return out
}

func setConstraints(pv pathValidators) []validator.Set {
	var out []validator.Set
	if len(pv.exactlyOneOf) > 0 {
		out = append(out, setvalidator.ExactlyOneOf(pv.exactlyOneOf...))
	}
	if len(pv.conflictsWith) > 0 {
		out = append(out, setvalidator.ConflictsWith(pv.conflictsWith...))
	}
	if len(pv.atLeastOneOf) > 0 {
		out = append(out, setvalidator.AtLeastOneOf(pv.atLeastOneOf...))
	}
	if len(pv.alsoRequires) > 0 {
		out = append(out, setvalidator.AlsoRequires(pv.alsoRequires...))
	}
	return out
}

func mapConstraints(pv pathValidators) []validator.Map {
	var out []validator.Map
	if len(pv.exactlyOneOf) > 0 {
		out = append(out, mapvalidator.ExactlyOneOf(pv.exactlyOneOf...))
	}
	if len(pv.conflictsWith) > 0 {
		out = append(out, mapvalidator.ConflictsWith(pv.conflictsWith...))
	}
	if len(pv.atLeastOneOf) > 0 {
		out = append(out, mapvalidator.AtLeastOneOf(pv.atLeastOneOf...))
	}
	if len(pv.alsoRequires) > 0 {
		out = append(out, mapvalidator.AlsoRequires(pv.alsoRequires...))
	}
	return out
}
