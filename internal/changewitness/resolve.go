package changewitness

import (
	"go/ast"
	"go/parser"
	"go/token"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

// The resolver boundary is decision
// 0151-change-witness-symbol-resolver-boundary-2026-09-12.md (CWR-V0-014):
// go/parser plus go/ast top-level definitions over target-revision blobs, no
// type checking, no import resolution, and no scanner fallback. A blob the
// grammar refuses makes every definition count uncertain, so the caller
// abstains rather than resolving against a partial table.

// definition is one top-level Go definition span at the target revision.
type definition struct {
	name, path, blobOID string
	start, end          int
}

// goDefinitions indexes every top-level func, method, type, var, and const
// name in the `.go` sources. unparsed reports that any `.go` source failed.
func goDefinitions(sources []Source) (map[string][]definition, bool) {
	index := map[string][]definition{}
	for _, source := range goSources(sources) {
		found, ok := sourceDefinitions(source)
		if !ok {
			return nil, true
		}
		for _, def := range found {
			index[def.name] = append(index[def.name], def)
		}
	}
	return index, false
}

func goSources(sources []Source) []Source {
	selected := []Source{}
	for _, source := range sources {
		if !isGoPath(source.Path) {
			continue
		}
		selected = append(selected, source)
	}
	sort.Slice(selected, func(i, j int) bool { return selected[i].Path < selected[j].Path })
	return selected
}

func isGoPath(path string) bool {
	return strings.HasSuffix(path, ".go")
}

func sourceDefinitions(source Source) ([]definition, bool) {
	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, source.Path, source.Bytes, parser.SkipObjectResolution)
	if err != nil {
		return nil, false
	}
	found := []definition{}
	for _, declaration := range file.Decls {
		for _, named := range declaredNames(declaration) {
			found = append(found, spanDefinition(fileSet, source, named))
		}
	}
	return found, true
}

// declaredName is one bound name and the node whose extent is its span.
type declaredName struct {
	name string
	node ast.Node
}

func declaredNames(declaration ast.Decl) []declaredName {
	switch typed := declaration.(type) {
	case *ast.FuncDecl:
		return []declaredName{{typed.Name.Name, typed}}
	case *ast.GenDecl:
		return groupNames(typed)
	}
	return nil
}

// groupNames spans an ungrouped declaration from its keyword and a grouped
// member by its own spec, so one member's edit does not touch its siblings.
func groupNames(declaration *ast.GenDecl) []declaredName {
	names := []declaredName{}
	for _, spec := range declaration.Specs {
		names = append(names, specNames(spec, extentNode(declaration, spec))...)
	}
	return names
}

func extentNode(declaration *ast.GenDecl, spec ast.Spec) ast.Node {
	if declaration.Lparen.IsValid() {
		return spec
	}
	return declaration
}

func specNames(spec ast.Spec, extent ast.Node) []declaredName {
	switch typed := spec.(type) {
	case *ast.TypeSpec:
		return []declaredName{{typed.Name.Name, extent}}
	case *ast.ValueSpec:
		return valueNames(typed, extent)
	}
	return nil
}

func valueNames(spec *ast.ValueSpec, extent ast.Node) []declaredName {
	names := []declaredName{}
	for _, name := range spec.Names {
		if name.Name == "_" {
			continue
		}
		names = append(names, declaredName{name.Name, extent})
	}
	return names
}

func spanDefinition(fileSet *token.FileSet, source Source, named declaredName) definition {
	start := fileSet.PositionFor(named.node.Pos(), false).Line
	end := fileSet.PositionFor(named.node.End(), false).Line
	return definition{named.name, source.Path, source.BlobOID, start, end}
}

// identifierTokens is the sorted unique set of Go-identifier-shaped tokens in
// the span. It is a token scan, not a parse: qualified names are deferred.
func identifierTokens(span []byte) []string {
	seen := map[string]bool{}
	for _, word := range strings.FieldsFunc(string(span), notIdentifierRune) {
		seen[word] = startsIdentifier(word)
	}
	tokens := []string{}
	for word, keep := range seen {
		if !keep {
			continue
		}
		tokens = append(tokens, word)
	}
	sort.Strings(tokens)
	return tokens
}

func notIdentifierRune(r rune) bool {
	if r == '_' {
		return false
	}
	return !unicode.IsLetter(r) && !unicode.IsDigit(r)
}

func startsIdentifier(word string) bool {
	first, _ := utf8.DecodeRuneInString(word)
	if word == "_" {
		return false
	}
	return first == '_' || unicode.IsLetter(first)
}
