package monitored_service

import (
	"context"

	"github.com/harness/terraform-provider-harness/helpers"
	"github.com/harness/terraform-provider-harness/internal"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func DataSourceMonitoredService() *schema.Resource {
	resource := &schema.Resource{
		Description: "Data source for retrieving a monitored service.",

		ReadContext: dataSourceMonitoredServiceRead,

		Schema: map[string]*schema.Schema{
			"org_id": {
				Description: "Identifier of the organization in which the monitored service is configured.",
				Type:        schema.TypeString,
				Required:    true,
			},
			"project_id": {
				Description: "Identifier of the project in which the monitored service is configured.",
				Type:        schema.TypeString,
				Required:    true,
			},
			"identifier": {
				Description: "Identifier of the monitored service.",
				Type:        schema.TypeString,
				Required:    true,
			},
		},
	}

	return resource
}

func dataSourceMonitoredServiceRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	c, ctx := meta.(*internal.Session).GetPlatformClientWithContext(ctx)
	var accountIdentifier, orgIdentifier, projectIdentifier string
	identifier := d.Get("identifier").(string)
	accountIdentifier = c.AccountId
	if attr, ok := d.GetOk("org_id"); ok {
		orgIdentifier = attr.(string)
	}
	if attr, ok := d.GetOk("project_id"); ok {
		projectIdentifier = attr.(string)
	}
	resp, httpResp, err := c.MonitoredServiceApi.GetMonitoredService(ctx, identifier, accountIdentifier, orgIdentifier, projectIdentifier)

	if err != nil {
		return helpers.HandleError(ctx, d, err, httpResp)
	}

	readMonitoredService(d, &resp.Data)
	return nil
}
