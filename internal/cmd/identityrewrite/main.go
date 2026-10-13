// identityrewrite marks identity attributes ForceNew in hand-written resource schemas, so
// a change to them plans a replacement instead of an in-place update the API rejects.
//
// Usage: go run ./internal/cmd/identityrewrite [-dry] [-keys a,b,c] <dir>...
//
// In every function that returns *schema.Resource (data sources excluded), the top-level
// Schema map literal is scanned; an entry whose key is one of -keys (default: identifier,
// org_id, project_id) and that is Required or Optional gets ForceNew: true, either by adding
// the field to its &schema.Schema{...} literal or by wrapping a helpers.Get*Schema(...)
// builder call in helpers.Identity(...). Schemas that are not literals are reported.
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
	"strconv"
	"strings"
)

var (
	dryRun = flag.Bool("dry", false, "report without writing")
	keys   = flag.String("keys", "identifier,org_id,project_id", "comma-separated attribute names to mark")
)

func main() {
	flag.Parse()
	if flag.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "usage: identityrewrite [-dry] [-keys a,b] <dir>...")
		os.Exit(2)
	}
	want := map[string]bool{}
	for _, k := range strings.Split(*keys, ",") {
		if k = strings.TrimSpace(k); k != "" {
			want[k] = true
		}
	}
	marked, wrapped := 0, 0
	for _, dir := range flag.Args() {
		err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			base := strings.ToLower(filepath.Base(path))
			if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") ||
				strings.HasPrefix(base, "data_source") || strings.HasPrefix(base, "datasource") || strings.HasPrefix(base, "data_") {
				return nil
			}
			m, w, err := rewriteFile(path, want)
			marked += m
			wrapped += w
			return err
		})
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
	fmt.Printf("ForceNew added to literals: %d\nbuilder calls wrapped in helpers.Identity: %d\n", marked, wrapped)
}

func returnsResource(fn *ast.FuncDecl) bool {
	if fn.Type.Results == nil || len(fn.Type.Results.List) != 1 {
		return false
	}
	star, ok := fn.Type.Results.List[0].Type.(*ast.StarExpr)
	if !ok {
		return false
	}
	sel, ok := star.X.(*ast.SelectorExpr)
	return ok && sel.Sel.Name == "Resource"
}

func isResourceLit(n ast.Node) (*ast.CompositeLit, bool) {
	var lit *ast.CompositeLit
	switch v := n.(type) {
	case *ast.UnaryExpr:
		if v.Op == token.AND {
			lit, _ = v.X.(*ast.CompositeLit)
		}
	case *ast.CompositeLit:
		lit = v
	}
	if lit == nil {
		return nil, false
	}
	sel, ok := lit.Type.(*ast.SelectorExpr)
	return lit, ok && sel.Sel.Name == "Resource"
}

func keyName(e ast.Expr) string {
	switch v := e.(type) {
	case *ast.BasicLit:
		s, err := strconv.Unquote(v.Value)
		if err == nil {
			return s
		}
	case *ast.Ident:
		return v.Name
	}
	return ""
}

func rewriteFile(path string, want map[string]bool) (marked, wrapped int, err error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return 0, 0, err
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, src, parser.ParseComments)
	if err != nil {
		return 0, 0, err
	}
	changed := false
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil || !returnsResource(fn) || strings.HasPrefix(strings.ToLower(fn.Name.Name), "datasource") {
			continue
		}
		var resLit *ast.CompositeLit
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			if resLit != nil {
				return false
			}
			if lit, ok := isResourceLit(n); ok {
				resLit = lit
				return false
			}
			return true
		})
		if resLit == nil {
			continue
		}
		var schemaMap *ast.CompositeLit
		for _, elt := range resLit.Elts {
			kv, ok := elt.(*ast.KeyValueExpr)
			if !ok || keyName(kv.Key) != "Schema" {
				continue
			}
			schemaMap, _ = kv.Value.(*ast.CompositeLit)
			if schemaMap == nil {
				fmt.Printf("NOT A LITERAL (check by hand): %s:%s Schema is %T\n", path, fn.Name.Name, kv.Value)
			}
		}
		if schemaMap == nil {
			continue
		}
		for _, elt := range schemaMap.Elts {
			kv, ok := elt.(*ast.KeyValueExpr)
			if !ok || !want[keyName(kv.Key)] {
				continue
			}
			switch v := kv.Value.(type) {
			case *ast.CallExpr:
				if sel, ok := v.Fun.(*ast.SelectorExpr); ok {
					if x, ok := sel.X.(*ast.Ident); ok && x.Name == "helpers" && sel.Sel.Name == "Identity" {
						continue
					}
				}
				kv.Value = &ast.CallExpr{Fun: &ast.SelectorExpr{X: &ast.Ident{Name: "helpers"}, Sel: &ast.Ident{Name: "Identity"}}, Args: []ast.Expr{v}}
				wrapped++
				changed = true
			default:
				var lit *ast.CompositeLit
				switch l := kv.Value.(type) {
				case *ast.CompositeLit:
					lit = l
				case *ast.UnaryExpr:
					lit, _ = l.X.(*ast.CompositeLit)
				}
				if lit == nil {
					fmt.Printf("UNHANDLED (check by hand): %s:%s %s is %T\n", path, fn.Name.Name, keyName(kv.Key), kv.Value)
					continue
				}
				required, optional, forceNewSet := false, false, false
				for _, f := range lit.Elts {
					fkv, ok := f.(*ast.KeyValueExpr)
					if !ok {
						continue
					}
					switch keyName(fkv.Key) {
					case "Required":
						required = keyName(fkv.Value) == "true"
					case "Optional":
						optional = keyName(fkv.Value) == "true"
					case "ForceNew":
						forceNewSet = true
						if keyName(fkv.Value) != "true" {
							fkv.Value = &ast.Ident{Name: "true"}
							marked++
							changed = true
						}
					}
				}
				if !required && !optional {
					continue
				}
				if !forceNewSet {
					lit.Elts = append(lit.Elts, &ast.KeyValueExpr{Key: &ast.Ident{Name: "ForceNew"}, Value: &ast.Ident{Name: "true"}})
					marked++
					changed = true
				}
			}
		}
	}
	if !changed {
		return marked, wrapped, nil
	}
	var buf bytes.Buffer
	if err := format.Node(&buf, fset, file); err != nil {
		return 0, 0, fmt.Errorf("%s: %w", path, err)
	}
	if *dryRun {
		return marked, wrapped, nil
	}
	return marked, wrapped, os.WriteFile(path, buf.Bytes(), 0o644)
}
