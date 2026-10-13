package helpers

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// The identity of a Harness object (its identifier and the scope it lives in) cannot be
// changed through the API: a different value names a different object. Terraform must
// therefore plan a replacement, never an in-place update, when one of them changes
// (#1347, #975, #1342). ForceNew is how the SDK expresses that; the shared schema setters
// apply it through MarkIdentity, hand-written schemas through Identity.

// MarkIdentity makes the named attributes ForceNew, so a change replaces the resource.
// Keys that are not in s are ignored. Data-source schemas must not use it.
func MarkIdentity(s map[string]*schema.Schema, keys ...string) {
	for _, k := range keys {
		if attr, ok := s[k]; ok {
			attr.ForceNew = true
		}
	}
}

// Identity returns s with ForceNew set, for schemas built inline from the Get*Schema
// builders, e.g. "org_id": helpers.Identity(helpers.GetOrgIdSchema(flag)).
func Identity(s *schema.Schema) *schema.Schema {
	s.ForceNew = true
	return s
}

// CreateOnly is a CustomizeDiff for inputs the API only accepts at create time and does
// not treat as identity (a repository's seed files or license): changing one after the
// resource exists is an error at plan time rather than a replacement, which would
// destroy the object to apply a cosmetic input.
func CreateOnly(keys ...string) schema.CustomizeDiffFunc {
	return func(ctx context.Context, d *schema.ResourceDiff, meta interface{}) error {
		if d.Id() == "" {
			return nil
		}
		var changed []string
		for _, k := range keys {
			if d.HasChange(k) {
				changed = append(changed, k)
			}
		}
		if len(changed) == 0 {
			return nil
		}
		return fmt.Errorf("%s can only be set when the resource is created; revert the change, or taint the resource to recreate it", strings.Join(changed, ", "))
	}
}

// WithIdentity is MarkIdentity for schemas built by a function:
// Schema: helpers.WithIdentity(resourceXSchema(), "org_id", "project_id").
func WithIdentity(s map[string]*schema.Schema, keys ...string) map[string]*schema.Schema {
	MarkIdentity(s, keys...)
	return s
}
