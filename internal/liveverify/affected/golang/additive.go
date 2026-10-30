package golang

import (
	"bytes"
	"context"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"io/fs"
	"maps"
	"os"
	"path"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/cem/gitrun"
	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// This file decides which changed Go packages only add top-level
// declarations, and which of their importers reference a new one
// (ADN-V0-001..009). Every rule is syntactic and conservative: anything it
// cannot show to be additive is a FALLBACK record with its reason, which the
// graph selects the ordinary way.

// Additive fallback reasons (ADN-V0-004).
const (
	AdditiveBaseUnreadable    = "BASE_UNREADABLE"
	AdditiveNotGoSource       = "NON_GO_OR_UNINDEXED_PATH"
	AdditiveDeletedFile       = "DELETED_FILE"
	AdditiveEmbeds            = "PACKAGE_EMBEDS"
	AdditiveMainPackage       = "MAIN_PACKAGE"
	AdditiveParseFailure      = "PARSE_FAILURE"
	AdditivePackageRenamed    = "PACKAGE_CLAUSE_CHANGED"
	AdditiveDirective         = "DIRECTIVE_CHANGED"
	AdditiveConstrainedFile   = "NEW_CONSTRAINED_FILE"
	AdditiveCgo               = "CGO"
	AdditiveImportChanged     = "IMPORT_REMOVED_OR_RENAMED"
	AdditiveImportAdded       = "IMPORT_NOT_ALREADY_IN_PACKAGE"
	AdditiveSpecialImport     = "DOT_OR_BLANK_IMPORT"
	AdditiveDeclarationEdited = "EXISTING_DECLARATION_CHANGED"
	AdditiveInit              = "INIT_FUNC"
	AdditiveBodyless          = "FUNC_WITHOUT_BODY"
	AdditiveExistingReceiver  = "METHOD_ON_EXISTING_TYPE"
	AdditiveVarInitializer    = "VAR_INITIALIZER_NOT_PROVED_SAFE"
	AdditiveNameDeclared      = "NAME_DECLARED_IN_ANOTHER_PACKAGE_FILE"
	AdditivePredeclared       = "SHADOWS_PREDECLARED_IDENTIFIER"
	AdditiveDotImporter       = "DOT_IMPORTED_BY_DEPENDENT"
	AdditiveNoDeclaration     = "NO_NEW_DECLARATION"
)

// AdditiveChanges compares each changed Go package at commit with the
// working tree under root and returns its additive-change record
// (ADN-V0-001). A Git failure makes every candidate a FALLBACK record.
func AdditiveChanges(ctx context.Context, root, commit string, graph *affected.Graph, dirty []string) []affected.GoAdditive {
	return AdditiveChangesAt(ctx, root, root, commit, graph, dirty)
}

// AdditiveChangesAt is AdditiveChanges with the base read from the repository
// under root and the head from the tree under head, as a review's
// materialized snapshot is (ADN-V0-009).
func AdditiveChangesAt(ctx context.Context, root, head, commit string, graph *affected.Graph, dirty []string) []affected.GoAdditive {
	candidates := additiveCandidates(graph, dirty)
	if len(candidates) == 0 {
		return nil
	}
	if commit == "" {
		return additiveAllFallback(candidates, AdditiveBaseUnreadable)
	}
	repository, err := gitauth.Open(root, gitrun.NewBudget(gitrun.DefaultOperations, gitrun.DefaultTotalBudget))
	if err != nil {
		return additiveAllFallback(candidates, AdditiveBaseUnreadable)
	}
	defer repository.BeginObjectSession()()
	base, err := repository.RevisionFS(ctx, commit, affected.MaxSourceBytes)
	if err != nil {
		return additiveAllFallback(candidates, AdditiveBaseUnreadable)
	}
	return AdditiveRecords(base, os.DirFS(head), graph, dirty)
}

// AdditiveRecords is AdditiveChanges over two source trees: base, the tree
// the change is measured from, and head, the observed tree the graph was
// built from.
func AdditiveRecords(base, head fs.FS, graph *affected.Graph, dirty []string) []affected.GoAdditive {
	var records []affected.GoAdditive
	for _, candidate := range additiveCandidates(graph, dirty) {
		records = append(records, additiveRecord(base, head, graph, candidate))
	}
	return records
}

type additiveCandidate struct {
	unit      affected.Unit
	directory string
	paths     []string
}

// additiveCandidates are the Go units with a changed non-test Go source of
// their own, each with every dirty path it owns or that lies in its
// directory.
func additiveCandidates(graph *affected.Graph, dirty []string) []additiveCandidate {
	normalized := affected.NormalizePaths(dirty)
	var candidates []additiveCandidate
	for _, id := range graph.UnitIDs() {
		unit, _ := graph.Unit(id)
		if !strings.HasPrefix(id, "go:") || len(unit.Sources) == 0 {
			continue
		}
		directory := path.Dir(unit.Sources[0])
		candidate := additiveCandidate{unit: unit, directory: directory}
		source := false
		for _, dirtyPath := range normalized {
			owner, _ := graph.OwnerOf(dirtyPath)
			if owner != id && path.Dir(dirtyPath) != directory {
				continue
			}
			candidate.paths = append(candidate.paths, dirtyPath)
			source = source || (owner == id && slices.Contains(unit.Sources, dirtyPath))
		}
		if source {
			candidates = append(candidates, candidate)
		}
	}
	return candidates
}

func additiveAllFallback(candidates []additiveCandidate, reason string) []affected.GoAdditive {
	records := make([]affected.GoAdditive, 0, len(candidates))
	for _, candidate := range candidates {
		records = append(records, additiveFallback(candidate, reason))
	}
	return records
}

func additiveFallback(candidate additiveCandidate, reason string) affected.GoAdditive {
	return affected.GoAdditive{Unit: candidate.unit.ID, State: affected.GoAdditiveFallback, Reason: reason, Paths: candidate.paths}
}

// additiveFile is one parsed Go file: its package name, directive lines,
// imports by path with their local name, and top-level declarations printed
// without comments, in source order.
type additiveFile struct {
	name         string
	directives   []string
	imports      map[string]string
	declarations []string
	nodes        []ast.Decl
	cgo          bool
}

func additiveRecord(base, head fs.FS, graph *affected.Graph, candidate additiveCandidate) affected.GoAdditive {
	unit := candidate.unit
	if unit.Embeds || len(unit.EmbedFiles) != 0 {
		return additiveFallback(candidate, AdditiveEmbeds)
	}
	var changed []string
	for _, dirtyPath := range candidate.paths {
		if !strings.HasSuffix(dirtyPath, ".go") || path.Dir(dirtyPath) != candidate.directory {
			return additiveFallback(candidate, AdditiveNotGoSource)
		}
		if _, err := fs.Stat(head, dirtyPath); err != nil {
			return additiveFallback(candidate, AdditiveDeletedFile)
		}
		switch {
		case slices.Contains(unit.Sources, dirtyPath):
			changed = append(changed, dirtyPath)
		case slices.Contains(unit.Tests, dirtyPath):
			// The package's own tests are selected whatever they hold.
		default:
			return additiveFallback(candidate, AdditiveNotGoSource)
		}
	}
	basePackage, reason := additiveBasePackage(base, candidate)
	if reason != "" {
		return additiveFallback(candidate, reason)
	}
	baseImports, packageName := basePackage.imports, basePackage.name
	if packageName == "main" {
		return additiveFallback(candidate, AdditiveMainPackage)
	}
	if basePackage.cgo {
		return additiveFallback(candidate, AdditiveCgo)
	}
	if cgo, err := additiveDirectoryCgo(head, candidate.directory); err != nil {
		return additiveFallback(candidate, AdditiveParseFailure)
	} else if cgo {
		return additiveFallback(candidate, AdditiveCgo)
	}
	newTypes := map[string]bool{}
	var added []additiveDeclaration
	for _, file := range changed {
		after, err := additiveParse(head, file)
		if err != nil {
			return additiveFallback(candidate, AdditiveParseFailure)
		}
		before, existed, err := additiveParseBase(base, file)
		if err != nil {
			return additiveFallback(candidate, AdditiveParseFailure)
		}
		if reason := additiveCompareFile(before, after, existed, file, packageName, baseImports); reason != "" {
			return additiveFallback(candidate, reason)
		}
		fresh := additiveNewDeclarations(before.declarations, after)
		if fresh == nil {
			return additiveFallback(candidate, AdditiveDeclarationEdited)
		}
		for _, declaration := range fresh {
			if general, ok := declaration.(*ast.GenDecl); ok && general.Tok == token.TYPE {
				for _, spec := range general.Specs {
					if typeSpec := spec.(*ast.TypeSpec); !typeSpec.Assign.IsValid() {
						newTypes[typeSpec.Name.Name] = true
					}
				}
			}
		}
		qualifiers := additiveQualifiers(after, basePackage.declared)
		for _, declaration := range fresh {
			added = append(added, additiveDeclaration{node: declaration, qualifiers: qualifiers})
		}
	}
	names, reason := additiveDeclarationNames(added, newTypes, basePackage)
	if reason != "" {
		return additiveFallback(candidate, reason)
	}
	if len(names) == 0 {
		return additiveFallback(candidate, AdditiveNoDeclaration)
	}
	record := affected.GoAdditive{Unit: unit.ID, State: affected.GoAdditiveNarrowed, Paths: candidate.paths, Declarations: names}
	exported := map[string]bool{}
	for _, name := range names {
		if !strings.Contains(name, ".") && token.IsExported(name) {
			exported[name] = true
		}
	}
	importPath := strings.TrimPrefix(unit.ID, "go:")
	for _, dependent := range graph.Dependents(unit.ID) {
		references, dotted := additiveReferences(head, graph, dependent, importPath, packageName, exported)
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

// additivePackage is the package at base: the import paths its unconstrained
// non-test files already hold, its package name, every top-level name any of
// its files declares under any build constraint (test files of the package
// included), the constants among them, and whether any file in
// its directory imports "C".
type additivePackage struct {
	imports  map[string]bool
	name     string
	declared map[string]bool
	consts   map[string]bool
	cgo      bool
}

// additiveBasePackage reads the package at base. A package absent at base,
// or one whose base files cannot be read or parsed, is no candidate. Every
// file is parsed, constrained ones too, so a name an unobserved build variant
// declares is known (ADN-V0-004).
func additiveBasePackage(base fs.FS, candidate additiveCandidate) (additivePackage, string) {
	entries, err := fs.ReadDir(base, candidate.directory)
	if err != nil {
		return additivePackage{}, AdditiveBaseUnreadable
	}
	result := additivePackage{imports: map[string]bool{}, declared: map[string]bool{}, consts: map[string]bool{}}
	type fileNames struct {
		pkg    string
		names  []string
		consts []string
	}
	var files []fileNames
	for _, entry := range entries {
		file := path.Join(candidate.directory, entry.Name())
		if entry.IsDir() || !strings.HasSuffix(file, ".go") {
			continue
		}
		body, err := fs.ReadFile(base, file)
		if err != nil {
			return additivePackage{}, AdditiveBaseUnreadable
		}
		parsed, err := parser.ParseFile(token.NewFileSet(), file, body, parser.ParseComments|parser.SkipObjectResolution)
		if err != nil {
			return additivePackage{}, AdditiveParseFailure
		}
		names, consts := additiveTopLevelNames(parsed)
		files = append(files, fileNames{pkg: parsed.Name.Name, names: names, consts: consts})
		for _, spec := range parsed.Imports {
			if quoted, err := strconv.Unquote(spec.Path.Value); err != nil || quoted == "C" {
				result.cgo = true
			}
		}
		if strings.HasSuffix(file, "_test.go") || hasPlatformSuffix(file) || hasBuildConstraint(body) {
			continue
		}
		if result.name == "" {
			result.name = parsed.Name.Name
		}
		for _, spec := range parsed.Imports {
			if quoted, err := strconv.Unquote(spec.Path.Value); err == nil && spec.Name == nil {
				result.imports[quoted] = true
			}
		}
	}
	if result.name == "" {
		// No unconstrained base file names the package: a new package, or
		// one the base cannot show the import set of.
		return additivePackage{}, AdditiveBaseUnreadable
	}
	for _, file := range files {
		if file.pkg != result.name {
			continue
		}
		for _, name := range file.names {
			result.declared[name] = true
		}
		for _, name := range file.consts {
			result.consts[name] = true
		}
	}
	return result, ""
}

// additiveTopLevelNames returns the package-block names a file declares, a
// method as Type.Method, and the constants among them.
func additiveTopLevelNames(parsed *ast.File) (names, consts []string) {
	for _, declaration := range parsed.Decls {
		switch typed := declaration.(type) {
		case *ast.FuncDecl:
			if typed.Recv != nil {
				names = append(names, additiveReceiver(typed.Recv)+"."+typed.Name.Name)
				continue
			}
			names = append(names, typed.Name.Name)
		case *ast.GenDecl:
			for _, spec := range typed.Specs {
				switch typedSpec := spec.(type) {
				case *ast.TypeSpec:
					names = append(names, typedSpec.Name.Name)
				case *ast.ValueSpec:
					for _, name := range typedSpec.Names {
						names = append(names, name.Name)
						if typed.Tok == token.CONST {
							consts = append(consts, name.Name)
						}
					}
				}
			}
		}
	}
	return names, consts
}

// additiveDirectoryCgo reports whether any Go file in directory, under any
// build constraint and test files included, imports "C" (ADN-V0-004). A file
// that cannot be read or parsed is an error.
func additiveDirectoryCgo(source fs.FS, directory string) (bool, error) {
	entries, err := fs.ReadDir(source, directory)
	if err != nil {
		return false, err
	}
	for _, entry := range entries {
		file := path.Join(directory, entry.Name())
		if entry.IsDir() || !strings.HasSuffix(file, ".go") {
			continue
		}
		body, err := fs.ReadFile(source, file)
		if err != nil {
			return false, err
		}
		parsed, err := parser.ParseFile(token.NewFileSet(), file, body, parser.ImportsOnly|parser.SkipObjectResolution)
		if err != nil {
			return false, err
		}
		for _, spec := range parsed.Imports {
			if quoted, err := strconv.Unquote(spec.Path.Value); err != nil || quoted == "C" {
				return true, nil
			}
		}
	}
	return false, nil
}

// additiveQualifiers are the names a file's initializers may use as package
// qualifiers: each import's explicit name, or the last element of its path
// when it has none, less any name the package declares. A name that is not
// the imported package's real name only makes a selector on it unproved.
func additiveQualifiers(file additiveFile, declared map[string]bool) map[string]bool {
	qualifiers := map[string]bool{}
	for key, local := range file.imports {
		if local == "" {
			local = path.Base(strings.SplitN(key, "\x00", 2)[0])
		}
		if local != "_" && local != "." && !declared[local] {
			qualifiers[local] = true
		}
	}
	return qualifiers
}

func additiveParse(source fs.FS, file string) (additiveFile, error) {
	body, err := fs.ReadFile(source, file)
	if err != nil {
		return additiveFile{}, err
	}
	return additiveParseBody(file, body)
}

func additiveParseBase(base fs.FS, file string) (additiveFile, bool, error) {
	body, err := fs.ReadFile(base, file)
	if err != nil {
		if _, statErr := fs.Stat(base, file); statErr != nil {
			return additiveFile{imports: map[string]string{}}, false, nil
		}
		return additiveFile{}, false, err
	}
	parsed, err := additiveParseBody(file, body)
	return parsed, true, err
}

func additiveParseBody(file string, body []byte) (additiveFile, error) {
	fileSet := token.NewFileSet()
	parsed, err := parser.ParseFile(fileSet, file, body, parser.ParseComments|parser.SkipObjectResolution)
	if err != nil {
		return additiveFile{}, err
	}
	result := additiveFile{name: parsed.Name.Name, imports: map[string]string{}}
	for _, group := range parsed.Comments {
		for _, comment := range group.List {
			if additiveDirective(comment.Text) {
				result.directives = append(result.directives, comment.Text)
			}
		}
	}
	sort.Strings(result.directives)
	for _, spec := range parsed.Imports {
		quoted, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			return additiveFile{}, err
		}
		local := ""
		if spec.Name != nil {
			local = spec.Name.Name
		}
		result.imports[quoted+"\x00"+local] = local
		result.cgo = result.cgo || quoted == "C"
	}
	for _, declaration := range parsed.Decls {
		if general, ok := declaration.(*ast.GenDecl); ok && general.Tok == token.IMPORT {
			continue
		}
		printed, err := additivePrint(declaration)
		if err != nil {
			return additiveFile{}, err
		}
		result.declarations = append(result.declarations, printed)
		result.nodes = append(result.nodes, declaration)
	}
	return result, nil
}

// additiveDirective reports a comment the toolchain reads: a //go: or
// //line directive, a +build line, a cgo //export, or any //go:linkname.
func additiveDirective(text string) bool {
	for _, prefix := range []string{"//go:", "//line ", "// +build", "//+build", "//export ", "/*line "} {
		if strings.HasPrefix(text, prefix) {
			return true
		}
	}
	return false
}

// additivePrint prints a declaration without its comments, so a doc-comment
// edit is no change while any other edit is.
func additivePrint(declaration ast.Decl) (string, error) {
	switch typed := declaration.(type) {
	case *ast.FuncDecl:
		copied := *typed
		copied.Doc = nil
		declaration = &copied
	case *ast.GenDecl:
		copied := *typed
		copied.Doc = nil
		declaration = &copied
	}
	var buffer bytes.Buffer
	if err := (&printer.Config{Mode: printer.RawFormat}).Fprint(&buffer, token.NewFileSet(), declaration); err != nil {
		return "", err
	}
	return buffer.String(), nil
}

func additiveCompareFile(before, after additiveFile, existed bool, file, packageName string, baseImports map[string]bool) string {
	if after.name != packageName || (existed && before.name != after.name) {
		return AdditivePackageRenamed
	}
	if !slices.Equal(before.directives, after.directives) {
		return AdditiveDirective
	}
	if !existed && hasPlatformSuffix(file) {
		return AdditiveConstrainedFile
	}
	if after.cgo || before.cgo {
		return AdditiveCgo
	}
	for key := range before.imports {
		if _, kept := after.imports[key]; !kept {
			return AdditiveImportChanged
		}
	}
	for key, local := range after.imports {
		if _, held := before.imports[key]; held {
			continue
		}
		if local == "." || local == "_" {
			return AdditiveSpecialImport
		}
		if local != "" || !baseImports[strings.SplitN(key, "\x00", 2)[0]] {
			return AdditiveImportAdded
		}
	}
	return ""
}

// additiveNewDeclarations returns the head declarations that are not the
// base declarations kept in order: the base list must be a subsequence of the
// head list, so no existing declaration is edited, removed or reordered. It is
// nil when that does not hold, and empty when nothing is new.
func additiveNewDeclarations(before []string, after additiveFile) []ast.Decl {
	fresh := []ast.Decl{}
	next := 0
	for index, printed := range after.declarations {
		if next < len(before) && before[next] == printed {
			next++
			continue
		}
		fresh = append(fresh, after.nodes[index])
	}
	if next != len(before) {
		return nil
	}
	return fresh
}

// additiveDeclaration is one new top-level declaration with the package
// qualifiers of the file that holds it.
type additiveDeclaration struct {
	node       ast.Decl
	qualifiers map[string]bool
}

// additiveDeclarationNames checks each new declaration against the additive
// rules and returns the new identifiers, a method as Type.Method, sorted. A
// new name must be declared by no base file of the package under any build
// constraint and only once among the new declarations; a method's receiver
// must be a type the change declares (ADN-V0-004).
func additiveDeclarationNames(added []additiveDeclaration, newTypes map[string]bool, pkg additivePackage) ([]string, string) {
	var names []string
	seen := map[string]bool{}
	packageNames := maps.Clone(pkg.declared)
	consts := maps.Clone(pkg.consts)
	declare := func(name string) string {
		if additivePredeclared[name] {
			return AdditivePredeclared
		}
		if name == "_" {
			return ""
		}
		if pkg.declared[name] || seen[name] {
			return AdditiveNameDeclared
		}
		seen[name] = true
		packageNames[name] = true
		names = append(names, name)
		return ""
	}
	var values []additiveDeclaration
	for _, declaration := range added {
		switch typed := declaration.node.(type) {
		case *ast.FuncDecl:
			if typed.Body == nil {
				return nil, AdditiveBodyless
			}
			if typed.Recv == nil {
				if typed.Name.Name == "init" {
					return nil, AdditiveInit
				}
				if reason := declare(typed.Name.Name); reason != "" {
					return nil, reason
				}
				continue
			}
			receiver := additiveReceiver(typed.Recv)
			if !newTypes[receiver] || pkg.declared[receiver] {
				return nil, AdditiveExistingReceiver
			}
			if typed.Name.Name == "_" {
				continue
			}
			method := receiver + "." + typed.Name.Name
			if seen[method] {
				return nil, AdditiveNameDeclared
			}
			seen[method] = true
			names = append(names, method)
		case *ast.GenDecl:
			for _, spec := range typed.Specs {
				switch typedSpec := spec.(type) {
				case *ast.TypeSpec:
					if reason := declare(typedSpec.Name.Name); reason != "" {
						return nil, reason
					}
				case *ast.ValueSpec:
					for _, name := range typedSpec.Names {
						if reason := declare(name.Name); reason != "" {
							return nil, reason
						}
						if typed.Tok == token.CONST {
							consts[name.Name] = true
						}
					}
				}
			}
			if typed.Tok == token.VAR {
				values = append(values, declaration)
			}
		default:
			return nil, AdditiveDeclarationEdited
		}
	}
	for _, declaration := range values {
		scope := additiveInitializerScope{qualifiers: declaration.qualifiers, packageNames: packageNames, consts: consts}
		for _, spec := range declaration.node.(*ast.GenDecl).Specs {
			for _, value := range spec.(*ast.ValueSpec).Values {
				if !scope.safe(value) {
					return nil, AdditiveVarInitializer
				}
			}
		}
	}
	sort.Strings(names)
	return names, ""
}

// additiveInitializerScope is what a new package variable's initializer may
// name: the file's package qualifiers, the package's top-level names, and the
// constants among them.
type additiveInitializerScope struct {
	qualifiers   map[string]bool
	packageNames map[string]bool
	consts       map[string]bool
}

// safe reports an initializer proved not to call, receive, panic or block at
// init: a literal, a name, a function literal (its body runs only when
// called), a qualified name, a composite literal of safe elements with
// constant or field keys, the address of a name or composite literal, unary
// + - ! ^, and binary + - * & | ^ &^ && || < <= > >= over safe operands. Any
// other form, among them a call or conversion, a receive, division, modulo,
// a shift, ==, !=, an index, a slice, a dereference and a type assertion, is
// unproved and makes the record a FALLBACK (ADN-V0-004).
func (scope additiveInitializerScope) safe(expression ast.Expr) bool {
	switch typed := expression.(type) {
	case *ast.BasicLit, *ast.Ident, *ast.FuncLit:
		return true
	case *ast.ParenExpr:
		return scope.safe(typed.X)
	case *ast.SelectorExpr:
		name, ok := typed.X.(*ast.Ident)
		return ok && scope.qualifiers[name.Name] && !scope.packageNames[name.Name]
	case *ast.UnaryExpr:
		switch typed.Op {
		case token.ADD, token.SUB, token.NOT, token.XOR:
			return scope.safe(typed.X)
		case token.AND:
			switch typed.X.(type) {
			case *ast.CompositeLit, *ast.Ident:
				return scope.safe(typed.X)
			}
		}
		return false
	case *ast.BinaryExpr:
		switch typed.Op {
		case token.ADD, token.SUB, token.MUL, token.AND, token.OR, token.XOR, token.AND_NOT, token.LAND, token.LOR, token.LSS, token.LEQ, token.GTR, token.GEQ:
			return scope.safe(typed.X) && scope.safe(typed.Y)
		}
		return false
	case *ast.CompositeLit:
		for _, element := range typed.Elts {
			if pair, ok := element.(*ast.KeyValueExpr); ok {
				if !scope.safeKey(pair.Key) || !scope.safe(pair.Value) {
					return false
				}
				continue
			}
			if !scope.safe(element) {
				return false
			}
		}
		return true
	}
	return false
}

// safeKey reports a composite-literal key that cannot panic when a map hashes
// it: a basic literal, a package constant, or a name that is no package-level
// declaration, so a struct field or a predeclared constant. A package variable
// or function as a key of an interface-keyed map can hold an unhashable value.
func (scope additiveInitializerScope) safeKey(key ast.Expr) bool {
	switch typed := key.(type) {
	case *ast.BasicLit:
		return true
	case *ast.Ident:
		return scope.consts[typed.Name] || !scope.packageNames[typed.Name]
	}
	return false
}

// additiveReceiver is the base type name of a method receiver.
func additiveReceiver(list *ast.FieldList) string {
	if list == nil || len(list.List) == 0 {
		return ""
	}
	expression := list.List[0].Type
	for {
		switch typed := expression.(type) {
		case *ast.StarExpr:
			expression = typed.X
		case *ast.ParenExpr:
			expression = typed.X
		case *ast.IndexExpr:
			expression = typed.X
		case *ast.IndexListExpr:
			expression = typed.X
		case *ast.Ident:
			return typed.Name
		default:
			return ""
		}
	}
}

// additivePredeclared are the universe-scope names a new package-level
// declaration would shadow for every file of the package.
var additivePredeclared = map[string]bool{
	"any": true, "append": true, "bool": true, "byte": true, "cap": true, "clear": true, "close": true,
	"comparable": true, "complex": true, "complex128": true, "complex64": true, "copy": true, "delete": true,
	"error": true, "false": true, "float32": true, "float64": true, "imag": true, "int": true, "int16": true,
	"int32": true, "int64": true, "int8": true, "iota": true, "len": true, "make": true, "max": true, "min": true,
	"new": true, "nil": true, "panic": true, "print": true, "println": true, "real": true, "recover": true,
	"rune": true, "string": true, "true": true, "uint": true, "uint16": true, "uint32": true, "uint64": true,
	"uint8": true, "uintptr": true,
}

// additiveReferences reports whether a dependent references one of the new
// exported identifiers, over-approximated as any selector naming one on a
// local name the package is imported under, and whether any file dot-imports
// the package. Every Go file in the dependent's directory is parsed, under
// any build constraint and test files included, and each import path is
// unquoted, so a raw or escaped spelling matches (ADN-V0-006). The dependent
// is taken to reference the package, and so is retained, when it is not a Go
// unit, when a file cannot be read or parsed, when a file blank-imports the
// package or holds a //go:linkname directive, when its directory holds
// assembly, or when none of its files imports the package.
func additiveReferences(head fs.FS, graph *affected.Graph, dependent, importPath, packageName string, exported map[string]bool) (references, dotted bool) {
	unit, known := graph.Unit(dependent)
	if !known || !strings.HasPrefix(dependent, "go:") {
		return true, false
	}
	owned := append(append([]string(nil), unit.Sources...), unit.Tests...)
	if len(owned) == 0 {
		return true, false
	}
	directory := path.Dir(owned[0])
	for _, file := range owned {
		if path.Dir(file) != directory {
			return true, false
		}
	}
	entries, err := fs.ReadDir(head, directory)
	if err != nil {
		return true, false
	}
	importing := false
	for _, entry := range entries {
		file := path.Join(directory, entry.Name())
		if entry.IsDir() {
			continue
		}
		switch path.Ext(file) {
		case ".s", ".S", ".sx":
			return true, false
		case ".go":
		default:
			continue
		}
		body, err := fs.ReadFile(head, file)
		if err != nil {
			return true, false
		}
		parsed, err := parser.ParseFile(token.NewFileSet(), file, body, parser.ParseComments|parser.SkipObjectResolution)
		if err != nil {
			return true, false
		}
		for _, group := range parsed.Comments {
			for _, comment := range group.List {
				if strings.HasPrefix(comment.Text, "//go:linkname") {
					return true, false
				}
			}
		}
		locals := map[string]bool{}
		for _, spec := range parsed.Imports {
			imported, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				return true, false
			}
			if imported != importPath {
				continue
			}
			importing = true
			local := packageName
			if spec.Name != nil {
				local = spec.Name.Name
			}
			switch local {
			case ".":
				return true, true
			case "_":
				return true, false
			}
			locals[local] = true
		}
		if len(locals) == 0 {
			continue
		}
		ast.Inspect(parsed, func(node ast.Node) bool {
			if selector, ok := node.(*ast.SelectorExpr); ok {
				if name, ok := selector.X.(*ast.Ident); ok && locals[name.Name] && exported[selector.Sel.Name] {
					references = true
				}
			}
			return !references
		})
		if references {
			return true, false
		}
	}
	return !importing, false
}
