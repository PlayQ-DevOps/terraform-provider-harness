// Package contract holds repository-wide guardrail tests that parse the provider's source
// with go/ast and fail when a known-bad pattern comes back.
package contract

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

var repoRoot = filepath.Join("..", "..")

// serviceFiles returns every non-test Go file under internal/service.
func serviceFiles(t *testing.T) []string {
	t.Helper()
	var files []string
	err := filepath.WalkDir(filepath.Join(repoRoot, "internal", "service"), func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(files)
	return files
}

func parse(t *testing.T, path string) (*token.FileSet, *ast.File) {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return fset, f
}

func rel(path string) string {
	r, err := filepath.Rel(repoRoot, path)
	if err != nil {
		return path
	}
	return filepath.ToSlash(r)
}

// The deprecated error wrappers were deleted in FORK-2.4; the lifecycle helpers
// (HandleRead/HandleDelete/HandleWrite/HandleWriteGit/HandleError/ReadMissing) are the
// only way a resource may interpret an API error.
var oldHelperRe = regexp.MustCompile(`\bhelpers\.(HandleApiError|HandleReadApiError|HandleDBOpsApiError|HandleDBOpsReadApiError|HandleIacmReadApiError|HandlePolicyApiError|HandleCodeApiError|HandleCodeReadApiError|HandleChaosApiError|HandleChaosReadApiError|HandleChaosReadApiErrorWithGracefulDestroy|HandleChaosGraphQLError|HandleChaosGraphQLReadError|HandleGitApiError|HandleGitApiErrorWithResourceData|ParseErrorBody)\b`)

func TestNoDeprecatedErrorHelpers(t *testing.T) {
	for _, path := range serviceFiles(t) {
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if m := oldHelperRe.FindAll(src, -1); len(m) > 0 {
			t.Errorf("%s uses deprecated %s; use the lifecycle helpers in helpers/lifecycle.go", rel(path), m[0])
		}
	}
}

// d.MarkNewResource() after d.SetId("") in a Read or Delete does nothing (only Create paths
// consult IsNewResource) and used to disguise the not-found handling.
func TestNoMarkNewResourceInReadOrDelete(t *testing.T) {
	for _, path := range serviceFiles(t) {
		fset, f := parse(t, path)
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			lower := strings.ToLower(fn.Name.Name)
			if !strings.Contains(lower, "read") && !strings.Contains(lower, "delete") {
				continue
			}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "MarkNewResource" {
					t.Errorf("%s:%d: %s calls MarkNewResource, which is a no-op outside Create; remove it", rel(path), fset.Position(call.Pos()).Line, fn.Name.Name)
				}
				return true
			})
		}
	}
}

// `if &resp == nil { ... }` is never true (the address of a local is never nil); it used
// to masquerade as an empty-response check in GitOps and SRM reads.
func TestNoAddressOfNilChecks(t *testing.T) {
	for _, path := range serviceFiles(t) {
		fset, f := parse(t, path)
		ast.Inspect(f, func(n ast.Node) bool {
			bin, ok := n.(*ast.BinaryExpr)
			if !ok || (bin.Op != token.EQL && bin.Op != token.NEQ) {
				return true
			}
			if un, ok := bin.X.(*ast.UnaryExpr); ok && un.Op == token.AND {
				if y, ok := bin.Y.(*ast.Ident); ok && y.Name == "nil" {
					t.Errorf("%s:%d: `&x == nil` is always false; use helpers.ReadMissing when the response carries no object", rel(path), fset.Position(bin.Pos()).Line)
				}
			}
			return true
		})
	}
}

// A *http.Response may be nil whenever err != nil (transport failures, exhausted retries).
// Reading resp.StatusCode before the error is checked is a nil-pointer crash during plan.
func TestNoStatusCodeBeforeErrorCheck(t *testing.T) {
	allow := map[string]string{
		// resource:function allowlist with reasons; keep empty unless a guard is proven elsewhere.
	}
	for _, path := range serviceFiles(t) {
		fset, f := parse(t, path)
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			key := rel(path) + ":" + fn.Name.Name
			if _, ok := allow[key]; ok {
				continue
			}
			// Positions of `err != nil` checks and `x != nil` guards, in source order.
			var guards []token.Pos
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				ifs, ok := n.(*ast.IfStmt)
				if !ok {
					return true
				}
				ast.Inspect(ifs.Cond, func(c ast.Node) bool {
					if b, ok := c.(*ast.BinaryExpr); ok && b.Op == token.NEQ {
						if y, ok := b.Y.(*ast.Ident); ok && y.Name == "nil" {
							guards = append(guards, ifs.Pos())
						}
					}
					return true
				})
				return true
			})
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				ifs, ok := n.(*ast.IfStmt)
				if !ok {
					return true
				}
				uses := false
				ast.Inspect(ifs.Cond, func(c ast.Node) bool {
					if sel, ok := c.(*ast.SelectorExpr); ok && sel.Sel.Name == "StatusCode" {
						uses = true
					}
					return true
				})
				if !uses {
					return true
				}
				for _, g := range guards {
					if g <= ifs.Pos() {
						return true // an err/nil guard precedes (or is) this check
					}
				}
				t.Errorf("%s:%d: %s reads .StatusCode before checking err or the response for nil; use helpers.StatusOf or check err first", rel(path), fset.Position(ifs.Pos()).Line, fn.Name.Name)
				return true
			})
		}
	}
}
