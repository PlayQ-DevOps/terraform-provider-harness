package framework

import (
	"context"
	"strings"

	"github.com/harness/terraform-provider-harness/helpers"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/identityschema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	sdkschema "github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// Import specs shared with the SDK v2 importers, so both halves accept the same IDs.
var (
	// ImportPassthrough is the account-level form: the identifier alone.
	ImportPassthrough = helpers.ImportSpec{Parts: []string{"identifier"}}
	// ImportOrgLevel is <org_id>/<identifier>.
	ImportOrgLevel = specOf(helpers.OrgResourceImporter)
	// ImportProjectLevel is <org_id>/<project_id>/<identifier>.
	ImportProjectLevel = specOf(helpers.ProjectResourceImporter)
	// ImportMultiLevel is [<org_id>/[<project_id>/]]<identifier>.
	ImportMultiLevel = specOf(helpers.MultiLevelResourceImporter)
)

func specOf(imp *sdkschema.ResourceImporter) helpers.ImportSpec {
	spec, ok := helpers.ImportSpecOf(imp)
	if !ok {
		panic("framework: importer has no ImportSpec")
	}
	return spec
}

// ImportState implements resource.ResourceWithImportState for a spec: the ID form
// ("org/project/identifier") is parsed with the spec, and an identity import (Terraform
// 1.12 import blocks, terraform query) is turned into the same form from its parts. Each
// part is set on the state under the attribute of its name, and the id is set.
func ImportState(ctx context.Context, spec helpers.ImportSpec, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	if spec.Tail != nil {
		resp.Diagnostics.AddError("Unsupported import spec", "framework resources do not support import ID tails yet")
		return
	}
	id := req.ID
	if id == "" && req.Identity != nil {
		var segs []string
		for _, name := range spec.Parts {
			var v types.String
			resp.Diagnostics.Append(req.Identity.GetAttribute(ctx, path.Root(name), &v)...)
			if resp.Diagnostics.HasError() {
				return
			}
			if !v.IsNull() && v.ValueString() != "" {
				segs = append(segs, v.ValueString())
			}
		}
		id = strings.Join(segs, "/")
	}
	attrs, resourceID, _, _, err := spec.Parse(id)
	if err != nil {
		resp.Diagnostics.AddError("Invalid import ID", err.Error())
		return
	}
	for name, v := range attrs {
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root(name), v)...)
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), resourceID)...)
	if resp.Identity != nil {
		for name, v := range attrs {
			resp.Diagnostics.Append(resp.Identity.SetAttribute(ctx, path.Root(name), v)...)
		}
	}
}

// IdentityAttributes derives the resource identity schema from a spec: every named part is
// a string; the optional scope parts are OptionalForImport, the rest RequiredForImport.
func IdentityAttributes(spec helpers.ImportSpec) map[string]identityschema.Attribute {
	out := map[string]identityschema.Attribute{}
	for name, s := range spec.IdentitySchema() {
		out[name] = identityschema.StringAttribute{RequiredForImport: s.RequiredForImport, OptionalForImport: s.OptionalForImport}
	}
	return out
}
