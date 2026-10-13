package project

import (
	"context"

	"github.com/antihax/optional"
	"github.com/harness/harness-go-sdk/harness/nextgen"
	"github.com/harness/terraform-provider-harness/internal/framework"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func (r *projectResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan projectModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() || !framework.RequireSession(r.session, &resp.Diagnostics) {
		return
	}
	c, ctx := r.session.GetPlatformClientWithContext(ctx)
	project := buildProject(ctx, &plan, &resp.Diagnostics)
	out, httpResp, err := c.ProjectApi.PostProject(ctx, nextgen.ProjectRequest{Project: project}, c.AccountId, &nextgen.ProjectApiPostProjectOpts{
		OrgIdentifier: optional.NewString(plan.OrgID.ValueString()),
	})
	if framework.WriteError(ctx, &resp.Diagnostics, err, httpResp) {
		return
	}
	if out.Data == nil || out.Data.Project == nil {
		resp.Diagnostics.AddError("Creating the project failed", "Harness returned no project.")
		return
	}
	readProject(&plan, out.Data.Project)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	setProjectIdentity(ctx, resp.Identity, &plan, &resp.Diagnostics)
}

func (r *projectResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state projectModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() || !framework.RequireSession(r.session, &resp.Diagnostics) {
		return
	}
	c, ctx := r.session.GetPlatformClientWithContext(ctx)
	out, httpResp, err := c.ProjectApi.GetProject(ctx, state.ID.ValueString(), c.AccountId, &nextgen.ProjectApiGetProjectOpts{
		OrgIdentifier: optional.NewString(state.OrgID.ValueString()),
	})
	if framework.ReadError(ctx, resp, err, httpResp) {
		return
	}
	if out.Data == nil || out.Data.Project == nil {
		framework.ReadMissing(ctx, resp, "project "+state.ID.ValueString())
		return
	}
	readProject(&state, out.Data.Project)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
	setProjectIdentity(ctx, resp.Identity, &state, &resp.Diagnostics)
}

func (r *projectResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan projectModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() || !framework.RequireSession(r.session, &resp.Diagnostics) {
		return
	}
	c, ctx := r.session.GetPlatformClientWithContext(ctx)
	project := buildProject(ctx, &plan, &resp.Diagnostics)
	out, httpResp, err := c.ProjectApi.PutProject(ctx, nextgen.ProjectRequest{Project: project}, c.AccountId, project.Identifier, &nextgen.ProjectApiPutProjectOpts{
		OrgIdentifier: optional.NewString(plan.OrgID.ValueString()),
	})
	if framework.WriteError(ctx, &resp.Diagnostics, err, httpResp) {
		return
	}
	if out.Data == nil || out.Data.Project == nil {
		resp.Diagnostics.AddError("Updating the project failed", "Harness returned no project.")
		return
	}
	readProject(&plan, out.Data.Project)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	setProjectIdentity(ctx, resp.Identity, &plan, &resp.Diagnostics)
}

func (r *projectResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state projectModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() || !framework.RequireSession(r.session, &resp.Diagnostics) {
		return
	}
	c, ctx := r.session.GetPlatformClientWithContext(ctx)
	_, httpResp, err := c.ProjectApi.DeleteProject(ctx, state.ID.ValueString(), c.AccountId, &nextgen.ProjectApiDeleteProjectOpts{
		OrgIdentifier: optional.NewString(state.OrgID.ValueString()),
	})
	framework.DeleteError(ctx, &resp.Diagnostics, err, httpResp)
}

func buildProject(ctx context.Context, m *projectModel, diags *diag.Diagnostics) *nextgen.Project {
	return &nextgen.Project{
		Identifier:    m.Identifier.ValueString(),
		OrgIdentifier: m.OrgID.ValueString(),
		Name:          m.Name.ValueString(),
		Color:         framework.ValueString(m.Color),
		Description:   framework.ValueString(m.Description),
		Modules:       framework.Strings(ctx, m.Modules, diags),
		Tags:          framework.TagsToAPI(ctx, m.Tags, diags),
	}
}

// readProject refreshes m from the API object; Optional attributes keep their null-ness
// when the API value is empty.
func readProject(m *projectModel, project *nextgen.Project) {
	m.ID = types.StringValue(project.Identifier)
	m.Identifier = types.StringValue(project.Identifier)
	m.OrgID = types.StringValue(project.OrgIdentifier)
	m.Name = types.StringValue(project.Name)
	m.Color = types.StringValue(project.Color) // Computed: always the API value
	m.Description = framework.StringFromAPI(project.Description, m.Description)
	m.Modules = framework.StringSet(project.Modules)
	m.Tags = framework.TagsFromAPI(project.Tags, m.Tags)
}
