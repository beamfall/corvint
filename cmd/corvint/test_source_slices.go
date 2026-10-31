package main

import (
	"go/ast"
	"go/parser"
	"go/scanner"
	"go/token"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/Beamfall/corvint/.github/cishards"
	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
	"github.com/Beamfall/corvint/internal/sessiontest"
)

// TSP-V0-006 slices a Go package the plan selects by DIRECT_SOURCE_CHANGE to
// the test functions that can reach what changed. The model is lexical and
// over-approximates reach; every proof it cannot make keeps the package whole
// with the missing proof named:
//
//   - the package's own dirty paths are existing non-test .go files present
//     at base, and its test files, and the plan of the other dirty paths does
//     not select the package (no dependency, reader or declared read reaches
//     it);
//   - when its test files changed too (TSP-V0-861), they pass every
//     STR-V0-063 proof testGoSlice makes (no initialization effect then or
//     now, no name an unchanged test file uses, the same imports), and their
//     test functions join the kept set;
//   - each changed file, with its receiver-less functions other than init
//     removed, is byte-identical to base, so only plain functions changed
//     (no import, type, method, variable, constant, init or build line);
//   - the package directory holds no assembly, C or other non-Go source, no
//     file has a //go:linkname comment, and none imports "C";
//   - no file of the package names a changed function other than in call
//     position or as its own declaration, so the function is never a value
//     that could travel into another package;
//   - package initialization (variable initializers, init functions,
//     TestMain) does not reach a changed function, and no changed function
//     reaches a package variable (state a skipped test could observe);
//   - a changed function's base body counts beside its head body, so state
//     the change stopped reaching or writing still keeps the package whole;
//   - each changed function's parameters and results are predeclared basic
//     value types (no pointer, slice, map, channel, function, interface,
//     struct, array, named, type-parameter or variadic type), so it cannot
//     write memory its caller shares; its file has no dot import;
//   - nothing the changed functions reach writes other than to a plain
//     identifier (no write through a pointer, selector or index, and no
//     copy, clear or delete), so every write is a local or a package
//     variable reach already refuses.
//
// Reach is the closure over top-level declarations: a declaration reaches
// every declaration of any identifier written inside it, a type reaches all
// its methods, and a name reaches every declaration of that name (methods of
// any receiver included). The selected name of a selector qualified by an
// unshadowed import name is not counted, so testing.T reaches no local T. Every test function of an external (_test package)
// file is kept, since a package it imports may call the changed function.
// The slice keeps each internal test function whose closure holds a changed
// function. A package whose every test is kept stays whole; one with none
// runs `-run ^$`, which still builds and vets the package and its tests.

// testSourceSliceLimit bounds the per-candidate plans source slicing makes.
const testSourceSliceLimit = 16

// testSourceForeign are directory suffixes whose code the lexical model
// cannot see but the package links.
var testSourceForeign = []string{".s", ".S", ".sx", ".syso", ".c", ".cc", ".cpp", ".cxx", ".m", ".h", ".hh", ".hpp", ".hxx", ".f", ".F", ".for", ".f90", ".swig", ".swigcxx"}

type testSourceCandidate struct {
	pkg      string
	unit     affected.Unit
	fallback string
	own      []string
	// tests are the package's own changed test files (TSP-V0-861).
	tests []string
}

// testSourceEligibles records in sliced the fallback of each candidate
// TSP-V0-006 cannot slice before reading base, and returns the rest.
func testSourceEligibles(plan affected.Plan, graph *affected.Graph, candidates []testSourceCandidate, sliced map[string]sessiontest.GoSlice) []testSourceCandidate {
	var eligible []testSourceCandidate
	for _, candidate := range candidates {
		if reason := testSourceEligible(plan, graph, &candidate, len(eligible)); reason != "" {
			sliced[candidate.pkg] = sessiontest.GoSlice{Reason: reason}
			continue
		}
		eligible = append(eligible, candidate)
	}
	return eligible
}

// testSourceOwnPaths lists the eligible candidates' changed source files.
func testSourceOwnPaths(eligible []testSourceCandidate) []string {
	var paths []string
	for _, candidate := range eligible {
		paths = append(append(paths, candidate.own...), candidate.tests...)
	}
	return paths
}

// testSourceEligible fills candidate.own and candidate.tests and returns ""
// when the package's own dirty paths are its non-test .go files and its test
// files, and the other dirty paths do not select it.
func testSourceEligible(plan affected.Plan, graph *affected.Graph, candidate *testSourceCandidate, planned int) string {
	var rest []string
	for _, dirty := range plan.Dirty {
		switch {
		case testFileOf(candidate.unit, dirty):
			candidate.tests = append(candidate.tests, dirty)
		case slices.Contains(candidate.unit.Sources, dirty):
			if !strings.HasSuffix(dirty, ".go") {
				return candidate.fallback + " (" + dirty + " is not Go source)"
			}
			candidate.own = append(candidate.own, dirty)
		default:
			rest = append(rest, dirty)
		}
	}
	if len(candidate.own) == 0 {
		return candidate.fallback
	}
	if len(rest) == 0 {
		return ""
	}
	if planned >= testSourceSliceLimit {
		return candidate.fallback + " (source slicing plans at most 16 packages)"
	}
	for _, other := range testSliceSelect(graph, rest).Selected {
		if other.UnitID == "go:"+candidate.pkg {
			return candidate.fallback + " (also selected by " + other.Witness.Kind + " via " + other.Witness.DirtyPath + ")"
		}
	}
	return ""
}

// testSourceDecl is one top-level declaration: the names it declares, the
// identifiers written inside it, and, for a type, its name.
type testSourceDecl struct {
	names []string
	uses  map[string]bool
	file  string
	test  string
	root  bool
	write string
}

func testSourceSlice(root string, candidate testSourceCandidate, atBase map[string]testBaseBlob) sessiontest.GoSlice {
	whole := func(detail string) sessiontest.GoSlice {
		return sessiontest.GoSlice{Reason: candidate.fallback + " (" + detail + ")"}
	}
	directory := path.Dir(candidate.own[0])
	entries, err := os.ReadDir(filepath.Join(root, filepath.FromSlash(directory)))
	if err != nil {
		return whole(directory + " cannot be listed")
	}
	type parsedFile struct {
		name   string
		source []byte
		file   *ast.File
		tests  []string
	}
	var files []parsedFile
	fileSet := token.NewFileSet()
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() {
			continue
		}
		for _, suffix := range testSourceForeign {
			if strings.HasSuffix(name, suffix) {
				return whole(path.Join(directory, name) + " is not Go source")
			}
		}
		if !strings.HasSuffix(name, ".go") {
			continue
		}
		relative := path.Join(directory, name)
		source, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(relative)))
		if err != nil {
			return whole(relative + " cannot be read")
		}
		file, err := parser.ParseFile(fileSet, relative, source, parser.ParseComments|parser.SkipObjectResolution)
		if err != nil {
			return whole(relative + " does not parse")
		}
		if testSourceLinkname(file) {
			return whole(relative + " names go:linkname")
		}
		for _, spec := range file.Imports {
			if spec.Path.Value == `"C"` {
				return whole(relative + " imports C")
			}
		}
		parsed := parsedFile{name: relative, source: source, file: file}
		if strings.HasSuffix(name, "_test.go") {
			declarations, err := golang.TestFileDeclarations(relative, source)
			if err != nil {
				return whole(relative + " does not parse")
			}
			parsed.tests = declarations.Tests
		}
		files = append(files, parsed)
	}

	changed := make(map[string]bool)
	type ownFile struct {
		name       string
		head, base *ast.File
	}
	var owned []ownFile
	for _, own := range candidate.own {
		prior := atBase[own]
		switch {
		case prior.absent:
			return whole(own + " is new")
		case prior.reason != "":
			return whole(own + " " + prior.reason)
		}
		index := slices.IndexFunc(files, func(file parsedFile) bool { return file.name == own })
		if index < 0 {
			return whole(own + " is removed")
		}
		baseSet := token.NewFileSet()
		baseFile, err := parser.ParseFile(baseSet, own, prior.content, parser.ParseComments|parser.SkipObjectResolution)
		if err != nil {
			return whole(own + " does not parse at base")
		}
		headRest, headFuncs := testSourcePlainFunctions(files[index].file, fileSet, files[index].source)
		baseRest, baseFuncs := testSourcePlainFunctions(baseFile, baseSet, prior.content)
		if headRest != baseRest {
			return whole(own + " changes more than plain functions")
		}
		for _, spec := range baseFile.Imports {
			if spec.Name != nil && spec.Name.Name == "." {
				return whole(own + " dot-imports " + spec.Path.Value)
			}
		}
		owned = append(owned, ownFile{name: own, head: files[index].file, base: baseFile})
		for name, text := range headFuncs {
			if prior, ok := baseFuncs[name]; !ok || prior != text {
				changed[name] = true
			}
		}
		for name := range baseFuncs {
			if _, ok := headFuncs[name]; !ok {
				changed[name] = true
			}
		}
	}
	if len(changed) == 0 {
		return sessiontest.GoSlice{Reason: candidate.fallback}
	}
	names := make([]string, 0, len(changed))
	for name := range changed {
		names = append(names, name)
	}
	slices.Sort(names)

	declared := make(map[string]bool)
	for _, file := range files {
		for _, decl := range file.file.Decls {
			for _, name := range testSourceDeclOf(file.name, nil, nil, decl).names {
				declared[name] = true
			}
		}
	}
	var decls []testSourceDecl
	external := make(map[string]bool)
	for _, file := range files {
		if strings.HasSuffix(file.file.Name.Name, "_test") {
			for _, test := range file.tests {
				external[test] = true
			}
		}
		if use := testSourceValueUse(file.file, changed); use != "" {
			return whole(file.name + " uses " + use + " other than in a call")
		}
		qualifiers := testSourceQualifiers(file.file, declared)
		for _, decl := range file.file.Decls {
			decls = append(decls, testSourceDeclOf(file.name, file.tests, qualifiers, decl))
		}
	}
	// A changed function's base body joins the graph beside its head body, so
	// state the change stopped reaching or writing still counts; and its
	// signature must carry no reference a caller's memory could travel through.
	for _, own := range owned {
		for _, version := range []*ast.File{own.head, own.base} {
			qualifiers := testSourceQualifiers(version, declared)
			for _, decl := range version.Decls {
				function, ok := decl.(*ast.FuncDecl)
				if !ok || function.Recv != nil || !changed[function.Name.Name] {
					continue
				}
				if reference := testSourceReferenceSignature(function.Type, declared); reference != "" {
					return whole(function.Name.Name + " in " + own.name + " has " + reference)
				}
				if version == own.base {
					decls = append(decls, testSourceDeclOf(own.name, nil, qualifiers, decl))
				}
			}
		}
	}
	declaring := make(map[string][]int)
	for index, decl := range decls {
		for _, name := range decl.names {
			declaring[name] = append(declaring[name], index)
		}
	}
	// One reverse walk from the declarations of the changed names finds every
	// declaration whose closure holds one; one forward walk finds what the
	// changed functions reach.
	users := make([][]int, len(decls))
	for index, decl := range decls {
		for use := range decl.uses {
			for _, target := range declaring[use] {
				users[target] = append(users[target], index)
			}
		}
	}
	walk := func(from []int, next func(int) []int) []bool {
		seen := make([]bool, len(decls))
		for len(from) != 0 {
			index := from[len(from)-1]
			from = from[:len(from)-1]
			if seen[index] {
				continue
			}
			seen[index] = true
			from = append(from, next(index)...)
		}
		return seen
	}
	var start []int
	for _, name := range names {
		start = append(start, declaring[name]...)
	}
	reaching := walk(slices.Clone(start), func(index int) []int { return users[index] })
	reached := walk(slices.Clone(start), func(index int) []int {
		var next []int
		for use := range decls[index].uses {
			next = append(next, declaring[use]...)
		}
		return next
	})
	for index, decl := range decls {
		switch {
		case decl.root && reaching[index]:
			return whole("package initialization in " + decl.file + " reaches " + strings.Join(names, ", "))
		case decl.root && decl.test == "" && reached[index] && len(decl.names) != 0:
			return whole(strings.Join(names, ", ") + " reaches the package variable " + strings.Join(decl.names, ", "))
		}
	}
	for index, decl := range decls {
		if !reached[index] || decl.write == "" {
			continue
		}
		writer := strings.Join(names, ", ") + " reaches " + strings.Join(decl.names, ", ") + ", which writes "
		if len(decl.names) == 1 && changed[decl.names[0]] {
			writer = decl.names[0] + " writes "
		}
		return whole(writer + decl.write + " (not provably local)")
	}

	var all, kept []string
	for index, decl := range decls {
		if decl.test == "" {
			continue
		}
		all = append(all, decl.test)
		if external[decl.test] || reaching[index] {
			kept = append(kept, decl.test)
		}
	}
	changedIn := strings.Join(names, ", ") + " changed in " + strings.Join(candidate.own, ", ")
	if len(candidate.tests) != 0 {
		// TSP-V0-861: the changed test files' own STR-V0-063 slice, under its
		// own proofs, joins the tests reaching the changed functions.
		changedTests := make(map[string]bool, len(candidate.tests))
		for _, test := range candidate.tests {
			changedTests[test] = true
		}
		own := testGoSlice(root, testSliceCandidate{id: "go:" + candidate.pkg, unit: candidate.unit, changed: changedTests}, testBaseDeclarations(atBase, candidate.tests))
		if own.Pattern == "" {
			return whole(strings.TrimPrefix(own.Reason, "whole: "))
		}
		kept = append(kept, strings.Split(strings.TrimSuffix(strings.TrimPrefix(own.Pattern, "^("), ")$"), "|")...)
		changedIn += " and the tests of " + strings.Join(candidate.tests, ", ")
	}
	slices.Sort(all)
	all = slices.Compact(all)
	slices.Sort(kept)
	kept = slices.Compact(kept)
	if len(kept) == len(all) {
		if len(candidate.tests) != 0 {
			return whole("every test function of the package reaches " + changedIn)
		}
		return sessiontest.GoSlice{Reason: candidate.fallback}
	}
	if len(kept) == 0 {
		return sessiontest.GoSlice{Pattern: "^$", Reason: "selected: no test reaches " + changedIn + "; build and vet only"}
	}
	pattern := "^(" + strings.Join(kept, "|") + ")$"
	if len(pattern) > cishards.MaxPatternBytes {
		return whole("the -run pattern exceeds the bound")
	}
	return sessiontest.GoSlice{Pattern: pattern, Reason: "selected: the tests reaching " + changedIn}
}

// testSourceLinkname reports whether a comment of file is a //go:linkname
// directive, the only form the compiler honours (TSP-V0-861); a string or
// prose naming it is no directive.
func testSourceLinkname(file *ast.File) bool {
	for _, group := range file.Comments {
		for _, comment := range group.List {
			if strings.HasPrefix(comment.Text, "//go:linkname") {
				return true
			}
		}
	}
	return false
}

// testSourcePlainFunctions splits a file into its receiver-less functions
// other than init, by name with their text and doc comment, and the rest of
// the file with those spans removed, as its token sequence (comments and
// directives included, layout ignored), so a function added or removed
// between declarations leaves the rest unchanged.
func testSourcePlainFunctions(file *ast.File, fileSet *token.FileSet, source []byte) (string, map[string]string) {
	offset := func(pos token.Pos) int { return fileSet.Position(pos).Offset }
	functions := make(map[string]string)
	var rest strings.Builder
	tokens := func(text []byte) {
		var scan scanner.Scanner
		scan.Init(token.NewFileSet().AddFile("", -1, len(text)), text, nil, scanner.ScanComments)
		for {
			_, tok, literal := scan.Scan()
			if tok == token.EOF {
				return
			}
			if tok == token.SEMICOLON && literal == "\n" {
				literal = ";"
			}
			rest.WriteString(tok.String() + "\x00" + literal + "\x00")
		}
	}
	last := 0
	for _, decl := range file.Decls {
		function, ok := decl.(*ast.FuncDecl)
		if !ok || function.Recv != nil || function.Name.Name == "init" {
			continue
		}
		start := offset(function.Pos())
		if function.Doc != nil {
			start = offset(function.Doc.Pos())
		}
		end := offset(function.End())
		functions[function.Name.Name] = string(source[start:end])
		tokens(source[last:start])
		last = end
	}
	tokens(source[last:])
	return rest.String(), functions
}

// testSourceValueUse names a changed function the file writes other than as
// the called expression of a call, a generic instantiation that is called,
// the selector of a call, or the name of a function declaration.
func testSourceValueUse(file *ast.File, changed map[string]bool) string {
	allowed := make(map[*ast.Ident]bool)
	ast.Inspect(file, func(node ast.Node) bool {
		switch node := node.(type) {
		case *ast.FuncDecl:
			allowed[node.Name] = true
		case *ast.CallExpr:
			callee := node.Fun
			for {
				switch inner := callee.(type) {
				case *ast.ParenExpr:
					callee = inner.X
					continue
				case *ast.IndexExpr:
					callee = inner.X
					continue
				case *ast.IndexListExpr:
					callee = inner.X
					continue
				}
				break
			}
			switch callee := callee.(type) {
			case *ast.Ident:
				allowed[callee] = true
			case *ast.SelectorExpr:
				allowed[callee.Sel] = true
			}
		}
		return true
	})
	use := ""
	ast.Inspect(file, func(node ast.Node) bool {
		if ident, ok := node.(*ast.Ident); ok && use == "" && changed[ident.Name] && !allowed[ident] {
			use = ident.Name
		}
		return use == ""
	})
	return use
}

// testSourceQualifiers names the file's imports that certainly denote a
// package wherever they qualify a selector: the import's name (its path's last
// element when unnamed; a wrong guess only keeps more uses) that no
// package-level declaration and no other identifier of the file shadows.
// `testing.T` then does not count as a use of the package's own T.
func testSourceQualifiers(file *ast.File, declared map[string]bool) map[string]bool {
	qualifiers := make(map[string]bool)
	for _, spec := range file.Imports {
		name := path.Base(strings.Trim(spec.Path.Value, "`\""))
		if spec.Name != nil {
			name = spec.Name.Name
		}
		if name != "_" && name != "." && !declared[name] {
			qualifiers[name] = true
		}
	}
	allowed := make(map[*ast.Ident]bool)
	ast.Inspect(file, func(node ast.Node) bool {
		switch node := node.(type) {
		case *ast.ImportSpec:
			if node.Name != nil {
				allowed[node.Name] = true
			}
		case *ast.SelectorExpr:
			if ident, ok := node.X.(*ast.Ident); ok {
				allowed[ident] = true
			}
		}
		return true
	})
	ast.Inspect(file, func(node ast.Node) bool {
		if ident, ok := node.(*ast.Ident); ok && !allowed[ident] {
			delete(qualifiers, ident.Name)
		}
		return true
	})
	return qualifiers
}

// testSourceDeclOf indexes one top-level declaration. A variable
// declaration, an init function and TestMain are roots (package
// initialization); a test function of a test file is named by test. The
// selected name of a selector qualified by a package (qualifiers) is no use.
func testSourceDeclOf(file string, tests []string, qualifiers map[string]bool, decl ast.Decl) testSourceDecl {
	entry := testSourceDecl{file: file, uses: make(map[string]bool)}
	ast.Inspect(decl, func(node ast.Node) bool {
		switch node := node.(type) {
		case *ast.SelectorExpr:
			if ident, ok := node.X.(*ast.Ident); ok && qualifiers[ident.Name] {
				return false
			}
		case *ast.Ident:
			entry.uses[node.Name] = true
		}
		return true
	})
	switch decl := decl.(type) {
	case *ast.FuncDecl:
		entry.write = testSourceWrite(decl)
		entry.names = []string{decl.Name.Name}
		delete(entry.uses, decl.Name.Name)
		if decl.Recv == nil {
			switch {
			case decl.Name.Name == "init" || decl.Name.Name == "TestMain":
				entry.root = true
				entry.names = nil
			case slices.Contains(tests, decl.Name.Name):
				entry.test = decl.Name.Name
				entry.names = nil
			}
		} else if receiver := testSourceReceiver(decl.Recv); receiver != "" {
			// A method is reached by its name and by its receiver type's name.
			entry.names = append(entry.names, receiver)
		}
	case *ast.GenDecl:
		for _, spec := range decl.Specs {
			switch spec := spec.(type) {
			case *ast.TypeSpec:
				entry.names = append(entry.names, spec.Name.Name)
			case *ast.ValueSpec:
				for _, name := range spec.Names {
					entry.names = append(entry.names, name.Name)
				}
				if decl.Tok == token.VAR {
					entry.root = true
				}
			}
		}
	}
	return entry
}

// testSourceReceiver names a method's receiver base type.
func testSourceReceiver(receiver *ast.FieldList) string {
	if len(receiver.List) == 0 {
		return ""
	}
	expression := receiver.List[0].Type
	for {
		switch inner := expression.(type) {
		case *ast.StarExpr:
			expression = inner.X
			continue
		case *ast.ParenExpr:
			expression = inner.X
			continue
		case *ast.IndexExpr:
			expression = inner.X
			continue
		case *ast.IndexListExpr:
			expression = inner.X
			continue
		}
		break
	}
	if ident, ok := expression.(*ast.Ident); ok {
		return ident.Name
	}
	return ""
}

// testSourceBasic are the predeclared value types whose values share no
// memory (uintptr is excluded: unsafe can turn it into a pointer).
var testSourceBasic = map[string]bool{
	"bool": true, "string": true, "byte": true, "rune": true,
	"int": true, "int8": true, "int16": true, "int32": true, "int64": true,
	"uint": true, "uint8": true, "uint16": true, "uint32": true, "uint64": true,
	"float32": true, "float64": true, "complex64": true, "complex128": true,
}

// testSourceReferenceSignature names the first parameter or result of a
// changed function whose type is not a predeclared basic value type the
// package does not redeclare: a pointer, slice, map, channel, function,
// interface, struct, array, named, type-parameter or variadic type could let
// the function write memory its caller shares with other tests.
func testSourceReferenceSignature(function *ast.FuncType, declared map[string]bool) string {
	if function.TypeParams != nil && len(function.TypeParams.List) != 0 {
		return "type parameters"
	}
	for _, list := range []*ast.FieldList{function.Params, function.Results} {
		if list == nil {
			continue
		}
		for _, field := range list.List {
			ident, ok := field.Type.(*ast.Ident)
			if !ok || !testSourceBasic[ident.Name] || declared[ident.Name] {
				return "a parameter or result of a type that may share memory"
			}
		}
	}
	return ""
}

// testSourceWrite names the first write in a function that the lexical model
// cannot prove local: an assignment, increment or range target other than a
// plain identifier or _ (a write through a pointer, selector or index), or a
// call of the builtins copy, clear or delete, which write through an argument.
// A plain identifier is a local or a package variable, which reach catches.
func testSourceWrite(function *ast.FuncDecl) string {
	if function.Body == nil {
		return ""
	}
	write := ""
	target := func(expression ast.Expr) {
		for {
			paren, ok := expression.(*ast.ParenExpr)
			if !ok {
				break
			}
			expression = paren.X
		}
		if _, ok := expression.(*ast.Ident); !ok && write == "" {
			write = "through " + testSourceExpression(expression)
		}
	}
	ast.Inspect(function.Body, func(node ast.Node) bool {
		switch node := node.(type) {
		case *ast.AssignStmt:
			if node.Tok != token.DEFINE {
				for _, left := range node.Lhs {
					target(left)
				}
			}
		case *ast.IncDecStmt:
			target(node.X)
		case *ast.RangeStmt:
			if node.Tok == token.ASSIGN {
				for _, left := range []ast.Expr{node.Key, node.Value} {
					if left != nil {
						target(left)
					}
				}
			}
		case *ast.CallExpr:
			if ident, ok := node.Fun.(*ast.Ident); ok && (ident.Name == "copy" || ident.Name == "clear" || ident.Name == "delete") && write == "" {
				write = "with " + ident.Name
			}
		}
		return write == ""
	})
	return write
}

// testSourceExpression renders a write target briefly for a reason.
func testSourceExpression(expression ast.Expr) string {
	switch expression := expression.(type) {
	case *ast.StarExpr:
		return "*" + testSourceExpression(expression.X)
	case *ast.SelectorExpr:
		return testSourceExpression(expression.X) + "." + expression.Sel.Name
	case *ast.IndexExpr:
		return testSourceExpression(expression.X) + "[...]"
	case *ast.Ident:
		return expression.Name
	}
	return "an expression"
}
