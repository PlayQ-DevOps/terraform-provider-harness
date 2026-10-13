package experiment

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/antihax/optional"
	"github.com/harness/harness-go-sdk/harness/chaos"
	"github.com/harness/terraform-provider-harness/helpers"
	"github.com/harness/terraform-provider-harness/internal"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func ResourceChaosExperiment() *schema.Resource {
	return &schema.Resource{
		Description: "Resource for creating chaos experiments from experiment templates.\n\n" +
			"## Not currently supported\n\n" +
			"- In-place updates: experiments are created from templates and cannot be updated directly. " +
			"Changing `template_identity`, `hub_identity`, `infra_ref`, or `revision` forces recreation, " +
			"and changing `name`, `description`, or `tags` alone is not supported by the API.\n",

		CreateContext: resourceExperimentCreate,
		ReadContext:   resourceExperimentRead,
		UpdateContext: resourceExperimentUpdate,
		DeleteContext: resourceExperimentDelete,

		Importer: &schema.ResourceImporter{
			StateContext: resourceExperimentImport,
		},

		Schema: resourceChaosExperimentSchema(),
	}
}

// resourceExperimentCreate creates a new experiment from a template
func resourceExperimentCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	c, ctx := meta.(*internal.Session).GetChaosClientWithContext(ctx)

	// Extract required fields
	orgID := d.Get("org_id").(string)
	projectID := d.Get("project_id").(string)
	templateIdentity := d.Get("template_identity").(string)
	hubIdentity := d.Get("hub_identity").(string)
	name := d.Get("name").(string)
	infraRef := d.Get("infra_ref").(string)
	tflog.Debug(ctx, fmt.Sprintf("Creating experiment from template: %s in hub: %s", templateIdentity, hubIdentity))

	// Build request
	req := chaos.TypesCreateExperimentFromTemplateRequest{
		AccountIdentifier:      c.AccountId,
		OrganizationIdentifier: orgID,
		ProjectIdentifier:      projectID,
		Name:                   name,
		InfraRef:               infraRef,
	}

	// Add optional fields
	if v, ok := d.GetOk("identity"); ok {
		req.Identity = v.(string)
		tflog.Debug(ctx, fmt.Sprintf("Using provided identity: %s", req.Identity))
	} else {
		// Generate identity from name
		// API requires: lowercase letters, numbers, and dashes only
		// Convert to lowercase and replace invalid characters with dashes
		identity := strings.ToLower(name)
		// Replace any character that's not lowercase letter, number, or dash with dash
		reg := regexp.MustCompile(`[^a-z0-9-]+`)
		identity = reg.ReplaceAllString(identity, "-")
		// Remove leading/trailing dashes
		identity = strings.Trim(identity, "-")
		req.Identity = identity
		tflog.Debug(ctx, fmt.Sprintf("Generated identity from name: %s -> %s", name, req.Identity))
	}

	if v, ok := d.GetOk("description"); ok {
		req.Description = v.(string)
	}

	if v, ok := d.GetOk("tags"); ok {
		req.Tags = expandTags(v.(*schema.Set).List())
	}

	// Add import_type (CRITICAL)
	importType := d.Get("import_type").(string)
	var importTypePtr *chaos.MongodbImportType
	if importType == "LOCAL" {
		local := chaos.LOCAL_MongodbImportType
		importTypePtr = &local
		tflog.Debug(ctx, "Using LOCAL import type (full copy)")
	} else {
		ref := chaos.REFERENCE_MongodbImportType
		importTypePtr = &ref
		tflog.Debug(ctx, "Using REFERENCE import type (template reference)")
	}
	req.ImportType = importTypePtr

	// Extract hub scope (where the template lives) - separate from experiment scope
	hubOrgID := d.Get("hub_org_id").(string)
	hubProjectID := d.Get("hub_project_id").(string)

	// Strip scope prefix from hub_identity before sending to API
	// User provides: "account.my-hub", "org.my-hub", or "my-hub"
	// API expects: "my-hub" (no prefix in query param)
	hubIdentityForAPI := hubIdentity
	hubIdentityForAPI = strings.TrimPrefix(hubIdentityForAPI, "account.")
	hubIdentityForAPI = strings.TrimPrefix(hubIdentityForAPI, "org.")
	// Project-level has no prefix, so no need to strip

	// Build optional parameters
	// CRITICAL: Query params define HUB scope (where template lives), NOT experiment scope
	opts := &chaos.ExperimenttemplateApiCreateExperimentFromTemplateOpts{
		HubIdentity:            optional.NewString(hubIdentityForAPI), // Stripped prefix
		OrganizationIdentifier: optional.NewString(hubOrgID),          // Hub's org (may be empty for account-level)
		ProjectIdentifier:      optional.NewString(hubProjectID),      // Hub's project (may be empty for org/account-level)
	}
	tflog.Debug(ctx, fmt.Sprintf("Hub location - org: %q, project: %q, identity: %q", hubOrgID, hubProjectID, hubIdentity))
	tflog.Debug(ctx, fmt.Sprintf("Experiment location - org: %q, project: %q", orgID, projectID))

	if v, ok := d.GetOk("revision"); ok {
		opts.Revision = optional.NewString(v.(string))
		tflog.Debug(ctx, fmt.Sprintf("Using template revision: %s", v.(string)))
	}
	tflog.

		// Call CreateExperimentFromTemplate API
		Debug(ctx, "Calling CreateExperimentFromTemplate API")
	resp, httpResp, err := c.ExperimenttemplateApi.CreateExperimentFromTemplate(
		ctx,
		req,
		c.AccountId,
		templateIdentity,
		opts,
	)

	if err != nil {
		return helpers.HandleWrite(ctx, d, err, httpResp)
	}

	// Extract experiment ID from response (CRITICAL: Use Id, not Identity)
	var experimentID string
	if resp.Data != nil && resp.Data.Id != "" {
		experimentID = resp.Data.Id
		tflog.Debug(ctx, fmt.Sprintf("Experiment created with ID: %s", experimentID))
	} else {
		return diag.Errorf("failed to get experiment ID from response")
	}

	// Store identity separately (different from ID)
	if resp.Data != nil && resp.Data.Identity != "" {
		d.Set("identity", resp.Data.Identity)
		tflog.Debug(ctx, fmt.Sprintf("Experiment identity: %s", resp.Data.Identity))
	}

	// Set ID: org_id/project_id/experiment_id (CRITICAL: Use experiment_id, not identity)
	d.SetId(fmt.Sprintf("%s/%s/%s", orgID, projectID, experimentID))
	tflog.Debug(ctx, fmt.Sprintf("Set resource ID: %s", d.Id()))

	// Store manifest if LOCAL import (only available in create response)
	if importType == "LOCAL" && resp.Data != nil && resp.Data.Manifest != "" {
		d.Set("manifest", resp.Data.Manifest)
		tflog.Debug(ctx, fmt.Sprintf("Stored manifest for LOCAL import, length: %d", len(resp.Data.Manifest)))
	}

	// Read back to populate all computed fields
	return resourceExperimentRead(ctx, d, meta)
}

// resourceExperimentRead reads the experiment details
func resourceExperimentRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	c, ctx := meta.(*internal.Session).GetChaosClientWithContext(ctx)

	// Parse ID: org_id/project_id/experiment_id (CRITICAL: This is experiment_id, not identity)
	parts := strings.Split(d.Id(), "/")
	if len(parts) != 3 {
		return diag.Errorf("invalid ID format, expected: org_id/project_id/experiment_id, got: %s", d.Id())
	}

	orgID := parts[0]
	projectID := parts[1]
	experimentID := parts[2]
	tflog. // This is the actual experiment ID from the API
		Debug(ctx, fmt.Sprintf("Reading experiment: %s (org: %s, project: %s)", experimentID, orgID, projectID))

	// Call GetChaosV2Experiment API (CRITICAL: Use experiment_id, not identity)
	experiment, httpResp, err := c.DefaultApi.GetChaosV2Experiment(
		ctx,
		c.AccountId,
		orgID,
		projectID,
		experimentID, // Use experiment_id from resource ID
	)

	if err != nil {
		return helpers.HandleRead(ctx, d, err, httpResp)
	}

	// Check if experiment is soft-deleted
	if experiment.IsRemoved {
		return helpers.ReadMissing(ctx, d, "chaos experiment "+experimentID+" (soft-deleted)")
	}

	// ========== SET ALL FIELDS ==========

	// Scope
	d.Set("org_id", experiment.OrgID)
	d.Set("project_id", experiment.ProjectID)

	// Basic info
	d.Set("name", experiment.Name)
	d.Set("identity", experiment.Identity)
	d.Set("description", experiment.Description)
	// Tags are refreshed from the API in full, so a tag added or removed outside Terraform
	// shows up in the plan.
	d.Set("tags", flattenTags(experiment.Tags))

	// Infrastructure
	d.Set("infra_id", experiment.InfraID)
	if experiment.InfraType != nil {
		d.Set("infra_type", string(*experiment.InfraType))
	}
	// infra_ref is the infrastructure identity/reference used during creation
	// The API returns infra_id which is the same value
	if experiment.InfraID != "" {
		d.Set("infra_ref", experiment.InfraID)
	}

	// Computed fields
	d.Set("experiment_id", experiment.ExperimentID)
	d.Set("is_custom_experiment", experiment.IsCustomExperiment)

	// Experiment type
	if experiment.ExperimentType != nil {
		d.Set("experiment_type", string(*experiment.ExperimentType))
	}

	// Fault IDs - always set to prevent drift
	d.Set("fault_ids", experiment.FaultIDs)

	// Scheduling
	d.Set("cron_syntax", experiment.CronSyntax)
	d.Set("is_cron_enabled", experiment.IsCronEnabled)
	d.Set("is_single_run_cron_enabled", experiment.IsSingleRunCronEnabled)

	// Execution history
	d.Set("last_executed_at", experiment.LastExecutedAt)
	d.Set("total_experiment_runs", experiment.TotalExperimentRuns)

	// Network
	d.Set("target_network_map_id", experiment.TargetNetworkMapID)

	// Metadata
	d.Set("created_at", experiment.CreatedAt)
	d.Set("created_by", experiment.CreatedBy)
	d.Set("updated_at", experiment.UpdatedAt)
	d.Set("updated_by", experiment.UpdatedBy)

	// Import type and manifest (infer from response)
	if experiment.TemplateDetails != nil {
		// REFERENCE import - has template details
		d.Set("import_type", "REFERENCE")
		tflog.Debug(ctx, "Experiment uses REFERENCE import (template reference)")

		templateDetails := []map[string]interface{}{
			{
				"identity":      experiment.TemplateDetails.Identity,
				"hub_reference": experiment.TemplateDetails.HubReference,
				"reference":     experiment.TemplateDetails.Reference,
				"revision":      experiment.TemplateDetails.Revision,
			},
		}
		d.Set("template_details", templateDetails)

		// Also set top-level hub_identity and template_identity for convenience
		hubRef := experiment.TemplateDetails.HubReference

		// Strip prefix from hub_reference to get hub_identity
		// API returns: "account.hub-name", "org.hub-name", or "hub-name"
		// User config expects: "hub-name" (no prefix)
		hubIdentity := hubRef
		if strings.HasPrefix(hubRef, "account.") {
			hubIdentity = strings.TrimPrefix(hubRef, "account.")
		} else if strings.HasPrefix(hubRef, "org.") {
			hubIdentity = strings.TrimPrefix(hubRef, "org.")
		}

		d.Set("hub_identity", hubIdentity)
		d.Set("template_identity", experiment.TemplateDetails.Identity)
		if experiment.TemplateDetails.Revision != "" {
			d.Set("revision", experiment.TemplateDetails.Revision)
		}

		// Derive hub scope from hub_reference prefix for better import UX
		// During normal operations, these come from user config and are preserved by Terraform
		// During import, we derive them so users get a complete state
		// Format: "account.hub-name", "org.hub-name", or "hub-name" (project-level)
		if strings.HasPrefix(hubRef, "account.") {
			// Account-level hub - hub_org_id and hub_project_id are empty
			d.Set("hub_org_id", "")
			d.Set("hub_project_id", "")
		} else if strings.HasPrefix(hubRef, "org.") {
			// Org-level hub - hub_org_id is experiment's org, hub_project_id is empty
			d.Set("hub_org_id", experiment.OrgID)
			d.Set("hub_project_id", "")
		} else {
			// Project-level hub (no prefix) - both match experiment scope
			d.Set("hub_org_id", experiment.OrgID)
			d.Set("hub_project_id", experiment.ProjectID)
		}
	} else {
		// LOCAL import - manifest stored during create (not available in read API)
		// Keep existing manifest value from state
		if manifestVal, ok := d.GetOk("manifest"); ok && manifestVal.(string) != "" {
			d.Set("import_type", "LOCAL")
			tflog.Debug(ctx, "Experiment uses LOCAL import (manifest preserved from state)")
		}
		// Set template_details to empty for LOCAL imports to prevent drift
		d.Set("template_details", []map[string]interface{}{})
	}
	tflog.Debug(ctx, fmt.Sprintf("Successfully read experiment: %s", experimentID))
	return nil
}

// resourceExperimentUpdate updates the experiment
func resourceExperimentUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	tflog.
		// Experiments created from templates have limited update support
		// Most fields are ForceNew, so changes will trigger recreation
		Debug(ctx, fmt.Sprintf("Update called for experiment: %s", d.Id()))

	// Check if only updatable fields changed
	if d.HasChanges("name", "description", "tags") {
		// For now, return error - updates not supported
		// Future: Implement via SaveChaosV2Experiment if needed
		return diag.Errorf("experiments created from templates cannot be updated directly. Changes to template_identity, hub_identity, infra_ref, or revision will trigger recreation.")
	}

	// If we reach here, no changes or only computed fields changed
	return resourceExperimentRead(ctx, d, meta)
}

// resourceExperimentDelete deletes the experiment
func resourceExperimentDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	c, ctx := meta.(*internal.Session).GetChaosClientWithContext(ctx)

	// Parse ID
	parts := strings.Split(d.Id(), "/")
	if len(parts) != 3 {
		return diag.Errorf("invalid ID format")
	}

	orgID := parts[0]
	projectID := parts[1]
	experimentIdentity := parts[2]
	tflog.Debug(ctx, fmt.Sprintf("Deleting experiment: %s (org: %s, project: %s)", experimentIdentity, orgID, projectID))

	// Call DeleteChaosV2Experiment API
	_, httpResp, err := c.DefaultApi.DeleteChaosV2Experiment(
		ctx,
		c.AccountId,
		orgID,
		projectID,
		experimentIdentity,
	)

	if err != nil {
		return helpers.HandleDelete(ctx, d, err, httpResp)
	}

	d.SetId("")
	tflog.Debug(ctx, fmt.Sprintf("Successfully deleted experiment: %s", experimentIdentity))
	return nil
}

// parseExperimentImportID parses an experiment import ID of the form
// "org_id/project_id/experiment_identity". All three segments are required.
func parseExperimentImportID(id string) (orgID, projectID, experimentID string, err error) {
	parts := strings.Split(id, "/")
	if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		return "", "", "", fmt.Errorf("invalid import ID format, expected: org_id/project_id/experiment_identity, got: %s", id)
	}
	return parts[0], parts[1], parts[2], nil
}

// resourceExperimentImport imports an existing experiment
func resourceExperimentImport(ctx context.Context, d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
	// Import format: org_id/project_id/experiment_identity
	orgID, projectID, _, err := parseExperimentImportID(d.Id())
	if err != nil {
		return nil, err
	}
	tflog.Debug(ctx, fmt.Sprintf("Importing experiment with ID: %s", d.Id()))

	// Set scope fields
	d.Set("org_id", orgID)
	d.Set("project_id", projectID)

	// Read to populate all fields
	diags := resourceExperimentRead(ctx, d, meta)
	if diags.HasError() {
		return nil, fmt.Errorf("failed to read experiment during import: %v", diags)
	}
	tflog.Debug(ctx, fmt.Sprintf("Successfully imported experiment: %s", d.Id()))
	return []*schema.ResourceData{d}, nil
}

// Helper functions

// expandTags converts Terraform tags to SDK format
func expandTags(tags []interface{}) []string {
	result := make([]string, 0, len(tags))
	for _, tag := range tags {
		if tag != nil {
			result = append(result, tag.(string))
		}
	}
	return result
}

// flattenTags converts SDK tags to Terraform format
func flattenTags(tags []string) *schema.Set {
	result := make([]interface{}, 0, len(tags))
	for _, tag := range tags {
		result = append(result, tag)
	}
	return schema.NewSet(schema.HashString, result)
}
