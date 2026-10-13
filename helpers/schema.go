package helpers

import (
	"context"
	"fmt"

	"github.com/antihax/optional"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func MergeSchemas(src map[string]*schema.Schema, dest map[string]*schema.Schema) {
	for k, v := range src {
		dest[k] = v
	}
}

func SetSchemaFlagType(s *schema.Schema, flag SchemaFlagType) {
	switch flag {
	case SchemaFlagTypes.Computed:
		s.Computed = true
	case SchemaFlagTypes.Optional:
		s.Optional = true
	case SchemaFlagTypes.Required:
		s.Required = true
	}
}

func GetTagsSchema(flag SchemaFlagType) *schema.Schema {
	s := &schema.Schema{
		Description: "Tags to associate with the resource.",
		Type:        schema.TypeSet,
		Elem: &schema.Schema{
			Type: schema.TypeString,
		},
	}
	SetSchemaFlagType(s, flag)
	return s
}

func GetIdentifierSchema(flag SchemaFlagType) *schema.Schema {
	s := &schema.Schema{
		Description: "Unique identifier of the resource.",
		Type:        schema.TypeString,
	}

	if flag == SchemaFlagTypes.Required {
		s.ForceNew = true
	}

	SetSchemaFlagType(s, flag)

	return s
}

func GetProjectIdSchema(flag SchemaFlagType) *schema.Schema {
	s := &schema.Schema{
		Description: "Unique identifier of the project.",
		Type:        schema.TypeString,
	}

	if flag == SchemaFlagTypes.Required {
		s.ForceNew = true
	}

	SetSchemaFlagType(s, flag)
	return s
}

func GetOrgIdSchema(flag SchemaFlagType) *schema.Schema {
	s := &schema.Schema{
		Description: "Unique identifier of the organization.",
		Type:        schema.TypeString,
	}

	if flag == SchemaFlagTypes.Required {
		s.ForceNew = true
	}

	SetSchemaFlagType(s, flag)
	return s
}

func GetNameSchema(flag SchemaFlagType) *schema.Schema {
	s := &schema.Schema{
		Description: "Name of the resource.",
		Type:        schema.TypeString,
	}

	SetSchemaFlagType(s, flag)
	return s
}

func GetDescriptionSchema(flag SchemaFlagType) *schema.Schema {
	s := &schema.Schema{
		Description: "Description of the resource.",
		Type:        schema.TypeString,
	}
	SetSchemaFlagType(s, flag)
	return s
}

// SetCommonResourceSchema sets the default schema objects used for most resources.
func SetCommonResourceSchema(s map[string]*schema.Schema) {
	s["identifier"] = GetIdentifierSchema(SchemaFlagTypes.Required)
	s["description"] = GetDescriptionSchema(SchemaFlagTypes.Optional)
	s["name"] = GetNameSchema(SchemaFlagTypes.Required)
	s["tags"] = GetTagsSchema(SchemaFlagTypes.Optional)
}

// SetCommonDataSourceSchema sets the default schema objects used for most data sources.
func SetCommonDataSourceSchema(s map[string]*schema.Schema) {
	s["identifier"] = GetIdentifierSchema(SchemaFlagTypes.Optional)
	s["description"] = GetDescriptionSchema(SchemaFlagTypes.Computed)
	s["name"] = GetNameSchema(SchemaFlagTypes.Optional)
	s["tags"] = GetTagsSchema(SchemaFlagTypes.Computed)
}

func SetCommonDataSourceSchemaWRequired(s map[string]*schema.Schema) {
	s["identifier"] = GetIdentifierSchema(SchemaFlagTypes.Required)
	s["description"] = GetDescriptionSchema(SchemaFlagTypes.Computed)
	s["name"] = GetNameSchema(SchemaFlagTypes.Required)
	s["tags"] = GetTagsSchema(SchemaFlagTypes.Computed)
}

func SetCommonDataSourceSchemaIdentifierRequired(s map[string]*schema.Schema) {
	s["identifier"] = GetIdentifierSchema(SchemaFlagTypes.Required)
	s["description"] = GetDescriptionSchema(SchemaFlagTypes.Computed)
	s["name"] = GetNameSchema(SchemaFlagTypes.Optional)
	s["tags"] = GetTagsSchema(SchemaFlagTypes.Computed)
}

func SetOrgLevelDataSourceSchema(s map[string]*schema.Schema) {
	SetCommonDataSourceSchema(s)
	s["org_id"] = GetOrgIdSchema(SchemaFlagTypes.Required)
}

func SetOrgLevelDataSourceSchemaIdentifierRequired(s map[string]*schema.Schema) {
	SetCommonDataSourceSchemaIdentifierRequired(s)
	s["org_id"] = GetOrgIdSchema(SchemaFlagTypes.Required)
}

func SetProjectLevelDataSourceSchema(s map[string]*schema.Schema) {
	SetOrgLevelDataSourceSchema(s)
	s["project_id"] = GetProjectIdSchema(SchemaFlagTypes.Required)
}

func SetProjectLevelDataSourceSchemaIdentifierRequired(s map[string]*schema.Schema) {
	SetOrgLevelDataSourceSchemaIdentifierRequired(s)
	s["project_id"] = GetProjectIdSchema(SchemaFlagTypes.Required)
}

func SetOptionalOrgAndProjectLevelDataSourceSchema(s map[string]*schema.Schema) {
	s["org_id"] = GetOrgIdSchema(SchemaFlagTypes.Optional)
	s["project_id"] = GetProjectIdSchema(SchemaFlagTypes.Optional)
}

// SetOrgLevelResourceSchema sets the default schema objects used for org level resources.
func SetOrgLevelResourceSchema(s map[string]*schema.Schema) {
	SetCommonResourceSchema(s)
	s["org_id"] = GetOrgIdSchema(SchemaFlagTypes.Required)
}

// SetProjectLevelResourceSchema sets the default schema objects used for project level resources.
func SetProjectLevelResourceSchema(s map[string]*schema.Schema) {
	SetOrgLevelResourceSchema(s)
	s["project_id"] = GetProjectIdSchema(SchemaFlagTypes.Required)
}

func SetMultiLevelResourceSchema(s map[string]*schema.Schema) {
	SetCommonResourceSchema(s)
	s["org_id"] = GetOrgIdSchema(SchemaFlagTypes.Optional)
	s["project_id"] = GetProjectIdSchema(SchemaFlagTypes.Optional)
	s["project_id"].RequiredWith = []string{"org_id"}
}

func SetMultiLevelDatasourceSchema(s map[string]*schema.Schema) {
	SetCommonDataSourceSchema(s)
	s["org_id"] = GetOrgIdSchema(SchemaFlagTypes.Optional)
	s["project_id"] = GetProjectIdSchema(SchemaFlagTypes.Optional)
	s["project_id"].RequiredWith = []string{"org_id"}
}

func SetMultiLevelDatasourceSchemaIdentifierRequired(s map[string]*schema.Schema) {
	SetCommonDataSourceSchemaIdentifierRequired(s)
	s["org_id"] = GetOrgIdSchema(SchemaFlagTypes.Optional)
	s["project_id"] = GetProjectIdSchema(SchemaFlagTypes.Optional)
	s["project_id"].RequiredWith = []string{"org_id"}
}

func SetMultiLevelDatasourceSchemaWithoutCommonFields(s map[string]*schema.Schema) {
	s["org_id"] = GetOrgIdSchema(SchemaFlagTypes.Optional)
	s["project_id"] = GetProjectIdSchema(SchemaFlagTypes.Optional)
	s["project_id"].RequiredWith = []string{"org_id"}
}

func BuildField(d *schema.ResourceData, field string) optional.String {
	if arr, ok := d.GetOk(field); ok {
		return optional.NewString(arr.(string))
	}
	return optional.EmptyString()
}

func BuildFieldInt32(d *schema.ResourceData, field string) optional.Int32 {
	if arr, ok := d.GetOk(field); ok {
		return optional.NewInt32(int32(arr.(int)))
	}
	return optional.EmptyInt32()
}

func BuildFieldForBoolean(d *schema.ResourceData, field string) optional.Bool {
	if arr, ok := d.GetOk(field); ok {

		return optional.NewBool(arr.(string) == "true")
	}
	return optional.EmptyBool()
}

func BuildFieldBool(d *schema.ResourceData, field string) optional.Bool {
	if b, ok := d.GetOk(field); ok {
		return optional.NewBool(b.(bool))
	}

	return optional.EmptyBool()
}

// Shared importers. Each is an ImportSpec (see importer.go): the accepted formats are
// documented by ImportSpec.Format, malformed IDs are errors rather than panics, and the
// resource ID is the bare identifier with the scope stored in attributes.

// PipelineResourceImporter: <org_id>/<project_id>/<pipeline_id>/<identifier>.
var PipelineResourceImporter = NewImporter(ImportSpec{Parts: []string{"org_id", "project_id", "pipeline_id", "identifier"}})

// DBInstanceResourceImporter: <org_id>/<project_id>/<schema>/<identifier>.
var DBInstanceResourceImporter = NewImporter(ImportSpec{Parts: []string{"org_id", "project_id", "schema", "identifier"}})

// TriggerResourceImporter: <org_id>/<project_id>/<target_id>/<identifier>.
var TriggerResourceImporter = NewImporter(ImportSpec{Parts: []string{"org_id", "project_id", "target_id", "identifier"}})

// EnvRelatedResourceImporter: [<org_id>/[<project_id>/]]<env_id>/<identifier>.
var EnvRelatedResourceImporter = NewImporter(ImportSpec{Parts: []string{"org_id", "project_id", "env_id", "identifier"}, OptionalPrefix: 2})

// ServiceOverrideResourceImporter: [<org_id>/[<project_id>/]]<env_id>; the full import ID
// stays the resource ID.
var ServiceOverrideResourceImporter = NewImporter(ImportSpec{Parts: []string{"org_id", "project_id", "env_id"}, OptionalPrefix: 2, RawID: true})

// ServiceOverrideV2ResourceImporter: [<org_id>/[<project_id>/]]<identifier>.
var ServiceOverrideV2ResourceImporter = NewImporter(ImportSpec{Parts: []string{"org_id", "project_id", ""}, OptionalPrefix: 2})

// UserResourceImporter: <email>[/<org_id>[/<project_id>]]; the full import ID stays the
// resource ID until Read resets it.
var UserResourceImporter = NewImporter(ImportSpec{Parts: []string{"email", "org_id", "project_id"}, OptionalSuffix: 2, RawID: true})

// ProjectResourceImporter: <org_id>/<project_id>/<identifier>.
var ProjectResourceImporter = NewImporter(ImportSpec{Parts: []string{"org_id", "project_id", "identifier"}})

// ProjectResourceWithBranchImporter: <org_id>/<project_id>/<identifier>[/<branch>] for
// git-backed project-level resources. The branch may itself contain "/".
var ProjectResourceWithBranchImporter = NewImporter(ImportSpec{Parts: []string{"org_id", "project_id", "identifier"}, Tail: BranchTail})

// GitopsAgentResourceImporter: [<org_id>/[<project_id>/]]<agent_id>/<identifier>.
var GitopsAgentResourceImporter = NewImporter(ImportSpec{Parts: []string{"org_id", "project_id", "agent_id", "identifier"}, OptionalPrefix: 2})

// GitopsAgentApplicationImporter: <org_id>/<project_id>/<agent_id>/<name>.
var GitopsAgentApplicationImporter = NewImporter(ImportSpec{Parts: []string{"org_id", "project_id", "agent_id", "name"}})

// GitopsAgentApplicationsetImporter: <org_id>/<project_id>/<agent_id>/<identifier>.
var GitopsAgentApplicationsetImporter = NewImporter(ImportSpec{Parts: []string{"org_id", "project_id", "agent_id", "identifier"}})

// GitopsAppProjectMappingImporter: <org_id>/<project_id>/<agent_id>/<argo_project_name>.
// The Argo project name becomes the ID; Read resolves the mapping from it.
var GitopsAppProjectMappingImporter = NewImporter(ImportSpec{Parts: []string{"org_id", "project_id", "agent_id", "argo_project_name"}})

// GitopsAgentProjectImporter: [<org_id>/[<project_id>/]]<agent_id>/<query_name>.
var GitopsAgentProjectImporter = NewImporter(ImportSpec{Parts: []string{"org_id", "project_id", "agent_id", "query_name"}, OptionalPrefix: 2})

// RepoRuleResourceImporter: [<org_id>/[<project_id>/]]<repo_identifier>/<identifier>.
var RepoRuleResourceImporter = NewImporter(ImportSpec{Parts: []string{"org_id", "project_id", "repo_identifier", "identifier"}, OptionalPrefix: 2})

// GitopsRepoCertResourceImporter: <agent_id>/<identifier>.
var GitopsRepoCertResourceImporter = NewImporter(ImportSpec{Parts: []string{"agent_id", ""}})

// OrgResourceImporter: <org_id>/<identifier>.
var OrgResourceImporter = NewImporter(ImportSpec{Parts: []string{"org_id", "identifier"}})

// GitOpsFilterImporter: <org_id>/<project_id>/<identifier>/<type>.
var GitOpsFilterImporter = NewImporter(ImportSpec{Parts: []string{"org_id", "project_id", "identifier", "type"}, IDFrom: "identifier"})

// MultiLevelResourceImporter: [<org_id>/[<project_id>/]]<identifier>.
var MultiLevelResourceImporter = NewImporter(ImportSpec{Parts: []string{"org_id", "project_id", "identifier"}, OptionalPrefix: 2})

// MultiLevelFilterImporter: [<org_id>/[<project_id>/]]<identifier>/<type>.
var MultiLevelFilterImporter = NewImporter(ImportSpec{Parts: []string{"org_id", "project_id", "identifier", "type"}, OptionalPrefix: 2, IDFrom: "identifier"})

// GitWebhookResourceImporter: <identifier>[/<org_id>[/<project_id>]] (scope after the identifier).
var GitWebhookResourceImporter = NewImporter(ImportSpec{Parts: []string{"identifier", "org_id", "project_id"}, OptionalSuffix: 2, IDFrom: "identifier"})

// AccountLevelResourceImporter keeps the import ID as the resource ID.
var AccountLevelResourceImporter = PassthroughImporter()

// MultiLevelTemplateImporter defines the importer configuration for template resources.
// When no version is specified, the stable version is fetched.
//
//	Account level stable:           <template_id>
//	Account level specific version: <template_id>/versions/<version>
//	Org level stable:               <org_id>/<template_id>
//	Org level specific version:     <org_id>/<template_id>/versions/<version>
//	Project level stable:           <org_id>/<project_id>/<template_id>
//	Project level specific version: <org_id>/<project_id>/<template_id>/versions/<version>
var MultiLevelTemplateImporter = &schema.ResourceImporter{
	StateContext: func(ctx context.Context, d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
		const format = "[<org_id>/][<project_id>/]<template_id>[/versions/<version>]"
		parts, err := SplitImportID(d.Id(), 1, 5)
		if err != nil {
			return nil, fmt.Errorf("%w; expected %s", err, format)
		}
		idParts := parts
		version := ""
		if n := len(parts); n >= 3 && parts[n-2] == "versions" {
			version = parts[n-1]
			idParts = parts[:n-2]
		}
		attrs := map[string]string{}
		switch len(idParts) {
		case 1:
			attrs["identifier"] = idParts[0]
		case 2:
			attrs["org_id"], attrs["identifier"] = idParts[0], idParts[1]
		case 3:
			attrs["org_id"], attrs["project_id"], attrs["identifier"] = idParts[0], idParts[1], idParts[2]
		default:
			return nil, fmt.Errorf("unexpected import ID %q: expected %s", d.Id(), format)
		}
		if version != "" {
			attrs["version"] = version
		}
		for k, v := range attrs {
			if err := d.Set(k, v); err != nil {
				return nil, fmt.Errorf("import: cannot set %s from the import ID: %w", k, err)
			}
		}
		d.SetId(attrs["identifier"])
		return []*schema.ResourceData{d}, nil
	},
}
