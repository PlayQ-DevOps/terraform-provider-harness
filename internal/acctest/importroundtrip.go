package acctest

import (
	"fmt"
	"os"
	"testing"

	"github.com/harness/terraform-provider-harness/internal/fakeharness"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// ImportRoundTrip describes a resource for RunImportRoundTrip: the import and empty-object
// cells of the lifecycle matrix, for resources whose identity is not "an identifier in a
// scope" (composite keys, list-backed reads, patch-style updates) and that RunLifecycle
// therefore cannot express.
type ImportRoundTrip struct {
	// ResourceType is the Terraform type, e.g. "harness_platform_delegatetoken".
	ResourceType string
	// ResourceName is the HCL label; default "test".
	ResourceName string
	// Kind is the fake store kind of the object and Key its store key.
	Kind string
	Key  fakeharness.Key
	// Config renders the resource block plus any parents (see ScopeParents); the provider
	// block is prepended.
	Config func(f *fakeharness.Server) string
	// ImportID is the import ID of the created object.
	ImportID string
	// ImportIgnore lists the attributes ImportStateVerify ignores.
	ImportIgnore []string
	// Checks are extra state checks after create.
	Checks []resource.TestCheckFunc
	// Fake customises the fake before each cell (extra mounts, seeded objects).
	Fake func(f *fakeharness.Server)
	// NotFoundStyles adds the not_found_styles cells, for resources read with a GET. A
	// list-backed read sees an empty list, so the body shape does not matter to it.
	NotFoundStyles bool
	// KnownFailing maps a cell name to the reason it still fails; see Lifecycle.KnownFailing.
	KnownFailing map[string]string
}

// CellCreateImport is the create-then-import cell of RunImportRoundTrip.
const CellCreateImport = "create_import"

// ScopeParents renders an organization and a project, with the HCL labels "org" and
// "project", for project-level configurations to reference as
// harness_platform_organization.org.id and harness_platform_project.project.id.
func ScopeParents(orgID, projectID string) string {
	return fmt.Sprintf(`
resource "harness_platform_organization" "org" {
  identifier = %[1]q
  name       = %[1]q
}

resource "harness_platform_project" "project" {
  identifier = %[2]q
  name       = %[2]q
  org_id     = harness_platform_organization.org.id
}
`, orgID, projectID)
}

// RunImportRoundTrip runs, against a fresh fake per cell:
//
//   - create_import: create (plan shows Create), then import with ImportStateVerify;
//   - disappears: delete the object out of band, refresh shows a non-empty plan, re-apply
//     creates it again (a list-backed read sees an empty list, a GET sees a 404);
//   - not_found_styles/<style> (opt-in): disappears for every not-found body shape.
//
// It needs the Terraform CLI and skips under -short.
func RunImportRoundTrip(t *testing.T, rt ImportRoundTrip) {
	t.Helper()
	if testing.Short() {
		t.Skip("import round trips drive the Terraform CLI; run without -short (make test)")
	}
	if rt.ResourceName == "" {
		rt.ResourceName = "test"
	}
	cells := []cell{
		{CellCreateImport, rt.cellCreateImport},
		{CellDisappears, rt.cellDisappears(fakeharness.NotFoundEntity)},
	}
	if rt.NotFoundStyles {
		for _, style := range fakeharness.NotFoundStyles {
			cells = append(cells, cell{CellNotFoundStyles + "/" + style.String(), rt.cellDisappears(style)})
		}
	}
	runCells(t, cells, rt.Fake, rt.KnownFailing, LifecycleParams{})
}

// runCells runs each cell against a fresh fake, honouring KnownFailing.
func runCells(t *testing.T, cells []cell, fake func(*fakeharness.Server), knownFailing map[string]string, p LifecycleParams) {
	t.Helper()
	ran := map[string]bool{}
	for _, c := range cells {
		c := c
		ran[c.name] = true
		t.Run(c.name, func(t *testing.T) {
			if reason, known := knownFailing[c.name]; known && os.Getenv("HARNESS_RUN_KNOWN_FAILING") == "" {
				t.Skipf("known failing (set HARNESS_RUN_KNOWN_FAILING=1 to run): %s", reason)
			}
			f := fakeharness.New(t)
			if fake != nil {
				fake(f)
			}
			c.run(t, f, p)
		})
	}
	for name := range knownFailing {
		if !ran[name] {
			t.Errorf("KnownFailing lists %q, which is not a cell of this test", name)
		}
	}
}

func (rt ImportRoundTrip) addr() string { return rt.ResourceType + "." + rt.ResourceName }

func (rt ImportRoundTrip) createStep(f *fakeharness.Server) resource.TestStep {
	checks := []resource.TestCheckFunc{func(*terraform.State) error {
		if !f.Store().Has(rt.Kind, rt.Key) {
			return fmt.Errorf("%s %+v is not in the fake store; objects present: %v", rt.Kind, rt.Key, f.Store().Keys(rt.Kind))
		}
		return nil
	}}
	checks = append(checks, rt.Checks...)
	return resource.TestStep{
		Config: f.ProviderHCL() + "\n" + rt.Config(f),
		ConfigPlanChecks: resource.ConfigPlanChecks{
			PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction(rt.addr(), plancheck.ResourceActionCreate)},
		},
		Check: resource.ComposeAggregateTestCheckFunc(checks...),
	}
}

func (rt ImportRoundTrip) testCase(f *fakeharness.Server, steps ...resource.TestStep) resource.TestCase {
	return resource.TestCase{
		ProtoV5ProviderFactories: OfflineFactories(),
		CheckDestroy:             CheckDestroyed(f, rt.Kind),
		Steps:                    steps,
	}
}

func (rt ImportRoundTrip) cellCreateImport(t *testing.T, f *fakeharness.Server, _ LifecycleParams) {
	resource.UnitTest(t, rt.testCase(f,
		rt.createStep(f),
		resource.TestStep{
			ResourceName:            rt.addr(),
			ImportState:             true,
			ImportStateId:           rt.ImportID,
			ImportStateVerify:       true,
			ImportStateVerifyIgnore: rt.ImportIgnore,
		},
	))
}

func (rt ImportRoundTrip) cellDisappears(style fakeharness.NotFoundStyle) func(*testing.T, *fakeharness.Server, LifecycleParams) {
	return func(t *testing.T, f *fakeharness.Server, _ LifecycleParams) {
		f.SetNotFoundStyle(style)
		resource.UnitTest(t, rt.testCase(f,
			rt.createStep(f),
			resource.TestStep{
				PreConfig: func() {
					if !f.Vanish(rt.Kind, rt.Key) {
						t.Fatalf("object %+v was not in the fake store before the out-of-band delete", rt.Key)
					}
				},
				RefreshState:       true,
				ExpectNonEmptyPlan: true,
			},
			rt.createStep(f),
		))
	}
}
