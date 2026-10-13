package helpers

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// ImportSpec describes the shape of a resource's import ID: "/"-separated attribute values,
// possibly with optional leading scope segments. NewImporter turns it into a validated
// importer that returns an error (never panics) on a malformed ID, reports every d.Set
// failure, and documents the accepted formats through Format.
//
// Keep the provider's ID convention: the resource ID is the bare identifier and the scope
// lives in attributes, so Parts usually ends with "identifier".
type ImportSpec struct {
	// Parts names the attribute each segment is stored in, in order. An empty name consumes
	// the segment without storing it (it can still become the ID).
	Parts []string
	// OptionalPrefix is how many leading Parts may be omitted, for account/org/project
	// scoping: "<id>", "<org_id>/<id>" and "<org_id>/<project_id>/<id>" are all accepted
	// when Parts is {org_id, project_id, identifier} and OptionalPrefix is 2.
	OptionalPrefix int
	// OptionalSuffix is how many trailing Parts may be omitted ("<email>",
	// "<email>/<org_id>", "<email>/<org_id>/<project_id>"). Mutually exclusive with
	// OptionalPrefix.
	OptionalSuffix int
	// IDFrom names the Part whose value becomes the resource ID. Default: the last segment.
	IDFrom string
	// RawID keeps the whole import string as the resource ID (composite-ID resources).
	RawID bool
	// Tail allows one extra trailing segment that may itself contain "/" (a git branch
	// name); it is split off with SplitN so it is never torn apart. Not combinable with
	// OptionalPrefix, which would make the segments ambiguous.
	Tail *ImportTail
	// Doc replaces the generated format description in errors and documentation.
	Doc string
}

// ImportTail describes the optional trailing segment of an ImportSpec.
type ImportTail struct {
	// Name is used in the format description, e.g. "branch".
	Name string
	// Set stores the value, e.g. into git_details.0.branch_name.
	Set func(d *schema.ResourceData, value string) error
}

// Validate checks the spec for internal consistency.
func (s ImportSpec) Validate() error {
	n := len(s.Parts)
	switch {
	case n == 0:
		return fmt.Errorf("ImportSpec needs at least one part")
	case s.OptionalPrefix < 0 || s.OptionalSuffix < 0:
		return fmt.Errorf("ImportSpec optional counts must not be negative")
	case s.OptionalPrefix > 0 && s.OptionalSuffix > 0:
		return fmt.Errorf("ImportSpec cannot have both an optional prefix and an optional suffix")
	case s.OptionalPrefix >= n || s.OptionalSuffix >= n:
		return fmt.Errorf("ImportSpec must keep at least one mandatory part")
	case s.Tail != nil && s.OptionalPrefix > 0:
		return fmt.Errorf("ImportSpec cannot combine a tail with an optional prefix")
	case s.Tail != nil && s.Tail.Set == nil:
		return fmt.Errorf("ImportSpec tail needs a Set function")
	}
	if s.IDFrom != "" {
		found := false
		for _, p := range s.Parts {
			if p == s.IDFrom {
				found = true
			}
		}
		if !found {
			return fmt.Errorf("ImportSpec IDFrom %q is not one of the parts", s.IDFrom)
		}
	}
	return nil
}

func (s ImportSpec) minParts() int { return len(s.Parts) - s.OptionalPrefix - s.OptionalSuffix }

// Forms lists the accepted import ID formats, shortest first.
func (s ImportSpec) Forms() []string {
	n := len(s.Parts)
	var forms []string
	for count := s.minParts(); count <= n; count++ {
		names := s.names(count)
		segs := make([]string, 0, len(names))
		for _, name := range names {
			if name == "" {
				name = "id"
			}
			segs = append(segs, "<"+name+">")
		}
		form := strings.Join(segs, "/")
		if s.Tail != nil && count == n {
			form += "[/<" + s.Tail.Name + ">]"
		}
		forms = append(forms, form)
	}
	return forms
}

// Format describes the accepted import ID formats for error messages and documentation.
func (s ImportSpec) Format() string {
	if s.Doc != "" {
		return s.Doc
	}
	return strings.Join(s.Forms(), " or ")
}

// names returns the part names that the given number of segments maps to.
func (s ImportSpec) names(count int) []string {
	n := len(s.Parts)
	switch {
	case s.OptionalPrefix > 0:
		// Omitted leading parts are the innermost scopes: "<id>", "<org>/<id>", "<org>/<project>/<id>".
		mandatory := n - s.OptionalPrefix
		present := count - mandatory
		names := make([]string, 0, count)
		names = append(names, s.Parts[:present]...)
		return append(names, s.Parts[s.OptionalPrefix:]...)
	case s.OptionalSuffix > 0:
		return s.Parts[:count]
	default:
		return s.Parts
	}
}

// Parse splits an import ID according to the spec. attrs maps part names to values; tail is
// the trailing segment when the spec has one and the ID carried it; hasTail says so.
func (s ImportSpec) Parse(id string) (attrs map[string]string, resourceID string, tail string, hasTail bool, err error) {
	if err := s.Validate(); err != nil {
		return nil, "", "", false, err
	}
	n := len(s.Parts)
	if strings.TrimSpace(id) == "" {
		return nil, "", "", false, fmt.Errorf("import ID is empty; expected %s", s.Format())
	}
	var segs []string
	if s.Tail != nil {
		segs = strings.SplitN(id, "/", n+1)
	} else {
		segs = strings.Split(id, "/")
	}
	if s.Tail != nil && len(segs) == n+1 {
		tail, hasTail = segs[n], true
		segs = segs[:n]
	}
	for _, seg := range segs {
		if seg == "" {
			return nil, "", "", false, fmt.Errorf("unexpected import ID %q: it contains an empty segment; expected %s", id, s.Format())
		}
	}
	if len(segs) < s.minParts() || len(segs) > n {
		return nil, "", "", false, fmt.Errorf("unexpected import ID %q: expected %s", id, s.Format())
	}
	if hasTail && tail == "" {
		return nil, "", "", false, fmt.Errorf("unexpected import ID %q: trailing %s is empty", id, s.Tail.Name)
	}
	names := s.names(len(segs))
	attrs = make(map[string]string, len(names))
	for i, name := range names {
		if name != "" {
			attrs[name] = segs[i]
		}
	}
	switch {
	case s.RawID:
		resourceID = id
	case s.IDFrom != "":
		v, ok := attrs[s.IDFrom]
		if !ok {
			return nil, "", "", false, fmt.Errorf("unexpected import ID %q: it does not include %s; expected %s", id, s.IDFrom, s.Format())
		}
		resourceID = v
	default:
		resourceID = segs[len(segs)-1]
	}
	return attrs, resourceID, tail, hasTail, nil
}

// IdentitySchema derives a resource identity schema (Terraform 1.12 import blocks and
// `terraform query`) from the spec: every named part is a string, optional scope parts are
// OptionalForImport and the rest RequiredForImport.
func (s ImportSpec) IdentitySchema() map[string]*schema.Schema {
	n := len(s.Parts)
	out := map[string]*schema.Schema{}
	for i, name := range s.Parts {
		if name == "" {
			continue
		}
		optional := (s.OptionalPrefix > 0 && i < s.OptionalPrefix) || (s.OptionalSuffix > 0 && i >= n-s.OptionalSuffix)
		out[name] = &schema.Schema{Type: schema.TypeString, RequiredForImport: !optional, OptionalForImport: optional}
	}
	return out
}

var importSpecs sync.Map // *schema.ResourceImporter -> ImportSpec

// ImportSpecOf returns the spec an importer was built from, for tests and documentation.
func ImportSpecOf(imp *schema.ResourceImporter) (ImportSpec, bool) {
	v, ok := importSpecs.Load(imp)
	if !ok {
		return ImportSpec{}, false
	}
	return v.(ImportSpec), true
}

// NewImporter builds a validated importer from spec. It panics on an invalid spec, which the
// provider's own tests catch at init.
func NewImporter(spec ImportSpec) *schema.ResourceImporter {
	if err := spec.Validate(); err != nil {
		panic(err)
	}
	imp := &schema.ResourceImporter{
		StateContext: func(ctx context.Context, d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
			attrs, id, tail, hasTail, err := spec.Parse(d.Id())
			if err != nil {
				return nil, err
			}
			for _, name := range spec.Parts {
				if v, ok := attrs[name]; ok {
					if err := d.Set(name, v); err != nil {
						return nil, fmt.Errorf("import: cannot set %s from the import ID: %w", name, err)
					}
				}
			}
			if hasTail {
				if err := spec.Tail.Set(d, tail); err != nil {
					return nil, fmt.Errorf("import: cannot set %s from the import ID: %w", spec.Tail.Name, err)
				}
			}
			d.SetId(id)
			return []*schema.ResourceData{d}, nil
		},
	}
	importSpecs.Store(imp, spec)
	return imp
}

// SplitImportID splits id on "/" and checks that it has between min and max non-empty
// segments, for the few importers whose format is too irregular for an ImportSpec.
func SplitImportID(id string, min, max int) ([]string, error) {
	if strings.TrimSpace(id) == "" {
		return nil, fmt.Errorf("import ID is empty")
	}
	segs := strings.Split(id, "/")
	for _, seg := range segs {
		if seg == "" {
			return nil, fmt.Errorf("unexpected import ID %q: it contains an empty segment", id)
		}
	}
	if len(segs) < min || len(segs) > max {
		if min == max {
			return nil, fmt.Errorf("unexpected import ID %q: expected %d '/'-separated segments", id, min)
		}
		return nil, fmt.Errorf("unexpected import ID %q: expected between %d and %d '/'-separated segments", id, min, max)
	}
	return segs, nil
}

// PassthroughImporter keeps the import ID as the resource ID and rejects an empty one.
func PassthroughImporter() *schema.ResourceImporter {
	return &schema.ResourceImporter{
		StateContext: func(ctx context.Context, d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
			if strings.TrimSpace(d.Id()) == "" {
				return nil, fmt.Errorf("import ID is empty")
			}
			return []*schema.ResourceData{d}, nil
		},
	}
}

// BranchTail is the ImportTail for git-backed resources: the trailing segment is the branch
// name (which may contain "/") and is stored in git_details.0.branch_name.
var BranchTail = &ImportTail{
	Name: "branch",
	Set: func(d *schema.ResourceData, branch string) error {
		return d.Set("git_details", []interface{}{map[string]interface{}{"branch_name": branch}})
	},
}
