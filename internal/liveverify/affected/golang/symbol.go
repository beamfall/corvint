package golang

import (
	"bytes"
	"go/ast"
	"go/scanner"
	"go/token"
	"io/fs"
	"maps"
	"path"
	"slices"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// This file is the symbol-level half of the Go change records (ADN-V0-660 to
// ADN-V0-664): a package whose change edits, removes or reorders existing
// top-level declarations carries the change only to the importers that
// reference a declaration the change can reach. Reachability is a syntactic
// over-approximation, closed within the package: a declaration reaches the
// change when it mentions, anywhere in its text, the name of a changed
// declaration or of one that already reaches it, and a method that reaches
// it makes its receiver type reach it. A value of a type can only leave the
// package through a declaration that mentions the type, so an importer that
// names no reaching declaration cannot run the changed code in its test
// process unless the package's initialization does, which falls back.

// Symbol-level fallback reasons (ADN-V0-662).
const (
	SymbolAlwaysRun    = "CHANGE_REACHES_PACKAGE_INITIALIZATION"
	SymbolUnmodelled   = "LINKNAME_ASSEMBLY_OR_BODYLESS_FUNC"
	SymbolUnmodelledIn = "LINKNAME_OR_ASSEMBLY_IN_DEPENDENT"
)

// GoSymbolMode is the GoAdditive mode of a symbol-level record (ADN-V0-660).
const GoSymbolMode = "SYMBOL"

// symbolPair is one changed file at base and head.
type symbolPair struct {
	file          string
	before, after additiveFile
}

// symbolDeclaration is one top-level declaration of the head package: the
// file and key it is compared under, the names it makes reach the change
// when it does (a method its receiver type), the names it mentions, and the
// qualifiers of its file for the initializer check.
type symbolDeclaration struct {
	file       string
	key        string
	names      []string
	label      []string
	mentions   map[string]bool
	node       ast.Decl
	qualifiers map[string]bool
}

// symbolKey is the comparison key of a declaration within one file and the
// names it carries: a function its name, a method Type.Method with its
// receiver type, and a general declaration its token and every name it
// declares. Repeated keys (init, blank names) are numbered in source order.
func symbolKey(declaration ast.Decl) (key string, names, label []string) {
	switch typed := declaration.(type) {
	case *ast.FuncDecl:
		if typed.Recv != nil {
			receiver := additiveReceiver(typed.Recv)
			return "method " + receiver + "." + typed.Name.Name, []string{receiver}, []string{receiver + "." + typed.Name.Name}
		}
		return "func " + typed.Name.Name, []string{typed.Name.Name}, []string{typed.Name.Name}
	case *ast.GenDecl:
		for _, spec := range typed.Specs {
			switch typedSpec := spec.(type) {
			case *ast.TypeSpec:
				names = append(names, typedSpec.Name.Name)
			case *ast.ValueSpec:
				for _, name := range typedSpec.Names {
					names = append(names, name.Name)
				}
			}
		}
		return typed.Tok.String() + " " + strings.Join(names, ","), names, names
	}
	return "other", nil, nil
}

// symbolFileKeys maps each declaration key of a file to its printed text and
// its names.
func symbolFileKeys(file additiveFile) (map[string]string, map[string][]string) {
	printed, names := map[string]string{}, map[string][]string{}
	seen := map[string]int{}
	for index, node := range file.nodes {
		key, _, label := symbolKey(node)
		seen[key]++
		if seen[key] > 1 {
			key += "#" + strings.Repeat("+", seen[key]-1)
		}
		printed[key] = file.declarations[index]
		names[key] = label
	}
	return printed, names
}

// symbolChanged returns, per changed file, the keys whose text differs or
// that exist on one side only, and the names they carry at either side.
func symbolChanged(pairs []symbolPair) (map[string]map[string]bool, []string) {
	keys := map[string]map[string]bool{}
	var names []string
	for _, pair := range pairs {
		beforeText, beforeNames := symbolFileKeys(pair.before)
		afterText, afterNames := symbolFileKeys(pair.after)
		changed := map[string]bool{}
		for key, text := range beforeText {
			if after, kept := afterText[key]; !kept || after != text {
				changed[key] = true
				names = append(names, beforeNames[key]...)
			}
		}
		for key, text := range afterText {
			if before, kept := beforeText[key]; !kept || before != text {
				changed[key] = true
				names = append(names, afterNames[key]...)
			}
		}
		keys[pair.file] = changed
	}
	sort.Strings(names)
	return keys, slices.Compact(names)
}

// symbolPackage parses every non-test Go file of the package directory at
// head, under any build constraint, into its declarations. It reports
// SymbolUnmodelled when the directory holds assembly, a file holds
// //go:linkname, or a function has no body, and a parse failure as such.
func symbolPackage(head fs.FS, directory, packageName string, declared map[string]bool) ([]symbolDeclaration, string) {
	entries, err := fs.ReadDir(head, directory)
	if err != nil {
		return nil, AdditiveParseFailure
	}
	var declarations []symbolDeclaration
	for _, entry := range entries {
		file := path.Join(directory, entry.Name())
		if entry.IsDir() {
			continue
		}
		switch path.Ext(file) {
		case ".s", ".S", ".sx", ".syso":
			return nil, SymbolUnmodelled
		case ".go":
		default:
			continue
		}
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		body, err := fs.ReadFile(head, file)
		if err != nil {
			return nil, AdditiveParseFailure
		}
		if len(symbolLinknames(body)) != 0 {
			return nil, SymbolUnmodelled
		}
		parsed, err := additiveParseBody(file, body)
		if err != nil {
			return nil, AdditiveParseFailure
		}
		if parsed.name != packageName {
			continue
		}
		qualifiers := additiveQualifiers(parsed, declared)
		seen := map[string]int{}
		for _, node := range parsed.nodes {
			if function, ok := node.(*ast.FuncDecl); ok && function.Body == nil {
				return nil, SymbolUnmodelled
			}
			key, names, label := symbolKey(node)
			seen[key]++
			if seen[key] > 1 {
				key += "#" + strings.Repeat("+", seen[key]-1)
			}
			mentions := map[string]bool{}
			ast.Inspect(node, func(n ast.Node) bool {
				if ident, ok := n.(*ast.Ident); ok {
					mentions[ident.Name] = true
				}
				return true
			})
			declarations = append(declarations, symbolDeclaration{file: file, key: key, names: names, label: label, mentions: mentions, node: node, qualifiers: qualifiers})
		}
	}
	return declarations, ""
}

// symbolClosure marks the declarations that reach the change: each changed
// one, and each that mentions a name of one already marked. It returns the
// marks and the reaching names (the changed names included).
func symbolClosure(declarations []symbolDeclaration, changedKeys map[string]map[string]bool, changedNames []string) ([]bool, map[string]bool) {
	reached := make([]bool, len(declarations))
	names := map[string]bool{}
	byMention := map[string][]int{}
	for index, declaration := range declarations {
		for name := range declaration.mentions {
			byMention[name] = append(byMention[name], index)
		}
	}
	var queue []string
	add := func(name string) {
		if name != "_" && name != "" && !names[name] {
			names[name] = true
			queue = append(queue, name)
		}
	}
	mark := func(index int) {
		if reached[index] {
			return
		}
		reached[index] = true
		for _, name := range declarations[index].names {
			add(name)
		}
	}
	for _, name := range changedNames {
		if !strings.Contains(name, ".") {
			add(name)
		} else {
			add(strings.SplitN(name, ".", 2)[0])
		}
	}
	for index, declaration := range declarations {
		if changedKeys[declaration.file][declaration.key] {
			mark(index)
		}
	}
	for len(queue) != 0 {
		name := queue[0]
		queue = queue[1:]
		for _, index := range byMention[name] {
			mark(index)
		}
	}
	return reached, names
}

// symbolAlwaysRun names the first reaching declaration that runs at package
// initialization: an init function, or a package variable whose initializer
// is not proved safe at init by the ADN-V0-004 rule.
func symbolAlwaysRun(declarations []symbolDeclaration, reached []bool, pkg additivePackage) string {
	for index, declaration := range declarations {
		if !reached[index] {
			continue
		}
		switch typed := declaration.node.(type) {
		case *ast.FuncDecl:
			if typed.Recv == nil && typed.Name.Name == "init" {
				return "init in " + declaration.file
			}
		case *ast.GenDecl:
			if typed.Tok != token.VAR {
				continue
			}
			scope := additiveInitializerScope{qualifiers: declaration.qualifiers, packageNames: pkg.declared, consts: pkg.consts}
			for _, spec := range typed.Specs {
				for _, value := range spec.(*ast.ValueSpec).Values {
					if !scope.safe(value) {
						return "var " + strings.Join(declaration.names, ",") + " in " + declaration.file
					}
				}
			}
		}
	}
	return ""
}

// symbolUnmodelledDependent names the first transitive dependent, reached
// through the import graph from unit (test imports included), whose
// directory holds assembly or a //go:linkname directive that pulls a symbol
// of unit (or whose target cannot be read): such a package can call the
// changed code under a name no selector shows. A directive that pulls
// another package's symbol, such as runtime.nanotime, cannot. Every test binary
// that links the package is the test binary of a transitive dependent.
func symbolUnmodelledDependent(head fs.FS, graph *affected.Graph, unit string) string {
	importPath := strings.TrimPrefix(unit, "go:")
	seen := map[string]bool{unit: true}
	queue := graph.Dependents(unit)
	for len(queue) != 0 {
		id := queue[0]
		queue = queue[1:]
		if seen[id] {
			continue
		}
		seen[id] = true
		queue = append(queue, graph.Dependents(id)...)
		if !strings.HasPrefix(id, "go:") {
			continue
		}
		dependent, _ := graph.Unit(id)
		owned := append(append([]string(nil), dependent.Sources...), dependent.Tests...)
		directories := map[string]bool{}
		for _, file := range owned {
			directories[path.Dir(file)] = true
		}
		for _, directory := range slices.Sorted(maps.Keys(directories)) {
			entries, err := fs.ReadDir(head, directory)
			if err != nil {
				return id
			}
			for _, entry := range entries {
				file := path.Join(directory, entry.Name())
				switch path.Ext(file) {
				case ".s", ".S", ".sx", ".syso":
					return id
				case ".go":
					body, err := fs.ReadFile(head, file)
					if err != nil {
						return id
					}
					for _, target := range symbolLinknames(body) {
						if target == "" || strings.HasPrefix(target, importPath+".") {
							return id
						}
					}
				}
			}
		}
	}
	return ""
}

// symbolRecord is the record of a package whose change edits an existing
// declaration (ADN-V0-660): NARROWED in mode SYMBOL with the changed
// declarations, the exported names that reach them, and the importers that
// reference one, or FALLBACK with the reason.
func symbolRecord(head fs.FS, graph *affected.Graph, candidate additiveCandidate, pkg additivePackage, pairs []symbolPair) affected.GoAdditive {
	changedKeys, changedNames := symbolChanged(pairs)
	if len(changedNames) == 0 {
		// Only the order changed; package variable initialization follows
		// declaration order, so it is no narrowing.
		return additiveFallback(candidate, AdditiveDeclarationEdited)
	}
	declarations, reason := symbolPackage(head, candidate.directory, pkg.name, pkg.declared)
	if reason != "" {
		return additiveFallback(candidate, reason)
	}
	reached, names := symbolClosure(declarations, changedKeys, changedNames)
	if always := symbolAlwaysRun(declarations, reached, pkg); always != "" {
		return additiveFallback(candidate, SymbolAlwaysRun+": "+always)
	}
	if dependent := symbolUnmodelledDependent(head, graph, candidate.unit.ID); dependent != "" {
		return additiveFallback(candidate, SymbolUnmodelledIn+": "+dependent)
	}
	exported := map[string]bool{}
	for name := range names {
		if token.IsExported(name) {
			exported[name] = true
		}
	}
	unit := candidate.unit
	record := affected.GoAdditive{
		Unit:         unit.ID,
		State:        affected.GoAdditiveNarrowed,
		Mode:         GoSymbolMode,
		Paths:        candidate.paths,
		Declarations: symbolLabels(pairs, changedKeys),
		Exposed:      slices.Sorted(maps.Keys(exported)),
	}
	importPath := strings.TrimPrefix(unit.ID, "go:")
	for _, dependent := range graph.Dependents(unit.ID) {
		references, dotted := additiveReferences(head, graph, dependent, importPath, pkg.name, exported)
		if dotted {
			return additiveFallback(candidate, AdditiveDotImporter+": "+dependent)
		}
		if references {
			record.Referencing = append(record.Referencing, dependent)
		} else {
			record.Unreferenced = append(record.Unreferenced, dependent)
		}
	}
	return record
}

// symbolLabels are the changed declarations' names, a method as
// Type.Method, sorted and unique.
func symbolLabels(pairs []symbolPair, changedKeys map[string]map[string]bool) []string {
	var labels []string
	for _, pair := range pairs {
		for _, file := range []additiveFile{pair.before, pair.after} {
			seen := map[string]int{}
			for _, node := range file.nodes {
				key, _, label := symbolKey(node)
				seen[key]++
				if seen[key] > 1 {
					key += "#" + strings.Repeat("+", seen[key]-1)
				}
				if changedKeys[pair.file][key] {
					labels = append(labels, label...)
				}
			}
		}
	}
	sort.Strings(labels)
	labels = slices.Compact(labels)
	return slices.DeleteFunc(labels, func(label string) bool { return label == "_" })
}

// symbolLinknames returns the target of each //go:linkname directive in a Go
// source, read with the scanner so a string literal holding the text is no
// directive; a directive without a target, or a source the scanner rejects,
// yields an empty target, which callers treat as unbounded.
func symbolLinknames(body []byte) []string {
	if !bytes.Contains(body, []byte("go:linkname")) {
		return nil
	}
	var targets []string
	failed := false
	var lexer scanner.Scanner
	files := token.NewFileSet()
	lexer.Init(files.AddFile("", -1, len(body)), body, func(token.Position, string) { failed = true }, scanner.ScanComments)
	for {
		_, kind, literal := lexer.Scan()
		if kind == token.EOF {
			break
		}
		if kind != token.COMMENT || !strings.HasPrefix(literal, "//go:linkname") {
			continue
		}
		fields := strings.Fields(strings.TrimPrefix(literal, "//go:linkname"))
		if len(fields) < 2 {
			targets = append(targets, "")
			continue
		}
		targets = append(targets, fields[1])
	}
	if failed {
		targets = append(targets, "")
	}
	return targets
}
