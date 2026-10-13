package contract

import (
	"fmt"
	"go/ast"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

// Three shrink-only ratchets over resource code, per package:
//
//   - unchecked d.Set: a `d.Set(k, v)` statement whose error is dropped hides a state
//     write that failed (a value of the wrong shape); use helpers.SetAll or check the error.
//   - scalar assertions on d.Get/d.GetOk: `d.GetOk(k)` cannot tell an explicit false or
//     zero from an omitted attribute, and `v.(bool)` / `v.(int)` on the result crashes on
//     the wrong type; use helpers.GetBool / GetInt / BuildFieldBool / BuildFieldInt32.
//   - panic: a panic in a CRUD function crashes the plugin and Terraform reports an opaque
//     "plugin crashed"; misshapen input is an error for the apply instead.
//
// Baselines live in ratchet_baseline_test.go; regenerate with:
//
//	CONTRACT_PRINT_BASELINE=1 go test ./internal/contract -run TestRatchets
func TestRatchets(t *testing.T) {
	unchecked := map[string]int{}
	scalar := map[string]int{}
	panics := map[string]int{}
	for _, path := range serviceFiles(t) {
		_, f := parse(t, path)
		pkg := filepath.ToSlash(filepath.Dir(rel(path)))
		ast.Inspect(f, func(n ast.Node) bool {
			switch v := n.(type) {
			case *ast.ExprStmt:
				if call, ok := v.X.(*ast.CallExpr); ok {
					if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "Set" && len(call.Args) == 2 {
						if x, ok := sel.X.(*ast.Ident); ok && x.Name == "d" {
							unchecked[pkg]++
						}
					}
				}
			case *ast.CallExpr:
				if id, ok := v.Fun.(*ast.Ident); ok && id.Name == "panic" {
					panics[pkg]++
				}
			case *ast.TypeAssertExpr:
				id, ok := v.Type.(*ast.Ident)
				if !ok || (id.Name != "bool" && id.Name != "int") {
					return true
				}
				if call, ok := v.X.(*ast.CallExpr); ok {
					if sel, ok := call.Fun.(*ast.SelectorExpr); ok && (sel.Sel.Name == "Get" || sel.Sel.Name == "GetOk") {
						if x, ok := sel.X.(*ast.Ident); ok && x.Name == "d" {
							scalar[pkg]++
						}
					}
				}
			}
			return true
		})
	}
	check := func(name string, counts, baseline map[string]int, hint string) {
		pkgs := map[string]bool{}
		for p := range counts {
			pkgs[p] = true
		}
		for p := range baseline {
			pkgs[p] = true
		}
		sorted := make([]string, 0, len(pkgs))
		for p := range pkgs {
			sorted = append(sorted, p)
		}
		sort.Strings(sorted)
		if os.Getenv("CONTRACT_PRINT_BASELINE") != "" {
			fmt.Printf("// %s\n", name)
			for _, p := range sorted {
				if counts[p] > 0 {
					fmt.Printf("\t%q: %d,\n", p, counts[p])
				}
			}
		}
		for _, p := range sorted {
			switch n, allowed := counts[p], baseline[p]; {
			case n > allowed:
				t.Errorf("%s: %s has %d (baseline %d); %s", name, p, n, allowed, hint)
			case n < allowed:
				t.Errorf("%s: %s has %d (baseline %d); lower the baseline in ratchet_baseline_test.go", name, p, n, allowed)
			}
		}
	}
	check("unchecked d.Set", unchecked, uncheckedSetBaseline, "check the error or use helpers.SetAll")
	check("bool/int assertion on d.Get/d.GetOk", scalar, scalarAssertionBaseline, "use helpers.GetBool / GetInt")
	check("panic", panics, panicBaseline, "return an error: a panic takes the whole plugin down")
}
