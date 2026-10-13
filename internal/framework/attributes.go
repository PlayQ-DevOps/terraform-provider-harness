package framework

import (
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	dsschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Level says which scope attributes a resource carries.
type Level int

const (
	// AccountLevel resources have no org_id or project_id.
	AccountLevel Level = iota
	// OrgLevel resources require org_id.
	OrgLevel
	// ProjectLevel resources require org_id and project_id.
	ProjectLevel
	// MultiLevel resources take optional org_id and project_id (project_id needs org_id).
	MultiLevel
)

// The descriptions match the SDK v2 helpers so the documentation does not change when a
// resource is migrated.
const (
	descID          = "The ID of this resource."
	descIdentifier  = "Unique identifier of the resource."
	descName        = "Name of the resource."
	descDescription = "Description of the resource."
	descTags        = "Tags to associate with the resource."
	descOrgID       = "Unique identifier of the organization."
	descProjectID   = "Unique identifier of the project."
)

// RequiresReplace is the plan modifier for identity attributes: a change plans a
// replacement, never the in-place update the API would reject.
func RequiresReplace() []planmodifier.String {
	return []planmodifier.String{stringplanmodifier.RequiresReplace()}
}

// IdentityString is a Required string attribute whose change replaces the resource.
func IdentityString(description string) schema.StringAttribute {
	return schema.StringAttribute{Required: true, MarkdownDescription: description, PlanModifiers: RequiresReplace()}
}

// CommonModel holds the attributes CommonResourceAttributes declares. Embed it by value
// in a resource model and add the scope attributes of the level.
type CommonModel struct {
	ID          types.String `tfsdk:"id"`
	Identifier  types.String `tfsdk:"identifier"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
	Tags        types.Set    `tfsdk:"tags"`
}

// CommonResourceAttributes returns the attributes every Harness resource shares (id,
// identifier, name, description, tags) plus the scope attributes of the level. The
// identity attributes (identifier, org_id, project_id) require replacement when changed.
func CommonResourceAttributes(level Level) map[string]schema.Attribute {
	attrs := map[string]schema.Attribute{
		"id": schema.StringAttribute{Computed: true, MarkdownDescription: descID,
			PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
		"identifier":  IdentityString(descIdentifier),
		"name":        schema.StringAttribute{Required: true, MarkdownDescription: descName},
		"description": schema.StringAttribute{Optional: true, MarkdownDescription: descDescription},
		"tags":        schema.SetAttribute{ElementType: types.StringType, Optional: true, MarkdownDescription: descTags},
	}
	switch level {
	case OrgLevel:
		attrs["org_id"] = IdentityString(descOrgID)
	case ProjectLevel:
		attrs["org_id"] = IdentityString(descOrgID)
		attrs["project_id"] = IdentityString(descProjectID)
	case MultiLevel:
		attrs["org_id"] = schema.StringAttribute{Optional: true, MarkdownDescription: descOrgID, PlanModifiers: RequiresReplace()}
		attrs["project_id"] = schema.StringAttribute{Optional: true, MarkdownDescription: descProjectID, PlanModifiers: RequiresReplace(),
			Validators: []validator.String{stringvalidator.AlsoRequires(path.MatchRoot("org_id"))}}
	}
	return attrs
}

// CommonDataSourceAttributes returns the data-source shape of the common attributes: the
// lookup keys (identifier or name, and the scope) are inputs, the rest is computed. With
// identifierRequired the identifier is the only lookup key.
func CommonDataSourceAttributes(level Level, identifierRequired bool) map[string]dsschema.Attribute {
	attrs := map[string]dsschema.Attribute{
		"id":          dsschema.StringAttribute{Computed: true, MarkdownDescription: descID},
		"identifier":  dsschema.StringAttribute{Optional: !identifierRequired, Required: identifierRequired, MarkdownDescription: descIdentifier},
		"name":        dsschema.StringAttribute{Optional: true, MarkdownDescription: descName},
		"description": dsschema.StringAttribute{Computed: true, MarkdownDescription: descDescription},
		"tags":        dsschema.SetAttribute{ElementType: types.StringType, Computed: true, MarkdownDescription: descTags},
	}
	switch level {
	case OrgLevel:
		attrs["org_id"] = dsschema.StringAttribute{Required: true, MarkdownDescription: descOrgID}
	case ProjectLevel:
		attrs["org_id"] = dsschema.StringAttribute{Required: true, MarkdownDescription: descOrgID}
		attrs["project_id"] = dsschema.StringAttribute{Required: true, MarkdownDescription: descProjectID}
	case MultiLevel:
		attrs["org_id"] = dsschema.StringAttribute{Optional: true, MarkdownDescription: descOrgID}
		attrs["project_id"] = dsschema.StringAttribute{Optional: true, MarkdownDescription: descProjectID,
			Validators: []validator.String{stringvalidator.AlsoRequires(path.MatchRoot("org_id"))}}
	}
	return attrs
}
