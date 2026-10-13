package contract

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"os"
	"strings"
	"testing"
)

// The full-refresh contract: a resource Read sets state from the API, never from the
// configuration, so drift is visible. The only way to keep a value the API does not report
// back is helpers.ConfigOnly / ConfigOnlyBool, which documents the attribute as not
// refreshed. Scope and identity values that address the object (org_id, project_id, the
// identifier) may be copied, since the API answered for exactly those.
var (
	// Attributes a Read may copy into state: they addressed the object the API answered for.
	refreshIdentityKeys = map[string]bool{
		"org_id": true, "project_id": true, "identifier": true, "id": true, "pipeline_id": true, "env_id": true,
		"environment_id": true, "service_id": true, "agent_id": true, "repo_identifier": true, "target_id": true, "account_id": true,
		"provider_id": true, "version": true, "filename": true,
	}
	// Reasoned exceptions, keyed "file:function"; an entry that no longer matches fails.
	refreshAllowlist = map[string]string{
		"internal/service/pipeline/policyset/resource_policyset.go:readPolicyset": "the API returns the policy list unordered; the prior order is kept so only a real reorder in the configuration shows as a diff",
	}
)

func isReadRole(name string) bool {
	lower := strings.ToLower(name)
	return strings.Contains(lower, "read") && !strings.Contains(lower, "datasource")
}

func isDataSourceFile(path string) bool {
	base := strings.ToLower(path)
	i := strings.LastIndexAny(base, `/\`)
	base = base[i+1:]
	return strings.HasPrefix(base, "data_source") || strings.HasPrefix(base, "datasource") || strings.HasPrefix(base, "data_")
}

// configReads returns the attribute keys a call expression reads from the configuration
// (d.Get, d.GetOk, helpers.BuildField), or nil.
func configReads(e ast.Expr) []string {
	var keys []string
	ast.Inspect(e, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		switch sel.Sel.Name {
		case "Get", "GetOk":
			if x, ok := sel.X.(*ast.Ident); ok && x.Name == "d" && len(call.Args) == 1 {
				keys = append(keys, keyOf(call.Args[0]))
			}
		case "BuildField", "BuildFieldBool":
			if x, ok := sel.X.(*ast.Ident); ok && x.Name == "helpers" && len(call.Args) == 2 {
				keys = append(keys, keyOf(call.Args[1]))
			}
		}
		return true
	})
	return keys
}

func keyOf(e ast.Expr) string {
	if lit, ok := e.(*ast.BasicLit); ok && lit.Kind == token.STRING {
		return strings.Trim(lit.Value, `"`)
	}
	return types.ExprString(e)
}

func allIdentity(keys []string) bool {
	for _, k := range keys {
		if !refreshIdentityKeys[k] {
			return false
		}
	}
	return true
}

func TestReadsRefreshFromTheAPI(t *testing.T) {
	matched := map[string]bool{}
	for _, path := range serviceFiles(t) {
		if isDataSourceFile(path) {
			continue
		}
		fset, f := parse(t, path)
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil || !isReadRole(fn.Name.Name) {
				continue
			}
			key := rel(path) + ":" + fn.Name.Name
			report := func(line int, msg string) {
				if reason, ok := refreshAllowlist[key]; ok {
					matched[key] = true
					_ = reason
					return
				}
				t.Errorf("%s:%d: %s: %s", rel(path), line, fn.Name.Name, msg)
			}

			// Identifiers bound to configuration reads in this function.
			bound := map[string][]string{}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				as, ok := n.(*ast.AssignStmt)
				if !ok || len(as.Lhs) != len(as.Rhs) {
					return true
				}
				for i, rhs := range as.Rhs {
					if id, ok := as.Lhs[i].(*ast.Ident); ok {
						if keys := configReads(rhs); len(keys) > 0 {
							bound[id.Name] = keys
						}
					}
				}
				return true
			})
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				// var x = helpers.BuildField(d, "...")
				if gd, ok := n.(*ast.GenDecl); ok && gd.Tok == token.VAR {
					for _, spec := range gd.Specs {
						vs, ok := spec.(*ast.ValueSpec)
						if !ok || len(vs.Values) != len(vs.Names) {
							continue
						}
						for i, v := range vs.Values {
							if keys := configReads(v); len(keys) > 0 {
								bound[vs.Names[i].Name] = keys
							}
						}
					}
				}
				return true
			})

			ast.Inspect(fn.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				switch fun := call.Fun.(type) {
				case *ast.SelectorExpr:
					// d.Set(k, <expr reading the configuration>)
					if x, ok := fun.X.(*ast.Ident); ok && x.Name == "d" && fun.Sel.Name == "Set" && len(call.Args) == 2 {
						keys := configReads(call.Args[1])
						for _, id := range identsIn(call.Args[1]) {
							keys = append(keys, bound[id]...)
						}
						if len(keys) > 0 && !allIdentity(keys) {
							report(fset.Position(call.Pos()).Line, "d.Set("+keyOf(call.Args[0])+", ...) writes a configuration value back to state; take it from the API, or declare it with helpers.ConfigOnly")
						}
					}
				case *ast.Ident:
					// readX(..., <identifier bound to a configuration read>, ...)
					lower := strings.ToLower(fun.Name)
					if !strings.HasPrefix(lower, "read") && !strings.HasPrefix(lower, "flatten") && !strings.HasPrefix(lower, "set") {
						return true
					}
					for _, arg := range call.Args {
						var keys []string
						keys = append(keys, configReads(arg)...)
						for _, id := range identsIn(arg) {
							keys = append(keys, bound[id]...)
						}
						if len(keys) > 0 && !allIdentity(keys) {
							report(fset.Position(call.Pos()).Line, fun.Name+"(...) receives configuration value(s) "+strings.Join(keys, ", ")+"; take them from the API, or declare them with helpers.ConfigOnly")
							break
						}
					}
				}
				return true
			})

			// if apiValue != "" { d.Set(k, apiValue) }: an attribute emptied in the API keeps its old
			// state. Pre-existing cases are a per-function baseline that may only shrink.
			conditional := 0
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				ifs, ok := n.(*ast.IfStmt)
				if !ok || ifs.Else != nil || len(ifs.Body.List) != 1 {
					return true
				}
				bin, ok := ifs.Cond.(*ast.BinaryExpr)
				if !ok || bin.Op != token.NEQ {
					return true
				}
				lit, ok := bin.Y.(*ast.BasicLit)
				if !ok || lit.Value != `""` {
					return true
				}
				es, ok := ifs.Body.List[0].(*ast.ExprStmt)
				if !ok {
					return true
				}
				call, ok := es.X.(*ast.CallExpr)
				if !ok || len(call.Args) != 2 {
					return true
				}
				sel, ok := call.Fun.(*ast.SelectorExpr)
				if !ok || sel.Sel.Name != "Set" {
					return true
				}
				if types.ExprString(call.Args[1]) == types.ExprString(bin.X) && len(configReads(bin.X)) == 0 {
					conditional++
				}
				return true
			})
			if allowed := refreshConditionalBaseline[key]; conditional > allowed {
				t.Errorf("%s: %d conditional d.Set calls skip empty API values (baseline %d), so a value removed in Harness stays in state; set them unconditionally", key, conditional, allowed)
			} else if conditional < allowed {
				t.Errorf("%s: %d conditional d.Set calls (baseline %d); lower the baseline in refresh_baseline_test.go", key, conditional, allowed)
			}
			if conditional > 0 && os.Getenv("CONTRACT_PRINT_BASELINE") != "" {
				fmt.Printf("\t%q: %d,\n", key, conditional)
			}
		}
	}
	for key, reason := range refreshAllowlist {
		if !matched[key] {
			t.Errorf("refreshAllowlist entry %q (%s) no longer matches anything; remove it", key, reason)
		}
	}
}

func identsIn(e ast.Expr) []string {
	var out []string
	ast.Inspect(e, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok {
			out = append(out, id.Name)
		}
		return true
	})
	return out
}
