package provider_test

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"

	fwdatasource "github.com/hashicorp/terraform-plugin-framework/datasource"
	fwresource "github.com/hashicorp/terraform-plugin-framework/resource"

	"github.com/harness/terraform-provider-harness/helpers"
	"github.com/harness/terraform-provider-harness/internal"
	"github.com/harness/terraform-provider-harness/internal/provider"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// The provider contract: properties every resource and data source must satisfy for this
// provider to behave like a Terraform provider. Each check yields a set of violating keys
// (a resource type name, or "type.attribute.path"). A violation is tolerated only if it is
// listed in contractAllowlist (a deliberate exception with a reason; see
// contract_allowlist_test.go) or in the generated contractBaseline (pre-existing debt; see
// contract_baseline_test.go). Both lists ratchet: a new violation fails the build, and an
// entry that no longer violates also fails until it is removed, so the lists only shrink.
//
// After fixing violations, regenerate the baseline and the golden name lists with:
//
//	go test ./internal/provider -run TestContract -args -update
var updateContract = flag.Bool("update", false, "regenerate contract_baseline_test.go and testdata golden name lists from the current provider")

// Check names. They are the keys of contractAllowlist and contractBaseline.
const (
	// C1: every managed resource is importable unless it is a one-shot action resource.
	checkImporterMissing = "C1_importer_missing"
	// C2: importers use StateContext; the deprecated State field is not used.
	checkImporterDeprecated = "C2_importer_deprecated_state"
	// C3: identity and scope attributes (identifier, org_id, project_id, and every part of the
	// import ID) are ForceNew when
	// settable, because Harness cannot move or rename an object; a change must replace it.
	checkMutableIdentity = "C3_identity_not_forcenew"
	// C4: client-side intent flags (force_delete, upsert, ...) are plain Optional booleans:
	// never Computed (the API does not return them) and never diff-suppressed.
	checkIntentFlag = "C4_intent_flag_shape"
	// C5: a DiffSuppressFunc must be a pure normalisation of old/new; one that reports
	// "__old__" and "__new__" as equal (or panics on an empty ResourceData) hides every change.
	checkDiffSuppressAlways = "C5_diff_suppress_always_true"
	// C6: attributes whose name says they hold a secret value are marked Sensitive.
	// Secret references (..._ref, ..._id) are excluded by name.
	checkSensitive = "C6_secret_not_sensitive"
	// C7: a resource whose description says it is deprecated carries a DeprecationMessage,
	// and a deprecated attribute is never Required.
	checkDeprecation = "C7_deprecation_inconsistent"
	// C8: data sources have no Importer, no CustomizeDiff and no ForceNew attributes.
	checkDataSourceShape = "C8_data_source_shape"
	// C9 (count ratchet): Optional+Computed attributes per resource may only decrease.
	checkOptionalComputed = "C9_optional_computed_count"
	// C12 (count ratchet): attributes without a Description per type may only decrease.
	checkMissingDescription = "C12_missing_description_count"
	// C10: an importer given a malformed ID returns an error; it never panics (index out of
	// range on a short ID crashes the plugin during terraform import).
	checkImporterPanics = "C10_importer_panics_on_malformed_id"
)

var (
	identityAttrs      = []string{"identifier", "org_id", "project_id"}
	malformedImportIDs = []string{"", "/", "a//b", "a/b/c/d/e/f/g/h"}
	intentFlags        = map[string]bool{
		"force_delete": true, "force_update": true, "upsert": true, "validate": true,
		"template_applied": true, "import_from_git": true, "is_force_import": true,
	}
	secretNameRe = regexp.MustCompile(`(?i)(^|_)(password|passphrase|secret|token|api_key|private_key|client_secret|secret_key|access_key|bearer)(_wo)?$`)
	secretRefRe  = regexp.MustCompile(`(?i)_(ref|refs|id|ids|name|names|type|types|status|version|identifier|identifiers|scope|path|enabled|ttl|url|file|length)(_wo)?$`)
)

// contractReport collects violations per check.
type contractReport struct {
	sets   map[string]map[string]bool
	counts map[string]map[string]int
}

func newContractReport() *contractReport {
	return &contractReport{sets: map[string]map[string]bool{}, counts: map[string]map[string]int{}}
}

func (r *contractReport) add(check, key string) {
	if r.sets[check] == nil {
		r.sets[check] = map[string]bool{}
	}
	r.sets[check][key] = true
}

func (r *contractReport) count(check, key string, n int) {
	if n == 0 {
		return
	}
	if r.counts[check] == nil {
		r.counts[check] = map[string]int{}
	}
	r.counts[check][key] = n
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// walkSchema visits every attribute in s in a stable order, descending into nested
// resources. Paths use "." separators, e.g. "git_details.branch_name".
func walkSchema(prefix string, s map[string]*schema.Schema, fn func(path string, s *schema.Schema)) {
	for _, k := range sortedKeys(s) {
		path := k
		if prefix != "" {
			path = prefix + "." + k
		}
		fn(path, s[k])
		if nested, ok := s[k].Elem.(*schema.Resource); ok && nested != nil {
			walkSchema(path, nested.Schema, fn)
		}
	}
}

func leafName(path string) string {
	return path[strings.LastIndex(path, ".")+1:]
}

// diffSuppressProbe calls f with values that are obviously different, with and without a
// resource ID, on an otherwise empty ResourceData. It reports whether f claimed equality
// for an existing resource (so it hides every update) or panicked. A pure normaliser never
// considers "__old__" and "__new__" equal.
func diffSuppressProbe(t *testing.T, r *schema.Resource, path string, f schema.SchemaDiffSuppressFunc) (hidesUpdates, panicked bool) {
	t.Helper()
	probe := func(id string) (res bool, p bool) {
		defer func() {
			if rec := recover(); rec != nil {
				p = true
			}
		}()
		d := schema.TestResourceDataRaw(t, r.Schema, map[string]interface{}{})
		if id != "" {
			d.SetId(id)
		}
		return f(path, "__old__", "__new__", d), false
	}
	_, p1 := probe("")
	r2, p2 := probe("probe")
	return r2, p1 || p2
}

func checkResource(t *testing.T, rep *contractReport, name string, r *schema.Resource) {
	t.Helper()
	if r.Importer == nil {
		rep.add(checkImporterMissing, name)
	} else if r.Importer.StateContext == nil && r.Importer.State != nil {
		rep.add(checkImporterDeprecated, name)
	}
	if r.Importer != nil {
		for _, id := range malformedImportIDs {
			if importerPanics(t, r, id) {
				rep.add(checkImporterPanics, name+":"+strconv.Quote(id))
			}
		}
	}
	// Identity: the attributes every resource addresses an object with, plus every part of
	// its import ID (an importer built from a helpers.ImportSpec).
	identity := append([]string{}, identityAttrs...)
	if r.Importer != nil {
		if spec, ok := helpers.ImportSpecOf(r.Importer); ok {
			for _, part := range spec.Parts {
				if part != "" && !slices.Contains(identity, part) {
					identity = append(identity, part)
				}
			}
		}
	}
	for _, attr := range identity {
		if s, ok := r.Schema[attr]; ok && (s.Required || s.Optional) && !s.ForceNew {
			rep.add(checkMutableIdentity, name+"."+attr)
		}
	}
	if strings.Contains(strings.ToLower(r.Description), "deprecated") && r.DeprecationMessage == "" {
		rep.add(checkDeprecation, name)
	}

	optionalComputed, missingDescription := 0, 0
	if r.Description == "" {
		missingDescription++
	}
	walkSchema("", r.Schema, func(path string, s *schema.Schema) {
		leaf := leafName(path)
		if intentFlags[leaf] && (s.Type != schema.TypeBool || s.Computed || s.Required || s.DiffSuppressFunc != nil) {
			rep.add(checkIntentFlag, name+"."+path)
		}
		if s.DiffSuppressFunc != nil {
			if hidesUpdates, panicked := diffSuppressProbe(t, r, path, s.DiffSuppressFunc); hidesUpdates || panicked {
				rep.add(checkDiffSuppressAlways, name+"."+path)
			}
		}
		if s.Type == schema.TypeString && !s.Sensitive && secretNameRe.MatchString(leaf) && !secretRefRe.MatchString(leaf) {
			rep.add(checkSensitive, name+"."+path)
		}
		if s.Deprecated != "" && s.Required {
			rep.add(checkDeprecation, name+"."+path)
		}
		if s.Optional && s.Computed {
			optionalComputed++
		}
		if s.Description == "" {
			missingDescription++
		}
	})
	rep.count(checkOptionalComputed, name, optionalComputed)
	rep.count(checkMissingDescription, "resource:"+name, missingDescription)
}

func checkDataSource(rep *contractReport, name string, ds *schema.Resource) {
	if ds.Importer != nil {
		rep.add(checkDataSourceShape, name+":importer")
	}
	if ds.CustomizeDiff != nil {
		rep.add(checkDataSourceShape, name+":customizediff")
	}
	missingDescription := 0
	if ds.Description == "" {
		missingDescription++
	}
	walkSchema("", ds.Schema, func(path string, s *schema.Schema) {
		if s.ForceNew {
			rep.add(checkDataSourceShape, name+"."+path+":forcenew")
		}
		if s.Description == "" {
			missingDescription++
		}
	})
	rep.count(checkMissingDescription, "data:"+name, missingDescription)
}

func checkProviderSchema(rep *contractReport, p *schema.Provider) {
	for _, attr := range []string{"api_key", "platform_api_key"} {
		if s, ok := p.Schema[attr]; ok && !s.Sensitive {
			rep.add(checkSensitive, "provider."+attr)
		}
	}
}

func TestContract(t *testing.T) {
	p := provider.Provider("test")()
	if err := p.InternalValidate(); err != nil {
		t.Fatalf("provider schema fails InternalValidate: %v", err)
	}

	rep := newContractReport()
	for _, name := range sortedKeys(p.ResourcesMap) {
		checkResource(t, rep, name, p.ResourcesMap[name])
	}
	for _, name := range sortedKeys(p.DataSourcesMap) {
		checkDataSource(rep, name, p.DataSourcesMap[name])
	}
	checkProviderSchema(rep, p)
	checkFrameworkTypes(t, rep)

	fwResources, fwDataSources := frameworkTypeNames(t)
	resourceNames := union(sortedKeys(p.ResourcesMap), fwResources)
	dataSourceNames := union(sortedKeys(p.DataSourcesMap), fwDataSources)

	if *updateContract {
		writeContractBaseline(t, rep)
		writeGolden(t, "resource_names.txt", resourceNames)
		writeGolden(t, "data_source_names.txt", dataSourceNames)
		compareFrozen(t, "sdk_v2_types.txt", append(sortedKeys(p.ResourcesMap), sortedKeys(p.DataSourcesMap)...), true)
		t.Log("regenerated contract_baseline_test.go and testdata golden files; re-run without -update")
		return
	}

	// C11: the set of registered types is frozen. Adding an SDK v2 resource is a deliberate
	// act (new resources are written with terraform-plugin-framework once Phase 4 lands);
	// removing one is a breaking change that needs a changelog entry.
	compareGolden(t, "resource_names.txt", resourceNames)
	compareGolden(t, "data_source_names.txt", dataSourceNames)
	// The SDK v2 maps are frozen: a type may leave them for the framework, but new types are
	// written with terraform-plugin-framework (internal/framework), never added here.
	compareFrozen(t, "sdk_v2_types.txt", append(sortedKeys(p.ResourcesMap), sortedKeys(p.DataSourcesMap)...), *updateContract)

	for _, check := range sortedKeys(rep.sets) {
		for _, key := range sortedKeys(rep.sets[check]) {
			if contractAllowlist[check][key] != "" || contractBaseline[check][key] {
				continue
			}
			t.Errorf("%s: %s violates the provider contract; fix it or add it to contractAllowlist with a reason", check, key)
		}
	}
	for _, check := range sortedKeys(rep.counts) {
		for _, key := range sortedKeys(rep.counts[check]) {
			if n, base := rep.counts[check][key], contractBaselineCounts[check][key]; n > base {
				t.Errorf("%s: %s has %d (baseline %d); this ratchet may only go down", check, key, n, base)
			}
		}
	}

	// Minimality: stale entries must be removed so the lists stay honest.
	for _, check := range sortedKeys(contractBaseline) {
		for _, key := range sortedKeys(contractBaseline[check]) {
			if !rep.sets[check][key] {
				t.Errorf("%s: baseline entry %q no longer violates; regenerate with -update", check, key)
			}
		}
	}
	for _, check := range sortedKeys(contractAllowlist) {
		for _, key := range sortedKeys(contractAllowlist[check]) {
			if !rep.sets[check][key] {
				t.Errorf("%s: allowlist entry %q no longer violates; remove it from contract_allowlist_test.go", check, key)
			}
		}
	}
	for _, check := range sortedKeys(contractBaselineCounts) {
		for _, key := range sortedKeys(contractBaselineCounts[check]) {
			if n, base := rep.counts[check][key], contractBaselineCounts[check][key]; n < base {
				t.Errorf("%s: %s improved to %d (baseline %d); regenerate with -update so the ratchet keeps the gain", check, key, n, base)
			}
		}
	}
}

func goldenPath(name string) string { return filepath.Join("testdata", name) }

func writeGolden(t *testing.T, name string, lines []string) {
	t.Helper()
	if err := os.WriteFile(goldenPath(name), []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

func compareGolden(t *testing.T, name string, got []string) {
	t.Helper()
	raw, err := os.ReadFile(goldenPath(name))
	if err != nil {
		t.Fatalf("read %s (regenerate with -update): %v", name, err)
	}
	want := strings.Fields(string(raw))
	wantSet, gotSet := map[string]bool{}, map[string]bool{}
	for _, w := range want {
		wantSet[w] = true
	}
	for _, g := range got {
		gotSet[g] = true
	}
	for _, w := range want {
		if !gotSet[w] {
			t.Errorf("%s: %s is no longer registered; removing a type is a breaking change (update the golden file with -update and add a changelog entry)", name, w)
		}
	}
	for _, g := range got {
		if !wantSet[g] {
			t.Errorf("%s: %s is newly registered; new types must be deliberate (update the golden file with -update)", name, g)
		}
	}
}

func writeContractBaseline(t *testing.T, rep *contractReport) {
	t.Helper()
	var b bytes.Buffer
	b.WriteString("// Code generated by `go test ./internal/provider -run TestContract -args -update`; DO NOT EDIT.\n")
	b.WriteString("//\n// It records pre-existing violations of the provider contract (see contract_test.go).\n")
	b.WriteString("// Entries may only disappear, by fixing the violation and regenerating. New violations\n")
	b.WriteString("// must be fixed, or deliberately allowlisted with a reason in contract_allowlist_test.go.\n\n")
	b.WriteString("package provider_test\n\n")
	b.WriteString("var contractBaseline = map[string]map[string]bool{\n")
	for _, check := range sortedKeys(rep.sets) {
		var keys []string
		for _, key := range sortedKeys(rep.sets[check]) {
			if contractAllowlist[check][key] == "" {
				keys = append(keys, key)
			}
		}
		if len(keys) == 0 {
			continue
		}
		fmt.Fprintf(&b, "\t%q: {\n", check)
		for _, key := range keys {
			fmt.Fprintf(&b, "\t\t%q: true,\n", key)
		}
		b.WriteString("\t},\n")
	}
	b.WriteString("}\n\n")
	b.WriteString("var contractBaselineCounts = map[string]map[string]int{\n")
	for _, check := range sortedKeys(rep.counts) {
		fmt.Fprintf(&b, "\t%q: {\n", check)
		for _, key := range sortedKeys(rep.counts[check]) {
			fmt.Fprintf(&b, "\t\t%q: %d,\n", key, rep.counts[check][key])
		}
		b.WriteString("\t},\n")
	}
	b.WriteString("}\n")
	src, err := format.Source(b.Bytes())
	if err != nil {
		t.Fatalf("format generated baseline: %v", err)
	}
	if err := os.WriteFile("contract_baseline_test.go", src, 0o644); err != nil {
		t.Fatalf("write contract_baseline_test.go: %v", err)
	}
}

// importerPanics runs the importer on id with an empty session and reports a panic. Importers
// that need the API with a real session are expected to fail cleanly with the empty one; a
// panic here is a panic during `terraform import` for users.
func importerPanics(t *testing.T, r *schema.Resource, id string) (panicked bool) {
	t.Helper()
	defer func() {
		if rec := recover(); rec != nil {
			panicked = true
		}
	}()
	d := schema.TestResourceDataRaw(t, r.Schema, map[string]interface{}{})
	d.SetId(id)
	switch {
	case r.Importer.StateContext != nil:
		_, _ = r.Importer.StateContext(context.Background(), d, &internal.Session{})
	case r.Importer.State != nil:
		_, _ = r.Importer.State(d, &internal.Session{})
	}
	return false
}

// frameworkTypeNames lists the resource and data source types the plugin-framework half of
// the provider serves.
func frameworkTypeNames(t *testing.T) (resources, dataSources []string) {
	t.Helper()
	ctx := context.Background()
	fw := provider.NewFrameworkProvider("test")()
	for _, newResource := range fw.Resources(ctx) {
		mr := &fwresource.MetadataResponse{}
		newResource().Metadata(ctx, fwresource.MetadataRequest{ProviderTypeName: "harness"}, mr)
		resources = append(resources, mr.TypeName)
	}
	for _, newDataSource := range fw.DataSources(ctx) {
		mr := &fwdatasource.MetadataResponse{}
		newDataSource().Metadata(ctx, fwdatasource.MetadataRequest{ProviderTypeName: "harness"}, mr)
		dataSources = append(dataSources, mr.TypeName)
	}
	sort.Strings(resources)
	sort.Strings(dataSources)
	return resources, dataSources
}

func union(a, b []string) []string {
	set := map[string]bool{}
	for _, s := range a {
		set[s] = true
	}
	for _, s := range b {
		set[s] = true
	}
	return sortedKeys(set)
}

// compareFrozen checks that every name is in the frozen list. The list is written once
// (with -update, when it does not exist yet) and then only edited by hand.
func compareFrozen(t *testing.T, name string, got []string, update bool) {
	t.Helper()
	if _, err := os.Stat(goldenPath(name)); os.IsNotExist(err) && update {
		writeGolden(t, name, got)
		return
	}
	raw, err := os.ReadFile(goldenPath(name))
	if err != nil {
		t.Fatalf("read %s (create it once with -update): %v", name, err)
	}
	frozen := map[string]bool{}
	for _, w := range strings.Fields(string(raw)) {
		frozen[w] = true
	}
	for _, g := range got {
		if !frozen[g] {
			t.Errorf("%s is an SDK v2 type registered after the framework freeze; write new types with terraform-plugin-framework (internal/framework) and register them in the framework provider", g)
		}
	}
}
