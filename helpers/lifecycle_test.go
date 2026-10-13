package helpers

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/harness/terraform-provider-harness/internal/apierr"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

type bodyErr struct {
	msg  string
	body string
}

func (e bodyErr) Error() string { return e.msg }
func (e bodyErr) Body() []byte  { return []byte(e.body) }

func newTestData(t *testing.T, id string) *schema.ResourceData {
	t.Helper()
	d := schema.TestResourceDataRaw(t, map[string]*schema.Schema{
		"name": {Type: schema.TypeString, Optional: true},
	}, map[string]interface{}{})
	d.SetId(id)
	return d
}

func gitData(t *testing.T) *schema.ResourceData {
	t.Helper()
	d := schema.TestResourceDataRaw(t, map[string]*schema.Schema{
		"git_details": {Type: schema.TypeList, Optional: true, MaxItems: 1, Elem: &schema.Resource{Schema: map[string]*schema.Schema{
			"connector_ref": {Type: schema.TypeString, Optional: true},
			"repo_name":     {Type: schema.TypeString, Optional: true},
		}}},
	}, map[string]interface{}{
		"git_details": []interface{}{map[string]interface{}{"connector_ref": "account.github", "repo_name": "myrepo"}},
	})
	d.SetId("obj")
	return d
}

func TestHandleRead(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		name    string
		err     error
		resp    *http.Response
		opts    []ReadOption
		cleared bool
		isError bool
	}{
		{name: "nil error", err: nil, cleared: false, isError: false},
		{name: "404 with envelope", err: bodyErr{"404 Not Found", `{"code":"ENTITY_NOT_FOUND","message":"gone"}`}, resp: &http.Response{StatusCode: 404}, cleared: true},
		{name: "404 html", err: bodyErr{"404 Not Found", `<html>nope</html>`}, resp: &http.Response{StatusCode: 404}, cleared: true},
		{name: "404 empty", err: errors.New("404 Not Found"), resp: &http.Response{StatusCode: 404}, cleared: true},
		{name: "400 with not-found code", err: bodyErr{"400 Bad Request", `{"code":"ENTITY_NOT_FOUND","message":"gone"}`}, resp: &http.Response{StatusCode: 400}, cleared: true},
		{name: "403 is an error even with not-found code", err: bodyErr{"403 Forbidden", `{"code":"ENTITY_NOT_FOUND"}`}, resp: &http.Response{StatusCode: 403}, isError: true},
		{name: "500 is an error", err: bodyErr{"500 Internal Server Error", `{"message":"boom"}`}, resp: &http.Response{StatusCode: 500}, isError: true},
		{name: "transport failure is an error", err: &url.Error{Op: "Get", URL: "x", Err: errors.New("refused")}, isError: true},
		{name: "retry budget exhausted is an error", err: errors.New("GET http://x giving up after 11 attempt(s)"), isError: true},
		{name: "predicate turns a 500 into gone", err: bodyErr{"500 Internal Server Error", `{"message":"no matching infra found"}`}, resp: &http.Response{StatusCode: 500},
			opts: []ReadOption{NotFoundWhen(apierr.And(apierr.StatusIs(500), apierr.MessageHasPhrase("no matching infra")))}, cleared: true},
		{name: "predicate does not match", err: bodyErr{"500 Internal Server Error", `{"message":"db down"}`}, resp: &http.Response{StatusCode: 500},
			opts: []ReadOption{NotFoundWhen(apierr.And(apierr.StatusIs(500), apierr.MessageHasPhrase("no matching infra")))}, isError: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := newTestData(t, "obj")
			diags := HandleRead(ctx, d, tc.err, tc.resp, tc.opts...)
			if diags.HasError() != tc.isError {
				t.Errorf("error = %v, want %v (%v)", diags.HasError(), tc.isError, diags)
			}
			if (d.Id() == "") != tc.cleared {
				t.Errorf("cleared = %v, want %v", d.Id() == "", tc.cleared)
			}
		})
	}
}

func TestHandleDelete(t *testing.T) {
	ctx := context.Background()
	d := newTestData(t, "obj")
	if diags := HandleDelete(ctx, d, errors.New("404 Not Found"), &http.Response{StatusCode: 404}); diags.HasError() {
		t.Errorf("deleting an already-deleted object must succeed: %v", diags)
	}
	if diags := HandleDelete(ctx, d, bodyErr{"500 Internal Server Error", `{"message":"boom"}`}, &http.Response{StatusCode: 500}); !diags.HasError() {
		t.Error("a 500 on delete must be an error")
	}
	if diags := HandleDelete(ctx, d, &url.Error{Op: "Delete", URL: "x", Err: errors.New("refused")}, nil); !diags.HasError() {
		t.Error("a transport failure on delete must be an error: the object may still exist")
	}
	if diags := HandleDelete(ctx, d, bodyErr{"500 Internal Server Error", `{"message":"infra not found"}`}, &http.Response{StatusCode: 500},
		NotFoundWhen(apierr.MessageHasPhrase("infra not found"))); diags.HasError() {
		t.Errorf("a predicate match on delete must succeed: %v", diags)
	}
	if d.Id() != "obj" {
		t.Errorf("HandleDelete must never touch the ID, got %q", d.Id())
	}
}

func TestHandleWrite(t *testing.T) {
	ctx := context.Background()
	d := newTestData(t, "obj")
	diags := HandleWrite(ctx, d, bodyErr{"404 Not Found", `{"code":"ENTITY_NOT_FOUND","message":"project p does not exist"}`}, &http.Response{StatusCode: 404})
	if !diags.HasError() || !strings.Contains(diags[0].Summary, "does not exist in Harness") || !strings.Contains(diags[0].Summary, "project p does not exist") {
		t.Errorf("not-found on write must be an actionable error: %v", diags)
	}
	diags = HandleWrite(ctx, d, bodyErr{"400 Bad Request", `{"code":"DUPLICATE_FIELD","message":"Connector with identifier x already exists"}`}, &http.Response{StatusCode: 400})
	if !diags.HasError() || !strings.Contains(diags[0].Summary, "terraform import") {
		t.Errorf("conflict on write must suggest import: %v", diags)
	}
	diags = HandleWrite(ctx, d, bodyErr{"401 Unauthorized", `{"message":"bad key"}`}, &http.Response{StatusCode: 401})
	if !diags.HasError() || !strings.Contains(diags[0].Summary, "platform_api_key") {
		t.Errorf("401 on write must carry the credentials hint: %v", diags)
	}
	if d.Id() != "obj" {
		t.Errorf("HandleWrite must never touch the ID, got %q", d.Id())
	}
	if diags := HandleWrite(ctx, d, nil, nil); diags != nil {
		t.Errorf("nil error must produce nil diagnostics, got %v", diags)
	}
}

func TestHandleWriteGit(t *testing.T) {
	ctx := context.Background()
	d := gitData(t)
	diags := HandleWriteGit(ctx, d, bodyErr{"400 Bad Request", `{"message":"No connector found with identifier account.github"}`}, &http.Response{StatusCode: 400})
	if !diags.HasError() || !strings.Contains(diags[0].Summary, "Invalid connector reference: account.github") {
		t.Errorf("connector rewrite: %v", diags)
	}
	diags = HandleWriteGit(ctx, d, bodyErr{"400 Bad Request", `{"message":"Please check the requested file path in myrepo"}`}, &http.Response{StatusCode: 400})
	if !diags.HasError() || !strings.Contains(diags[0].Summary, "Invalid repository name: myrepo") {
		t.Errorf("repo rewrite: %v", diags)
	}
	diags = HandleWriteGit(ctx, d, bodyErr{"400 Bad Request", `{"message":"yaml is invalid"}`}, &http.Response{StatusCode: 400})
	if !diags.HasError() || !strings.Contains(diags[0].Summary, "yaml is invalid") {
		t.Errorf("other 400s pass through HandleWrite: %v", diags)
	}
}

func TestHandleErrorAndReadMissing(t *testing.T) {
	ctx := context.Background()
	d := newTestData(t, "obj")
	if diags := HandleError(ctx, d, errors.New("404 Not Found"), &http.Response{StatusCode: 404}); !diags.HasError() || d.Id() != "obj" {
		t.Errorf("HandleError must error and keep the ID: %v %q", diags, d.Id())
	}
	if diags := ReadMissing(ctx, d, "connector obj"); diags != nil || d.Id() != "" {
		t.Errorf("ReadMissing must clear the ID without diagnostics: %v %q", diags, d.Id())
	}
	if StatusOf(nil) != 0 || StatusOf(&http.Response{StatusCode: 418}) != 418 {
		t.Error("StatusOf")
	}
	if !IsTransportError(&url.Error{Op: "Get", URL: "x", Err: errors.New("refused")}, nil) || IsTransportError(errors.New("x"), &http.Response{StatusCode: 500}) {
		t.Error("IsTransportError")
	}
}

func TestReadAfterWrite(t *testing.T) {
	ctx := context.Background()
	readAfterWriteDelay, readAfterWriteMaxDelay = time.Millisecond, 2*time.Millisecond
	t.Cleanup(func() { readAfterWriteDelay, readAfterWriteMaxDelay = time.Second, 10*time.Second })

	// missingFor returns a Read that reports the object missing for the first n calls.
	missingFor := func(n int, calls *int) schema.ReadContextFunc {
		return func(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
			*calls++
			if *calls <= n {
				d.SetId("")
				return nil
			}
			_ = d.Set("name", "from api")
			return nil
		}
	}

	t.Run("readable at once", func(t *testing.T) {
		d := newTestData(t, "obj")
		var calls int
		diags := ReadAfterWrite(ctx, d, nil, time.Second, missingFor(0, &calls))
		if diags.HasError() || calls != 1 || d.Id() != "obj" || d.Get("name") != "from api" {
			t.Errorf("diags=%v calls=%d id=%q name=%v", diags, calls, d.Id(), d.Get("name"))
		}
	})
	t.Run("missing then readable within the window", func(t *testing.T) {
		d := newTestData(t, "obj")
		var calls int
		diags := ReadAfterWrite(ctx, d, nil, time.Second, missingFor(2, &calls))
		if diags.HasError() || calls != 3 || d.Id() != "obj" || d.Get("name") != "from api" {
			t.Errorf("diags=%v calls=%d id=%q", diags, calls, d.Id())
		}
	})
	t.Run("window zero reads once and keeps the ID", func(t *testing.T) {
		d := newTestData(t, "obj")
		var calls int
		diags := ReadAfterWrite(ctx, d, nil, 0, missingFor(1, &calls))
		if !diags.HasError() || calls != 1 || d.Id() != "obj" || !strings.Contains(diags[0].Summary, "obj was written but could not be read back.") {
			t.Errorf("diags=%v calls=%d id=%q", diags, calls, d.Id())
		}
	})
	t.Run("still missing after the window keeps the ID and errors", func(t *testing.T) {
		d := newTestData(t, "obj")
		var calls int
		diags := ReadAfterWrite(ctx, d, nil, 20*time.Millisecond, missingFor(1000, &calls))
		if !diags.HasError() || calls < 2 || d.Id() != "obj" || !strings.Contains(diags[0].Summary, "could not be read back within 20ms") {
			t.Errorf("diags=%v calls=%d id=%q", diags, calls, d.Id())
		}
	})
	t.Run("read errors are returned unchanged", func(t *testing.T) {
		d := newTestData(t, "obj")
		var calls int
		diags := ReadAfterWrite(ctx, d, nil, time.Second, func(context.Context, *schema.ResourceData, interface{}) diag.Diagnostics {
			calls++
			return diag.Errorf("boom")
		})
		if !diags.HasError() || calls != 1 || diags[0].Summary != "boom" || d.Id() != "obj" {
			t.Errorf("diags=%v calls=%d id=%q", diags, calls, d.Id())
		}
	})
	t.Run("a cancelled context stops the wait", func(t *testing.T) {
		readAfterWriteDelay = time.Minute
		t.Cleanup(func() { readAfterWriteDelay = time.Millisecond })
		cctx, cancel := context.WithCancel(ctx)
		cancel()
		d := newTestData(t, "obj")
		var calls int
		diags := ReadAfterWrite(cctx, d, nil, time.Hour, missingFor(1000, &calls))
		if !diags.HasError() || calls != 1 || d.Id() != "obj" || !strings.Contains(diags[0].Summary, "context canceled") {
			t.Errorf("diags=%v calls=%d id=%q", diags, calls, d.Id())
		}
	})
	t.Run("the ID must be set first", func(t *testing.T) {
		d := newTestData(t, "")
		var calls int
		diags := ReadAfterWrite(ctx, d, nil, 0, missingFor(0, &calls))
		if !diags.HasError() || calls != 0 || !strings.Contains(diags[0].Summary, "provider bug") {
			t.Errorf("diags=%v calls=%d", diags, calls)
		}
	})
}
