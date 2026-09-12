// Package exports lists the exported identifiers of a package directory, so a
// test can hold the module's public surface against a golden file.
//
// It is test support and is not part of the public API. It reads source rather
// than types: the surface a consumer can name is what the declarations say, and
// reading them needs no build of the package under inspection.
package exports

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// Surface lists every exported identifier declared in dir, one per line,
// sorted, in the forms "type T", "func F", "method T.M", "imethod I.M",
// "const C", "var V" and "field T.F".
//
// Test files are skipped: they declare nothing a consumer can import. A method
// on an unexported receiver is skipped for the same reason. Struct fields and
// interface methods are listed because both are surface — a consumer reads the
// first and implements the second.
func Surface(dir string) ([]string, error) {
	sources, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil {
		return nil, fmt.Errorf("listing %s: %w", dir, err)
	}

	fset := token.NewFileSet()

	var out []string

	for _, src := range sources {
		if strings.HasSuffix(src, "_test.go") {
			continue
		}

		file, err := parser.ParseFile(fset, src, nil, parser.SkipObjectResolution)
		if err != nil {
			return nil, fmt.Errorf("parsing %s: %w", src, err)
		}

		for _, decl := range file.Decls {
			out = append(out, declExports(decl)...)
		}
	}

	if len(out) == 0 {
		return nil, fmt.Errorf("no exported identifier found in %s", dir)
	}

	slices.Sort(out)

	return out, nil
}

// Render turns the surfaces of several package directories into the module's
// contract table: one "## <dir>  (<count>)" header per package in the order
// given, its identifiers indented beneath it, and a TOTAL line.
//
// The count is part of the rendering so that a hand-edit which adds a line
// without adding to its count is a difference rather than a silent success.
func Render(dirs []string) (string, error) {
	var (
		b     strings.Builder
		total int
	)

	for _, dir := range dirs {
		names, err := Surface(dir)
		if err != nil {
			return "", err
		}

		total += len(names)

		fmt.Fprintf(&b, "## %s  (%d)\n", dir, len(names))

		for _, n := range names {
			b.WriteString("    " + n + "\n")
		}
	}

	fmt.Fprintf(&b, "TOTAL %d\n", total)

	return b.String(), nil
}

// Diff names the lines that are in one rendering and not the other, so a
// failure reads as what moved rather than as two walls of text.
func Diff(got, want string) string {
	gotLines := strings.Split(strings.TrimSpace(got), "\n")
	wantLines := strings.Split(strings.TrimSpace(want), "\n")

	var b strings.Builder

	for _, l := range gotLines {
		if !slices.Contains(wantLines, l) {
			b.WriteString("+ " + strings.TrimSpace(l) + "\n")
		}
	}

	for _, l := range wantLines {
		if !slices.Contains(gotLines, l) {
			b.WriteString("- " + strings.TrimSpace(l) + "\n")
		}
	}

	return b.String()
}

// Write rewrites the golden file at path with the given rendering.
func Write(path, rendered string) error {
	if err := os.WriteFile(path, []byte(rendered), 0o600); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}

	return nil
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
			out = append(out, memberExports(s)...)
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

// memberExports lists a struct's exported fields and an interface's exported
// methods. A statistic could hide as a field as easily as a method, and an
// interface method set is frozen on both sides: a consumer's test double
// implements it, so an added method breaks the implementer.
func memberExports(s *ast.TypeSpec) []string {
	var (
		out  []string
		kind string
		list *ast.FieldList
	)

	switch t := s.Type.(type) {
	case *ast.StructType:
		kind, list = "field ", t.Fields
	case *ast.InterfaceType:
		kind, list = "imethod ", t.Methods
	default:
		return nil
	}

	for _, f := range list.List {
		for _, n := range f.Names {
			if n.IsExported() {
				out = append(out, kind+s.Name.Name+"."+n.Name)
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
