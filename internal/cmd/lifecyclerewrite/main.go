// lifecyclerewrite migrates resource code from the deprecated helpers.Handle*ApiError
// wrappers to the lifecycle helpers (HandleRead / HandleDelete / HandleWrite /
// HandleWriteGit / HandleError), choosing the helper from the role of the enclosing
// function. It also removes d.MarkNewResource() calls in Read/Delete functions (they are
// no-ops there) and the always-false `if &x == nil { ... }` checks.
//
// Usage: go run ./internal/cmd/lifecyclerewrite [-dry] [-fromerr] <dir>...
//
// Functions whose role cannot be inferred from their name are reported and left alone.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type role int

const (
	roleUnknown role = iota
	roleRead
	roleDelete
	roleWrite
	roleDataSource
)

func (r role) String() string {
	return [...]string{"unknown", "read", "delete", "write", "datasource"}[r]
}

var (
	dryRun  = flag.Bool("dry", false, "report without writing")
	fromErr = flag.Bool("fromerr", false, "also rewrite `return diag.FromErr(err)` in read/delete functions to HandleRead/HandleDelete (for clients without an *http.Response)")
)

type stats struct {
	rewritten  map[string]int
	marks      int
	nilChecks  int
	unknown    []string
	noCtx      []string
	elseChecks []string
}

func main() {
	flag.Parse()
	if flag.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "usage: lifecyclerewrite [-dry] [-fromerr] <dir>...")
		os.Exit(2)
	}
	st := &stats{rewritten: map[string]int{}}
	for _, dir := range flag.Args() {
		err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			return rewriteFile(path, st)
		})
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
	keys := make([]string, 0, len(st.rewritten))
	for k := range st.rewritten {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Printf("%-70s %d\n", k, st.rewritten[k])
	}
	fmt.Printf("MarkNewResource removed: %d\n", st.marks)
	fmt.Printf("&x == nil checks removed: %d\n", st.nilChecks)
	for _, u := range st.unknown {
		fmt.Printf("UNKNOWN ROLE (left unchanged): %s\n", u)
	}
	for _, u := range st.noCtx {
		fmt.Printf("NO CTX PARAM (context.TODO() used): %s\n", u)
	}
	for _, u := range st.elseChecks {
		fmt.Printf("NIL CHECK WITH ELSE (left unchanged): %s\n", u)
	}
}

func roleOf(filename string, fn *ast.FuncDecl) role {
	name := fn.Name.Name
	lower := strings.ToLower(name)
	base := strings.ToLower(filepath.Base(filename))
	switch {
	case strings.Contains(lower, "datasource") || strings.HasPrefix(base, "data_source") || strings.HasPrefix(base, "datasource") || strings.HasPrefix(base, "data_"):
		return roleDataSource
	case strings.Contains(lower, "delete") || strings.Contains(lower, "destroy") || strings.Contains(lower, "revoke"):
		return roleDelete
	case strings.Contains(lower, "read"):
		return roleRead
	case strings.Contains(lower, "create") || strings.Contains(lower, "update") || strings.Contains(lower, "upsert") || strings.Contains(lower, "link") ||
		strings.Contains(lower, "sync") || strings.Contains(lower, "publish") || strings.Contains(lower, "import") || strings.Contains(lower, "patch") ||
		strings.Contains(lower, "apply"):
		return roleWrite
	}
	return roleUnknown
}

// target returns the lifecycle helper for an old wrapper in a function of the given role,
// and whether the old helper's arguments are (err, d, resp).
func target(old string, r role) (helper string, ok bool) {
	readish := map[string]bool{
		"HandleReadApiError": true, "HandleDBOpsReadApiError": true, "HandleIacmReadApiError": true,
		"HandleCodeReadApiError": true, "HandleChaosReadApiError": true, "HandlePolicyApiError": true,
	}
	generic := map[string]bool{
		"HandleApiError": true, "HandleDBOpsApiError": true, "HandleCodeApiError": true, "HandleChaosApiError": true,
	}
	switch {
	case readish[old] || generic[old]:
		switch r {
		case roleRead:
			return "HandleRead", true
		case roleDelete:
			return "HandleDelete", true
		case roleWrite:
			return "HandleWrite", true
		case roleDataSource:
			return "HandleError", true
		}
		return "", false
	case old == "HandleGitApiErrorWithResourceData":
		switch r {
		case roleRead:
			return "HandleRead", true
		case roleDelete:
			return "HandleDelete", true
		case roleDataSource:
			return "HandleError", true
		default:
			return "HandleWriteGit", true
		}
	}
	return "", false
}

func ctxName(fn *ast.FuncDecl) string {
	if fn.Type.Params == nil {
		return ""
	}
	for _, p := range fn.Type.Params.List {
		if sel, ok := p.Type.(*ast.SelectorExpr); ok {
			if x, ok := sel.X.(*ast.Ident); ok && x.Name == "context" && sel.Sel.Name == "Context" && len(p.Names) > 0 {
				if p.Names[0].Name == "_" {
					return ""
				}
				return p.Names[0].Name
			}
		}
	}
	return ""
}

func helpersCall(call *ast.CallExpr) (string, bool) {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return "", false
	}
	x, ok := sel.X.(*ast.Ident)
	if !ok || x.Name != "helpers" {
		return "", false
	}
	return sel.Sel.Name, true
}

func ident(name string) *ast.Ident { return &ast.Ident{Name: name} }

func selector(pkg, name string) *ast.SelectorExpr {
	return &ast.SelectorExpr{X: ident(pkg), Sel: ident(name)}
}

func ctxExpr(fn *ast.FuncDecl, st *stats, where string) ast.Expr {
	if name := ctxName(fn); name != "" {
		return ident(name)
	}
	st.noCtx = append(st.noCtx, where)
	return &ast.CallExpr{Fun: selector("context", "TODO")}
}

func rewriteFile(path string, st *stats) error {
	src, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, src, parser.ParseComments)
	if err != nil {
		return err
	}
	changed := false
	needContext := false

	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		r := roleOf(path, fn)
		where := fmt.Sprintf("%s:%s", path, fn.Name.Name)

		// 1. Rewrite helper calls.
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			old, ok := helpersCall(call)
			if !ok {
				return true
			}
			switch old {
			case "HandleChaosReadApiErrorWithGracefulDestroy":
				// (err, d, resp, patterns) -> HandleRead/HandleDelete(ctx, d, err, resp, helpers.GracefulNotFound(500, patterns...))
				if len(call.Args) != 4 {
					return true
				}
				helper := "HandleRead"
				if r == roleDelete {
					helper = "HandleDelete"
				} else if r != roleRead {
					st.unknown = append(st.unknown, where+" ("+old+")")
					return true
				}
				pred := &ast.CallExpr{Fun: selector("helpers", "GracefulNotFound"), Args: []ast.Expr{&ast.BasicLit{Kind: token.INT, Value: "500"}, call.Args[3]}, Ellipsis: token.Pos(1)}
				c := ctxExpr(fn, st, where)
				if _, isTODO := c.(*ast.CallExpr); isTODO {
					needContext = true
				}
				call.Fun = selector("helpers", helper)
				call.Args = []ast.Expr{c, call.Args[1], call.Args[0], call.Args[2], pred}
				st.rewritten[old+" -> "+helper]++
				changed = true
			case "HandleChaosGraphQLError", "HandleChaosGraphQLReadError":
				// (err, d, operation) -> helper(ctx, d, err, nil[, helpers.GraphQLNotFound()])
				if len(call.Args) != 3 {
					return true
				}
				var helper string
				var opts []ast.Expr
				switch {
				case old == "HandleChaosGraphQLReadError" || r == roleRead:
					helper = "HandleRead"
					opts = []ast.Expr{&ast.CallExpr{Fun: selector("helpers", "GraphQLNotFound")}}
				case r == roleDelete:
					helper = "HandleDelete"
					opts = []ast.Expr{&ast.CallExpr{Fun: selector("helpers", "GraphQLNotFound")}}
				case r == roleWrite:
					helper = "HandleWrite"
				default:
					helper = "HandleError"
				}
				c := ctxExpr(fn, st, where)
				if _, isTODO := c.(*ast.CallExpr); isTODO {
					needContext = true
				}
				call.Fun = selector("helpers", helper)
				call.Args = append([]ast.Expr{c, call.Args[1], call.Args[0], ident("nil")}, opts...)
				st.rewritten[old+" -> "+helper]++
				changed = true
			default:
				helper, ok := target(old, r)
				if !ok {
					if _, known := map[string]bool{"HandleApiError": true, "HandleReadApiError": true, "HandleDBOpsApiError": true, "HandleDBOpsReadApiError": true, "HandleIacmReadApiError": true, "HandlePolicyApiError": true, "HandleCodeApiError": true, "HandleCodeReadApiError": true, "HandleChaosApiError": true, "HandleChaosReadApiError": true, "HandleGitApiErrorWithResourceData": true}[old]; known {
						st.unknown = append(st.unknown, where+" ("+old+")")
					}
					return true
				}
				if len(call.Args) != 3 {
					return true
				}
				c := ctxExpr(fn, st, where)
				if _, isTODO := c.(*ast.CallExpr); isTODO {
					needContext = true
				}
				call.Fun = selector("helpers", helper)
				call.Args = []ast.Expr{c, call.Args[1], call.Args[0], call.Args[2]}
				st.rewritten[old+" -> "+helper]++
				changed = true
			}
			return true
		})

		// 2. Remove d.MarkNewResource() in read/delete/data-source functions and `if &x == nil {}` anywhere;
		//    optionally rewrite diag.FromErr(err) returns in read/delete functions.
		var rewriteBlock func(b *ast.BlockStmt)
		rewriteBlock = func(b *ast.BlockStmt) {
			var out []ast.Stmt
			for _, s := range b.List {
				switch stmt := s.(type) {
				case *ast.ExprStmt:
					if call, ok := stmt.X.(*ast.CallExpr); ok && (r == roleRead || r == roleDelete || r == roleDataSource) {
						if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "MarkNewResource" && len(call.Args) == 0 {
							st.marks++
							changed = true
							continue
						}
					}
				case *ast.IfStmt:
					if isAddrNilCheck(stmt.Cond) {
						if stmt.Else != nil {
							st.elseChecks = append(st.elseChecks, where)
						} else {
							st.nilChecks++
							changed = true
							continue
						}
					}
				case *ast.ReturnStmt:
					if *fromErr && (r == roleRead || r == roleDelete) && len(stmt.Results) == 1 {
						if call, ok := stmt.Results[0].(*ast.CallExpr); ok {
							if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
								if x, ok := sel.X.(*ast.Ident); ok && x.Name == "diag" && sel.Sel.Name == "FromErr" && len(call.Args) == 1 {
									if argIdent, ok := call.Args[0].(*ast.Ident); ok && argIdent.Name == "err" {
										helper := "HandleRead"
										if r == roleDelete {
											helper = "HandleDelete"
										}
										c := ctxExpr(fn, st, where)
										if _, isTODO := c.(*ast.CallExpr); isTODO {
											needContext = true
										}
										call.Fun = selector("helpers", helper)
										call.Args = []ast.Expr{c, ident("d"), ident("err"), ident("nil")}
										st.rewritten["diag.FromErr(err) -> "+helper]++
										changed = true
									}
								}
							}
						}
					}
				}
				out = append(out, s)
			}
			b.List = out
			for _, s := range b.List {
				ast.Inspect(s, func(n ast.Node) bool {
					if inner, ok := n.(*ast.BlockStmt); ok && inner != b {
						rewriteBlock(inner)
						return false
					}
					return true
				})
			}
		}
		rewriteBlock(fn.Body)
	}

	if !changed {
		return nil
	}
	if needContext {
		ensureImport(file, "context")
	}
	var buf bytes.Buffer
	if err := format.Node(&buf, fset, file); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	if *dryRun {
		return nil
	}
	return os.WriteFile(path, buf.Bytes(), 0o644)
}

func isAddrNilCheck(cond ast.Expr) bool {
	bin, ok := cond.(*ast.BinaryExpr)
	if !ok || bin.Op != token.EQL {
		return false
	}
	un, ok := bin.X.(*ast.UnaryExpr)
	if !ok || un.Op != token.AND {
		return false
	}
	y, ok := bin.Y.(*ast.Ident)
	return ok && y.Name == "nil"
}

func ensureImport(file *ast.File, pkg string) {
	for _, imp := range file.Imports {
		if strings.Trim(imp.Path.Value, `"`) == pkg {
			return
		}
	}
	spec := &ast.ImportSpec{Path: &ast.BasicLit{Kind: token.STRING, Value: `"` + pkg + `"`}}
	for _, decl := range file.Decls {
		if gd, ok := decl.(*ast.GenDecl); ok && gd.Tok == token.IMPORT {
			gd.Specs = append(gd.Specs, spec)
			file.Imports = append(file.Imports, spec)
			return
		}
	}
	gd := &ast.GenDecl{Tok: token.IMPORT, Specs: []ast.Spec{spec}}
	file.Decls = append([]ast.Decl{gd}, file.Decls...)
	file.Imports = append(file.Imports, spec)
}
