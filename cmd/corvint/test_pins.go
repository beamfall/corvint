package main

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// This file is the advisory pins list of `corvint test --dry-run --pins`
// (TSP-V0-944, V1-3808). A package the plan selects whole runs every test, but
// a lane choosing a focused -run filter needs to know which of its tests read
// what changed. The pins of a selected Go package are its test functions whose
// bodies, or the test-file helpers they call, name a changed declaration or
// hold a string that overlaps a changed literal. The list is advice for which
// tests to run first: it never narrows the plan, the manifest or the gate.

const (
	// testPinMaxFiles bounds the dirty Go files whose tokens are compared.
	testPinMaxFiles = 256
	// testPinMaxTokens bounds the changed names and the changed literals.
	testPinMaxTokens = 512
	// testPinMaxTests bounds the pinned tests named per package.
	testPinMaxTests = 64
	// testPinMinLiteral is the shortest changed literal compared, and
	// testPinMinHeld the shortest test literal a changed literal may hold,
	// so a short word such as "test" or "%s" pins nothing; testPinMaxHolder
	// is the longest changed literal that may hold a test literal, so a
	// changed help text does not pin every test quoting a flag.
	testPinMinLiteral = 8
	testPinMinHeld    = 10
	testPinMaxHolder  = 256
)

// testPinTokens are the names and string literals one change touches; a
// name is keyed by the directory of the package that declares it, so a test
// reads it only as a bare name in that directory or through a selector on an
// import of that package.
type testPinTokens struct {
	names    map[string]map[string]bool
	literals []string
	// clipped names what the bounds left out, if anything.
	clipped string
}

// testPinUnit is one selected Go package's pinned tests.
type testPinUnit struct {
	unit  string
	tests []string
	more  int
}

// testChangedTokens compares each dirty non-test Go file with its base blob:
// a top-level function, method, type, var or const whose text differs, or
// that exists on one side only, contributes its name; a string literal whose
// count differs contributes its value. An unreadable or unparsable side
// contributes nothing from that side.
func testChangedTokens(ctx context.Context, root, base string, dirty []string) testPinTokens {
	tokens := testPinTokens{names: map[string]map[string]bool{}}
	var files []string
	for _, file := range dirty {
		if strings.HasSuffix(file, ".go") && !strings.HasSuffix(file, "_test.go") {
			files = append(files, file)
		}
	}
	if len(files) > testPinMaxFiles {
		tokens.clipped = fmt.Sprintf("%d of %d dirty Go files compared", testPinMaxFiles, len(files))
		files = files[:testPinMaxFiles]
	}
	blobs := testBaseBlobs(ctx, root, base, files)
	literals := map[string]bool{}
	for _, file := range files {
		head, _ := os.ReadFile(filepath.Join(root, filepath.FromSlash(file)))
		headDecls, headLits := testPinDeclarations(file, head)
		baseDecls, baseLits := testPinDeclarations(file, blobs[file].content)
		directory := path.Dir(file)
		add := func(key string) {
			if tokens.names[directory] == nil {
				tokens.names[directory] = map[string]bool{}
			}
			tokens.names[directory][testPinName(key)] = true
		}
		for name, text := range headDecls {
			if baseDecls[name] != text {
				add(name)
			}
		}
		for name := range baseDecls {
			if _, ok := headDecls[name]; !ok {
				add(name)
			}
		}
		for value, count := range headLits {
			if baseLits[value] != count {
				literals[value] = true
			}
		}
		for value := range baseLits {
			if headLits[value] == 0 {
				literals[value] = true
			}
		}
	}
	tokens.literals = slices.Sorted(func(yield func(string) bool) {
		for value := range literals {
			if !yield(value) {
				return
			}
		}
	})
	if count := tokens.nameCount(); count > testPinMaxTokens || len(tokens.literals) > testPinMaxTokens {
		clip := fmt.Sprintf("%d names and %d literals changed, over the bound of %d each; no test is pinned", count, len(tokens.literals), testPinMaxTokens)
		tokens.clipped = strings.TrimPrefix(tokens.clipped+"; "+clip, "; ")
		tokens.names, tokens.literals = map[string]map[string]bool{}, nil
	}
	return tokens
}

// testPinDeclarations returns a file's top-level declarations as name to
// source text (a method under Receiver.Name, so its bare name is what a
// selector reads) and its string literals with their counts.
func testPinDeclarations(path string, source []byte) (map[string]string, map[string]int) {
	declarations, literals := map[string]string{}, map[string]int{}
	if len(source) == 0 {
		return declarations, literals
	}
	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, path, source, parser.SkipObjectResolution)
	if err != nil {
		return declarations, literals
	}
	text := func(node ast.Node) string {
		return string(source[fileSet.Position(node.Pos()).Offset:fileSet.Position(node.End()).Offset])
	}
	for _, declaration := range file.Decls {
		switch declaration := declaration.(type) {
		case *ast.FuncDecl:
			name := declaration.Name.Name
			if declaration.Recv != nil && len(declaration.Recv.List) == 1 {
				name = testSourceReceiver(declaration.Recv) + "." + name
			}
			declarations[name] = text(declaration)
		case *ast.GenDecl:
			for _, spec := range declaration.Specs {
				switch spec := spec.(type) {
				case *ast.TypeSpec:
					declarations[spec.Name.Name] = text(spec)
				case *ast.ValueSpec:
					for _, name := range spec.Names {
						declarations[name.Name] = text(spec)
					}
				}
			}
		}
	}
	// Import paths and struct tags are not literals a test reads.
	tags := map[*ast.BasicLit]bool{}
	ast.Inspect(file, func(node ast.Node) bool {
		switch node := node.(type) {
		case *ast.ImportSpec:
			return false
		case *ast.Field:
			tags[node.Tag] = true
		case *ast.BasicLit:
			if node.Kind == token.STRING && !tags[node] {
				if value, err := strconv.Unquote(node.Value); err == nil && testPinDistinctive(value) {
					literals[value]++
				}
			}
		}
		return true
	})
	return declarations, literals
}

// nameCount is the number of changed names over every package.
func (tokens testPinTokens) nameCount() int {
	count := 0
	for _, names := range tokens.names {
		count += len(names)
	}
	return count
}

// testPinDistinctive reports a changed literal worth matching: at least
// testPinMinLiteral bytes with four letters, and either a space or one of
// "_-:./" or at least 12 bytes, so a bare word such as "language" or a
// format such as " %s %s" is not compared.
func testPinDistinctive(value string) bool {
	letters := 0
	for _, r := range value {
		if 'a' <= r && r <= 'z' || 'A' <= r && r <= 'Z' {
			letters++
		}
	}
	return len(value) >= testPinMinLiteral && letters >= 4 && (strings.ContainsAny(value, " _-:./") || len(value) >= 12)
}

// testPinName is the bare name a declaration key is referenced by.
func testPinName(key string) string {
	if _, name, ok := strings.Cut(key, "."); ok {
		return name
	}
	return key
}

// testPins names, for each selected Go package, the test functions that read
// a changed token directly or through the top-level helpers of its test
// files. A package with no pinned test is left out.
func testPins(plan affected.Plan, graph *affected.Graph, root string, tokens testPinTokens) []testPinUnit {
	var units []testPinUnit
	for _, selection := range plan.Selected {
		if !strings.HasPrefix(selection.UnitID, "go:") {
			continue
		}
		unit, ok := graph.Unit(selection.UnitID)
		if !ok {
			continue
		}
		pinned := testPinnedFunctions(root, unit.Tests, tokens)
		if len(pinned) == 0 {
			continue
		}
		entry := testPinUnit{unit: selection.UnitID, tests: pinned}
		if len(pinned) > testPinMaxTests {
			entry.tests, entry.more = pinned[:testPinMaxTests], len(pinned)-testPinMaxTests
		}
		units = append(units, entry)
	}
	slices.SortFunc(units, func(a, b testPinUnit) int { return strings.Compare(a.unit, b.unit) })
	return units
}

// testPinnedFunctions parses the test files and returns, sorted, the Test,
// Fuzz and Example functions other than TestMain whose closure over the
// files' top-level functions reads a changed name or a literal that overlaps
// a changed one. A changed name is read as a bare identifier in its own
// directory, or as the selector of an import whose path ends in that
// directory.
func testPinnedFunctions(root string, files []string, tokens testPinTokens) []string {
	bodies := map[string]*ast.FuncDecl{}
	direct, calls := map[string]bool{}, map[string][]string{}
	type parsed struct {
		file    *ast.File
		bare    map[string]bool
		qualify map[string]map[string]bool
	}
	var parsedFiles []parsed
	for _, name := range files {
		source, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
		if err != nil {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), name, source, parser.SkipObjectResolution)
		if err != nil {
			continue
		}
		entry := parsed{file: file, bare: tokens.names[path.Dir(name)], qualify: map[string]map[string]bool{}}
		for _, spec := range file.Imports {
			imported, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				continue
			}
			local := path.Base(imported)
			if spec.Name != nil {
				local = spec.Name.Name
			}
			for directory, names := range tokens.names {
				if directory != "." && (imported == directory || strings.HasSuffix(imported, "/"+directory)) {
					entry.qualify[local] = names
				}
			}
		}
		parsedFiles = append(parsedFiles, entry)
		for _, declaration := range file.Decls {
			if function, ok := declaration.(*ast.FuncDecl); ok && function.Recv == nil && function.Body != nil {
				bodies[function.Name.Name] = function
			}
		}
	}
	for _, entry := range parsedFiles {
		for _, declaration := range entry.file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok || function.Recv != nil || function.Body == nil || bodies[function.Name.Name] != function {
				continue
			}
			name := function.Name.Name
			ast.Inspect(function.Body, func(node ast.Node) bool {
				switch node := node.(type) {
				case *ast.SelectorExpr:
					if qualifier, ok := node.X.(*ast.Ident); ok && entry.qualify[qualifier.Name][node.Sel.Name] {
						direct[name] = true
					}
				case *ast.Ident:
					if entry.bare[node.Name] {
						direct[name] = true
					}
					if _, ok := bodies[node.Name]; ok && node.Name != name {
						calls[name] = append(calls[name], node.Name)
					}
				case *ast.BasicLit:
					if node.Kind == token.STRING && testPinLiteral(node.Value, tokens.literals) {
						direct[name] = true
					}
				}
				return true
			})
		}
	}
	var pinned []string
	for name := range bodies {
		if !testPinEntry(name) || name == "TestMain" {
			continue
		}
		seen, queue, reads := map[string]bool{name: true}, []string{name}, false
		for len(queue) > 0 && !reads {
			current := queue[0]
			queue = queue[1:]
			reads = direct[current]
			for _, next := range calls[current] {
				if !seen[next] {
					seen[next] = true
					queue = append(queue, next)
				}
			}
		}
		if reads {
			pinned = append(pinned, name)
		}
	}
	slices.Sort(pinned)
	return pinned
}

// testPinEntry reports a function go test runs by name.
func testPinEntry(name string) bool {
	for _, prefix := range []string{"Test", "Fuzz", "Example"} {
		if rest, ok := strings.CutPrefix(name, prefix); ok && (rest == "" || !('a' <= rest[0] && rest[0] <= 'z')) {
			return true
		}
	}
	return false
}

// testPinLiteral reports a test literal that holds a changed literal or is
// held by one.
func testPinLiteral(quoted string, literals []string) bool {
	value, err := strconv.Unquote(quoted)
	if err != nil || len(value) < testPinMinLiteral {
		return false
	}
	for _, changed := range literals {
		if strings.Contains(value, changed) || len(value) >= testPinMinHeld && len(changed) <= testPinMaxHolder && strings.Contains(changed, value) {
			return true
		}
	}
	return false
}

// testPinsReport renders the pins: a header with the token counts and any
// bound that clipped them, then one `pins <unit> -run <pattern>` line per
// package with pinned tests, then the reminder that the plan still runs
// every selected test.
func testPinsReport(digest string, tokens testPinTokens, units []testPinUnit) string {
	var b strings.Builder
	fmt.Fprintf(&b, "corvint-test: PINS manifest=%s names=%d literals=%d packages=%d\n", digest, tokens.nameCount(), len(tokens.literals), len(units))
	if tokens.clipped != "" {
		fmt.Fprintf(&b, "  clipped: %s\n", tokens.clipped)
	}
	for _, unit := range units {
		fmt.Fprintf(&b, "pins %s -run '^(%s)$'", unit.unit, strings.Join(unit.tests, "|"))
		if unit.more != 0 {
			fmt.Fprintf(&b, " (+%d more)", unit.more)
		}
		b.WriteString("\n")
	}
	b.WriteString("  advisory: run the pinned tests first; the plan still selects every listed unit\n")
	return b.String()
}
