package helpers

import (
	"errors"
	"net/http"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func newTestData(t *testing.T, id string) *schema.ResourceData {
	t.Helper()
	d := schema.TestResourceDataRaw(t, map[string]*schema.Schema{
		"name": {Type: schema.TypeString, Optional: true},
	}, map[string]interface{}{})
	d.SetId(id)
	return d
}

// The deprecated wrappers must keep their historical not-found behaviour while call sites
// migrate to the lifecycle helpers.
func TestDeprecatedReadWrappers_404ClearsState(t *testing.T) {
	wrappers := map[string]func(error, *schema.ResourceData, *http.Response) diagResult{
		"HandleReadApiError": func(e error, d *schema.ResourceData, r *http.Response) diagResult {
			return diagResult(HandleReadApiError(e, d, r))
		},
		"HandleIacmReadApiError": func(e error, d *schema.ResourceData, r *http.Response) diagResult {
			return diagResult(HandleIacmReadApiError(e, d, r))
		},
		"HandleDBOpsReadApiError": func(e error, d *schema.ResourceData, r *http.Response) diagResult {
			return diagResult(HandleDBOpsReadApiError(e, d, r))
		},
		"HandleCodeReadApiError": func(e error, d *schema.ResourceData, r *http.Response) diagResult {
			return diagResult(HandleCodeReadApiError(e, d, r))
		},
		"HandleChaosReadApiError": func(e error, d *schema.ResourceData, r *http.Response) diagResult {
			return diagResult(HandleChaosReadApiError(e, d, r))
		},
		"HandlePolicyApiError": func(e error, d *schema.ResourceData, r *http.Response) diagResult {
			return diagResult(HandlePolicyApiError(e, d, r))
		},
	}
	for name, fn := range wrappers {
		t.Run(name, func(t *testing.T) {
			d := newTestData(t, "obj")
			if diags := fn(errors.New("404 Not Found"), d, &http.Response{StatusCode: 404}); diags.HasError() {
				t.Fatalf("404 on read must not be an error, got %v", diags)
			}
			if d.Id() != "" {
				t.Errorf("404 on read must clear the ID, got %q", d.Id())
			}
			d = newTestData(t, "obj")
			if diags := fn(errors.New("500 Internal Server Error"), d, &http.Response{StatusCode: 500}); !diags.HasError() {
				t.Fatal("500 on read must be an error")
			}
			if d.Id() != "obj" {
				t.Errorf("500 on read must keep the ID, got %q", d.Id())
			}
		})
	}
}

func TestDeprecatedWriteWrappers_404IsAnError(t *testing.T) {
	d := newTestData(t, "obj")
	for name, diags := range map[string]diagResult{
		"HandleApiError":      diagResult(HandleApiError(errors.New("404 Not Found"), d, &http.Response{StatusCode: 404})),
		"HandleDBOpsApiError": diagResult(HandleDBOpsApiError(errors.New("404 Not Found"), d, &http.Response{StatusCode: 404})),
		"HandleCodeApiError":  diagResult(HandleCodeApiError(errors.New("404 Not Found"), d, &http.Response{StatusCode: 404})),
		"HandleChaosApiError": diagResult(HandleChaosApiError(errors.New("404 Not Found"), d, &http.Response{StatusCode: 404})),
	} {
		if !diags.HasError() {
			t.Errorf("%s: 404 outside a read must be an error", name)
		}
	}
	if d.Id() != "obj" {
		t.Errorf("write wrappers must never clear the ID, got %q", d.Id())
	}
}

func TestDeprecatedChaosWrappers(t *testing.T) {
	d := newTestData(t, "obj")
	diags := HandleChaosReadApiErrorWithGracefulDestroy(errors.New("500 Internal Server Error"), d, &http.Response{StatusCode: 500}, []string{"no matching infra"})
	if !diags.HasError() || d.Id() != "obj" {
		t.Errorf("a 500 whose message does not match the pattern must be an error without clearing: %v %q", diags, d.Id())
	}

	d = newTestData(t, "obj")
	if diags := HandleChaosGraphQLReadError(errors.New("graphql error: hub not found (path: [getHub])"), d, "read"); diags.HasError() || d.Id() != "" {
		t.Errorf("GraphQL 'not found' on read must clear state: %v %q", diags, d.Id())
	}
	d = newTestData(t, "obj")
	if diags := HandleChaosGraphQLError(errors.New("graphql error: hub not found (path: [getHub])"), d, "delete"); !diags.HasError() || d.Id() != "obj" {
		t.Errorf("GraphQL errors outside a read must be errors: %v %q", diags, d.Id())
	}
}

type diagResult = interface{ HasError() bool }
