package lspprovider

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

// Query kinds and the relation type each one yields (EEP-V0-025). The type
// reads from the origin: its symbol is referenced by the other file, or it
// uses a definition in the other file.
const (
	queryReferences    = "textDocument/references"
	queryDefinition    = "textDocument/definition"
	typeReferencedBy   = ProviderID + ":referenced-by"
	typeUsesDefinition = ProviderID + ":uses-definition"
)

// target is one query position in an origin file: an LSP position (zero
// based line, UTF-16 character) and the symbol it names.
type target struct {
	method, symbol  string
	line, character int
}

// targets lists an origin's query positions in source order: its top-level
// function, method and type declarations for references, then the distinct
// selector and call names it uses for definitions, at most perKind each.
// Unparseable text yields what the partial syntax tree holds.
func targets(text string, perKind int) []target {
	files := token.NewFileSet()
	file, _ := parser.ParseFile(files, "origin.go", text, parser.SkipObjectResolution)
	if file == nil {
		return nil
	}
	lines := strings.SplitAfter(text, "\n")
	at := func(method, symbol string, pos token.Pos) target {
		position := files.Position(pos)
		line := lines[position.Line-1]
		prefix := line[:min(position.Column-1, len(line))]
		return target{method: method, symbol: symbol, line: position.Line - 1, character: utf16Length(prefix)}
	}
	declared := declarationTargets(file, perKind, at)
	used := usageTargets(file, perKind, at)
	return append(declared, used...)
}

func declarationTargets(file *ast.File, limit int, at func(string, string, token.Pos) target) []target {
	var out []target
	add := func(name *ast.Ident) {
		if len(out) < limit && name != nil && name.Name != "_" && name.Name != "init" && name.Name != "main" {
			out = append(out, at(queryReferences, name.Name, name.Pos()))
		}
	}
	for _, declaration := range file.Decls {
		switch typed := declaration.(type) {
		case *ast.FuncDecl:
			add(typed.Name)
		case *ast.GenDecl:
			for _, spec := range typed.Specs {
				if typeSpec, ok := spec.(*ast.TypeSpec); ok && typed.Tok == token.TYPE {
					add(typeSpec.Name)
				}
			}
		}
	}
	return out
}

func usageTargets(file *ast.File, limit int, at func(string, string, token.Pos) target) []target {
	var out []target
	seen := map[string]bool{}
	add := func(key string, name *ast.Ident) {
		if len(out) < limit && !seen[key] {
			seen[key] = true
			out = append(out, at(queryDefinition, key, name.Pos()))
		}
	}
	ast.Inspect(file, func(node ast.Node) bool {
		switch typed := node.(type) {
		case *ast.SelectorExpr:
			if receiver, ok := typed.X.(*ast.Ident); ok {
				add(receiver.Name+"."+typed.Sel.Name, typed.Sel)
			}
		case *ast.CallExpr:
			if name, ok := typed.Fun.(*ast.Ident); ok {
				add(name.Name, name)
			}
		}
		return len(out) < limit
	})
	return out
}

// utf16Length is the LSP character offset of a line prefix.
func utf16Length(prefix string) int {
	count := 0
	for len(prefix) > 0 {
		r, size := utf8.DecodeRuneInString(prefix)
		prefix = prefix[size:]
		count += len(utf16.Encode([]rune{r}))
	}
	return count
}
