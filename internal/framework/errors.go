package framework

import (
	"context"
	"net/http"

	"github.com/harness/terraform-provider-harness/internal/apierr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// The error helpers mirror helpers/lifecycle.go for framework resources: they are the only
// place a resource interprets an API error. Each returns whether err was set, so a call
// site reads `if framework.ReadError(...) { return }`.

func gone(c apierr.Classified, notFound []apierr.Predicate) bool {
	if c.Kind == apierr.NotFound {
		return true
	}
	for _, pred := range notFound {
		if pred(c) {
			return true
		}
	}
	return false
}

// IsNotFound reports whether err/resp classify as a missing object.
func IsNotFound(err error, resp *http.Response) bool { return apierr.IsNotFound(err, resp) }

// ReadError handles the error of a resource Read. When the object is gone (HTTP 404, or
// any extra notFound predicate) the resource is removed from state so Terraform plans a
// re-create; anything else is an error.
func ReadError(ctx context.Context, resp *resource.ReadResponse, err error, httpResp *http.Response, notFound ...apierr.Predicate) bool {
	if err == nil {
		return false
	}
	c := apierr.Classify(err, httpResp)
	if gone(c, notFound) {
		tflog.Warn(ctx, "resource no longer exists in Harness; removing it from state so it can be recreated", map[string]any{
			"http_status": c.Status, "code": c.Code, "message": c.Message,
		})
		resp.State.RemoveResource(ctx)
		return true
	}
	tflog.Debug(ctx, "read failed", map[string]any{"kind": c.Kind.String(), "http_status": c.Status, "code": c.Code})
	resp.Diagnostics.AddError("Reading the resource failed", apierr.Describe(c))
	return true
}

// ReadMissing handles a Read whose call succeeded but returned no object (nil data, an
// empty page, a filter that matched nothing): the resource is removed from state so
// Terraform plans a re-create. what names the object for the log.
func ReadMissing(ctx context.Context, resp *resource.ReadResponse, what string) {
	tflog.Warn(ctx, "Harness returned no object for the resource; removing it from state so it can be recreated", map[string]any{"object": what})
	resp.State.RemoveResource(ctx)
}

// DeleteError handles the error of a Delete. An object that is already gone is a
// successful delete; a transport failure never is, because the object may still exist.
func DeleteError(ctx context.Context, diags *diag.Diagnostics, err error, httpResp *http.Response, notFound ...apierr.Predicate) bool {
	if err == nil {
		return false
	}
	c := apierr.Classify(err, httpResp)
	if !c.Transport && gone(c, notFound) {
		tflog.Debug(ctx, "resource was already deleted in Harness", map[string]any{"http_status": c.Status, "code": c.Code})
		return false
	}
	diags.AddError("Deleting the resource failed", apierr.Describe(c))
	return true
}

// WriteError handles the error of a Create or Update. It never changes state; a missing
// scope and a conflict get actionable detail.
func WriteError(ctx context.Context, diags *diag.Diagnostics, err error, httpResp *http.Response) bool {
	if err == nil {
		return false
	}
	c := apierr.Classify(err, httpResp)
	msg := apierr.Describe(c)
	switch c.Kind {
	case apierr.NotFound:
		msg += "\nThe resource, or its scope (organization, project or parent object), does not exist in Harness. If it was deleted outside Terraform, run `terraform apply` again to recreate it."
	case apierr.Conflict:
		msg += "\nThe resource already exists in Harness. Import it with `terraform import`, or choose a different identifier."
	}
	tflog.Debug(ctx, "write failed", map[string]any{"kind": c.Kind.String(), "http_status": c.Status, "code": c.Code})
	diags.AddError("Writing the resource failed", msg)
	return true
}

// Error handles the error of a data source read or an auxiliary call: always an error,
// never a state change.
func Error(ctx context.Context, diags *diag.Diagnostics, summary string, err error, httpResp *http.Response) bool {
	if err == nil {
		return false
	}
	c := apierr.Classify(err, httpResp)
	tflog.Debug(ctx, "request failed", map[string]any{"kind": c.Kind.String(), "http_status": c.Status, "code": c.Code})
	diags.AddError(summary, apierr.Describe(c))
	return true
}
