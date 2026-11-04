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
	// File is the changed file's slash path; Mapped are the import paths
	// whose local name binds differently at head, and Added and Removed the
	// import paths only one side has (AFP-V0-2376).
	File                   string
	Mapped, Added, Removed []string
}

// Declarations returns the change as declarations of the package at
// importPath; a change that runs at initialization is one Initialization
// declaration naming it, so it bears only on test binaries linking the
// package (AFP-V0-2376).
func (change FileChange) Declarations(importPath string) []Declaration {
	if change.AlwaysRun != "" {
		return []Declaration{{Package: importPath, Name: change.AlwaysRun, Initialization: true}}
	}
	var out []Declaration
	for kind, paths := range map[string][]string{importMapped: change.Mapped, importAdded: change.Added, importRemoved: change.Removed} {
		for _, path := range paths {
			out = append(out, Declaration{Package: importPath, Name: kind + change.File + "\x00" + path})
		}
	}
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
	change := FileChange{File: name}
	if before[importsKey].text != after[importsKey].text {
		if before[importsKey].alwaysRun || after[importsKey].alwaysRun {
			return FileChange{AlwaysRun: name + ": a blank or dot import changed"}
		}
		change.Mapped, change.Added, change.Removed = importDelta(before[importsKey].text, after[importsKey].text)
	}
	delete(before, importsKey)
	delete(after, importsKey)
	names := map[string]bool{}
	for _, side := range [2][2]map[string]declText{{before, after}, {after, before}} {
		for key, text := range side[0] {
			if other, ok := side[1][key]; ok && other.text == text.text && other.positions == text.positions {
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

// importsKey keys a file's import set among its declarations.
const importsKey = "\x00imports"

type declText struct {
	name      string
	text      string
	positions string
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
	// Compiled file/line/column positions are observable through runtime and
	// logging APIs, even when the printed declaration is unchanged. Include
	// interior nodes and adjusted //line positions, but not byte offsets.
	positions := func(node ast.Node) string {
		var out strings.Builder
		ast.Inspect(node, func(n ast.Node) bool {
			if n != nil {
				out.WriteString(fset.Position(n.Pos()).String())
				out.WriteByte('\n')
				out.WriteString(fset.Position(n.End()).String())
				out.WriteByte('\n')
			}
			return true
		})
		return out.String()
	}
	inits := 0
	var imports []string
	initializing := false
	for _, spec := range file.Imports {
		local := ""
		if spec.Name != nil {
			local = spec.Name.Name
			initializing = initializing || local == "_" || local == "."
		}
		imports = append(imports, local+" "+spec.Path.Value)
	}
	sort.Strings(imports)
	out[importsKey] = declText{text: strings.Join(imports, "\n"), alwaysRun: initializing}
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
			out[key] = declText{name: strings.SplitN(key, "#", 2)[0], text: print(decl), positions: positions(decl), alwaysRun: alwaysRun}
		case *ast.GenDecl:
			for _, spec := range decl.Specs {
				switch spec := spec.(type) {
				case *ast.ImportSpec:
					// Imports change nothing a declaration does not show.
				case *ast.TypeSpec:
					out[spec.Name.Name] = declText{name: spec.Name.Name, text: decl.Tok.String() + " " + print(spec), positions: positions(spec)}
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
						out[ident.Name] = declText{name: ident.Name, text: text, positions: positions(spec), alwaysRun: decl.Tok == token.VAR && calls}
					}
				}
			}
		}
	}
	return out, file.Name.Name, true
}

// Import declaration kinds, carried in Declaration.Name.
const (
	importMapped  = "\x00import-mapped:"
	importAdded   = "\x00import-added:"
	importRemoved = "\x00import-removed:"
)

// importDelta compares two import sets ("name path" lines): a path both
// sides import under different names is mapped, the rest added or removed.
func importDelta(before, after string) (mapped, added, removed []string) {
	paths := func(set string) map[string]string {
		out := map[string]string{}
		for _, line := range strings.Split(set, "\n") {
			if line == "" {
				continue
			}
			name, quoted, _ := strings.Cut(line, " ")
			path := strings.Trim(quoted, "\"`")
			out[path] += name + ","
		}
		return out
	}
	b, a := paths(before), paths(after)
	for path, names := range a {
		switch old, ok := b[path]; {
		case !ok:
			added = append(added, path)
		case old != names:
			mapped = append(mapped, path)
		}
	}
	for path := range b {
		if _, ok := a[path]; !ok {
			removed = append(removed, path)
		}
	}
	sort.Strings(mapped)
	sort.Strings(added)
	sort.Strings(removed)
	return mapped, added, removed
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
