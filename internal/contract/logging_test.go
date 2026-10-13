package contract

import (
	"fmt"
	"go/ast"
	"os"
	"path/filepath"
	"testing"
)

// Resources log through tflog, so messages carry the provider's structured context and
// honour TF_LOG. The standard library logger bypasses both. What is left are helpers that
// have no context to log with; the per-package baseline may only shrink, and a package not
// listed here must stay free of log.Print*.
//
// Regenerate the baseline with:
//
//	CONTRACT_PRINT_BASELINE=1 go test ./internal/contract -run TestLoggingRatchet
var loggingBaseline = map[string]int{
	"internal/service/cd/application":                     1,
	"internal/service/chaos/chaos_hub":                    1,
	"internal/service/chaos/experiment":                   1,
	"internal/service/chaos/experiment_template":          1,
	"internal/service/chaos/fault_template":               5,
	"internal/service/chaos/infrastructure_v2":            10,
	"internal/service/chaos/security_governance":          4,
	"internal/service/platform/feature_flag":              4,
	"internal/service/platform/feature_flag_target_group": 2,
	"internal/service/platform/provider":                  2,
	"internal/service/split":                              5,
}

func TestLoggingRatchet(t *testing.T) {
	counts := map[string]int{}
	for _, path := range serviceFiles(t) {
		_, f := parse(t, path)
		pkg := filepath.ToSlash(filepath.Dir(rel(path)))
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			if x, ok := sel.X.(*ast.Ident); ok && x.Name == "log" && (sel.Sel.Name == "Printf" || sel.Sel.Name == "Println" || sel.Sel.Name == "Print") {
				counts[pkg]++
			}
			return true
		})
	}
	for pkg, n := range counts {
		if os.Getenv("CONTRACT_PRINT_BASELINE") != "" {
			fmt.Printf("\t%q: %d,\n", pkg, n)
		}
		switch allowed := loggingBaseline[pkg]; {
		case n > allowed:
			t.Errorf("%s: %d log.Print* calls (baseline %d); use tflog with the request context", pkg, n, allowed)
		case n < allowed:
			t.Errorf("%s: %d log.Print* calls (baseline %d); lower the baseline in logging_test.go", pkg, n, allowed)
		}
	}
	for pkg, allowed := range loggingBaseline {
		if counts[pkg] == 0 && allowed > 0 {
			t.Errorf("%s: no log.Print* calls left; remove it from the baseline in logging_test.go", pkg)
		}
	}
}
