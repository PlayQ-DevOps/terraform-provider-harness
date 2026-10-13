package acctest

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/harness/harness-go-sdk/harness/nextgen"
	"github.com/harness/terraform-provider-harness/internal"
	"github.com/harness/terraform-provider-harness/internal/fakeharness"
	"github.com/harness/terraform-provider-harness/internal/provider"
	"github.com/hashicorp/terraform-plugin-go/tfprotov5"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// OfflineFactories serves the provider in-process, so a resource.UnitTest drives it with real
// Terraform core. The provider is pointed at a fake through the provider block that
// fakeharness.Server.ProviderHCL renders.
func OfflineFactories() map[string]func() (tfprotov5.ProviderServer, error) {
	return map[string]func() (tfprotov5.ProviderServer, error){
		"harness": func() (tfprotov5.ProviderServer, error) {
			server, err := provider.NewMuxServer(context.Background(), "test")
			if err != nil {
				return nil, err
			}
			return server(), nil
		},
	}
}

// NewSession returns a provider session whose platform client talks to the fake without
// retries, for unit tests that call a resource's CRUD functions directly.
func NewSession(t testing.TB, f *fakeharness.Server) *internal.Session {
	t.Helper()
	cfg := nextgen.NewConfiguration()
	cfg.BasePath = f.Endpoint()
	cfg.AccountId = f.Account()
	cfg.ApiKey = f.APIKey()
	cfg.HTTPClient.RetryMax = 0
	return &internal.Session{AccountId: f.Account(), Endpoint: f.Endpoint(), PLClient: nextgen.NewAPIClient(cfg)}
}

// CheckDestroyed is a CheckDestroy that fails if the fake still holds objects of kind.
func CheckDestroyed(f *fakeharness.Server, kind string) resource.TestCheckFunc {
	return func(*terraform.State) error {
		if keys := f.Store().Keys(kind); len(keys) != 0 {
			return fmt.Errorf("%d %s object(s) still exist in the fake after destroy: %v", len(keys), kind, keys)
		}
		return nil
	}
}

// LifecycleParams are the identity values a lifecycle configuration is rendered with.
type LifecycleParams struct {
	Identifier string
	Name       string
	OrgID      string
	ProjectID  string
}

// ScopeMode says where the resource under test lives, which determines its fake store key
// and default import ID.
type ScopeMode int

const (
	// ScopeAccount: account level (organizations, account-level connectors).
	ScopeAccount ScopeMode = iota
	// ScopeOrg: inside an organization (projects, org-level connectors).
	ScopeOrg
	// ScopeProject: inside a project.
	ScopeProject
)

// Lifecycle describes a resource for RunLifecycle.
type Lifecycle struct {
	// ResourceType is the Terraform type, e.g. "harness_platform_project".
	ResourceType string
	// ResourceName is the HCL label; default "test".
	ResourceName string
	// Kind is the fake store kind of the object the resource manages.
	Kind string
	// Scope is where the object lives.
	Scope ScopeMode
	// Config renders the resource block, plus any parent resources it needs, for p.
	Config func(p LifecycleParams) string
	// Update returns the params for the in-place update step; default changes Name.
	Update func(p LifecycleParams) LifecycleParams
	// ImportID renders the import ID for p; default "[org/][project/]identifier" by Scope.
	ImportID func(p LifecycleParams) string
	// ImportIgnore lists attributes ImportStateVerify ignores (write-only inputs).
	ImportIgnore []string
	// Key returns the fake store key for p; default by Scope.
	Key func(p LifecycleParams) fakeharness.Key
	// Checks returns extra state checks after create and after update.
	Checks func(p LifecycleParams) []resource.TestCheckFunc
	// Fake customises the fake before each cell (extra mounts, seeded objects).
	Fake func(f *fakeharness.Server)
	// EventuallyConsistentReads, when > 0, adds the eventually_consistent cell: the fake
	// answers that many not-found reads after create, and the resource must still converge
	// (which needs a read-after-write window on the resource).
	EventuallyConsistentReads int
	// Drift, when set, adds the drift_detected cell: after create it changes the stored
	// object out of band; the refresh must then show a non-empty plan (the Read refreshes
	// that attribute from the API rather than echoing the configuration) and the re-apply
	// must plan an in-place update that converges.
	Drift func(obj map[string]any)
	// KnownFailing maps a cell name to the reason (an issue or plan step) it still fails.
	// Those cells are skipped unless HARNESS_RUN_KNOWN_FAILING is set, so CI stays green
	// while the fix is tracked. They must be removed when the fix lands: a passing cell that
	// is still listed fails the test.
	KnownFailing map[string]string
}

// Cell names, for KnownFailing.
const (
	CellCreateUpdateImport = "create_update_import"
	CellDisappears         = "disappears"
	CellNotFoundStyles     = "not_found_styles" // sub-cells: not_found_styles/<style>
	CellScopeChange        = "scope_change_replaces"
	CellIdentifierChange   = "identifier_change_replaces"
	CellTransient503       = "transient_503_retried"
	CellRetryExhausted     = "retry_exhausted_is_error_not_crash"
	CellEventualConsistent = "eventually_consistent"
	CellDrift              = "drift_detected"
)

func (lc Lifecycle) withDefaults() Lifecycle {
	if lc.ResourceName == "" {
		lc.ResourceName = "test"
	}
	if lc.Update == nil {
		lc.Update = func(p LifecycleParams) LifecycleParams {
			p.Name = p.Name + " updated"
			return p
		}
	}
	if lc.ImportID == nil {
		scope := lc.Scope
		lc.ImportID = func(p LifecycleParams) string {
			switch scope {
			case ScopeOrg:
				return p.OrgID + "/" + p.Identifier
			case ScopeProject:
				return p.OrgID + "/" + p.ProjectID + "/" + p.Identifier
			default:
				return p.Identifier
			}
		}
	}
	if lc.Key == nil {
		scope := lc.Scope
		lc.Key = func(p LifecycleParams) fakeharness.Key {
			switch scope {
			case ScopeOrg:
				return fakeharness.Key{Org: p.OrgID, ID: p.Identifier}
			case ScopeProject:
				return fakeharness.Key{Org: p.OrgID, Project: p.ProjectID, ID: p.Identifier}
			default:
				return fakeharness.Key{ID: p.Identifier}
			}
		}
	}
	if lc.Checks == nil {
		lc.Checks = func(LifecycleParams) []resource.TestCheckFunc { return nil }
	}
	return lc
}

func (lc Lifecycle) addr() string { return lc.ResourceType + "." + lc.ResourceName }

func (lc Lifecycle) config(f *fakeharness.Server, p LifecycleParams) string {
	return f.ProviderHCL() + "\n" + lc.Config(p)
}

func (lc Lifecycle) inStore(f *fakeharness.Server, p LifecycleParams) resource.TestCheckFunc {
	return func(*terraform.State) error {
		if !f.Store().Has(lc.Kind, lc.Key(p)) {
			return fmt.Errorf("%s %+v is not in the fake store; objects present: %v", lc.Kind, lc.Key(p), f.Store().Keys(lc.Kind))
		}
		return nil
	}
}

func (lc Lifecycle) checks(f *fakeharness.Server, p LifecycleParams, extra ...resource.TestCheckFunc) resource.TestCheckFunc {
	all := []resource.TestCheckFunc{resource.TestCheckResourceAttr(lc.addr(), "identifier", p.Identifier), lc.inStore(f, p)}
	all = append(all, lc.Checks(p)...)
	all = append(all, extra...)
	return resource.ComposeAggregateTestCheckFunc(all...)
}

func (lc Lifecycle) createStep(f *fakeharness.Server, p LifecycleParams) resource.TestStep {
	return resource.TestStep{
		Config: lc.config(f, p),
		ConfigPlanChecks: resource.ConfigPlanChecks{
			PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction(lc.addr(), plancheck.ResourceActionCreate)},
		},
		Check: lc.checks(f, p),
	}
}

func (lc Lifecycle) testCase(f *fakeharness.Server, steps ...resource.TestStep) resource.TestCase {
	return resource.TestCase{
		ProtoV5ProviderFactories: OfflineFactories(),
		CheckDestroy:             CheckDestroyed(f, lc.Kind),
		Steps:                    steps,
	}
}

type cell struct {
	name string
	run  func(t *testing.T, f *fakeharness.Server, p LifecycleParams)
}

// RunLifecycle runs the standard lifecycle matrix for lc against a fresh fake per cell:
//
//   - create_update_import: create (plan shows Create), update in place (plan shows Update,
//     exactly one PUT on the wire), import with ImportStateVerify;
//   - disappears: delete the object out of band, refresh shows a non-empty plan, re-apply
//     creates it again;
//   - not_found_styles/<style>: disappears, for every not-found body shape the API produces;
//   - scope_change_replaces / identifier_change_replaces: changing scope or identifier plans
//     a replacement and is a DELETE then a POST on the wire, never a PUT;
//   - transient_503_retried: a single 503 on read is retried, the plan stays empty;
//   - retry_exhausted_is_error_not_crash: a persistent 503 is a clean error, not a crash;
//   - eventually_consistent (opt-in): the object is not readable right after create;
//   - drift_detected (opt-in): an out-of-band change to the object shows up in the plan and
//     the re-apply converges.
//
// It needs the Terraform CLI and skips under -short.
func RunLifecycle(t *testing.T, lc Lifecycle) {
	t.Helper()
	if testing.Short() {
		t.Skip("lifecycle tests drive the Terraform CLI; run without -short (make test)")
	}
	lc = lc.withDefaults()
	base := LifecycleParams{Identifier: "lc_resource", Name: "Lifecycle resource", OrgID: "lc_org", ProjectID: "lc_project"}

	cells := []cell{
		{CellCreateUpdateImport, lc.cellCreateUpdateImport},
		{CellDisappears, lc.cellDisappears(fakeharness.NotFoundEntity)},
	}
	for _, style := range fakeharness.NotFoundStyles {
		cells = append(cells, cell{CellNotFoundStyles + "/" + style.String(), lc.cellDisappears(style)})
	}
	if lc.Scope != ScopeAccount {
		cells = append(cells, cell{CellScopeChange, lc.cellScopeChange})
	}
	cells = append(cells,
		cell{CellIdentifierChange, lc.cellIdentifierChange},
		cell{CellTransient503, lc.cellTransient503},
		cell{CellRetryExhausted, lc.cellRetryExhausted},
	)
	if lc.EventuallyConsistentReads > 0 {
		cells = append(cells, cell{CellEventualConsistent, lc.cellEventuallyConsistent})
	}
	if lc.Drift != nil {
		cells = append(cells, cell{CellDrift, lc.cellDrift})
	}

	runCells(t, cells, lc.Fake, lc.KnownFailing, base)
}

func (lc Lifecycle) cellCreateUpdateImport(t *testing.T, f *fakeharness.Server, p LifecycleParams) {
	up := lc.Update(p)
	var mark int
	resource.UnitTest(t, lc.testCase(f,
		lc.createStep(f, p),
		resource.TestStep{
			PreConfig: func() { mark = f.Mark() },
			Config:    lc.config(f, up),
			ConfigPlanChecks: resource.ConfigPlanChecks{
				PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction(lc.addr(), plancheck.ResourceActionUpdate)},
			},
			Check: lc.checks(f, up, func(*terraform.State) error {
				if got := f.MutatingMethods(mark, lc.Kind); strings.Join(got, ",") != "PUT" {
					return fmt.Errorf("an in-place update must be exactly one PUT on the wire, got %v", got)
				}
				return nil
			}),
		},
		resource.TestStep{
			ResourceName:            lc.addr(),
			ImportState:             true,
			ImportStateId:           lc.ImportID(up),
			ImportStateVerify:       true,
			ImportStateVerifyIgnore: lc.ImportIgnore,
		},
	))
}

func (lc Lifecycle) cellDisappears(style fakeharness.NotFoundStyle) func(*testing.T, *fakeharness.Server, LifecycleParams) {
	return func(t *testing.T, f *fakeharness.Server, p LifecycleParams) {
		f.SetNotFoundStyle(style)
		resource.UnitTest(t, lc.testCase(f,
			lc.createStep(f, p),
			resource.TestStep{
				PreConfig: func() {
					if !f.Vanish(lc.Kind, lc.Key(p)) {
						t.Fatalf("object %+v was not in the fake store before the out-of-band delete", lc.Key(p))
					}
				},
				RefreshState:       true,
				ExpectNonEmptyPlan: true,
			},
			lc.createStep(f, p),
		))
	}
}

func (lc Lifecycle) cellReplace(t *testing.T, f *fakeharness.Server, p, p2 LifecycleParams) {
	var mark int
	resource.UnitTest(t, lc.testCase(f,
		lc.createStep(f, p),
		resource.TestStep{
			PreConfig: func() { mark = f.Mark() },
			Config:    lc.config(f, p2),
			ConfigPlanChecks: resource.ConfigPlanChecks{
				PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction(lc.addr(), plancheck.ResourceActionReplace)},
			},
			Check: lc.checks(f, p2, func(*terraform.State) error {
				got := f.MutatingMethods(mark, lc.Kind)
				if strings.Join(got, ",") != "DELETE,POST" && strings.Join(got, ",") != "POST,DELETE" {
					return fmt.Errorf("a replacement must be a DELETE and a POST on the wire (never a PUT), got %v", got)
				}
				if f.Store().Has(lc.Kind, lc.Key(p)) {
					return fmt.Errorf("the old object %+v still exists after the replacement", lc.Key(p))
				}
				return nil
			}),
		},
	))
}

func (lc Lifecycle) cellScopeChange(t *testing.T, f *fakeharness.Server, p LifecycleParams) {
	p2 := p
	switch lc.Scope {
	case ScopeProject:
		p2.ProjectID = p.ProjectID + "_2"
	case ScopeOrg:
		p2.OrgID = p.OrgID + "_2"
	}
	lc.cellReplace(t, f, p, p2)
}

func (lc Lifecycle) cellIdentifierChange(t *testing.T, f *fakeharness.Server, p LifecycleParams) {
	p2 := p
	p2.Identifier = p.Identifier + "_2"
	lc.cellReplace(t, f, p, p2)
}

func (lc Lifecycle) cellTransient503(t *testing.T, f *fakeharness.Server, p LifecycleParams) {
	var mark int
	resource.UnitTest(t, lc.testCase(f,
		lc.createStep(f, p),
		resource.TestStep{
			PreConfig: func() {
				mark = f.Mark()
				f.FailNext(fakeharness.Match{Method: "GET", Kind: lc.Kind}, 503, `{"status":"ERROR","code":"SERVICE_UNAVAILABLE","message":"try again"}`)
			},
			Config:   lc.config(f, p),
			PlanOnly: true,
		},
	))
	if n := f.Count(mark, "GET", ""); n < 2 {
		t.Errorf("the 503 must be retried: expected at least 2 GETs after the mark, got %d", n)
	}
}

var retryExhaustedError = regexp.MustCompile(`(?i)giving up after|could not reach|retry budget|503|Service Unavailable|try again`)

func (lc Lifecycle) cellRetryExhausted(t *testing.T, f *fakeharness.Server, p LifecycleParams) {
	var cancel func()
	resource.UnitTest(t, lc.testCase(f,
		lc.createStep(f, p),
		resource.TestStep{
			PreConfig: func() {
				cancel = f.FailAlways(fakeharness.Match{Method: "GET", Kind: lc.Kind}, 503, `{"status":"ERROR","code":"SERVICE_UNAVAILABLE","message":"try again"}`)
			},
			Config:      lc.config(f, p),
			PlanOnly:    true,
			ExpectError: retryExhaustedError,
		},
		resource.TestStep{
			PreConfig: func() { cancel() },
			Config:    lc.config(f, p),
			PlanOnly:  true,
		},
	))
}

func (lc Lifecycle) cellEventuallyConsistent(t *testing.T, f *fakeharness.Server, p LifecycleParams) {
	f.EventuallyConsistent(lc.Kind, lc.EventuallyConsistentReads)
	resource.UnitTest(t, lc.testCase(f, lc.createStep(f, p)))
}

func (lc Lifecycle) cellDrift(t *testing.T, f *fakeharness.Server, p LifecycleParams) {
	resource.UnitTest(t, lc.testCase(f,
		lc.createStep(f, p),
		resource.TestStep{
			PreConfig: func() {
				if !f.Store().Has(lc.Kind, lc.Key(p)) {
					t.Fatalf("object %+v was not in the fake store before the out-of-band change", lc.Key(p))
				}
				f.Store().Mutate(lc.Kind, lc.Key(p), lc.Drift)
			},
			RefreshState:       true,
			ExpectNonEmptyPlan: true,
		},
		resource.TestStep{
			Config: lc.config(f, p),
			ConfigPlanChecks: resource.ConfigPlanChecks{
				PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction(lc.addr(), plancheck.ResourceActionUpdate)},
			},
			Check: lc.checks(f, p),
		},
	))
}
