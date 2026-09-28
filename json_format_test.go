package kraken

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

// TestTimeFieldsDeclareFormat requires every exported time.Time / *time.Time
// struct field that takes part in JSON to declare its wire format with the
// `format` tag option (e.g. `json:"opentm,format:unix"`). Without one,
// encoding/json/v2 falls back to RFC 3339, so a missing tag on one of Kraken's
// UNIX-seconds timestamps would only surface as a decode error against the
// live API, and the "not set" sentinels (0, "0", "") would not decode at all.
func TestTimeFieldsDeclareFormat(t *testing.T) {
	fset := token.NewFileSet()
	pkgs := map[string][]*ast.File{} // non-test files by directory
	err := filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if name := d.Name(); path != "." && (strings.HasPrefix(name, ".") || name == "testdata") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		f, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if err != nil {
			return err
		}
		pkgs[filepath.Dir(path)] = append(pkgs[filepath.Dir(path)], f)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, files := range pkgs {
		custom := customJSONTypes(files)
		for _, f := range files {
			ast.Inspect(f, func(n ast.Node) bool {
				ts, ok := n.(*ast.TypeSpec)
				if !ok {
					return true
				}
				// A type with its own JSON methods (Kraken's positional
				// arrays) encodes its fields itself, so their tags are unused.
				if custom[ts.Name.Name] {
					return false
				}
				ast.Inspect(ts.Type, func(n ast.Node) bool {
					st, ok := n.(*ast.StructType)
					if !ok {
						return true
					}
					for _, field := range st.Fields.List {
						if !isTimeType(field.Type) || len(field.Names) == 0 || !field.Names[0].IsExported() {
							continue
						}
						var tag string
						if field.Tag != nil {
							raw, _ := strconv.Unquote(field.Tag.Value)
							tag = reflect.StructTag(raw).Get("json")
						}
						if tag == "-" {
							continue
						}
						checked++
						opts := strings.Split(tag, ",")[1:]
						if len(opts) == 0 || !strings.HasPrefix(opts[len(opts)-1], "format:") {
							t.Errorf("%s: field %s has json tag %q without a trailing format option", fset.Position(field.Pos()), field.Names[0].Name, tag)
						}
					}
					return true
				})
				return false
			})
		}
	}
	if checked == 0 {
		t.Fatal("no time.Time fields found; is the test running from the module root?")
	}
}

// customJSONTypes returns the types in a package's files that have both a
// JSON unmarshal and a JSON marshal method.
func customJSONTypes(files []*ast.File) map[string]bool {
	methods := map[string]int{}
	for _, f := range files {
		for _, decl := range f.Decls {
			fd, ok := decl.(*ast.FuncDecl)
			if !ok || fd.Recv == nil || len(fd.Recv.List) == 0 {
				continue
			}
			recv := fd.Recv.List[0].Type
			if star, ok := recv.(*ast.StarExpr); ok {
				recv = star.X
			}
			id, ok := recv.(*ast.Ident)
			if !ok {
				continue
			}
			switch fd.Name.Name {
			case "UnmarshalJSON", "UnmarshalJSONFrom":
				methods[id.Name] |= 1
			case "MarshalJSON", "MarshalJSONTo":
				methods[id.Name] |= 2
			}
		}
	}
	custom := map[string]bool{}
	for name, m := range methods {
		custom[name] = m == 3
	}
	return custom
}

func isTimeType(e ast.Expr) bool {
	if star, ok := e.(*ast.StarExpr); ok {
		e = star.X
	}
	sel, ok := e.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	pkg, ok := sel.X.(*ast.Ident)
	return ok && pkg.Name == "time" && sel.Sel.Name == "Time"
}
