// lifecyclerewrite migrates resource code from the deprecated helpers.Handle*ApiError
// wrappers to the lifecycle helpers (HandleRead / HandleDelete / HandleWrite /
// HandleWriteGit / HandleError), choosing the helper from the role of the enclosing
// function. It also removes d.MarkNewResource() calls in Read/Delete functions (they are
// no-ops there) and the always-false `if &x == nil { ... }` checks.
//
// Usage: go run ./internal/cmd/lifecyclerewrite [-dry] [-fromerr] [-local names] [-dsnotfound] [-nilclear] [-tflog] <dir>...
//
//   - -local a,b: package-local `a(err, resp)` error helpers (the IaCM parseError copies)
//     are rewritten by role like the old wrappers; the family-specific
//     HandleCFApiError(err, d, resp) is always recognised.
//   - -dsnotfound: in data-source functions, `d.SetId(""); return nil` (a silent empty
//     result) becomes a not-found error.
//   - -nilclear: in read functions, `if x == nil { d.SetId(""); return nil }` becomes
//     helpers.ReadMissing, so the removal is logged like every other one.
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
	"strconv"
	"strings"
	"unicode"
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

const helpersImport = "github.com/harness/terraform-provider-harness/helpers"

var (
	dryRun     = flag.Bool("dry", false, "report without writing")
	fromErr    = flag.Bool("fromerr", false, "also rewrite `return diag.FromErr(err)` in read/delete functions to HandleRead/HandleDelete (for clients without an *http.Response)")
	localNames = flag.String("local", "", "comma-separated names of package-local `name(err, resp)` error helpers to rewrite by role like the old wrappers")
	dsNotFound = flag.Bool("dsnotfound", false, "in data-source functions, turn `d.SetId(\"\"); return nil` into a not-found error")
	nilClear   = flag.Bool("nilclear", false, "in read functions, turn `if x == nil { d.SetId(\"\"); return nil }` into helpers.ReadMissing")
	tflogMode  = flag.Bool("tflog", false, "in functions that have a context.Context parameter, turn log.Printf/Println/Print into tflog calls (the [LEVEL] prefix picks the level)")
)

const tflogImport = "github.com/hashicorp/terraform-plugin-log/tflog"

var logLevels = []struct{ prefix, level string }{
	{"[TRACE]", "Trace"}, {"[DEBUG]", "Debug"}, {"[INFO]", "Info"}, {"[WARN]", "Warn"}, {"[WARNING]", "Warn"}, {"[ERROR]", "Error"},
}

// rewriteLogCall turns log.Printf("[DEBUG] x %s", a) into tflog.Debug(ctx, fmt.Sprintf("x %s", a)).
// It reports whether fmt is now needed.
func rewriteLogCall(call *ast.CallExpr, sel *ast.SelectorExpr, ctx string) (needFmt bool, level string, ok bool) {
	if len(call.Args) == 0 {
		return false, "", false
	}
	level = "Debug"
	var msg ast.Expr
	if lit, isLit := call.Args[0].(*ast.BasicLit); isLit && lit.Kind == token.STRING {
		s, err := strconv.Unquote(lit.Value)
		if err != nil {
			return false, "", false
		}
		for _, l := range logLevels {
			if strings.HasPrefix(strings.ToUpper(s), l.prefix) {
				s = strings.TrimLeft(s[len(l.prefix):], " ")
				level = l.level
				break
			}
		}
		s = strings.TrimRight(s, "\n")
		switch {
		case sel.Sel.Name == "Printf" && len(call.Args) > 1:
			msg = &ast.CallExpr{Fun: selector("fmt", "Sprintf"), Args: append([]ast.Expr{strLit(s)}, call.Args[1:]...)}
			needFmt = true
		case len(call.Args) > 1:
			msg = &ast.CallExpr{Fun: selector("fmt", "Sprint"), Args: append([]ast.Expr{strLit(s + " ")}, call.Args[1:]...)}
			needFmt = true
		default:
			msg = strLit(s)
		}
	} else {
		fn := "Sprint"
		if sel.Sel.Name == "Printf" {
			fn = "Sprintf"
		}
		msg = &ast.CallExpr{Fun: selector("fmt", fn), Args: call.Args}
		needFmt = true
	}
	call.Fun = selector("tflog", level)
	call.Args = []ast.Expr{ident(ctx), msg}
	return needFmt, level, true
}

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
		fmt.Fprintln(os.Stderr, "usage: lifecyclerewrite [-dry] [-fromerr] [-local names] [-dsnotfound] [-nilclear] <dir>...")
		os.Exit(2)
	}
	locals := map[string]bool{}
	for _, n := range strings.Split(*localNames, ",") {
		if n = strings.TrimSpace(n); n != "" {
			locals[n] = true
		}
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
			return rewriteFile(path, st, locals)
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
		"HandleApiError": true, "HandleDBOpsApiError": true, "HandleCodeApiError": true, "HandleChaosApiError": true, "HandleCFApiError": true,
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

// dataName returns the name of the *schema.ResourceData parameter, "d" by default.
func dataName(fn *ast.FuncDecl) string {
	if fn.Type.Params != nil {
		for _, p := range fn.Type.Params.List {
			if star, ok := p.Type.(*ast.StarExpr); ok {
				if sel, ok := star.X.(*ast.SelectorExpr); ok && sel.Sel.Name == "ResourceData" && len(p.Names) > 0 {
					return p.Names[0].Name
				}
			}
		}
	}
	return "d"
}

// callForm classifies a call: "helpers" for helpers.X, "cf3" for the (err, d, resp)
// HandleCFApiError, "local2" for a package-local (err, resp) helper named in -local.
func callForm(call *ast.CallExpr, locals map[string]bool) (name, form string) {
	switch fun := call.Fun.(type) {
	case *ast.SelectorExpr:
		x, ok := fun.X.(*ast.Ident)
		if !ok {
			return "", ""
		}
		switch {
		case x.Name == "helpers":
			return fun.Sel.Name, "helpers"
		case x.Name == "feature_flag" && fun.Sel.Name == "HandleCFApiError":
			return fun.Sel.Name, "cf3"
		}
	case *ast.Ident:
		switch {
		case fun.Name == "HandleCFApiError":
			return fun.Name, "cf3"
		case locals[fun.Name]:
			return fun.Name, "local2"
		}
	}
	return "", ""
}

func ident(name string) *ast.Ident { return &ast.Ident{Name: name} }

func selector(pkg, name string) *ast.SelectorExpr {
	return &ast.SelectorExpr{X: ident(pkg), Sel: ident(name)}
}

func strLit(s string) *ast.BasicLit {
	return &ast.BasicLit{Kind: token.STRING, Value: strconv.Quote(s)}
}

func ctxExpr(fn *ast.FuncDecl, st *stats, where string) ast.Expr {
	if name := ctxName(fn); name != "" {
		return ident(name)
	}
	st.noCtx = append(st.noCtx, where)
	return &ast.CallExpr{Fun: selector("context", "TODO")}
}

// humanName turns resourceGitopsAgentRead or readCloudProviderAws into "gitops agent" or
// "cloud provider aws", for messages.
func humanName(fn string) string {
	name := fn
	for _, p := range []string{"dataSource", "datasource", "resource"} {
		if strings.HasPrefix(name, p) {
			name = name[len(p):]
			break
		}
	}
	name = strings.TrimSuffix(name, "Read")
	name = strings.TrimPrefix(name, "read")
	name = strings.TrimPrefix(name, "Read")
	var words []string
	start := 0
	for i := 1; i <= len(name); i++ {
		if i == len(name) || (unicode.IsUpper(rune(name[i])) && !unicode.IsUpper(rune(name[i-1]))) {
			words = append(words, strings.ToLower(name[start:i]))
			start = i
		}
	}
	return strings.TrimSpace(strings.Join(words, " "))
}

// isSetIDEmpty reports whether s is `<d>.SetId("")`.
func isSetIDEmpty(s ast.Stmt, d string) bool {
	es, ok := s.(*ast.ExprStmt)
	if !ok {
		return false
	}
	call, ok := es.X.(*ast.CallExpr)
	if !ok || len(call.Args) != 1 {
		return false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "SetId" {
		return false
	}
	if x, ok := sel.X.(*ast.Ident); !ok || x.Name != d {
		return false
	}
	lit, ok := call.Args[0].(*ast.BasicLit)
	return ok && lit.Kind == token.STRING && lit.Value == `""`
}

func isMarkNew(s ast.Stmt) bool {
	es, ok := s.(*ast.ExprStmt)
	if !ok {
		return false
	}
	call, ok := es.X.(*ast.CallExpr)
	if !ok {
		return false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	return ok && sel.Sel.Name == "MarkNewResource"
}

func isReturnNil(s ast.Stmt) bool {
	ret, ok := s.(*ast.ReturnStmt)
	if !ok || len(ret.Results) != 1 {
		return false
	}
	id, ok := ret.Results[0].(*ast.Ident)
	return ok && id.Name == "nil"
}

// isSilentClear reports whether a block is `d.SetId(""); [d.MarkNewResource();] return nil`.
func isSilentClear(b *ast.BlockStmt, d string) bool {
	stmts := b.List
	if len(stmts) == 3 && isMarkNew(stmts[1]) {
		stmts = []ast.Stmt{stmts[0], stmts[2]}
	}
	return len(stmts) == 2 && isSetIDEmpty(stmts[0], d) && isReturnNil(stmts[1])
}

func isNilCompare(cond ast.Expr) bool {
	bin, ok := cond.(*ast.BinaryExpr)
	if !ok || bin.Op != token.EQL {
		return false
	}
	if _, isAddr := bin.X.(*ast.UnaryExpr); isAddr {
		return false
	}
	y, ok := bin.Y.(*ast.Ident)
	return ok && y.Name == "nil"
}

func rewriteFile(path string, st *stats, locals map[string]bool) error {
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
	needHelpers := false
	needTflog := false
	needFmt := false

	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		r := roleOf(path, fn)
		where := fmt.Sprintf("%s:%s", path, fn.Name.Name)
		dName := dataName(fn)

		// 0. log.Print* -> tflog in functions that carry a context.
		if *tflogMode {
			if ctx := ctxName(fn); ctx != "" {
				ast.Inspect(fn.Body, func(n ast.Node) bool {
					call, ok := n.(*ast.CallExpr)
					if !ok {
						return true
					}
					sel, ok := call.Fun.(*ast.SelectorExpr)
					if !ok {
						return true
					}
					if x, ok := sel.X.(*ast.Ident); !ok || x.Name != "log" || (sel.Sel.Name != "Printf" && sel.Sel.Name != "Println" && sel.Sel.Name != "Print") {
						return true
					}
					if fmtNeeded, level, ok := rewriteLogCall(call, sel, ctx); ok {
						needTflog = true
						needFmt = needFmt || fmtNeeded
						st.rewritten["log.Print* -> tflog."+level]++
						changed = true
					}
					return true
				})
			}
		}

		// 1. Rewrite helper calls.
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			old, form := callForm(call, locals)
			if form == "" {
				return true
			}
			switch {
			case form == "local2":
				helper, ok := target("HandleApiError", r)
				if !ok {
					st.unknown = append(st.unknown, where+" ("+old+")")
					return true
				}
				if len(call.Args) != 2 {
					return true
				}
				c := ctxExpr(fn, st, where)
				if _, isTODO := c.(*ast.CallExpr); isTODO {
					needContext = true
				}
				call.Fun = selector("helpers", helper)
				call.Args = []ast.Expr{c, ident(dName), call.Args[0], call.Args[1]}
				needHelpers = true
				st.rewritten[old+" -> "+helper]++
				changed = true
			case form == "cf3":
				helper, ok := target("HandleCFApiError", r)
				if !ok {
					st.unknown = append(st.unknown, where+" ("+old+")")
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
				needHelpers = true
				st.rewritten[old+" -> "+helper]++
				changed = true
			case old == "HandleChaosReadApiErrorWithGracefulDestroy":
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
			case old == "HandleChaosGraphQLError" || old == "HandleChaosGraphQLReadError":
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

		// transformIf applies -dsnotfound and -nilclear to an if statement and its else-if chain.
		var transformIf func(ifs *ast.IfStmt)
		transformIf = func(ifs *ast.IfStmt) {
			switch {
			case *dsNotFound && r == roleDataSource && isSilentClear(ifs.Body, dName):
				msg := humanName(fn.Name.Name) + " not found: no object matched the given arguments"
				ifs.Body.List = []ast.Stmt{&ast.ReturnStmt{Results: []ast.Expr{
					&ast.CallExpr{Fun: selector("diag", "Errorf"), Args: []ast.Expr{strLit(msg)}},
				}}}
				st.rewritten["data source silent empty -> diag.Errorf"]++
				changed = true
			case *nilClear && r == roleRead && isNilCompare(ifs.Cond) && isSilentClear(ifs.Body, dName):
				c := ctxExpr(fn, st, where)
				if _, isTODO := c.(*ast.CallExpr); isTODO {
					needContext = true
				}
				what := &ast.BinaryExpr{X: strLit(humanName(fn.Name.Name) + " "), Op: token.ADD,
					Y: &ast.CallExpr{Fun: selector(dName, "Id")}}
				ifs.Body.List = []ast.Stmt{&ast.ReturnStmt{Results: []ast.Expr{
					&ast.CallExpr{Fun: selector("helpers", "ReadMissing"), Args: []ast.Expr{c, ident(dName), what}},
				}}}
				needHelpers = true
				st.rewritten["if x == nil { SetId(\"\") } -> ReadMissing"]++
				changed = true
			}
			if next, ok := ifs.Else.(*ast.IfStmt); ok {
				transformIf(next)
			}
		}

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
					transformIf(stmt)
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
										call.Args = []ast.Expr{c, ident(dName), ident("err"), ident("nil")}
										needHelpers = true
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
	if needHelpers {
		ensureImport(file, helpersImport)
	}
	if needTflog {
		ensureImport(file, tflogImport)
	}
	if needFmt {
		ensureImport(file, "fmt")
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
