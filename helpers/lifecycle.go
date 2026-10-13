package helpers

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/harness/terraform-provider-harness/internal/apierr"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// The lifecycle helpers turn an SDK error into the right Terraform behaviour for the
// operation that produced it. They are the only place a resource should interpret API
// errors:
//
//   - HandleRead: a missing object is removed from state (Terraform then plans a
//     re-create); anything else is an error.
//   - HandleDelete: a missing object is a successful delete.
//   - HandleWrite / HandleWriteGit: never touch state; not-found and conflicts get
//     actionable messages.
//   - HandleError: data sources and auxiliary calls; always an error.
//   - ReadMissing: the call succeeded but returned no object; same as a not-found read.
//
// Classification is done by internal/apierr, which works for every Harness SDK client and
// never panics. Callers must never dereference the *http.Response before checking the
// error; use StatusOf when a status code is needed.

// ReadOption customises how HandleRead and HandleDelete recognise a missing object.
type ReadOption func(*readOptions)

type readOptions struct {
	notFound []apierr.Predicate
}

// NotFoundWhen treats errors matching pred as "the object is gone". HTTP 404 is always
// treated as gone; use this only for backends that report a missing object some other way
// (a chaos 500 whose message names the missing infrastructure, an IDP 5xx that says the
// entity was not found). Keep predicates narrow: a missing parent must not look like a
// missing child.
func NotFoundWhen(pred apierr.Predicate) ReadOption {
	return func(o *readOptions) { o.notFound = append(o.notFound, pred) }
}

func gone(c apierr.Classified, opts []ReadOption) bool {
	if c.Kind == apierr.NotFound {
		return true
	}
	var o readOptions
	for _, opt := range opts {
		opt(&o)
	}
	for _, pred := range o.notFound {
		if pred(c) {
			return true
		}
	}
	return false
}

// StatusOf returns the HTTP status of resp, or 0 when there was no response.
func StatusOf(resp *http.Response) int { return apierr.StatusOf(resp) }

// IsTransportError reports whether the request never reached the API (DNS, connection,
// TLS or timeout failure).
func IsTransportError(err error, resp *http.Response) bool { return apierr.IsTransport(err, resp) }

// HandleRead handles the error of a resource Read. When the object is gone it logs, clears
// the ID so Terraform plans a re-create, and returns no diagnostics.
func HandleRead(ctx context.Context, d *schema.ResourceData, err error, resp *http.Response, opts ...ReadOption) diag.Diagnostics {
	if err == nil {
		return nil
	}
	c := apierr.Classify(err, resp)
	if gone(c, opts) {
		tflog.Warn(ctx, "resource no longer exists in Harness; removing it from state so it can be recreated", map[string]any{
			"id": d.Id(), "http_status": c.Status, "code": c.Code, "message": c.Message,
		})
		d.SetId("")
		return nil
	}
	tflog.Debug(ctx, "read failed", map[string]any{"id": d.Id(), "kind": c.Kind.String(), "http_status": c.Status, "code": c.Code})
	return diag.Errorf("%s", apierr.Describe(c))
}

// HandleDelete handles the error of a resource Delete. An object that is already gone is a
// successful delete. A transport failure is never treated as gone: the object may still exist.
func HandleDelete(ctx context.Context, d *schema.ResourceData, err error, resp *http.Response, opts ...ReadOption) diag.Diagnostics {
	if err == nil {
		return nil
	}
	c := apierr.Classify(err, resp)
	if !c.Transport && gone(c, opts) {
		tflog.Debug(ctx, "resource was already deleted in Harness", map[string]any{"id": d.Id(), "http_status": c.Status, "code": c.Code})
		return nil
	}
	return diag.Errorf("%s", apierr.Describe(c))
}

// HandleWrite handles the error of a Create or Update. It never changes state.
func HandleWrite(ctx context.Context, d *schema.ResourceData, err error, resp *http.Response) diag.Diagnostics {
	if err == nil {
		return nil
	}
	c := apierr.Classify(err, resp)
	msg := apierr.Describe(c)
	switch c.Kind {
	case apierr.NotFound:
		msg += "\nThe resource, or its scope (organization, project or parent object), does not exist in Harness. If it was deleted outside Terraform, run `terraform apply` again to recreate it."
	case apierr.Conflict:
		msg += "\nThe resource already exists in Harness. Import it with `terraform import`, or choose a different identifier."
	}
	tflog.Debug(ctx, "write failed", map[string]any{"id": d.Id(), "kind": c.Kind.String(), "http_status": c.Status, "code": c.Code})
	return diag.Errorf("%s", msg)
}

// HandleWriteGit is HandleWrite for resources with a git_details block: the two Git-related
// 400s the API returns for a bad connector reference or repository name become actionable.
func HandleWriteGit(ctx context.Context, d *schema.ResourceData, err error, resp *http.Response) diag.Diagnostics {
	if err == nil {
		return nil
	}
	c := apierr.Classify(err, resp)
	connectorRef, _ := d.Get("git_details.0.connector_ref").(string)
	repoName, _ := d.Get("git_details.0.repo_name").(string)
	if msg, ok := apierr.GitRewrite(c, connectorRef, repoName); ok {
		return diag.Errorf("%s", msg)
	}
	return HandleWrite(ctx, d, err, resp)
}

// HandleError handles the error of a data source read or an auxiliary call: always an error,
// never a state change.
func HandleError(ctx context.Context, d *schema.ResourceData, err error, resp *http.Response) diag.Diagnostics {
	if err == nil {
		return nil
	}
	c := apierr.Classify(err, resp)
	tflog.Debug(ctx, "request failed", map[string]any{"id": d.Id(), "kind": c.Kind.String(), "http_status": c.Status, "code": c.Code})
	return diag.Errorf("%s", apierr.Describe(c))
}

// ReadMissing handles a Read whose call succeeded but returned no object (nil data, an
// empty page, a list filter that matched nothing). It behaves like a not-found read: the
// resource is removed from state so Terraform plans a re-create. what names the object for
// the log, e.g. "connector myconn".
func ReadMissing(ctx context.Context, d *schema.ResourceData, what string) diag.Diagnostics {
	tflog.Warn(ctx, "Harness returned no object for the resource; removing it from state so it can be recreated", map[string]any{"id": d.Id(), "object": strings.TrimSpace(what)})
	d.SetId("")
	return nil
}

// MessageContains matches when the message contains any phrase (case-insensitive). Unlike
// apierr.MessageHasPhrase it accepts any phrase length; it exists so call sites migrated
// mechanically keep their historical matching until each is narrowed by hand.
func MessageContains(phrases ...string) apierr.Predicate {
	return func(c apierr.Classified) bool {
		msg := strings.ToLower(c.Message)
		for _, p := range phrases {
			if p != "" && strings.Contains(msg, strings.ToLower(p)) {
				return true
			}
		}
		return false
	}
}

// GracefulNotFound treats a response with the given status whose message contains one of
// the phrases as "gone" (chaos APIs answer 500 for some missing objects).
func GracefulNotFound(status int, phrases ...string) ReadOption {
	return NotFoundWhen(apierr.And(apierr.StatusIs(status), MessageContains(phrases...)))
}

// GraphQLNotFound treats a chaos GraphQL error mentioning "not found" as "gone". Prefer a
// phrase that names the object (for example "hub not found") where the API provides one.
func GraphQLNotFound() ReadOption {
	return NotFoundWhen(MessageContains("not found"))
}

// Read-after-write pacing. The first wait doubles on every attempt up to the maximum; tests
// shorten both.
var (
	readAfterWriteDelay    = time.Second
	readAfterWriteMaxDelay = 10 * time.Second
)

// ReadAfterWrite finishes a Create or Update by reading the object back through read, the
// resource's own ReadContext, so state always reflects what the API stored. d.Id() must be
// set before the call.
//
// Some Harness backends index writes asynchronously, so the object may not be readable for a
// short while after the write succeeded (#1385). While read reports the object as missing (it
// clears the ID, as HandleRead and ReadMissing do) the read is repeated, with backoff, until
// window has elapsed; a window of zero means a single read. Any other read failure is
// returned as is.
//
// If the object is still missing when the window ends, the ID is restored and an error is
// returned: Terraform keeps the resource in state and the next apply retries. Returning with
// an empty ID instead would make Terraform report an opaque "inconsistent result after apply"
// and forget an object that exists.
//
// Use it only from Create and Update, never from Read or plan paths.
func ReadAfterWrite(ctx context.Context, d *schema.ResourceData, meta interface{}, window time.Duration, read schema.ReadContextFunc) diag.Diagnostics {
	id := d.Id()
	if id == "" {
		return diag.Errorf("provider bug: ReadAfterWrite called before the resource ID was set; report this at https://github.com/harness/terraform-provider-harness/issues")
	}
	deadline := time.Now().Add(window)
	delay := readAfterWriteDelay
	for attempt := 1; ; attempt++ {
		d.SetId(id)
		diags := read(ctx, d, meta)
		if diags.HasError() || d.Id() != "" {
			return diags
		}
		remaining := time.Until(deadline)
		if remaining <= 0 {
			break
		}
		wait := min(delay, remaining)
		tflog.Debug(ctx, "resource is not readable yet after a write; retrying", map[string]any{"id": id, "attempt": attempt, "retry_in": wait.String()})
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			d.SetId(id)
			return diag.FromErr(ctx.Err())
		case <-timer.C:
		}
		delay = min(delay*2, readAfterWriteMaxDelay)
	}
	d.SetId(id)
	tflog.Warn(ctx, "resource was written but could not be read back", map[string]any{"id": id, "window": window.String()})
	within := ""
	if window > 0 {
		within = fmt.Sprintf(" within %s", window)
	}
	return diag.Errorf("%s was written but could not be read back%s. Terraform has kept the resource in state; run `terraform apply` again. If this persists, the API may not expose the object at the scope Terraform reads it from.", id, within)
}
