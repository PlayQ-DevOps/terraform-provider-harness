package organization

import (
	"context"
	"errors"
	"net/http"

	"github.com/harness/harness-go-sdk/harness/nextgen"
	"github.com/harness/terraform-provider-harness/helpers"
	"github.com/harness/terraform-provider-harness/internal"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// LegacyDataSourceOrganization is the SDK v2 implementation, kept for the legacy provider used by
// the migration test.
func LegacyDataSourceOrganization() *schema.Resource {
	resource := &schema.Resource{
		Description: "Data source for retrieving a Harness organization",
		ReadContext: dataSourceOrganizationRead,
		Schema:      map[string]*schema.Schema{},
	}

	helpers.SetCommonDataSourceSchema(resource.Schema)

	return resource
}

func dataSourceOrganizationRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	c, ctx := meta.(*internal.Session).GetPlatformClientWithContext(ctx)

	var err error
	var org *nextgen.OrganizationResponse
	var httpResp *http.Response

	id := d.Get("identifier").(string)
	name := d.Get("name").(string)

	if id != "" {
		var resp nextgen.ResponseDtoOrganizationResponse
		resp, httpResp, err = c.OrganizationApi.GetOrganization(ctx, id, c.AccountId)
		org = resp.Data
	} else if name != "" {
		org, httpResp, err = c.OrganizationApi.GetOrganizationByName(ctx, c.AccountId, name)
	} else {
		return diag.FromErr(errors.New("either identifier or name must be specified"))
	}

	if err != nil {
		return helpers.HandleError(ctx, d, err, httpResp)
	}

	if org == nil || org.Organization == nil {
		return diag.Errorf("organization not found: no object matched the given arguments")

	}

	legacyReadOrganization(d, org.Organization)

	return nil
}
