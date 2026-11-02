// SPDX-License-Identifier: Apache-2.0
package main

import (
	"go/ast"
	"go/token"
	"regexp"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
)

// tempRootOnly returns, for each test file of one declared package, the
// AFP-V0-021 tokens whose every literal occurrence in that file lies inside a
// temp-root-only test function (AFP-V0-828, V1-3137). Such a token is never
// resolved from the repository root: the function writes its fixture under a
// temporary root and cannot reach the repository's own, so a subject ID such
// as ".github/workflows/" + file names a fixture file, not a repository read.
// It is still resolved against the package directory, where `go test` runs.
//
// A function is temp-root-only when it, or a function literal inside it, calls
// TempDir on a *testing.T, *testing.B, *testing.F or testing.TB parameter (or
// calls os.MkdirTemp), and neither it nor any same-package test function,
// method or package-level variable it names, transitively:
//   - holds a root-locating literal (a parent-only path, the Git root lookup,
//     GOMOD or GOWORK);
//   - calls runtime.Caller, runtime.Callers, os.Getwd, os.Executable or
//     filepath.Abs, which locate the source or working directory; or
//   - selects anything from a non-standard import, whose body is not read here;
//   - sits in a file with any dot import, whose unqualified names (Caller,
//     Getwd, Abs) are not resolved here;
//   - selects through a name it declares locally that also names an import
//     (a shadowed import is not the import); or
//   - selects through a qualifier that is neither an import name, a name it
//     declares, nor a same-package declaration, such as an import whose
//     package name its path does not spell (gopkg.in/yaml.v3 as yaml).
//
// A literal outside every function (a package-level constant or variable) is
// never exempt, and a package whose tests call Chdir anywhere has no exemption.
// Names are matched without scope resolution, so shadowing only withholds an
// exemption, never grants one.
func tempRootOnly(files []string, bodies [][]byte) ([]map[string]bool, error) {
	exempt := make([]map[string]bool, len(files))
	for index := range exempt {
		exempt[index] = map[string]bool{}
	}
	parsed := make([]*ast.File, len(files))
	for index, name := range files {
		_, file, err := parseFile(name, bodies[index])
		if err != nil {
			return nil, err
		}
		parsed[index] = file
	}
	scan := tempRootScan{tokens: map[string][]string{}, reach: map[string]bool{}, declared: packageNames(parsed)}
	deps := map[string][]string{}
	for _, file := range parsed {
		changesDirectory := false
		ast.Inspect(file, func(node ast.Node) bool {
			if call, ok := node.(*ast.CallExpr); ok {
				if selector, ok := call.Fun.(*ast.SelectorExpr); ok && selector.Sel.Name == "Chdir" {
					changesDirectory = true
				}
			}
			return !changesDirectory
		})
		if changesDirectory {
			return exempt, nil
		}
		imports := importAliases(file)
		declare := func(name string, node ast.Node) {
			// The blank identifier declares nothing a later name can refer to.
			if name == "_" {
				return
			}
			direct, names := scan.direct(node, imports)
			scan.reach[name] = scan.reach[name] || direct
			deps[name] = append(deps[name], names...)
		}
		for _, decl := range file.Decls {
			switch decl := decl.(type) {
			case *ast.FuncDecl:
				declare(decl.Name.Name, decl)
			case *ast.GenDecl:
				for _, spec := range decl.Specs {
					if value, ok := spec.(*ast.ValueSpec); ok {
						for _, name := range value.Names {
							declare(name.Name, value)
						}
					}
				}
			}
		}
	}
	// Propagate reach to every declaration that names one that reaches, until
	// nothing changes; a name absent from deps is no same-package declaration.
	for changed := true; changed; {
		changed = false
		for name, names := range deps {
			if scan.reach[name] {
				continue
			}
			for _, dependency := range names {
				if scan.reach[dependency] {
					scan.reach[name], changed = true, true
					break
				}
			}
		}
	}
	for index, file := range parsed {
		imports := importAliases(file)
		blocked := map[string]bool{}
		for _, decl := range file.Decls {
			if general, ok := decl.(*ast.GenDecl); ok && general.Tok == token.IMPORT {
				continue
			}
			function, isFunction := decl.(*ast.FuncDecl)
			only := isFunction && function.Body != nil && tempRooted(function, imports) && !scan.reaches(function, imports)
			var err error
			ast.Inspect(decl, func(node ast.Node) bool {
				literal, ok := node.(*ast.BasicLit)
				if !ok || literal.Kind != token.STRING || err != nil {
					return err == nil
				}
				var tokens []string
				if tokens, err = scan.literalTokens(literal.Value); err != nil {
					return false
				}
				for _, value := range tokens {
					if only {
						exempt[index][value] = true
					} else {
						blocked[value] = true
					}
				}
				return true
			})
			if err != nil {
				return nil, err
			}
		}
		for value := range blocked {
			delete(exempt[index], value)
		}
	}
	return exempt, nil
}

// tempRootScan caches literal tokens and records which same-package
// test-file declarations reach the repository root, transitively.
type tempRootScan struct {
	tokens   map[string][]string
	reach    map[string]bool
	declared map[string]bool
}

// packageNames is every package-level function, method, variable, constant and
// type name the test files declare.
func packageNames(files []*ast.File) map[string]bool {
	names := map[string]bool{}
	for _, file := range files {
		for _, decl := range file.Decls {
			switch decl := decl.(type) {
			case *ast.FuncDecl:
				names[decl.Name.Name] = true
			case *ast.GenDecl:
				for _, spec := range decl.Specs {
					switch spec := spec.(type) {
					case *ast.ValueSpec:
						for _, name := range spec.Names {
							names[name.Name] = true
						}
					case *ast.TypeSpec:
						names[spec.Name.Name] = true
					}
				}
			}
		}
	}
	return names
}

// localNames is every name node declares inside itself: receivers, parameters
// and results (of function literals too), := and range definitions, and var,
// const and type declarations. Scopes are not resolved, so a name declared
// anywhere in node counts throughout it.
func localNames(node ast.Node) map[string]bool {
	names := map[string]bool{}
	fields := func(list *ast.FieldList) {
		if list == nil {
			return
		}
		for _, field := range list.List {
			for _, name := range field.Names {
				names[name.Name] = true
			}
		}
	}
	idents := func(expressions ...ast.Expr) {
		for _, expression := range expressions {
			if ident, ok := expression.(*ast.Ident); ok {
				names[ident.Name] = true
			}
		}
	}
	ast.Inspect(node, func(child ast.Node) bool {
		switch child := child.(type) {
		case *ast.FuncDecl:
			fields(child.Recv)
		case *ast.FuncType:
			fields(child.TypeParams)
			fields(child.Params)
			fields(child.Results)
		case *ast.AssignStmt:
			if child.Tok == token.DEFINE {
				idents(child.Lhs...)
			}
		case *ast.RangeStmt:
			if child.Tok == token.DEFINE {
				idents(child.Key, child.Value)
			}
		case *ast.ValueSpec:
			for _, name := range child.Names {
				names[name.Name] = true
			}
		case *ast.TypeSpec:
			names[child.Name.Name] = true
		}
		return true
	})
	return names
}

// literalTokens is the AFP-V0-021 lexicon over one quoted literal, read through
// FilePathTokens so that it is exactly the token set the check resolves.
func (s *tempRootScan) literalTokens(quoted string) ([]string, error) {
	if tokens, ok := s.tokens[quoted]; ok {
		return tokens, nil
	}
	tokens, err := golang.FilePathTokens("literal.go", []byte("package p\n\nvar _ = "+quoted+"\n"), "")
	if err != nil {
		return nil, err
	}
	s.tokens[quoted] = tokens
	return tokens, nil
}

// reaches reports whether node, or a same-package test declaration it names,
// locates the repository root or calls code not read here.
func (s *tempRootScan) reaches(node ast.Node, imports map[string]string) bool {
	direct, names := s.direct(node, imports)
	if direct {
		return true
	}
	for _, name := range names {
		if s.reach[name] {
			return true
		}
	}
	return false
}

// direct reports whether node itself locates the repository root, selects
// from a non-standard import, or cannot be read with certainty (a dot import, a
// shadowed import or an unresolved qualifier), and returns every name it
// spells, which may name a same-package declaration (a call, a method or a
// package-level variable).
func (s *tempRootScan) direct(node ast.Node, imports map[string]string) (bool, []string) {
	if _, ok := imports["."]; ok {
		return true, nil
	}
	locals := localNames(node)
	found := false
	var names []string
	ast.Inspect(node, func(child ast.Node) bool {
		if found {
			return false
		}
		switch child := child.(type) {
		case *ast.BasicLit:
			if child.Kind == token.STRING {
				found = s.locatesRoot(child.Value)
			}
		case *ast.SelectorExpr:
			if pkg, ok := child.X.(*ast.Ident); ok {
				importPath, imported := imports[pkg.Name]
				switch {
				case locals[pkg.Name]:
					// A local value; one that shadows an import is not the import.
					found = imported
				case imported:
					found = !standardImport(importPath) || locatingCall[importPath+"."+child.Sel.Name]
					return false
				case !s.declared[pkg.Name]:
					// Possibly an import whose package name its path does not spell.
					found = true
				}
			}
			names = append(names, child.Sel.Name)
		case *ast.Ident:
			names = append(names, child.Name)
		}
		return true
	})
	return found, names
}

func (s *tempRootScan) locatesRoot(quoted string) bool {
	value, err := strconv.Unquote(quoted)
	if err != nil {
		return true
	}
	if strings.Contains(value, "GOMOD") || strings.Contains(value, "GOWORK") {
		return true
	}
	tokens, err := s.literalTokens(quoted)
	if err != nil {
		return true
	}
	for _, token := range tokens {
		if token == rootLocatorToken || parentOnly(token) {
			return true
		}
	}
	return false
}

// locatingCall lists the standard calls that find the source file or the
// working directory, from which a test can climb to the root without a literal.
var locatingCall = map[string]bool{
	"runtime.Caller": true, "runtime.Callers": true, "os.Getwd": true, "os.Executable": true, "path/filepath.Abs": true,
}

// standardImport reports a standard-library import path: its first element has
// no dot.
func standardImport(importPath string) bool {
	first, _, _ := strings.Cut(importPath, "/")
	return !strings.Contains(first, ".")
}

var majorVersion = regexp.MustCompile(`^v[0-9]+$`)

// importAliases maps every name a file may select an import through to its
// path: the explicit name, else every plausible package name of the path (its
// last element, the one before a major-version suffix, and that element without
// a "go-" prefix or "-" and "." bytes), so a misread name over-matches. A dot
// import maps "." to its path, which a non-standard one makes every name reach.
func importAliases(file *ast.File) map[string]string {
	aliases := map[string]string{}
	if file == nil {
		return aliases
	}
	for _, spec := range file.Imports {
		importPath, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			continue
		}
		if spec.Name != nil {
			// Of several dot imports, a non-standard one is kept.
			if previous, ok := aliases[spec.Name.Name]; !ok || standardImport(previous) {
				aliases[spec.Name.Name] = importPath
			}
			continue
		}
		elements := strings.Split(importPath, "/")
		last := elements[len(elements)-1]
		candidates := []string{last}
		if majorVersion.MatchString(last) && len(elements) > 1 {
			candidates = append(candidates, elements[len(elements)-2])
		}
		for _, candidate := range candidates {
			aliases[candidate] = importPath
			trimmed := strings.NewReplacer("-", "", ".", "").Replace(strings.TrimPrefix(candidate, "go-"))
			aliases[trimmed] = importPath
		}
	}
	return aliases
}

// tempRooted reports a function that creates a temporary root: it, or a
// function literal inside it, calls TempDir on a testing parameter that is never
// reassigned, or calls os.MkdirTemp.
func tempRooted(function *ast.FuncDecl, imports map[string]string) bool {
	testingAlias := ""
	for alias, importPath := range imports {
		if importPath == "testing" {
			testingAlias = alias
		}
	}
	parameters, assigned := map[string]bool{}, map[string]bool{}
	ast.Inspect(function, func(node ast.Node) bool {
		switch node := node.(type) {
		case *ast.FuncType:
			if node.Params != nil {
				for _, field := range node.Params.List {
					if testingAlias != "" && testingType(field.Type, testingAlias) {
						for _, name := range field.Names {
							parameters[name.Name] = true
						}
					}
				}
			}
		case *ast.AssignStmt:
			for _, target := range node.Lhs {
				if ident, ok := target.(*ast.Ident); ok {
					assigned[ident.Name] = true
				}
			}
		case *ast.ValueSpec:
			for _, name := range node.Names {
				assigned[name.Name] = true
			}
		case *ast.RangeStmt:
			for _, target := range []ast.Expr{node.Key, node.Value} {
				if ident, ok := target.(*ast.Ident); ok {
					assigned[ident.Name] = true
				}
			}
		}
		return true
	})
	found := false
	ast.Inspect(function, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok || found {
			return !found
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		receiver, ok := selector.X.(*ast.Ident)
		if !ok {
			return true
		}
		switch {
		case selector.Sel.Name == "TempDir" && parameters[receiver.Name] && !assigned[receiver.Name]:
			found = true
		case selector.Sel.Name == "MkdirTemp" && imports[receiver.Name] == "os" && !localNames(function)[receiver.Name]:
			found = true
		}
		return !found
	})
	return found
}

// testingType reports *testing.T, *testing.B, *testing.F or testing.TB.
func testingType(expression ast.Expr, alias string) bool {
	pointer := false
	if star, ok := expression.(*ast.StarExpr); ok {
		expression, pointer = star.X, true
	}
	selector, ok := expression.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	if pkg, ok := selector.X.(*ast.Ident); !ok || pkg.Name != alias {
		return false
	}
	switch selector.Sel.Name {
	case "T", "B", "F":
		return pointer
	case "TB":
		return !pointer
	}
	return false
}

// fixtureMarker is the per-literal marker (ASD-V0-895, V1-3391): a string
// literal that starts or ends on a line carrying this comment is a fixture
// path, such as a path written into a temporary repository, and names no read.
const fixtureMarker = "//corvint:fixture-path"

// fixtureMarked maps, per test file, each AFP-V0-021 token that only marked
// or assertion literals of that file hold; a token any other literal also
// holds is never exempt, so neither a marker nor an assertion can hide a real
// read the same file makes. An assertion literal is expected text, which names
// no read (ASD-V0-613): an argument of a testing report method such as
// t.Errorf, the second or later argument of a strings or bytes search such as
// strings.Contains, or an operand of == or !=.
func fixtureMarked(files []string, bodies [][]byte) ([]map[string]bool, error) {
	marked := make([]map[string]bool, len(files))
	scan := tempRootScan{tokens: map[string][]string{}}
	for index, name := range files {
		marked[index] = map[string]bool{}
		fileSet, file, err := parseFile(name, bodies[index])
		if err != nil {
			return nil, err
		}
		lines := map[int]bool{}
		for _, group := range file.Comments {
			for _, comment := range group.List {
				if strings.TrimSpace(comment.Text) == fixtureMarker {
					lines[fileSet.Position(comment.Pos()).Line] = true
				}
			}
		}
		asserted := assertionLiterals(file)
		if len(lines) == 0 && len(asserted) == 0 {
			continue
		}
		blocked := map[string]bool{}
		ast.Inspect(file, func(node ast.Node) bool {
			literal, ok := node.(*ast.BasicLit)
			if !ok || literal.Kind != token.STRING || err != nil {
				return err == nil
			}
			var values []string
			if values, err = scan.literalTokens(literal.Value); err != nil {
				return false
			}
			fixture := asserted[literal] || lines[fileSet.Position(literal.Pos()).Line] || lines[fileSet.Position(literal.End()).Line]
			for _, value := range values {
				if fixture {
					marked[index][value] = true
				} else {
					blocked[value] = true
				}
			}
			return true
		})
		if err != nil {
			return nil, err
		}
		for value := range blocked {
			delete(marked[index], value)
		}
	}
	return marked, nil
}

// reportMethods are the testing methods whose arguments are messages.
var reportMethods = map[string]bool{"Error": true, "Errorf": true, "Fatal": true, "Fatalf": true, "Log": true, "Logf": true, "Skip": true, "Skipf": true}

// searchFunctions are the strings and bytes functions whose second and later
// arguments are the text searched for.
var searchFunctions = map[string]bool{"Contains": true, "ContainsAny": true, "HasPrefix": true, "HasSuffix": true, "Index": true, "LastIndex": true, "Count": true, "EqualFold": true}

// assertionLiterals returns the string literals of file that are expected
// text (ASD-V0-613). A literal nested inside such an argument, as in
// t.Errorf("%s", "x"+y), counts too.
func assertionLiterals(file *ast.File) map[*ast.BasicLit]bool {
	asserted := map[*ast.BasicLit]bool{}
	mark := func(expression ast.Expr) {
		ast.Inspect(expression, func(node ast.Node) bool {
			if call, ok := node.(*ast.CallExpr); ok {
				// A call inside the argument, such as os.ReadFile, may read;
				// a conversion such as []byte("x") does not.
				_, conversion := call.Fun.(*ast.ArrayType)
				return conversion || isIdent(call.Fun, "string")
			}
			if literal, ok := node.(*ast.BasicLit); ok && literal.Kind == token.STRING {
				asserted[literal] = true
			}
			return true
		})
	}
	ast.Inspect(file, func(node ast.Node) bool {
		switch node := node.(type) {
		case *ast.BinaryExpr:
			if node.Op == token.EQL || node.Op == token.NEQ {
				mark(node.X)
				mark(node.Y)
			}
		case *ast.CallExpr:
			selector, ok := node.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			if reportMethods[selector.Sel.Name] {
				for _, argument := range node.Args {
					mark(argument)
				}
			}
			if pkg, ok := selector.X.(*ast.Ident); ok && (pkg.Name == "strings" || pkg.Name == "bytes") && searchFunctions[selector.Sel.Name] {
				for _, argument := range node.Args[min(1, len(node.Args)):] {
					mark(argument)
				}
			}
		}
		return true
	})
	return asserted
}
