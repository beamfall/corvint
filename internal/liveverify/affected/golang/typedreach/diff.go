package typedreach

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"sort"
	"strings"
)

// FileChange is what one changed non-test Go file of a module package
// changes (AFP-V0-2376): the names of the top-level declarations whose
// printed text differs or that exist on one side only, a method as
// Type.Method, and, when the change runs at package initialization (an init
// function, or a package variable whose initializer calls), the declaration
// that does, which keeps every test binary linking the package whole.
type FileChange struct {
	Names     []string
	AlwaysRun string
}

// Declarations returns the change as declarations of the package at
// importPath; a change that runs at initialization is one Initialization
// declaration naming it, so it bears only on test binaries linking the
// package (AFP-V0-2376).
func (change FileChange) Declarations(importPath string) []Declaration {
	if change.AlwaysRun != "" {
		return []Declaration{{Package: importPath, Name: change.AlwaysRun, Initialization: true}}
	}
	out := make([]Declaration, 0, len(change.Names))
	for _, name := range change.Names {
		out = append(out, Declaration{Package: importPath, Name: name})
	}
	return out
}

// DiffFile compares one Go file at base and head; a nil side is an added or
// removed file. A parse failure, or a package clause that differs, is
// reported as AlwaysRun so the caller keeps the package whole.
func DiffFile(name string, base, head []byte) FileChange {
	before, beforeName, okBefore := fileDecls(name, base)
	after, afterName, okAfter := fileDecls(name, head)
	if !okBefore || !okAfter {
		return FileChange{AlwaysRun: name + ": does not parse"}
	}
	if base != nil && head != nil && beforeName != afterName {
		return FileChange{AlwaysRun: name + ": package clause changed"}
	}
	var change FileChange
	names := map[string]bool{}
	for _, side := range [2][2]map[string]declText{{before, after}, {after, before}} {
		for key, text := range side[0] {
			if other, ok := side[1][key]; ok && other.text == text.text {
				continue
			}
			names[text.name] = true
			if text.alwaysRun && change.AlwaysRun == "" {
				change.AlwaysRun = name + ": " + key
			}
		}
	}
	for n := range names {
		change.Names = append(change.Names, n)
	}
	sort.Strings(change.Names)
	return change
}

type declText struct {
	name      string
	text      string
	alwaysRun bool
}

// fileDecls keys every top-level declaration of a file: a function by its
// name, a method as Type.Method, a general declaration per name it declares
// (a group's members compared by their own spec). An absent file has none.
func fileDecls(name string, source []byte) (map[string]declText, string, bool) {
	out := map[string]declText{}
	if source == nil {
		return out, "", true
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, name, source, parser.SkipObjectResolution)
	if err != nil {
		return nil, "", false
	}
	print := func(node any) string {
		var buffer bytes.Buffer
		if err := printer.Fprint(&buffer, fset, node); err != nil {
			return "\x00unprintable"
		}
		return buffer.String()
	}
	inits := 0
	for _, decl := range file.Decls {
		switch decl := decl.(type) {
		case *ast.FuncDecl:
			key := decl.Name.Name
			if decl.Recv != nil && len(decl.Recv.List) > 0 {
				key = receiverTypeName(decl.Recv.List[0].Type) + "." + key
			}
			alwaysRun := decl.Recv == nil && key == "init"
			if alwaysRun {
				inits++
				key += "#" + strings.Repeat("+", inits)
			}
			out[key] = declText{name: strings.SplitN(key, "#", 2)[0], text: print(decl), alwaysRun: alwaysRun}
		case *ast.GenDecl:
			for _, spec := range decl.Specs {
				switch spec := spec.(type) {
				case *ast.ImportSpec:
					// Imports change nothing a declaration does not show.
				case *ast.TypeSpec:
					out[spec.Name.Name] = declText{name: spec.Name.Name, text: decl.Tok.String() + " " + print(spec)}
				case *ast.ValueSpec:
					// A const spec without values repeats the previous one's
					// (iota), so the whole group is its text.
					text := decl.Tok.String() + " " + print(spec)
					if decl.Tok == token.CONST {
						text = print(decl)
					}
					calls := false
					for _, value := range spec.Values {
						ast.Inspect(value, func(x ast.Node) bool {
							if _, ok := x.(*ast.CallExpr); ok {
								calls = true
							}
							return !calls
						})
					}
					for _, ident := range spec.Names {
						if ident.Name == "_" {
							// A blank variable exists only for its
							// initializer's effect.
							calls = calls || decl.Tok == token.VAR
						}
						out[ident.Name] = declText{name: ident.Name, text: text, alwaysRun: decl.Tok == token.VAR && calls}
					}
				}
			}
		}
	}
	return out, file.Name.Name, true
}

func receiverTypeName(expr ast.Expr) string {
	for {
		switch typed := expr.(type) {
		case *ast.StarExpr:
			expr = typed.X
		case *ast.ParenExpr:
			expr = typed.X
		case *ast.IndexExpr:
			expr = typed.X
		case *ast.IndexListExpr:
			expr = typed.X
		case *ast.Ident:
			return typed.Name
		default:
			return "?"
		}
	}
}
