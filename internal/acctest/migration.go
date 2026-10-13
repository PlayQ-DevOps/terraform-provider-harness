package acctest

import (
	"fmt"
	"testing"

	"github.com/harness/terraform-provider-harness/internal/fakeharness"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// Migration describes a resource that moved from SDK v2 to the framework, for RunMigration.
type Migration struct {
	ResourceType string
	// ResourceName is the HCL label; default "test".
	ResourceName string
	// Kind is the fake store kind of the object the resource manages.
	Kind string
	// Scope is where the object lives.
	Scope ScopeMode
	// Config renders the resource block, plus any parents it needs, for p. Leave Optional
	// attributes out so the SDK v2 zero values are exercised.
	Config func(p LifecycleParams) string
	// V0State lists attributes the SDK v2 implementation is expected to write, as flatmap
	// keys ("description": "", "tags.#": "0"): the golden v0 shape the upgrader handles.
	V0State map[string]string
	// NullAfterUpgrade lists flatmap keys that must be absent from the state once the
	// framework implementation has upgraded it.
	NullAfterUpgrade []string
	// ImportIgnore lists attributes ImportStateVerify ignores.
	ImportIgnore []string
	// Fake customises the fake before the run.
	Fake func(f *fakeharness.Server)
}

// RunMigration proves the framework implementation of a resource takes over state written
// by its SDK v2 predecessor without a diff:
//
//  1. apply the configuration with the legacy SDK v2 provider and check the v0 state shape;
//  2. plan the same configuration with the muxed provider: the state is upgraded and the
//     plan must be empty;
//  3. apply with the muxed provider (a no-op) and check the upgraded state;
//  4. import through the framework implementation with ImportStateVerify.
//
// It needs the Terraform CLI and skips under -short.
func RunMigration(t *testing.T, m Migration) {
	t.Helper()
	if testing.Short() {
		t.Skip("migration tests drive the Terraform CLI; run without -short (make test)")
	}
	lc := Lifecycle{ResourceType: m.ResourceType, ResourceName: m.ResourceName, Kind: m.Kind, Scope: m.Scope, Config: m.Config, ImportIgnore: m.ImportIgnore}.withDefaults()
	p := LifecycleParams{Identifier: "mig_resource", Name: "Migrated resource", OrgID: "mig_org", ProjectID: "mig_project"}
	f := fakeharness.New(t)
	if m.Fake != nil {
		m.Fake(f)
	}
	v0 := []resource.TestCheckFunc{lc.inStore(f, p)}
	for k, v := range m.V0State {
		v0 = append(v0, resource.TestCheckResourceAttr(lc.addr(), k, v))
	}
	upgraded := []resource.TestCheckFunc{lc.inStore(f, p)}
	for _, k := range m.NullAfterUpgrade {
		upgraded = append(upgraded, attributeAbsent(lc.addr(), k))
	}
	resource.UnitTest(t, resource.TestCase{
		CheckDestroy: CheckDestroyed(f, lc.Kind),
		Steps: []resource.TestStep{
			{
				ProtoV5ProviderFactories: LegacyFactories(),
				Config:                   lc.config(f, p),
				Check:                    resource.ComposeAggregateTestCheckFunc(v0...),
			},
			{
				ProtoV5ProviderFactories: OfflineFactories(),
				Config:                   lc.config(f, p),
				PlanOnly:                 true,
			},
			{
				ProtoV5ProviderFactories: OfflineFactories(),
				Config:                   lc.config(f, p),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
				Check: resource.ComposeAggregateTestCheckFunc(upgraded...),
			},
			{
				ProtoV5ProviderFactories: OfflineFactories(),
				ResourceName:             lc.addr(),
				Config:                   lc.config(f, p),
				ImportState:              true,
				ImportStateId:            lc.ImportID(p),
				ImportStateVerify:        true,
				ImportStateVerifyIgnore:  lc.ImportIgnore,
			},
			// A step that brings its own providers first writes a stub configuration (just the
			// required_providers block) into the working directory; an import step never
			// replaces it with a real one, so a final no-op apply puts the configuration back
			// for the post-test destroy.
			{
				ProtoV5ProviderFactories: OfflineFactories(),
				Config:                   lc.config(f, p),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}

// attributeAbsent fails when key is in the flatmap state at all (a null attribute has no
// key; TestCheckNoResourceAttr would also accept an empty collection).
func attributeAbsent(addr, key string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[addr]
		if !ok {
			return fmt.Errorf("%s is not in the state", addr)
		}
		if v, ok := rs.Primary.Attributes[key]; ok {
			return fmt.Errorf("%s: %s is still in the state (%q); the state upgrade must turn it null", addr, key, v)
		}
		return nil
	}
}
