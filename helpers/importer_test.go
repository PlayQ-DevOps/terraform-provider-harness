package helpers

import (
	"context"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func importData(t *testing.T, id string, attrs ...string) *schema.ResourceData {
	t.Helper()
	s := map[string]*schema.Schema{
		"git_details": {Type: schema.TypeList, Optional: true, MaxItems: 1, Elem: &schema.Resource{Schema: map[string]*schema.Schema{
			"branch_name": {Type: schema.TypeString, Optional: true},
		}}},
	}
	for _, a := range attrs {
		s[a] = &schema.Schema{Type: schema.TypeString, Optional: true}
	}
	d := schema.TestResourceDataRaw(t, s, map[string]interface{}{})
	d.SetId(id)
	return d
}

func TestImportSpecParse(t *testing.T) {
	multi := ImportSpec{Parts: []string{"org_id", "project_id", "identifier"}, OptionalPrefix: 2}
	strict := ImportSpec{Parts: []string{"org_id", "project_id", "identifier"}}
	branch := ImportSpec{Parts: []string{"org_id", "project_id", "identifier"}, Tail: BranchTail}
	suffix := ImportSpec{Parts: []string{"email", "org_id", "project_id"}, OptionalSuffix: 2, RawID: true}
	idFrom := ImportSpec{Parts: []string{"org_id", "project_id", "identifier", "type"}, OptionalPrefix: 2, IDFrom: "identifier"}
	unnamed := ImportSpec{Parts: []string{"agent_id", ""}}

	cases := []struct {
		name    string
		spec    ImportSpec
		id      string
		attrs   map[string]string
		resID   string
		tail    string
		wantErr string
	}{
		{"multi account", multi, "c1", map[string]string{"identifier": "c1"}, "c1", "", ""},
		{"multi org", multi, "o/c1", map[string]string{"org_id": "o", "identifier": "c1"}, "c1", "", ""},
		{"multi project", multi, "o/p/c1", map[string]string{"org_id": "o", "project_id": "p", "identifier": "c1"}, "c1", "", ""},
		{"multi too many", multi, "o/p/c1/x", nil, "", "", "expected <identifier> or <org_id>/<identifier> or <org_id>/<project_id>/<identifier>"},
		{"strict ok", strict, "o/p/c1", map[string]string{"org_id": "o", "project_id": "p", "identifier": "c1"}, "c1", "", ""},
		{"strict too few", strict, "o/c1", nil, "", "", "expected <org_id>/<project_id>/<identifier>"},
		{"empty id", strict, "", nil, "", "", "import ID is empty"},
		{"empty segment", strict, "o//c1", nil, "", "", "empty segment"},
		{"only slash", strict, "/", nil, "", "", "empty segment"},
		{"branch absent", branch, "o/p/pipe", map[string]string{"org_id": "o", "project_id": "p", "identifier": "pipe"}, "pipe", "", ""},
		{"branch simple", branch, "o/p/pipe/main", map[string]string{"org_id": "o", "project_id": "p", "identifier": "pipe"}, "pipe", "main", ""},
		{"branch with slashes", branch, "o/p/pipe/feature/x/y", map[string]string{"org_id": "o", "project_id": "p", "identifier": "pipe"}, "pipe", "feature/x/y", ""},
		{"branch empty", branch, "o/p/pipe/", nil, "", "", "trailing branch is empty"},
		{"suffix one", suffix, "a@b.c", map[string]string{"email": "a@b.c"}, "a@b.c", "", ""},
		{"suffix three", suffix, "a@b.c/o/p", map[string]string{"email": "a@b.c", "org_id": "o", "project_id": "p"}, "a@b.c/o/p", "", ""},
		{"idfrom", idFrom, "o/f1/Connector", map[string]string{"org_id": "o", "identifier": "f1", "type": "Connector"}, "f1", "", ""},
		{"unnamed part becomes id", unnamed, "agent/cert1", map[string]string{"agent_id": "agent"}, "cert1", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			attrs, resID, tail, hasTail, err := tc.spec.Parse(tc.id)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("err = %v, want it to contain %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if resID != tc.resID {
				t.Errorf("resource ID = %q, want %q", resID, tc.resID)
			}
			if tail != tc.tail || hasTail != (tc.tail != "") {
				t.Errorf("tail = %q (%v), want %q", tail, hasTail, tc.tail)
			}
			if len(attrs) != len(tc.attrs) {
				t.Errorf("attrs = %v, want %v", attrs, tc.attrs)
			}
			for k, v := range tc.attrs {
				if attrs[k] != v {
					t.Errorf("attrs[%s] = %q, want %q", k, attrs[k], v)
				}
			}
		})
	}
}

func TestImportSpecValidateAndFormat(t *testing.T) {
	bad := []ImportSpec{
		{},
		{Parts: []string{"a"}, OptionalPrefix: 1},
		{Parts: []string{"a", "b"}, OptionalPrefix: 1, OptionalSuffix: 1},
		{Parts: []string{"a", "b"}, OptionalPrefix: 1, Tail: BranchTail},
		{Parts: []string{"a", "b"}, IDFrom: "zzz"},
		{Parts: []string{"a", "b"}, Tail: &ImportTail{Name: "x"}},
	}
	for i, s := range bad {
		if err := s.Validate(); err == nil {
			t.Errorf("spec %d should be invalid: %+v", i, s)
		}
	}
	func() {
		defer func() {
			if recover() == nil {
				t.Error("NewImporter must panic on an invalid spec")
			}
		}()
		NewImporter(ImportSpec{})
	}()
	multi := ImportSpec{Parts: []string{"org_id", "project_id", "identifier"}, OptionalPrefix: 2}
	if got := multi.Format(); got != "<identifier> or <org_id>/<identifier> or <org_id>/<project_id>/<identifier>" {
		t.Errorf("Format = %q", got)
	}
	branch := ImportSpec{Parts: []string{"org_id", "project_id", "identifier"}, Tail: BranchTail}
	if got := branch.Format(); got != "<org_id>/<project_id>/<identifier>[/<branch>]" {
		t.Errorf("Format = %q", got)
	}
	if got := (ImportSpec{Parts: []string{"a"}, Doc: "custom"}).Format(); got != "custom" {
		t.Errorf("Doc override = %q", got)
	}
	ident := multi.IdentitySchema()
	if !ident["identifier"].RequiredForImport || ident["identifier"].OptionalForImport || !ident["org_id"].OptionalForImport || ident["org_id"].RequiredForImport {
		t.Errorf("IdentitySchema = %+v", ident)
	}
}

func TestNewImporterSetsAttributesAndID(t *testing.T) {
	ctx := context.Background()
	d := importData(t, "o/p/pipe/feature/x", "org_id", "project_id", "identifier")
	res, err := ProjectResourceWithBranchImporter.StateContext(ctx, d, nil)
	if err != nil || len(res) != 1 {
		t.Fatalf("import failed: %v", err)
	}
	if d.Id() != "pipe" || d.Get("org_id") != "o" || d.Get("project_id") != "p" || d.Get("identifier") != "pipe" || d.Get("git_details.0.branch_name") != "feature/x" {
		t.Errorf("imported state: id=%q org=%v project=%v identifier=%v branch=%v", d.Id(), d.Get("org_id"), d.Get("project_id"), d.Get("identifier"), d.Get("git_details.0.branch_name"))
	}

	d = importData(t, "o/p/c1", "org_id", "project_id", "identifier")
	if _, err := MultiLevelResourceImporter.StateContext(ctx, d, nil); err != nil || d.Id() != "c1" || d.Get("project_id") != "p" {
		t.Errorf("multi-level import: %v id=%q", err, d.Id())
	}

	d = importData(t, "o/p/c1/extra", "org_id", "project_id", "identifier")
	if _, err := MultiLevelResourceImporter.StateContext(ctx, d, nil); err == nil {
		t.Error("a malformed ID must be an error")
	}

	// Setting an attribute the resource does not declare is reported, not ignored.
	d = importData(t, "o/c1", "identifier")
	if _, err := OrgResourceImporter.StateContext(ctx, d, nil); err == nil || !strings.Contains(err.Error(), "org_id") {
		t.Errorf("missing attribute must be reported: %v", err)
	}

	if spec, ok := ImportSpecOf(MultiLevelResourceImporter); !ok || spec.OptionalPrefix != 2 {
		t.Errorf("ImportSpecOf = %+v, %v", spec, ok)
	}
	if _, ok := ImportSpecOf(MultiLevelTemplateImporter); ok {
		t.Error("the custom template importer has no spec")
	}
}

func TestSplitImportIDAndPassthrough(t *testing.T) {
	if parts, err := SplitImportID("a/b/c", 1, 3); err != nil || len(parts) != 3 {
		t.Errorf("SplitImportID = %v, %v", parts, err)
	}
	for _, id := range []string{"", "/", "a//b", "a/b/c/d"} {
		if _, err := SplitImportID(id, 1, 3); err == nil {
			t.Errorf("SplitImportID(%q) must fail", id)
		}
	}
	d := importData(t, "acct-thing")
	if _, err := PassthroughImporter().StateContext(context.Background(), d, nil); err != nil || d.Id() != "acct-thing" {
		t.Errorf("passthrough: %v %q", err, d.Id())
	}
	d = importData(t, "  ")
	if _, err := PassthroughImporter().StateContext(context.Background(), d, nil); err == nil {
		t.Error("passthrough must reject an empty ID")
	}
}

func TestMultiLevelTemplateImporterRejectsMalformed(t *testing.T) {
	for _, id := range []string{"", "a//b", "o/p/t/versions", "a/b/c/d/e/f"} {
		d := importData(t, id, "org_id", "project_id", "identifier", "version")
		if _, err := MultiLevelTemplateImporter.StateContext(context.Background(), d, nil); err == nil {
			t.Errorf("template import %q must fail", id)
		}
	}
	d := importData(t, "o/p/t/versions/v1", "org_id", "project_id", "identifier", "version")
	if _, err := MultiLevelTemplateImporter.StateContext(context.Background(), d, nil); err != nil || d.Id() != "t" || d.Get("version") != "v1" {
		t.Errorf("template import: %v id=%q version=%v", err, d.Id(), d.Get("version"))
	}
}
