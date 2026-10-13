package bridge

import (
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	sdk "github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// DataSourceSchema is ResourceSchema for a data source: the same shapes and validators,
// without plan modifiers or defaults (a data source has no plan).
func DataSourceSchema(r *sdk.Resource) (schema.Schema, error) {
	attrs, blocks, err := convertDataBlock(r.Schema, r.Schema, "")
	if err != nil {
		return schema.Schema{}, err
	}
	if _, ok := attrs["id"]; !ok {
		attrs["id"] = schema.StringAttribute{Computed: true, MarkdownDescription: "The ID of this resource."}
	}
	return schema.Schema{Attributes: attrs, Blocks: blocks, MarkdownDescription: r.Description, DeprecationMessage: r.DeprecationMessage}, nil
}

func convertDataBlock(m, root map[string]*sdk.Schema, prefix string) (map[string]schema.Attribute, map[string]schema.Block, error) {
	attrs := map[string]schema.Attribute{}
	blocks := map[string]schema.Block{}
	for name, s := range m {
		key := joinKey(prefix, name)
		if res, ok := s.Elem.(*sdk.Resource); ok && s.Type != sdk.TypeMap {
			if s.Computed && !s.Optional {
				obj, err := objectType(res.Schema)
				if err != nil {
					return nil, nil, fmt.Errorf("%s: %w", key, err)
				}
				if s.Type == sdk.TypeSet {
					attrs[name] = schema.SetAttribute{ElementType: obj, Computed: true, MarkdownDescription: s.Description, DeprecationMessage: s.Deprecated}
				} else {
					attrs[name] = schema.ListAttribute{ElementType: obj, Computed: true, MarkdownDescription: s.Description, DeprecationMessage: s.Deprecated}
				}
				continue
			}
			nestedAttrs, nestedBlocks, err := convertDataBlock(res.Schema, root, key)
			if err != nil {
				return nil, nil, err
			}
			pv, err := constraintsOf(s, key, root)
			if err != nil {
				return nil, nil, err
			}
			obj := schema.NestedBlockObject{Attributes: nestedAttrs, Blocks: nestedBlocks}
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
				blocks[name] = b
			case sdk.TypeSet:
				b := schema.SetNestedBlock{NestedObject: obj, MarkdownDescription: s.Description, DeprecationMessage: s.Deprecated}
				if minItems > 0 {
					b.Validators = append(b.Validators, setvalidator.SizeAtLeast(minItems))
				}
				if s.MaxItems > 0 {
					b.Validators = append(b.Validators, setvalidator.SizeAtMost(s.MaxItems))
				}
				b.Validators = append(b.Validators, setConstraints(pv)...)
				blocks[name] = b
			default:
				return nil, nil, fmt.Errorf("%s: a nested Resource needs a TypeList or TypeSet, got %s", key, s.Type)
			}
			continue
		}
		a, err := convertDataAttribute(key, s, root)
		if err != nil {
			return nil, nil, err
		}
		attrs[name] = a
	}
	return attrs, blocks, nil
}

func convertDataAttribute(key string, s *sdk.Schema, root map[string]*sdk.Schema) (schema.Attribute, error) {
	pv, err := constraintsOf(s, key, root)
	if err != nil {
		return nil, err
	}
	required, optional, computed := s.Required, s.Optional, s.Computed
	switch s.Type {
	case sdk.TypeString:
		a := schema.StringAttribute{Required: required, Optional: optional, Computed: computed, Sensitive: s.Sensitive, MarkdownDescription: s.Description, DeprecationMessage: s.Deprecated}
		if s.ValidateFunc != nil {
			a.Validators = append(a.Validators, sdkValidator{s.ValidateFunc})
		}
		a.Validators = append(a.Validators, stringConstraints(pv)...)
		return a, nil
	case sdk.TypeBool:
		a := schema.BoolAttribute{Required: required, Optional: optional, Computed: computed, Sensitive: s.Sensitive, MarkdownDescription: s.Description, DeprecationMessage: s.Deprecated}
		if s.ValidateFunc != nil {
			a.Validators = append(a.Validators, sdkValidator{s.ValidateFunc})
		}
		a.Validators = append(a.Validators, boolConstraints(pv)...)
		return a, nil
	case sdk.TypeInt:
		a := schema.Int64Attribute{Required: required, Optional: optional, Computed: computed, Sensitive: s.Sensitive, MarkdownDescription: s.Description, DeprecationMessage: s.Deprecated}
		if s.ValidateFunc != nil {
			a.Validators = append(a.Validators, sdkValidator{s.ValidateFunc})
		}
		a.Validators = append(a.Validators, int64Constraints(pv)...)
		return a, nil
	case sdk.TypeFloat:
		a := schema.Float64Attribute{Required: required, Optional: optional, Computed: computed, Sensitive: s.Sensitive, MarkdownDescription: s.Description, DeprecationMessage: s.Deprecated}
		a.Validators = append(a.Validators, float64Constraints(pv)...)
		return a, nil
	}
	elem, err := elemType(s)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", key, err)
	}
	switch s.Type {
	case sdk.TypeList:
		a := schema.ListAttribute{ElementType: elem, Required: required, Optional: optional, Computed: computed, Sensitive: s.Sensitive, MarkdownDescription: s.Description, DeprecationMessage: s.Deprecated}
		a.Validators = append(a.Validators, listConstraints(pv)...)
		return a, nil
	case sdk.TypeSet:
		a := schema.SetAttribute{ElementType: elem, Required: required, Optional: optional, Computed: computed, Sensitive: s.Sensitive, MarkdownDescription: s.Description, DeprecationMessage: s.Deprecated}
		a.Validators = append(a.Validators, setConstraints(pv)...)
		return a, nil
	case sdk.TypeMap:
		a := schema.MapAttribute{ElementType: elem, Required: required, Optional: optional, Computed: computed, Sensitive: s.Sensitive, MarkdownDescription: s.Description, DeprecationMessage: s.Deprecated}
		a.Validators = append(a.Validators, mapConstraints(pv)...)
		return a, nil
	}
	return nil, fmt.Errorf("%s: unsupported SDK type %s", key, s.Type)
}
