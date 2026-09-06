package model_test

import (
	"flag"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "rewrite testdata/exports.golden from the package's exported surface")

// This package computes no statistic, and the promise is worth only what
// enforces it. Every exported identifier — type, function, method, constant,
// variable and struct field — is listed in testdata/exports.golden, and an
// addition fails here until the golden is rewritten on purpose: a diff a
// reviewer reads with one question in front of them, whether the new line is a
// count, a mean, an extreme, a percentile, a range or a series.
func TestExportedSurfaceIsGolden(t *testing.T) {
	t.Parallel()

	got := strings.Join(exportedSurface(t), "\n") + "\n"
	golden := filepath.Join("testdata", "exports.golden")

	if *update {
		if err := os.WriteFile(golden, []byte(got), 0o600); err != nil {
			t.Fatal(err)
		}

		t.Logf("rewrote %s — review every changed line before committing it", golden)

		return
	}

	want, err := os.ReadFile(golden) //nolint:gosec // a fixed path under the package's own testdata
	if err != nil {
		t.Fatalf("%v — generate it with -update, then review it", err)
	}

	if got != string(want) {
		t.Fatalf("the exported surface differs from %s; if that is intended, rerun with -update and review the diff:\n%s",
			golden, surfaceDiff(got, string(want)))
	}
}

// exportedSurface lists every exported identifier of package model, one per
// line, sorted, in the forms "type T", "func F", "method T.M", "const C",
// "var V" and "field T.F".
func exportedSurface(t *testing.T) []string {
	t.Helper()

	sources, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}

	fset := token.NewFileSet()

	var out []string

	for _, src := range sources {
		if strings.HasSuffix(src, "_test.go") {
			continue
		}

		file, err := parser.ParseFile(fset, src, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatal(err)
		}

		for _, decl := range file.Decls {
			out = append(out, declExports(decl)...)
		}
	}

	if len(out) == 0 {
		t.Fatal("no exported identifier found; the test is not running in the package directory")
	}

	slices.Sort(out)

	return out
}

func declExports(decl ast.Decl) []string {
	switch d := decl.(type) {
	case *ast.FuncDecl:
		if !d.Name.IsExported() {
			return nil
		}

		if d.Recv == nil {
			return []string{"func " + d.Name.Name}
		}

		if recv := receiverName(d.Recv.List[0].Type); ast.IsExported(recv) {
			return []string{"method " + recv + "." + d.Name.Name}
		}

		return nil
	case *ast.GenDecl:
		return genDeclExports(d)
	default:
		return nil
	}
}

func genDeclExports(d *ast.GenDecl) []string {
	var out []string

	for _, spec := range d.Specs {
		switch s := spec.(type) {
		case *ast.TypeSpec:
			if !s.Name.IsExported() {
				continue
			}

			out = append(out, "type "+s.Name.Name)
			out = append(out, fieldExports(s)...)
		case *ast.ValueSpec:
			kind := "var"
			if d.Tok == token.CONST {
				kind = "const"
			}

			for _, n := range s.Names {
				if n.IsExported() {
					out = append(out, kind+" "+n.Name)
				}
			}
		}
	}

	return out
}

// fieldExports lists a struct's exported fields: a statistic could hide as a
// field as easily as a method.
func fieldExports(s *ast.TypeSpec) []string {
	st, ok := s.Type.(*ast.StructType)
	if !ok {
		return nil
	}

	var out []string

	for _, f := range st.Fields.List {
		for _, n := range f.Names {
			if n.IsExported() {
				out = append(out, "field "+s.Name.Name+"."+n.Name)
			}
		}
	}

	return out
}

// receiverName strips the pointer and any type parameters off a receiver, so
// that a method on *T or on Opt[T] is listed under its type's name.
func receiverName(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.StarExpr:
		return receiverName(e.X)
	case *ast.IndexExpr:
		return receiverName(e.X)
	case *ast.IndexListExpr:
		return receiverName(e.X)
	case *ast.Ident:
		return e.Name
	default:
		return ""
	}
}

// surfaceDiff names the lines that are in one surface and not the other.
func surfaceDiff(got, want string) string {
	gotLines := strings.Split(strings.TrimSpace(got), "\n")
	wantLines := strings.Split(strings.TrimSpace(want), "\n")

	var b strings.Builder

	for _, l := range gotLines {
		if !slices.Contains(wantLines, l) {
			b.WriteString("+ " + l + "\n")
		}
	}

	for _, l := range wantLines {
		if !slices.Contains(gotLines, l) {
			b.WriteString("- " + l + "\n")
		}
	}

	return b.String()
}
