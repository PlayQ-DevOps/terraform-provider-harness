package idp

import (
	"context"
	"fmt"
	"net/http"

	"github.com/antihax/optional"
	"github.com/harness/harness-go-sdk/harness/idp"
	"github.com/harness/terraform-provider-harness/helpers"
	"github.com/harness/terraform-provider-harness/internal"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

const environmentBlueprintKind = "environmentblueprint"

func ResourceEnvironmentBlueprint() *schema.Resource {
	resource := &schema.Resource{
		Description:   "Resource for creating IDP environment blueprints.",
		CreateContext: resourceEnvironmentBlueprintCreateOrUpdate,
		ReadContext:   resourceEnvironmentBlueprintRead,
		UpdateContext: resourceEnvironmentBlueprintCreateOrUpdate,
		DeleteContext: resourceEnvironmentBlueprintDelete,
		Importer:      blueprintImporter,
		Schema: map[string]*schema.Schema{
			"identifier": helpers.Identity(helpers.GetIdentifierSchema(helpers.SchemaFlagTypes.Required)),
			"version": {
				Type:        schema.TypeString,
				Description: "Version of the catalog entity",
				Required:    true,
				ForceNew:    true,
			},
			"yaml": {
				Type:             schema.TypeString,
				Description:      "YAML definition of the catalog entity",
				Required:         true,
				DiffSuppressFunc: helpers.YamlDiffSuppressFunction,
			},
			"description": {
				Type:        schema.TypeString,
				Description: "Description of the catalog entity",
				Optional:    true,
			},
			"deprecated": {
				Type:        schema.TypeBool,
				Description: "Whether the catalog entity is deprecated",
				Optional:    true,
			},
			"stable": {
				Type:        schema.TypeBool,
				Description: "Whether the catalog entity is stable",
				Optional:    true,
			},
		},
	}

	return resource
}

func resourceEnvironmentBlueprintRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	c, ctx := meta.(*internal.Session).GetIDPClientWithContext(ctx)

	version := d.Get("version").(string)

	id := d.Id()
	if id == "" {
		id = d.Get("identifier").(string)
	}

	resp, httpResp, err := c.EntitiesApi.GetEntityVersion(ctx, "account", environmentBlueprintKind, id, version, &idp.EntitiesApiGetEntityVersionOpts{
		HarnessAccount: optional.NewString(c.AccountId),
	})
	if err != nil {
		return helpers.HandleRead(ctx, d, err, httpResp, idpNotFound)
	}

	readEnvironmentBlueprint(d, resp)

	return nil
}

func resourceEnvironmentBlueprintCreateOrUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	c, ctx := meta.(*internal.Session).GetIDPClientWithContext(ctx)

	var err error
	var resp idp.EntityVersionResponse
	var httpResp *http.Response

	id := d.Id()

	if id == "" {
		_, httpResp, err = c.EntitiesApi.GetEntity(ctx, "account", environmentBlueprintKind, d.Get("identifier").(string), &idp.EntitiesApiGetEntityOpts{
			HarnessAccount: optional.NewString(c.AccountId),
		})
		if err != nil {
			if httpResp != nil && httpResp.StatusCode == 404 {
				_, _, err := c.EntitiesApi.CreateEntity(ctx, idp.EntityCreateRequest{
					Yaml: d.Get("yaml").(string),
				}, &idp.EntitiesApiCreateEntityOpts{
					HarnessAccount: optional.NewString(c.AccountId),
				})
				if err != nil {
					return diag.Errorf("error in validating the existance of parent catalog entity: %v", err)
				}
			}
		}

		resp, httpResp, err = c.EntitiesApi.CreateEntityVersion(ctx, idp.EntityVersionCreateRequest{
			Version:     d.Get("version").(string),
			Yaml:        d.Get("yaml").(string),
			Description: d.Get("description").(string),
			Deprecated:  d.Get("deprecated").(bool),
			Stable:      d.Get("stable").(bool),
		}, &idp.EntitiesApiCreateEntityVersionOpts{
			HarnessAccount: optional.NewString(c.AccountId),
		})
	} else {
		resp, httpResp, err = c.EntitiesApi.UpdateEntityVersion(ctx, idp.EntityVersionUpdateRequest{
			Yaml:        d.Get("yaml").(string),
			Description: d.Get("description").(string),
			Deprecated:  d.Get("deprecated").(bool),
			Stable:      d.Get("stable").(bool),
		}, "account", environmentBlueprintKind, d.Get("identifier").(string), d.Get("version").(string), &idp.EntitiesApiUpdateEntityVersionOpts{
			HarnessAccount: optional.NewString(c.AccountId),
		})
	}
	if err != nil {
		return helpers.HandleWrite(ctx, d, err, httpResp)
	}

	readEnvironmentBlueprint(d, resp)

	return nil
}

func resourceEnvironmentBlueprintDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	c, ctx := meta.(*internal.Session).GetIDPClientWithContext(ctx)

	id := d.Id()
	version := d.Get("version").(string)

	httpResp, err := c.EntitiesApi.DeleteEntityVersion(ctx, "account", environmentBlueprintKind, id, version, &idp.EntitiesApiDeleteEntityVersionOpts{
		HarnessAccount: optional.NewString(c.AccountId),
	})
	if err != nil {
		if helpers.StatusOf(httpResp) == 400 && isOnlyVersionDeleteError(err) {
			// The last version cannot be deleted on its own: delete the parent entity instead.
			// This belongs in idp-service.
			_, delErr := c.EntitiesApi.DeleteEntity(ctx, "account", environmentBlueprintKind, id, &idp.EntitiesApiDeleteEntityOpts{
				HarnessAccount: optional.NewString(c.AccountId),
			})
			if delErr != nil {
				return helpers.HandleDelete(ctx, d, delErr, httpResp, idpNotFound)
			}
			return nil
		}

		return helpers.HandleDelete(ctx, d, err, httpResp, idpNotFound)
	}

	return nil
}

func readEnvironmentBlueprint(d *schema.ResourceData, entityVersion idp.EntityVersionResponse) {
	d.SetId(entityVersion.Identifier)
	d.Set("identifier", entityVersion.Identifier)
	d.Set("version", entityVersion.Version)
	d.Set("description", entityVersion.Description)
	d.Set("deprecated", entityVersion.Deprecated)
	d.Set("stable", entityVersion.Stable)
	if v, ok := d.GetOk("yaml"); ok && v.(string) != "" {
		d.Set("yaml", v.(string))
	} else {
		d.Set("yaml", entityVersion.Yaml)
	}
}

var blueprintImporter = &schema.ResourceImporter{
	StateContext: func(ctx context.Context, d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
		// Expected format: <identifier>/<version>
		id := d.Id()
		parts, err := helpers.SplitImportID(id, 2, 2)
		if err != nil {
			return nil, fmt.Errorf("%w; expected <identifier>/<version>", err)
		}

		identifier := parts[0]
		version := parts[1]

		c, ctx := meta.(*internal.Session).GetIDPClientWithContext(ctx)

		resp, _, err := c.EntitiesApi.GetEntityVersion(ctx, "account", environmentBlueprintKind, identifier, version, &idp.EntitiesApiGetEntityVersionOpts{
			HarnessAccount: optional.NewString(c.AccountId),
		})
		if err != nil {
			return nil, fmt.Errorf("failed to fetch entity for import: %w", err)
		}

		readEnvironmentBlueprint(d, resp)

		return []*schema.ResourceData{d}, nil
	},
}
