package iacm

import (
	"context"
	"fmt"

	"github.com/harness/harness-go-sdk/harness/nextgen"
	"github.com/harness/terraform-provider-harness/helpers"
	"github.com/harness/terraform-provider-harness/internal"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func ResourceIacmWorkspaceTemplate() *schema.Resource {
	resource := &schema.Resource{
		Description: "Resource for managing workspace template associations in IaCM.",

		ReadContext:   resourceIacmWorkspaceTemplateRead,
		DeleteContext: resourceIacmWorkspaceTemplateDelete,
		CreateContext: resourceIacmWorkspaceTemplateCreate,
		UpdateContext: resourceIacmWorkspaceTemplateUpdate,
		Importer:      resourceIacmWorkspaceTemplateImporter,

		Schema: map[string]*schema.Schema{
			"org_id": {
				Description: "Organization identifier.",
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
			},
			"project_id": {
				Description: "Project identifier.",
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
			},
			"workspace_id": {
				Description: "Workspace identifier to associate the template with.",
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
			},
			"template_id": {
				Description: "Template identifier to associate with the workspace.",
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
			},
			"version": {
				Description: "Template version.",
				Type:        schema.TypeString,
				Required:    true,
			},
			"created_at": {
				Description: "Timestamp when the association was created.",
				Type:        schema.TypeInt,
				Computed:    true,
			},
			"updated_at": {
				Description: "Timestamp when the association was last updated.",
				Type:        schema.TypeInt,
				Computed:    true,
			},
		},
	}

	return resource
}

// resourceIacmWorkspaceTemplateImporter parses the import id in the format
// <org_id>/<project_id>/<template_id>/<workspace_id> and populates the schema fields
// this resource is keyed on. The resource id mirrors what Create sets
// (<template_id>/<workspace_id>) so imported and created state stay consistent.
var resourceIacmWorkspaceTemplateImporter = &schema.ResourceImporter{
	StateContext: func(ctx context.Context, d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
		parts, err := helpers.SplitImportID(d.Id(), 4, 4)
		if err != nil {
			return nil, fmt.Errorf("%w; expected <org_id>/<project_id>/<template_id>/<workspace_id>", err)
		}
		for i, name := range []string{"org_id", "project_id", "template_id", "workspace_id"} {
			if err := d.Set(name, parts[i]); err != nil {
				return nil, err
			}
		}
		d.SetId(fmt.Sprintf("%s/%s", parts[2], parts[3]))
		return []*schema.ResourceData{d}, nil
	},
}

func resourceIacmWorkspaceTemplateRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	c, ctx := meta.(*internal.Session).GetPlatformClientWithContext(ctx)

	orgId := d.Get("org_id").(string)
	projectId := d.Get("project_id").(string)
	templateId := d.Get("template_id").(string)

	results, httpResp, err := c.WorkspaceTemplatesApi.WorkspaceTemplatesGetWorkspacesByTemplateID(
		ctx,
		c.AccountId,
		orgId,
		projectId,
		templateId,
	)
	if err != nil {
		return helpers.HandleRead(ctx, d, err, httpResp)
	}

	workspaceId := d.Get("workspace_id").(string)
	for _, r := range results {
		if r.WorkspaceID == workspaceId {
			d.SetId(fmt.Sprintf("%s/%s", templateId, workspaceId))
			d.Set("workspace_id", r.WorkspaceID)
			d.Set("template_id", r.TemplateID)
			d.Set("version", r.Version)
			d.Set("created_at", r.CreatedAt)
			d.Set("updated_at", r.UpdatedAt)
			return nil
		}
	}

	return helpers.ReadMissing(ctx, d, fmt.Sprintf("workspace %s linked to template %s", workspaceId, templateId))
}

func resourceIacmWorkspaceTemplateDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	c, ctx := meta.(*internal.Session).GetPlatformClientWithContext(ctx)

	orgId := d.Get("org_id").(string)
	projectId := d.Get("project_id").(string)
	templateId := d.Get("template_id").(string)
	workspaceId := d.Get("workspace_id").(string)

	httpResp, err := c.WorkspaceTemplatesApi.WorkspaceTemplatesRemoveWorkspaceTemplate(
		ctx,
		c.AccountId,
		orgId,
		projectId,
		templateId,
		workspaceId,
	)
	if err != nil {
		return helpers.HandleDelete(ctx, d, err, httpResp)
	}

	d.SetId("")
	return nil
}

func resourceIacmWorkspaceTemplateCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	c, ctx := meta.(*internal.Session).GetPlatformClientWithContext(ctx)

	orgId := d.Get("org_id").(string)
	projectId := d.Get("project_id").(string)
	templateId := d.Get("template_id").(string)
	workspaceId := d.Get("workspace_id").(string)
	version := d.Get("version").(string)

	body := nextgen.IacmCreateWorkspaceTemplateRequestBody{
		WorkspaceID: workspaceId,
		TemplateID:  templateId,
		Version:     version,
	}

	_, httpResp, err := c.WorkspaceTemplatesApi.WorkspaceTemplatesAddWorkspaceTemplate(
		ctx,
		body,
		c.AccountId,
		orgId,
		projectId,
	)
	if err != nil {
		return helpers.HandleWrite(ctx, d, err, httpResp)
	}

	d.SetId(fmt.Sprintf("%s/%s", templateId, workspaceId))
	return resourceIacmWorkspaceTemplateRead(ctx, d, meta)
}

func resourceIacmWorkspaceTemplateUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	c, ctx := meta.(*internal.Session).GetPlatformClientWithContext(ctx)

	orgId := d.Get("org_id").(string)
	projectId := d.Get("project_id").(string)
	templateId := d.Get("template_id").(string)
	workspaceId := d.Get("workspace_id").(string)
	version := d.Get("version").(string)

	body := nextgen.IacmUpdateWorkspaceTemplateRequestBody{
		Version: version,
	}

	_, httpResp, err := c.WorkspaceTemplatesApi.WorkspaceTemplatesUpdateWorkspaceTemplate(
		ctx,
		body,
		c.AccountId,
		orgId,
		projectId,
		templateId,
		workspaceId,
	)
	if err != nil {
		return helpers.HandleWrite(ctx, d, err, httpResp)
	}

	return resourceIacmWorkspaceTemplateRead(ctx, d, meta)
}
