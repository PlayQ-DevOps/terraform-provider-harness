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

func ResourceIacmDefaultPipeline() *schema.Resource {
	resource := &schema.Resource{
		Description: "Resource for managing IAC default pipelines",

		ReadContext:   resourceIacmDefaultPipelineRead,
		DeleteContext: resourceIacmDefaultPipelineDelete,
		CreateContext: resourceIacmDefaultPipelineCreate,
		UpdateContext: resourceIacmDefaultPipelineUpdate,
		Importer:      iacmDefaultPipelineImporter,

		Schema: map[string]*schema.Schema{
			"org_id": {
				Description: "Organization identifier of the organization the default pipelines resides in.",
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
			},
			"project_id": {
				Description: "Project identifier of the project the default pipelines resides in.",
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
			},
			"provisioner_type": {
				Description: "The provisioner associated with this default.",
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
			},
			"operation": {
				Description: "The operation associated with this default.",
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
			},
			"pipeline": {
				Description: "The pipeline associated with this default.",
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    false,
			},
		},
	}

	return resource
}

func resourceIacmDefaultPipelineRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	c, ctx := meta.(*internal.Session).GetPlatformClientWithContext(ctx)
	defaultPipelines, httpResp, err := c.SettingsApi.SettingsListDefaultPipelines(
		ctx,
		d.Get("org_id").(string),
		d.Get("project_id").(string),
		c.AccountId,
		nil,
	)
	if err != nil {
		return helpers.HandleRead(ctx, d, err, httpResp)
	}
	for _, defaultPipeline := range defaultPipelines {
		if defaultPipeline.Provisioner == d.Get("provisioner_type").(string) &&
			defaultPipeline.Operation == d.Get("operation").(string) {
			d.SetId(fmt.Sprintf("%s-%s", defaultPipeline.Provisioner, defaultPipeline.Operation))
			d.Set("pipeline", defaultPipeline.Pipeline)
			return nil
		}
	}
	return helpers.ReadMissing(ctx, d, fmt.Sprintf("default pipeline for provisioner %s and operation %s", d.Get("provisioner_type"), d.Get("operation")))
}

func resourceIacmDefaultPipelineDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	c, ctx := meta.(*internal.Session).GetPlatformClientWithContext(ctx)
	id := d.Id()
	if id == "" {
		return nil
	}

	httpResp, err := c.SettingsApi.SettingsDeleteDefaultPipeline(
		ctx,
		nextgen.IacmDeleteDefaultPipelineRequestBody{
			Provisioner: d.Get("provisioner_type").(string),
			Operation:   d.Get("operation").(string),
		},
		c.AccountId,
		d.Get("org_id").(string),
		d.Get("project_id").(string),
	)
	if err != nil {
		return helpers.HandleDelete(ctx, d, err, httpResp)
	}

	return nil
}

func resourceIacmDefaultPipelineCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	return resourceIacmDefaultPipelineUpdate(ctx, d, meta)
}

func resourceIacmDefaultPipelineUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	c, ctx := meta.(*internal.Session).GetPlatformClientWithContext(ctx)
	id := d.Id()
	if id == "" {
		d.MarkNewResource()
	}

	httpResp, err := c.SettingsApi.SettingsUpsertDefaultPipeline(
		ctx,
		nextgen.IacmUpsertDefaultPipelineRequestBody{
			Provisioner: d.Get("provisioner_type").(string),
			Operation:   d.Get("operation").(string),
			Pipeline:    d.Get("pipeline").(string),
		},
		c.AccountId,
		d.Get("org_id").(string),
		d.Get("project_id").(string),
	)
	if err != nil {
		return helpers.HandleWrite(ctx, d, err, httpResp)
	}

	d.SetId(fmt.Sprintf("%s-%s", d.Get("provisioner_type").(string), d.Get("operation").(string)))
	return helpers.ReadAfterWrite(ctx, d, meta, 0, resourceIacmDefaultPipelineRead)
}

// iacmDefaultPipelineImporter: <org_id>/<project_id>/<provisioner_type>/<operation>; Read
// derives the "<provisioner>-<operation>" resource ID.
var iacmDefaultPipelineImporter = helpers.NewImporter(helpers.ImportSpec{Parts: []string{"org_id", "project_id", "provisioner_type", "operation"}})
