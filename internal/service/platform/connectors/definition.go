// Package connectors serves the Harness connector resources and data sources on
// terraform-plugin-framework. Each connector type is a Definition: its SDK v2 schema
// declaration and its type-specific build and read functions, which internal/framework/
// bridge runs inside one generic framework resource. The connector lifecycle (scope-aware
// read, create or update, delete, common attributes, import, identity, state upgrade) is
// implemented once here.
package connectors

import (
	"fmt"

	"github.com/harness/harness-go-sdk/harness/nextgen"
	"github.com/harness/terraform-provider-harness/helpers"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	sdkschema "github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// Definition declares one connector type.
type Definition struct {
	// Suffix is the type name after "harness_platform_connector_", e.g. "github".
	Suffix string
	// Type is the connector type Harness reports; reading an object of another type is
	// an error.
	Type nextgen.ConnectorType
	// Resource is the SDK v2 declaration of the resource: its Schema, Description and
	// Importer are used; its CRUD functions and CustomizeDiff are not.
	Resource func() *sdkschema.Resource
	// DataSource is the SDK v2 declaration of the data source schema; nil when the type
	// has no data source.
	DataSource func() *sdkschema.Resource
	// Build fills the type-specific part of a connector from the configuration.
	Build func(d *sdkschema.ResourceData) *nextgen.ConnectorInfo
	// Read writes the type-specific part of a connector into the state.
	Read func(d *sdkschema.ResourceData, conn *nextgen.ConnectorInfo) error
	// Validate checks the configuration as a whole (what the SDK v2 resource did in
	// CustomizeDiff); it sees the configuration through a ResourceData and returns the
	// error to report. Optional.
	Validate func(d *sdkschema.ResourceData) error
}

// TypeName is the resource and data source type name.
func (d Definition) TypeName() string { return "harness_platform_connector_" + d.Suffix }

func (d Definition) importSpec() (helpers.ImportSpec, error) {
	spec, ok := helpers.ImportSpecOf(d.Resource().Importer)
	if !ok {
		return helpers.ImportSpec{}, fmt.Errorf("the resource importer is not built from a helpers.ImportSpec")
	}
	return spec, nil
}

// Resources returns the framework resource constructors of defs.
func Resources(defs []Definition) []func() resource.Resource {
	out := make([]func() resource.Resource, 0, len(defs))
	for _, def := range defs {
		out = append(out, newResource(def))
	}
	return out
}

// DataSources returns the framework data source constructors of the defs that have one.
func DataSources(defs []Definition) []func() datasource.DataSource {
	var out []func() datasource.DataSource
	for _, def := range defs {
		if def.DataSource != nil {
			out = append(out, newDataSource(def))
		}
	}
	return out
}

// LegacyResources returns the SDK v2 resources of defs by type name, for the legacy
// provider the migration tests apply state with.
func LegacyResources(defs []Definition) map[string]*sdkschema.Resource {
	out := map[string]*sdkschema.Resource{}
	for _, def := range defs {
		out[def.TypeName()] = def.Resource()
	}
	return out
}

// LegacyDataSources is LegacyResources for the data sources.
func LegacyDataSources(defs []Definition) map[string]*sdkschema.Resource {
	out := map[string]*sdkschema.Resource{}
	for _, def := range defs {
		if def.DataSource != nil {
			out[def.TypeName()] = def.DataSource()
		}
	}
	return out
}

// buildCommon copies the common attributes (identifier, name, description, scope, tags)
// from the configuration into the connector.
func buildCommon(d *sdkschema.ResourceData, connector *nextgen.ConnectorInfo) {
	connector.Identifier = d.Get("identifier").(string)
	connector.Name = d.Get("name").(string)
	connector.Description = d.Get("description").(string)
	connector.OrgIdentifier = d.Get("org_id").(string)
	connector.ProjectIdentifier = d.Get("project_id").(string)
	connector.Tags = helpers.ExpandTags(d.Get("tags").(*sdkschema.Set).List())
}

// readCommon writes the common attributes of the connector into the state.
func readCommon(d *sdkschema.ResourceData, connector *nextgen.ConnectorInfo) error {
	d.SetId(connector.Identifier)
	return helpers.SetAll(d, map[string]interface{}{
		"identifier":  connector.Identifier,
		"description": connector.Description,
		"name":        connector.Name,
		"org_id":      connector.OrgIdentifier,
		"project_id":  connector.ProjectIdentifier,
		"tags":        helpers.FlattenTags(connector.Tags),
	})
}

// readOpts scopes a connector lookup with the org, project and git sync details in d.
func readOpts(d *sdkschema.ResourceData) *nextgen.ConnectorsApiGetConnectorOpts {
	opts := &nextgen.ConnectorsApiGetConnectorOpts{
		OrgIdentifier:     helpers.BuildField(d, "org_id"),
		ProjectIdentifier: helpers.BuildField(d, "project_id"),
	}
	if attr, ok := d.GetOk("git_sync"); ok {
		if sync, ok := attr.([]interface{})[0].(map[string]interface{}); ok {
			opts.Branch = helpers.BuildField(d, "git_sync.0.branch")
			opts.RepoIdentifier = helpers.BuildField(d, "git_sync.0.repo_id")
			_ = sync
		}
	}
	return opts
}

// Check converts a definition the way the provider will and returns the first problem:
// the schemas the bridge cannot express, an importer without an ImportSpec. The provider
// tests run it over every definition.
func Check(def Definition) error {
	if p := prepareResource(def); p.err != nil {
		return p.err
	}
	if def.DataSource != nil {
		if p := prepareDataSource(def); p.err != nil {
			return p.err
		}
	}
	return nil
}
