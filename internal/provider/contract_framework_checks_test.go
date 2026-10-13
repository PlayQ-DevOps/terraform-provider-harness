package provider_test

import (
	"context"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/harness/terraform-provider-harness/internal/provider"
	fwdatasource "github.com/hashicorp/terraform-plugin-framework/datasource"
	dsschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	fwresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
)

func checkFrameworkResource(t *testing.T, rep *contractReport, name string, res fwresource.Resource) {
	t.Helper()
	ctx := context.Background()
	sr := &fwresource.SchemaResponse{}
	res.Schema(ctx, fwresource.SchemaRequest{}, sr)
	if sr.Diagnostics.HasError() {
		t.Fatalf("%s: schema: %v", name, sr.Diagnostics)
	}
	s := sr.Schema

	imp, importable := res.(fwresource.ResourceWithImportState)
	if !importable {
		rep.add(checkImporterMissing, name)
	} else {
		for _, id := range malformedImportIDs {
			if fwImporterPanics(ctx, imp, s, id) {
				rep.add(checkImporterPanics, name+":"+strconv.Quote(id))
			}
		}
	}

	// Identity: the common identity attributes plus every attribute of the resource
	// identity schema (the parts of the import ID).
	identity := append([]string{}, identityAttrs...)
	if withIdentity, ok := res.(fwresource.ResourceWithIdentity); ok {
		ir := &fwresource.IdentitySchemaResponse{}
		withIdentity.IdentitySchema(ctx, fwresource.IdentitySchemaRequest{}, ir)
		for _, part := range sortedKeys(ir.IdentitySchema.Attributes) {
			if !slices.Contains(identity, part) {
				identity = append(identity, part)
			}
		}
	}
	for _, attr := range identity {
		a, ok := s.Attributes[attr]
		if !ok {
			continue
		}
		if fa := a.(fwAttribute); (fa.IsRequired() || fa.IsOptional()) && !requiresReplace(a) {
			rep.add(checkMutableIdentity, name+"."+attr)
		}
	}
	if strings.Contains(strings.ToLower(s.MarkdownDescription+s.Description), "deprecated") && s.DeprecationMessage == "" {
		rep.add(checkDeprecation, name)
	}

	optionalComputed, missingDescription := 0, 0
	if s.MarkdownDescription == "" && s.Description == "" {
		missingDescription++
	}
	walkResourceSchema("", s.Attributes, s.Blocks, func(path string, a schema.Attribute) {
		fa := a.(fwAttribute)
		leaf := leafName(path)
		if intentFlags[leaf] {
			if _, isBool := a.(schema.BoolAttribute); !isBool || fa.IsComputed() || fa.IsRequired() {
				rep.add(checkIntentFlag, name+"."+path)
			}
		}
		if _, isString := a.(schema.StringAttribute); isString && !fa.IsSensitive() && secretNameRe.MatchString(leaf) && !secretRefRe.MatchString(leaf) {
			rep.add(checkSensitive, name+"."+path)
		}
		if fa.GetDeprecationMessage() != "" && fa.IsRequired() {
			rep.add(checkDeprecation, name+"."+path)
		}
		if fa.IsOptional() && fa.IsComputed() {
			optionalComputed++
		}
		if !fwDescribed(fa) {
			missingDescription++
		}
	})
	rep.count(checkOptionalComputed, name, optionalComputed)
	rep.count(checkMissingDescription, "resource:"+name, missingDescription)
}

func checkFrameworkDataSource(t *testing.T, rep *contractReport, name string, ds fwdatasource.DataSource) {
	t.Helper()
	ctx := context.Background()
	sr := &fwdatasource.SchemaResponse{}
	ds.Schema(ctx, fwdatasource.SchemaRequest{}, sr)
	if sr.Diagnostics.HasError() {
		t.Fatalf("%s: schema: %v", name, sr.Diagnostics)
	}
	missingDescription := 0
	if sr.Schema.MarkdownDescription == "" && sr.Schema.Description == "" {
		missingDescription++
	}
	walkDataSourceSchema("", sr.Schema.Attributes, sr.Schema.Blocks, func(path string, a dsschema.Attribute) {
		if !fwDescribed(a.(fwAttribute)) {
			missingDescription++
		}
	})
	rep.count(checkMissingDescription, "data:"+name, missingDescription)
}

// checkFrameworkTypes runs the contract over the framework half of the provider.
func checkFrameworkTypes(t *testing.T, rep *contractReport) {
	t.Helper()
	ctx := context.Background()
	fw := provider.NewFrameworkProvider("test")()
	for _, newResource := range fw.Resources(ctx) {
		res := newResource()
		mr := &fwresource.MetadataResponse{}
		res.Metadata(ctx, fwresource.MetadataRequest{ProviderTypeName: "harness"}, mr)
		checkFrameworkResource(t, rep, mr.TypeName, res)
	}
	for _, newDataSource := range fw.DataSources(ctx) {
		ds := newDataSource()
		mr := &fwdatasource.MetadataResponse{}
		ds.Metadata(ctx, fwdatasource.MetadataRequest{ProviderTypeName: "harness"}, mr)
		checkFrameworkDataSource(t, rep, mr.TypeName, ds)
	}
}
