package bridge

import (
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	sdk "github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// ResourceSchema converts an SDK v2 resource schema into the framework schema with the
// same wire shape:
//   - TypeString/Bool/Int/Float become String/Bool/Int64/Float64 attributes; lists, sets
//     and maps of primitives become List/Set/Map attributes;
//   - a list or set of a nested Resource becomes a nested block when it is settable and a
//     list or set of objects attribute when it is Computed only, as the SDK does;
//   - ForceNew becomes RequiresReplace; Computed attributes get UseStateForUnknown;
//     Default becomes Computed plus a static default;
//   - ValidateFunc, ExactlyOneOf, ConflictsWith, AtLeastOneOf, RequiredWith, MinItems and
//     MaxItems become validators; a Required block gets a minimum size of one;
//   - a string DiffSuppressFunc becomes a plan modifier (see suppressModifier);
//   - Description is Markdown, Deprecated is the deprecation message, Sensitive is kept.
//
// The implicit SDK "id" attribute is added as Computed.
func ResourceSchema(r *sdk.Resource) (schema.Schema, error) {
	attrs, blocks, err := convertBlock(r.Schema, r.Schema, "")
	if err != nil {
		return schema.Schema{}, err
	}
	if _, ok := attrs["id"]; !ok {
		attrs["id"] = schema.StringAttribute{Computed: true, MarkdownDescription: "The ID of this resource.",
			PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}}
	}
	return schema.Schema{Attributes: attrs, Blocks: blocks, MarkdownDescription: r.Description, DeprecationMessage: r.DeprecationMessage}, nil
}

func convertBlock(m, root map[string]*sdk.Schema, prefix string) (map[string]schema.Attribute, map[string]schema.Block, error) {
	attrs := map[string]schema.Attribute{}
	blocks := map[string]schema.Block{}
	for name, s := range m {
		key := joinKey(prefix, name)
		if res, ok := s.Elem.(*sdk.Resource); ok && s.Type != sdk.TypeMap {
			if s.Computed && !s.Optional {
				a, err := computedNested(s, res)
				if err != nil {
					return nil, nil, fmt.Errorf("%s: %w", key, err)
				}
				attrs[name] = a
				continue
			}
			b, err := convertNestedBlock(key, s, res, root)
			if err != nil {
				return nil, nil, err
			}
			blocks[name] = b
			continue
		}
		a, err := convertAttribute(key, s, root)
		if err != nil {
			return nil, nil, err
		}
		attrs[name] = a
	}
	return attrs, blocks, nil
}

func convertNestedBlock(key string, s *sdk.Schema, res *sdk.Resource, root map[string]*sdk.Schema) (schema.Block, error) {
	nestedAttrs, nestedBlocks, err := convertBlock(res.Schema, root, key)
	if err != nil {
		return nil, err
	}
	obj := schema.NestedBlockObject{Attributes: nestedAttrs, Blocks: nestedBlocks}
	pv, err := constraintsOf(s, key, root)
	if err != nil {
		return nil, err
	}
	minItems := s.MinItems
	if s.Required && minItems < 1 {
		minItems = 1
	}
	switch s.Type {
	case sdk.TypeList:
		b := schema.ListNestedBlock{NestedObject: obj, MarkdownDescription: s.Description, DeprecationMessage: s.Deprecated}
		if minItems > 0 {
			b.Validators = append(b.Validators, listvalidator.SizeAtLeast(minItems))
		}
		if s.MaxItems > 0 {
			b.Validators = append(b.Validators, listvalidator.SizeAtMost(s.MaxItems))
		}
		b.Validators = append(b.Validators, listConstraints(pv)...)
		if s.ForceNew {
			b.PlanModifiers = []planmodifier.List{listplanmodifier.RequiresReplace()}
		}
		return b, nil
	case sdk.TypeSet:
		b := schema.SetNestedBlock{NestedObject: obj, MarkdownDescription: s.Description, DeprecationMessage: s.Deprecated}
		if minItems > 0 {
			b.Validators = append(b.Validators, setvalidator.SizeAtLeast(minItems))
		}
		if s.MaxItems > 0 {
			b.Validators = append(b.Validators, setvalidator.SizeAtMost(s.MaxItems))
		}
		b.Validators = append(b.Validators, setConstraints(pv)...)
		if s.ForceNew {
			b.PlanModifiers = []planmodifier.Set{setplanmodifier.RequiresReplace()}
		}
		return b, nil
	}
	return nil, fmt.Errorf("%s: a nested Resource needs a TypeList or TypeSet, got %s", key, s.Type)
}

// computedNested is the attribute form of a Computed-only nested Resource: a list or set
// of objects.
func computedNested(s *sdk.Schema, res *sdk.Resource) (schema.Attribute, error) {
	obj, err := objectType(res.Schema)
	if err != nil {
		return nil, err
	}
	switch s.Type {
	case sdk.TypeList:
		return schema.ListAttribute{ElementType: obj, Computed: true, MarkdownDescription: s.Description, DeprecationMessage: s.Deprecated,
			PlanModifiers: []planmodifier.List{listplanmodifier.UseStateForUnknown()}}, nil
	case sdk.TypeSet:
		return schema.SetAttribute{ElementType: obj, Computed: true, MarkdownDescription: s.Description, DeprecationMessage: s.Deprecated,
			PlanModifiers: []planmodifier.Set{setplanmodifier.UseStateForUnknown()}}, nil
	}
	return nil, fmt.Errorf("a computed nested Resource needs a TypeList or TypeSet, got %s", s.Type)
}

// objectType is the framework type of a nested Resource.
func objectType(m map[string]*sdk.Schema) (types.ObjectType, error) {
	attrTypes := make(map[string]attr.Type, len(m))
	for name, s := range m {
		t, err := attrType(s)
		if err != nil {
			return types.ObjectType{}, fmt.Errorf("%s: %w", name, err)
		}
		attrTypes[name] = t
	}
	return types.ObjectType{AttrTypes: attrTypes}, nil
}

func attrType(s *sdk.Schema) (attr.Type, error) {
	switch s.Type {
	case sdk.TypeString:
		return types.StringType, nil
	case sdk.TypeBool:
		return types.BoolType, nil
	case sdk.TypeInt:
		return types.Int64Type, nil
	case sdk.TypeFloat:
		return types.Float64Type, nil
	case sdk.TypeList, sdk.TypeSet, sdk.TypeMap:
		elem, err := elemType(s)
		if err != nil {
			return nil, err
		}
		switch s.Type {
		case sdk.TypeList:
			return types.ListType{ElemType: elem}, nil
		case sdk.TypeSet:
			return types.SetType{ElemType: elem}, nil
		default:
			return types.MapType{ElemType: elem}, nil
		}
	}
	return nil, fmt.Errorf("unsupported SDK type %s", s.Type)
}

// elemType is the element type of an SDK collection; a map without Elem holds strings.
func elemType(s *sdk.Schema) (attr.Type, error) {
	switch e := s.Elem.(type) {
	case *sdk.Schema:
		return attrType(e)
	case *sdk.Resource:
		if s.Type == sdk.TypeMap {
			return types.StringType, nil
		}
		return objectType(e.Schema)
	case nil:
		if s.Type == sdk.TypeMap {
			return types.StringType, nil
		}
	}
	return nil, fmt.Errorf("collection without an element type")
}

var _ validator.String = sdkValidator{}
