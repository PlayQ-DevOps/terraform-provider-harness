package provider_test

import (
	"context"
	"fmt"
	"strings"

	dsschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	fwresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// The same contract for the plugin-framework half of the provider. Framework schemas have
// no ForceNew flag and no DiffSuppressFunc: C3 looks for a RequiresReplace plan modifier
// on the identity attributes, C5 does not apply, and data sources cannot carry importers
// or plan modifiers at all (C8 holds by construction).

// fwAttribute is the part of the framework attribute interface the checks use (the
// framework's own interface lives in an internal package).
type fwAttribute interface {
	IsRequired() bool
	IsOptional() bool
	IsComputed() bool
	IsSensitive() bool
	GetDescription() string
	GetMarkdownDescription() string
	GetDeprecationMessage() string
}

func fwDescribed(a fwAttribute) bool {
	return a.GetDescription() != "" || a.GetMarkdownDescription() != ""
}

func joinPath(prefix, name string) string {
	if prefix == "" {
		return name
	}
	return prefix + "." + name
}

// walkResourceSchema visits every attribute of a framework resource schema, descending
// into nested attributes and blocks. Paths use "." separators.
func walkResourceSchema(prefix string, attrs map[string]schema.Attribute, blocks map[string]schema.Block, fn func(path string, a schema.Attribute)) {
	for _, name := range sortedKeys(attrs) {
		a := attrs[name]
		fn(joinPath(prefix, name), a)
		switch n := a.(type) {
		case schema.SingleNestedAttribute:
			walkResourceSchema(joinPath(prefix, name), n.Attributes, nil, fn)
		case schema.ListNestedAttribute:
			walkResourceSchema(joinPath(prefix, name), n.NestedObject.Attributes, nil, fn)
		case schema.SetNestedAttribute:
			walkResourceSchema(joinPath(prefix, name), n.NestedObject.Attributes, nil, fn)
		case schema.MapNestedAttribute:
			walkResourceSchema(joinPath(prefix, name), n.NestedObject.Attributes, nil, fn)
		}
	}
	for _, name := range sortedKeys(blocks) {
		switch b := blocks[name].(type) {
		case schema.SingleNestedBlock:
			walkResourceSchema(joinPath(prefix, name), b.Attributes, b.Blocks, fn)
		case schema.ListNestedBlock:
			walkResourceSchema(joinPath(prefix, name), b.NestedObject.Attributes, b.NestedObject.Blocks, fn)
		case schema.SetNestedBlock:
			walkResourceSchema(joinPath(prefix, name), b.NestedObject.Attributes, b.NestedObject.Blocks, fn)
		}
	}
}

// walkDataSourceSchema is walkResourceSchema for a data source schema.
func walkDataSourceSchema(prefix string, attrs map[string]dsschema.Attribute, blocks map[string]dsschema.Block, fn func(path string, a dsschema.Attribute)) {
	for _, name := range sortedKeys(attrs) {
		a := attrs[name]
		fn(joinPath(prefix, name), a)
		switch n := a.(type) {
		case dsschema.SingleNestedAttribute:
			walkDataSourceSchema(joinPath(prefix, name), n.Attributes, nil, fn)
		case dsschema.ListNestedAttribute:
			walkDataSourceSchema(joinPath(prefix, name), n.NestedObject.Attributes, nil, fn)
		case dsschema.SetNestedAttribute:
			walkDataSourceSchema(joinPath(prefix, name), n.NestedObject.Attributes, nil, fn)
		case dsschema.MapNestedAttribute:
			walkDataSourceSchema(joinPath(prefix, name), n.NestedObject.Attributes, nil, fn)
		}
	}
	for _, name := range sortedKeys(blocks) {
		switch b := blocks[name].(type) {
		case dsschema.SingleNestedBlock:
			walkDataSourceSchema(joinPath(prefix, name), b.Attributes, b.Blocks, fn)
		case dsschema.ListNestedBlock:
			walkDataSourceSchema(joinPath(prefix, name), b.NestedObject.Attributes, b.NestedObject.Blocks, fn)
		case dsschema.SetNestedBlock:
			walkDataSourceSchema(joinPath(prefix, name), b.NestedObject.Attributes, b.NestedObject.Blocks, fn)
		}
	}
}

// requiresReplace reports whether a string attribute carries a RequiresReplace plan
// modifier (any of the stringplanmodifier RequiresReplace variants).
func requiresReplace(a schema.Attribute) bool {
	s, ok := a.(schema.StringAttribute)
	if !ok {
		return false
	}
	for _, pm := range s.PlanModifiers {
		if strings.Contains(strings.ToLower(fmt.Sprintf("%T", pm)), "requiresreplace") {
			return true
		}
	}
	return false
}

func nullState(ctx context.Context, s schema.Schema) tfsdk.State {
	typ := s.Type().TerraformType(ctx).(tftypes.Object)
	vals := map[string]tftypes.Value{}
	for name, at := range typ.AttributeTypes {
		vals[name] = tftypes.NewValue(at, nil)
	}
	return tfsdk.State{Schema: s, Raw: tftypes.NewValue(typ, vals)}
}

// fwImporterPanics calls ImportState with id and reports whether it panicked.
func fwImporterPanics(ctx context.Context, imp fwresource.ResourceWithImportState, s schema.Schema, id string) (panicked bool) {
	defer func() {
		if r := recover(); r != nil {
			panicked = true
		}
	}()
	resp := &fwresource.ImportStateResponse{State: nullState(ctx, s)}
	imp.ImportState(ctx, fwresource.ImportStateRequest{ID: id}, resp)
	return false
}
