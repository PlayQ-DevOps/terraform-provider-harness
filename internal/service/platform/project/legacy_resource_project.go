package project

import (
	"context"
	"net/http"

	"github.com/antihax/optional"
	"github.com/harness/harness-go-sdk/harness/nextgen"
	"github.com/harness/terraform-provider-harness/helpers"
	"github.com/harness/terraform-provider-harness/internal"
	"github.com/harness/terraform-provider-harness/internal/utils"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// LegacyResourceProject is the SDK v2 implementation, kept only for the migration test
// (acctest.RunMigration applies state with it, then the framework implementation takes over).
func LegacyResourceProject() *schema.Resource {
	resource := &schema.Resource{
		Description: "Resource for creating a Harness project.",

		ReadContext:   resourceProjectRead,
		UpdateContext: resourceProjectCreateOrUpdate,
		DeleteContext: resourceProjectDelete,
		CreateContext: resourceProjectCreateOrUpdate,
		Importer:      helpers.OrgResourceImporter,

		Schema: map[string]*schema.Schema{
			"color": {
				Description: "Color of the project.",
				Type:        schema.TypeString,
				Optional:    true,
				Computed:    true,
			},
			"modules": {
				Description: "Modules in the project.",
				Type:        schema.TypeSet,
				Computed:    true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},
		},
	}

	helpers.SetOrgLevelResourceSchema(resource.Schema)

	return resource
}

func resourceProjectRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	c, ctx := meta.(*internal.Session).GetPlatformClientWithContext(ctx)

	id := d.Id()

	resp, httpResp, err := c.ProjectApi.GetProject(ctx, id, c.AccountId, &nextgen.ProjectApiGetProjectOpts{
		OrgIdentifier: optional.NewString(d.Get("org_id").(string)),
	})

	if err != nil {
		return helpers.HandleRead(ctx, d, err, httpResp)
	}

	if resp.Data == nil || resp.Data.Project == nil {
		return helpers.ReadMissing(ctx, d, "project "+id)
	}

	legacyReadProject(d, resp.Data.Project)

	return nil
}

func resourceProjectCreateOrUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	c, ctx := meta.(*internal.Session).GetPlatformClientWithContext(ctx)

	var err error
	var resp nextgen.ResponseDtoProjectResponse
	var httpResp *http.Response
	id := d.Id()
	project := legacyBuildProject(d)

	if id == "" {
		resp, httpResp, err = c.ProjectApi.PostProject(ctx, nextgen.ProjectRequest{Project: project}, c.AccountId, &nextgen.ProjectApiPostProjectOpts{
			OrgIdentifier: optional.NewString(d.Get("org_id").(string)),
		})
	} else {
		resp, httpResp, err = c.ProjectApi.PutProject(ctx, nextgen.ProjectRequest{Project: project}, c.AccountId, id, &nextgen.ProjectApiPutProjectOpts{
			OrgIdentifier: optional.NewString(d.Get("org_id").(string)),
		})
	}

	if err != nil {
		return helpers.HandleWrite(ctx, d, err, httpResp)
	}

	legacyReadProject(d, resp.Data.Project)

	return nil
}

func resourceProjectDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	c, ctx := meta.(*internal.Session).GetPlatformClientWithContext(ctx)

	_, httpResp, err := c.ProjectApi.DeleteProject(ctx, d.Id(), c.AccountId, &nextgen.ProjectApiDeleteProjectOpts{OrgIdentifier: optional.NewString(d.Get("org_id").(string))})
	if err != nil {
		return helpers.HandleDelete(ctx, d, err, httpResp)
	}

	return nil
}

func legacyBuildProject(d *schema.ResourceData) *nextgen.Project {
	return &nextgen.Project{
		Identifier:    d.Get("identifier").(string),
		OrgIdentifier: d.Get("org_id").(string),
		Name:          d.Get("name").(string),
		Color:         d.Get("color").(string),
		Description:   d.Get("description").(string),
		Modules:       utils.InterfaceSliceToStringSlice(d.Get("modules").(*schema.Set).List()),
		Tags:          helpers.ExpandTags(d.Get("tags").(*schema.Set).List()),
	}
}

func legacyReadProject(d *schema.ResourceData, project *nextgen.Project) {
	d.SetId(project.Identifier)
	d.Set("identifier", project.Identifier)
	d.Set("org_id", project.OrgIdentifier)
	d.Set("name", project.Name)
	d.Set("color", project.Color)
	d.Set("description", project.Description)
	d.Set("modules", project.Modules)
	d.Set("tags", helpers.FlattenTags(project.Tags))
}
