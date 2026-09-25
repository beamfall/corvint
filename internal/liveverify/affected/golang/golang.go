// Package golang is the Go implementation of the affected-selection language
// seam.
//
// A unit is one directory of Go files, which is exactly one Go package. Import
// edges are read from source text with the standard parser in imports-only
// mode; the Go toolchain is never executed, so the graph can be rebuilt on a
// dirty worktree with no build cache and no module download.
//
// The observed modules are the root module, or every module a root go.work
// lists. Each module is walked under its own module path, so a unit identity is
// always an import path and an edge between two workspace modules resolves the
// same way an edge inside one module does.
package golang

import (
	"bytes"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/scanner"
	"go/token"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// Frontier reasons this plugin can raise.
const (
	// FrontierBuildConstraint reports files excluded from the observed graph by
	// a build constraint. Their imports are not edges in this graph.
	FrontierBuildConstraint = "go:build-constraint-variants"
	// FrontierUnparsedSource reports a file the parser rejected.
	FrontierUnparsedSource = "go:unparsed-source"
	// FrontierCgo reports cgo, whose C side carries edges Go source cannot show.
	FrontierCgo = "go:cgo-frontier"
	// FrontierModulePath reports that an observed module's path could not be
	// read, which makes every import into that module unresolvable.
	FrontierModulePath = "go:module-path-unresolved"
	// FrontierNestedModule reports a go.mod below the root that no go.work use
	// directive lists. Its packages belong to a module path this plugin did not
	// observe, so they are absent from the graph.
	FrontierNestedModule = "go:nested-module-frontier"
	// FrontierIncludedDirectoryWalkBounded reports an opted-in build directory
	// whose independent entry bound was exhausted.
	FrontierIncludedDirectoryWalkBounded = "go:included-directory-walk-bounded"
	// FrontierWorkspaceModuleOutsideRoot reports a go.work use directive that
	// names a directory outside the root. That module cannot be observed, so
	// its packages and every import edge into them are absent from the graph.
	FrontierWorkspaceModuleOutsideRoot = "go:workspace-module-outside-root"
)

// maxPathTokens bounds one package's distinct path tokens. A package over it
// keeps none and is marked PathTokensBounded (AFP-V0-021); tests lower it.
var maxPathTokens = affected.MaxPathsPerUnit

// The path-token lexicon of tools/gate-affected-select (AFP-V0-012): printf
// verbs are removed, then every run of path characters is one token.
var (
	printfVerb = regexp.MustCompile(`%[-+# 0-9.*]*[a-zA-Z%]`)
	pathToken  = regexp.MustCompile(`[A-Za-z0-9._~@+/-]+`)
)

// module is one go.mod directory. Listed modules are observed; an unlisted one
// is a frontier. Path is empty when the manifest declares no readable module
// path, and unit identities then fall back to repository-relative directories.
type module struct {
	dir    string
	path   string
	listed bool
}

// Language observes Go packages under one module root.
type Language struct{}

// New returns the Go language plugin.
func New() Language { return Language{} }

// Name is the plugin namespace.
func (Language) Name() string { return "go" }

// Owns reports whether a path is Go source text. A file below a testdata
// directory is fixture data, which the go tool never builds, so a change to it
// is an unowned data path rather than a package source. Owns sees only the
// repository-relative path, so it also disowns an unindexed file of a workspace
// module that lies below testdata; the plan is UNKNOWN either way.
func (Language) Owns(relative string) bool {
	return strings.HasSuffix(relative, ".go") && !inTestdata(relative)
}

// Units observes every Go package in the repository rooted at root: the root
// module alone, or every module a root go.work lists.
func (language Language) Units(root string) (affected.Result, error) {
	frontier := map[string]bool{}
	observed, includedDirectoryBounded, err := affected.SourceFilesIncluding(root, func(name string) bool {
		return strings.HasSuffix(name, ".go") || name == "go.mod"
	}, "build", "dist", "target")
	if err != nil {
		return affected.Result{}, err
	}
	if includedDirectoryBounded {
		frontier[FrontierIncludedDirectoryWalkBounded] = true
	}
	files := make([]string, 0, len(observed))
	manifests := make([]string, 0, len(observed))
	for _, relative := range observed {
		if path.Base(relative) == "go.mod" {
			manifests = append(manifests, relative)
			continue
		}
		files = append(files, relative)
	}
	if len(files) == 0 && len(manifests) == 0 {
		_, statErr := os.Stat(filepath.Join(root, "go.work"))
		if errors.Is(statErr, fs.ErrNotExist) {
			return affected.Result{Frontier: sortedKeys(frontier)}, nil
		}
		if statErr != nil {
			return affected.Result{}, statErr
		}
	}
	modules, err := observeModules(root, manifests, frontier)
	if err != nil {
		return affected.Result{}, err
	}
	directories, owners := groupByDirectory(files, modules)
	units := make([]affected.Unit, 0, len(directories))
	imports := make(map[string]map[string]bool, len(directories))
	testImports := make(map[string]map[string]bool, len(directories))
	for _, directory := range sortedKeys(directories) {
		unit, importPaths, testImportPaths, err := language.observeDirectory(root, owners[directory], directory, directories[directory], frontier)
		if err != nil {
			return affected.Result{}, err
		}
		if unit.ID == "" {
			continue
		}
		units = append(units, unit)
		imports[unit.ID] = importPaths
		testImports[unit.ID] = testImportPaths
	}
	resolve(units, imports, testImports, modulePaths(modules))
	return affected.Result{Units: units, Frontier: sortedKeys(frontier)}, nil
}

// observeDirectory turns one directory of Go files into one unit plus the raw
// import path sets its non-test files and its test files declare.
func (Language) observeDirectory(root string, owner module, directory string, files []string, frontier map[string]bool) (affected.Unit, map[string]bool, map[string]bool, error) {
	sources := make([]string, 0, len(files))
	tests := make([]string, 0, len(files))
	importPaths := make(map[string]bool, 16)
	testImportPaths := make(map[string]bool, 16)
	names := make(map[string]bool, 16)
	embeds := false
	var reads unboundedReads
	fileSet := token.NewFileSet()
	for _, relative := range files {
		body, err := affected.ReadSource(root, relative)
		if err != nil {
			frontier[FrontierUnparsedSource] = true
			continue
		}
		if hasBuildConstraint(body) {
			frontier[FrontierBuildConstraint] = true
		}
		file, err := parser.ParseFile(fileSet, relative, body, parser.ImportsOnly)
		if err != nil {
			frontier[FrontierUnparsedSource] = true
			continue
		}
		isTest := strings.HasSuffix(relative, "_test.go")
		embeds = embeds || (!isTest && bytes.Contains(body, []byte("//go:embed")))
		declared := importPaths
		if isTest {
			declared = testImportPaths
		}
		for _, spec := range file.Imports {
			value, unquoteErr := strconv.Unquote(spec.Path.Value)
			if unquoteErr != nil {
				frontier[FrontierUnparsedSource] = true
				continue
			}
			if value == "C" {
				frontier[FrontierCgo] = true
				continue
			}
			declared[value] = true
		}
		aliases, dotImported := rootLocatorImports(file.Imports)
		scan := pathTokens(body[importsEnd(fileSet, file):], owner.path, aliases, dotImported)
		if scan.err != nil && !ignoredByGo(directory) {
			frontier[FrontierUnparsedSource] = true
			reads.mark(relative+" does not tokenize", isTest)
		}
		for _, call := range scan.calls {
			reads.mark(relative+" calls "+call, isTest)
		}
		for _, name := range scan.tokens {
			names[name] = true
			if reason := escapesPackage(directory, name, isTest); reason != "" {
				reads.mark(relative+" "+reason, isTest)
			}
		}
		if isTest {
			tests = append(tests, relative)
			continue
		}
		sources = append(sources, relative)
	}
	if len(sources) == 0 && len(tests) == 0 {
		return affected.Unit{}, nil, nil, nil
	}
	sort.Strings(sources)
	sort.Strings(tests)
	bounded := len(names) > maxPathTokens
	if bounded {
		names = nil
	}
	return affected.Unit{ID: unitID(owner, directory), Sources: sources, Tests: tests, PathTokens: sortedKeys(names), PathTokensBounded: bounded, Embeds: embeds, UnboundedReads: reads.reason, LocatesRoot: reads.locatesRoot}, importPaths, testImportPaths, nil
}

// ignoredByGo reports a repository-relative directory the go tool's package
// patterns and the fast tier's index skip: one with a testdata or `_`-prefixed
// component. A file there that does not lex is no build input, so it raises no
// frontier.
func ignoredByGo(directory string) bool {
	for _, component := range strings.Split(directory, "/") {
		if component == "testdata" || strings.HasPrefix(component, "_") {
			return true
		}
	}
	return false
}

// importsEnd is the byte offset just past a file's import declarations, which
// are the only declarations an imports-only parse keeps. An import path is an
// edge, never a path token.
func importsEnd(fileSet *token.FileSet, file *ast.File) int {
	end := file.Name.End()
	for _, decl := range file.Decls {
		end = decl.End()
	}
	return fileSet.Position(end).Offset
}

// fileScan is what one lexing pass over a file's body after its imports
// yields: its path tokens, its root-locating calls, and any lexical error.
type fileScan struct {
	tokens []string
	calls  []string
	err    error
}

// pathTokens lexes body for the path tokens of every string literal, with the
// owning module's import path rewritten to a path anchored at that module's
// directory (AFP-V0-021, the AFP-V0-012 lexicon), and for calls of a
// root-locating function under the local name the file's imports gave it
// (AFP-V0-012 rule (d)).
func pathTokens(body []byte, modulePath string, aliases map[string]string, dotImported map[string]bool) fileScan {
	var scan fileScan
	var lexer scanner.Scanner
	lexer.Init(token.NewFileSet().AddFile("", -1, len(body)), body, func(_ token.Position, message string) { scan.err = errors.New(message) }, 0)
	previous := [2]string{}
	for {
		_, kind, text := lexer.Scan()
		if kind == token.EOF {
			return scan
		}
		if kind == token.IDENT {
			scan.calls = append(scan.calls, rootLocatorCall(previous, text, aliases, dotImported)...)
		}
		previous = [2]string{previous[1], kind.String() + text}
		if kind != token.STRING {
			continue
		}
		value, err := strconv.Unquote(text)
		if err != nil {
			continue
		}
		for _, name := range pathToken.FindAllString(printfVerb.ReplaceAllString(value, " "), -1) {
			scan.tokens = append(scan.tokens, rootAnchored(name, modulePath))
		}
	}
}

// rootAnchored rewrites an import path under the module to its directory
// relative to the module's own directory, with a leading slash; the module path
// itself is "/". Matching ignores the anchor, so a workspace module's paths
// still name its files by their trailing components.
func rootAnchored(name, modulePath string) string {
	if modulePath == "" {
		return name
	}
	if name == modulePath {
		return "/"
	}
	if rest, under := strings.CutPrefix(name, modulePath+"/"); under {
		return "/" + rest
	}
	return name
}

// resolve rewrites each unit's raw Go import paths into the unit identities
// this graph knows. An import outside every observed module is an external
// dependency and is dropped: it cannot be a dirty repository path. An import
// under an observed module that names no unit, such as a deleted package, is
// kept as an edge to that absent unit, so a dirty path in the deleted
// package's directory can still reach its importers (V1-0340). A path the
// unit's non-test files also import is an ordinary edge, never a test-only one.
func resolve(units []affected.Unit, imports, testImports map[string]map[string]bool, modules []string) {
	byImportPath := make(map[string]string, len(units))
	for _, unit := range units {
		byImportPath[strings.TrimPrefix(unit.ID, "go:")] = unit.ID
	}
	for index := range units {
		unit := &units[index]
		unit.Imports = resolved(unit.ID, imports[unit.ID], nil, byImportPath, modules)
		testOnly := resolved(unit.ID, testImports[unit.ID], imports[unit.ID], byImportPath, modules)
		if len(testOnly) != 0 {
			unit.TestImports = testOnly
		}
	}
}

// resolved maps raw import paths not in skip to sorted unit identities other
// than self: a known unit, or an absent one under an observed module.
func resolved(self string, paths, skip map[string]bool, byImportPath map[string]string, modules []string) []string {
	edges := make([]string, 0, len(paths))
	for value := range paths {
		target, known := byImportPath[value]
		if !known && underModule(value, modules) {
			target, known = "go:"+value, true
		}
		if !known || target == self || skip[value] {
			continue
		}
		edges = append(edges, target)
	}
	sort.Strings(edges)
	return edges
}

// modulePaths lists the readable paths of the observed modules.
func modulePaths(modules map[string]module) []string {
	paths := make([]string, 0, len(modules))
	for _, owner := range modules {
		if owner.listed && owner.path != "" {
			paths = append(paths, owner.path)
		}
	}
	return paths
}

func underModule(importPath string, modules []string) bool {
	for _, modulePath := range modules {
		if importPath == modulePath || strings.HasPrefix(importPath, modulePath+"/") {
			return true
		}
	}
	return false
}

// unitID is the import path of the package in directory, which lies inside its
// owning module. Without a module path the repository-relative directory is the
// only identity available.
func unitID(owner module, directory string) string {
	if owner.path == "" {
		return "go:" + directory
	}
	return "go:" + path.Join(owner.path, relativeTo(directory, owner.dir))
}

func relativeTo(directory, moduleDir string) string {
	if moduleDir == "." {
		return directory
	}
	if directory == moduleDir {
		return "."
	}
	return strings.TrimPrefix(directory, moduleDir+"/")
}

// observeModules maps every go.mod directory to its module. The observed set is
// the root module, or the go.work use set when the root declares one; a go.mod
// outside that set is a frontier, and its packages are dropped from the graph
// rather than attributed to the module above them.
func observeModules(root string, manifests []string, frontier map[string]bool) (map[string]module, error) {
	listed, err := workspaceDirectories(root, frontier)
	if err != nil {
		return nil, err
	}
	modules := make(map[string]module, len(listed)+len(manifests))
	for _, directory := range listed {
		modulePath, err := readModulePath(filepath.Join(root, filepath.FromSlash(directory)))
		if err != nil {
			frontier[FrontierModulePath] = true
		}
		modules[directory] = module{dir: directory, path: modulePath, listed: true}
	}
	for _, manifest := range manifests {
		directory := path.Dir(manifest)
		if _, known := modules[directory]; known {
			continue
		}
		if fixtureOfObservedModule(directory, modules) {
			continue
		}
		frontier[FrontierNestedModule] = true
		modules[directory] = module{dir: directory}
	}
	return modules, nil
}

// workspaceDirectories lists the module directories the root's go.work uses,
// or the root alone when there is no go.work. Only the use grammar is read: a
// "use DIR" line or a "use (" block with one directory per line. A directory
// outside the root cannot be observed; it is skipped and raised as a frontier.
func workspaceDirectories(root string, frontier map[string]bool) ([]string, error) {
	body, err := os.ReadFile(filepath.Join(root, "go.work"))
	if errors.Is(err, fs.ErrNotExist) {
		return []string{"."}, nil
	}
	if err != nil {
		return nil, err
	}
	directories := make([]string, 0, 8)
	inBlock := false
	for _, line := range strings.Split(string(body), "\n") {
		fields := strings.Fields(stripComment(line))
		switch {
		case len(fields) == 0:
		case inBlock && fields[0] == ")":
			inBlock = false
		case inBlock:
			directories = appendUse(directories, fields[0], frontier)
		case fields[0] != "use" || len(fields) != 2:
		case fields[1] == "(":
			inBlock = true
		default:
			directories = appendUse(directories, fields[1], frontier)
		}
	}
	if len(directories) == 0 {
		return []string{"."}, nil
	}
	return directories, nil
}

func stripComment(line string) string {
	if index := strings.Index(line, "//"); index >= 0 {
		return line[:index]
	}
	return line
}

func appendUse(directories []string, value string, frontier map[string]bool) []string {
	cleaned := path.Clean(filepath.ToSlash(strings.Trim(value, "\"`")))
	if cleaned == "." || affected.ValidRelativePath(cleaned) {
		return append(directories, cleaned)
	}
	frontier[FrontierWorkspaceModuleOutsideRoot] = true
	return directories
}

// enclosingModule finds the nearest module at or above directory and reports
// whether it is one this plugin observes.
func enclosingModule(directory string, modules map[string]module) (module, bool) {
	for {
		owner, known := modules[directory]
		if known {
			return owner, owner.listed
		}
		if directory == "." {
			return module{}, false
		}
		directory = path.Dir(directory)
	}
}

// readModulePath extracts the module path from go.mod without importing the
// module tooling: the first "module <path>" line wins, which is the whole of
// the grammar that matters here.
func readModulePath(root string) (string, error) {
	body, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(body), "\n") {
		fields := strings.Fields(stripComment(line))
		if len(fields) != 2 || fields[0] != "module" {
			continue
		}
		value := strings.Trim(fields[1], "\"")
		if value == "" {
			continue
		}
		return value, nil
	}
	return "", fmt.Errorf("%w: go.mod declares no module path", affected.ErrInvalidLanguage)
}

// hasBuildConstraint reports a //go:build line in the file header. Its presence
// means the observed graph may differ from the graph a different GOOS, GOARCH,
// or tag set would produce, which is a frontier rather than an error.
func hasBuildConstraint(body []byte) bool {
	for _, line := range strings.Split(string(body), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "package ") {
			return false
		}
		if strings.HasPrefix(trimmed, "//go:build") {
			return true
		}
	}
	return false
}

// groupByDirectory buckets files by directory and names the module each
// directory belongs to. A directory inside an unlisted module, outside every
// module, or below a testdata directory of its module is dropped.
func groupByDirectory(files []string, modules map[string]module) (map[string][]string, map[string]module) {
	directories := make(map[string][]string, 256)
	owners := make(map[string]module, 256)
	for _, file := range files {
		directory := path.Dir(file)
		owner, observed := enclosingModule(directory, modules)
		if !observed {
			continue
		}
		if inTestdata(relativeTo(directory, owner.dir)) {
			continue
		}
		directories[directory] = append(directories[directory], file)
		owners[directory] = owner
	}
	return directories, owners
}

// fixtureOfObservedModule reports whether a go.mod directory lies below a
// testdata directory of an observed module. The go tool ignores testdata, so
// such a manifest is fixture data, not a nested module frontier.
func fixtureOfObservedModule(directory string, modules map[string]module) bool {
	owner, observed := enclosingModule(path.Dir(directory), modules)
	return observed && inTestdata(relativeTo(directory, owner.dir))
}

// inTestdata reports whether any component of a slash-separated relative path
// is testdata, the directory name the go tool never treats as a package.
func inTestdata(relative string) bool {
	for _, component := range strings.Split(relative, "/") {
		if component == "testdata" {
			return true
		}
	}
	return false
}

func sortedKeys[Value any](values map[string]Value) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
