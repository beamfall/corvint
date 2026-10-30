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
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/scanner"
	"go/token"
	"io/fs"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// Frontier reasons this plugin can raise.
const (
	// FrontierBuildConstraint reports a package with a file under a //go:build
	// constraint or a GOOS/GOARCH file-name suffix. Every variant's imports
	// are edges here, so the closure is a superset of each variant's; the
	// reason is the unit's own and bears only on a plan that reaches it, whose
	// tests one run exercises in one variant (AFP-V0-580).
	FrontierBuildConstraint = "go:build-constraint-variants"
	// FrontierUnparsedSource reports a file the parser rejected.
	FrontierUnparsedSource = "go:unparsed-source"
	// FrontierCgo reports cgo, whose C side carries edges Go source cannot show.
	FrontierCgo = "go:cgo-frontier"
	// FrontierModulePath reports that an observed module's path could not be
	// read, which makes every import into that module unresolvable.
	FrontierModulePath = "go:module-path-unresolved"
	// FrontierNestedModule reports a go.mod below the root that no go.work use
	// directive lists and whose manifest does not rule out an import of an
	// observed module. Its packages belong to a module path this plugin did not
	// observe, so they are absent from the graph (V1-0867).
	FrontierNestedModule = "go:nested-module-frontier"
	// FrontierIncludedDirectoryWalkBounded reports an opted-in build directory
	// whose independent entry bound was exhausted.
	FrontierIncludedDirectoryWalkBounded = "go:included-directory-walk-bounded"
	// FrontierWorkspaceModuleOutsideRoot reports a go.work use directive that
	// names a directory outside the root. That module cannot be observed, so
	// its packages and every import edge into them are absent from the graph.
	FrontierWorkspaceModuleOutsideRoot = "go:workspace-module-outside-root"
	// FrontierWorkspaceUnparsed reports a root go.work the strict manifest
	// reader refuses. Its use set is unknown, so only the root module is
	// observed and every other module's packages are absent (AFP-V0-190).
	FrontierWorkspaceUnparsed = "go:workspace-unparsed"
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

// ReadsAnyPath reports that Go units carry path tokens and rule (d) reads, so a
// unit this plugin could not observe may read any dirty path (V1-0289).
func (Language) ReadsAnyPath() bool { return true }

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
func (language Language) Units(ctx context.Context, root string) (affected.Result, error) {
	return language.UnitsSource(affected.DiskSourceContext(ctx, root))
}

// includedDirectories are the affected.SkippedDirectories names that can hold
// first-party Go source, so the walk descends them.
var includedDirectories = []string{"build", "dist", "target"}

// UnitsSource reads only the explicitly supplied source universe.
func (language Language) UnitsSource(root *affected.Source) (affected.Result, error) {
	frontier := map[string]bool{}
	observed, includedDirectoryBounded, err := root.FilesIncluding(func(name string) bool {
		return strings.HasSuffix(name, ".go") || name == "go.mod"
	}, includedDirectories...)
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
		_, statErr := root.Stat("go.work")
		if errors.Is(statErr, fs.ErrNotExist) {
			return affected.Result{Frontier: sortedKeys(frontier)}, nil
		}
		if statErr != nil {
			return affected.Result{}, statErr
		}
	}
	modules, evidence, err := observeModules(root, manifests, frontier)
	if err != nil {
		return affected.Result{}, err
	}
	directories, owners := groupByDirectory(files, modules)
	units := make([]affected.Unit, 0, len(directories))
	unitDirectories := make([]string, 0, len(directories))
	commands := make(map[string]string)
	imports := make(map[string]map[string]bool, len(directories))
	testImports := make(map[string]map[string]bool, len(directories))
	ordered := sortedKeys(directories)
	observedDirectories := language.observeDirectories(root, owners, directories, ordered)
	embedding := make(map[int][]string)
	for offset, directory := range ordered {
		observation := observedDirectories[offset]
		for reason := range observation.frontier {
			frontier[reason] = true
		}
		unit, importPaths, testImportPaths, err := observation.unit, observation.imports, observation.testImports, observation.err
		if err != nil {
			return affected.Result{}, err
		}
		if unit.ID == "" {
			continue
		}
		units = append(units, unit)
		unitDirectories = append(unitDirectories, directory)
		if len(observation.embeds) != 0 {
			embedding[len(units)-1] = observation.embeds
		}
		if observation.command {
			commands[directory] = unit.ID
		}
		imports[unit.ID] = importPaths
		testImports[unit.ID] = testImportPaths
	}
	observedUnits := len(units)
	if err := resolveEmbeds(root, units, unitDirectories, embedding, manifests); err != nil {
		return affected.Result{}, err
	}
	for _, dependent := range dependentModuleUnits(root, files, modules, evidence, frontier) {
		if _, taken := imports[dependent.unit.ID]; taken {
			frontier[FrontierNestedModule] = true
			continue
		}
		units = append(units, dependent.unit)
		imports[dependent.unit.ID] = dependent.imports
	}
	resolve(units, imports, testImports, modulePaths(modules))
	applyReadScopes(root, units[:observedUnits], unitDirectories, frontier)
	applyBinaryExecs(root, units[:observedUnits], unitDirectories, owners, commands, frontier)
	// A read this plugin tolerates (an unparsed file, an invalid declaration)
	// may be an immutable source's fatal failure; surface it rather than a
	// partial unit set (V1-1416, DLT-V0-002).
	if err := root.Err(); err != nil {
		return affected.Result{}, err
	}
	unobserved := unobservedModules(evidence, modules)
	return affected.Result{Units: units, Frontier: sortedKeys(frontier), Unobserved: unobserved, Runnable: runnableModules(evidence, unobserved)}, nil
}

// runnableModules lists the unobserved nested modules that no observed
// module builds (AFP-V0-650): the AFP-V0-031 evidence closed the frontier,
// or left it open only in the dependent direction (AFP-V0-582). The module's
// own `go test ./...`, run from its directory, then covers every change to
// its files. A module the observed side requires or replaces, one whose
// manifest cannot be read and parsed, one under a go.work, one judged
// while an observed manifest is unreadable, or one another unlisted module
// draws in through a directory replacement (or judged while a nested
// manifest that could do so is unreadable) is not runnable and keeps the
// AFP-V0-191 unobserved-directory frontier.
func runnableModules(evidence []nestedModule, unobserved []string) []string {
	directories := make([]string, 0, len(unobserved))
	for _, judged := range evidence {
		directory := path.Dir(judged.Manifest)
		if (judged.Open == "" || judged.Dependent) && judged.Drawn == "" && slices.Contains(unobserved, directory) {
			directories = append(directories, directory)
		}
	}
	sort.Strings(directories)
	return slices.Compact(directories)
}

// resolveEmbeds sets each embedding unit's EmbedFiles from its directives'
// patterns against every file the walk lists (AFP-V0-600). The listing is the
// same shared walk the Go files came from, so a file the walk skips is never
// claimed and stays owned by none.
func resolveEmbeds(root *affected.Source, units []affected.Unit, unitDirectories []string, embedding map[int][]string, manifests []string) error {
	if len(embedding) == 0 {
		return nil
	}
	files, _, err := root.FilesIncluding(func(string) bool { return true }, includedDirectories...)
	if err != nil {
		return err
	}
	sort.Strings(files)
	moduleRoots := make(map[string]bool, len(manifests))
	for _, manifest := range manifests {
		moduleRoots[path.Dir(manifest)] = true
	}
	for offset, patterns := range embedding {
		targets := embedTargets(unitDirectories[offset], patterns, files, moduleRoots)
		if len(targets) == 0 || len(targets) > affected.MaxPathsPerUnit {
			continue
		}
		units[offset].EmbedFiles = targets
	}
	return nil
}

// unobservedModules lists the nested module directories whose files no unit
// declares (AFP-V0-191): every unlisted module below the root that contains no
// observed module. The root and a directory holding an observed module are
// left out, since an unindexed file there may belong to an observed module.
func unobservedModules(evidence []nestedModule, modules map[string]module) []string {
	directories := make([]string, 0, len(evidence))
	for _, judged := range evidence {
		directory := path.Dir(judged.Manifest)
		if directory == "." {
			continue
		}
		holdsObserved := false
		for _, owner := range modules {
			if owner.listed && (owner.dir == directory || strings.HasPrefix(owner.dir, directory+"/")) {
				holdsObserved = true
				break
			}
		}
		if !holdsObserved {
			directories = append(directories, directory)
		}
	}
	sort.Strings(directories)
	return slices.Compact(directories)
}

// directoryObservation is one observeDirectory result with the frontier
// reasons that directory raised.
type directoryObservation struct {
	unit                 affected.Unit
	imports, testImports map[string]bool
	frontier             map[string]bool
	command              bool
	// embeds holds the patterns of the non-test files' //go:embed
	// directives (AFP-V0-600).
	embeds []string
	err    error
}

// observeDirectories observes every directory in order. A disk source reads
// files concurrently, so its directories are observed by a bounded pool; each
// result lands at its directory's offset and the caller merges them in order,
// so the units, edges and frontier are those of the serial pass.
func (language Language) observeDirectories(root *affected.Source, owners map[string]module, directories map[string][]string, ordered []string) []directoryObservation {
	observations := make([]directoryObservation, len(ordered))
	observe := func(offset int) {
		directory := ordered[offset]
		observation := &observations[offset]
		observation.frontier = map[string]bool{}
		observation.unit, observation.imports, observation.testImports, observation.command, observation.embeds, observation.err = language.observeDirectory(root, owners[directory], directory, directories[directory], observation.frontier)
	}
	workers := min(runtime.GOMAXPROCS(0), observeWorkers, len(ordered))
	if !root.ConcurrentReads() || workers < 2 {
		for offset := range ordered {
			observe(offset)
		}
		return observations
	}
	var next atomic.Int64
	var pending sync.WaitGroup
	for range workers {
		pending.Add(1)
		go func() {
			defer pending.Done()
			for offset := int(next.Add(1)) - 1; offset < len(ordered); offset = int(next.Add(1)) - 1 {
				observe(offset)
			}
		}()
	}
	pending.Wait()
	return observations
}

// observeWorkers bounds the directories observed at once.
const observeWorkers = 8

// observeDirectory turns one directory of Go files into one unit plus the raw
// import path sets its non-test files and its test files declare, and reports
// whether a non-test file declares package main (a command, AFP-V0-037). It
// also returns the patterns of the non-test files' //go:embed directives; a
// file whose directives do not parse contributes none (AFP-V0-600).
func (Language) observeDirectory(root *affected.Source, owner module, directory string, files []string, frontier map[string]bool) (affected.Unit, map[string]bool, map[string]bool, bool, []string, error) {
	sources := make([]string, 0, len(files))
	tests := make([]string, 0, len(files))
	importPaths := make(map[string]bool, 16)
	testImportPaths := make(map[string]bool, 16)
	names := make(map[string]bool, 16)
	globs := make(map[string]bool)
	embeds, constrained, command := false, false, false
	var patterns []string
	var reads unboundedReads
	fileSet := token.NewFileSet()
	for _, relative := range files {
		body, err := root.Read(relative)
		if err != nil {
			frontier[FrontierUnparsedSource] = true
			continue
		}
		// A variant is a file only some builds compile: a //go:build line or
		// a GOOS/GOARCH file-name suffix (AFP-V0-580).
		constrained = constrained || hasPlatformSuffix(relative) || hasBuildConstraint(body)
		file, err := parser.ParseFile(fileSet, relative, body, parser.ImportsOnly)
		if err != nil {
			frontier[FrontierUnparsedSource] = true
			continue
		}
		isTest := strings.HasSuffix(relative, "_test.go")
		command = command || (!isTest && file.Name.Name == "main")
		if !isTest && bytes.Contains(body, []byte(embedDirective)) {
			embeds = true
			if parsed, ok := embedPatterns(body); ok {
				patterns = append(patterns, parsed...)
			}
		}
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
		scan := pathTokens(body[importsEnd(fileSet, file):], owner.path, file.Imports)
		if scan.err != nil && !ignoredByGo(directory) {
			frontier[FrontierUnparsedSource] = true
			reads.mark(relative+" does not tokenize", isTest)
		}
		for _, call := range scan.calls {
			reads.mark(relative+" calls "+call, isTest)
		}
		for _, glob := range literalGlobs(directory, body, file.Imports) {
			globs[glob] = true
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
		return affected.Unit{}, nil, nil, false, nil, nil
	}
	sort.Strings(sources)
	sort.Strings(tests)
	bounded := len(names) > maxPathTokens || len(globs) > maxPathTokens
	if bounded {
		names, globs = nil, nil
	}
	var unitFrontier []string
	if constrained {
		unitFrontier = []string{FrontierBuildConstraint}
	}
	return affected.Unit{ID: unitID(owner, directory), Sources: sources, Tests: tests, PathTokens: sortedKeys(names), PathGlobs: globSlice(globs), PathTokensBounded: bounded, Embeds: embeds, UnboundedReads: reads.reason, LocatesRoot: reads.locatesRoot, Frontier: unitFrontier}, importPaths, testImportPaths, command, patterns, nil
}

// platformNames are the GOOS and GOARCH values a file-name suffix constrains
// to (go/build's syslist).
var platformNames = map[string]bool{
	"aix": true, "android": true, "darwin": true, "dragonfly": true, "freebsd": true, "hurd": true,
	"illumos": true, "ios": true, "js": true, "linux": true, "nacl": true, "netbsd": true,
	"openbsd": true, "plan9": true, "solaris": true, "wasip1": true, "windows": true, "zos": true,
	"386": true, "amd64": true, "amd64p32": true, "arm": true, "armbe": true, "arm64": true,
	"arm64be": true, "loong64": true, "mips": true, "mipsle": true, "mips64": true,
	"mips64le": true, "mips64p32": true, "mips64p32le": true, "ppc": true, "ppc64": true,
	"ppc64le": true, "riscv": true, "riscv64": true, "s390": true, "s390x": true, "sparc": true,
	"sparc64": true, "wasm": true,
}

// hasPlatformSuffix reports a file name the go tool builds only for some
// GOOS or GOARCH: one whose stem, past its first "_" and without a "_test"
// suffix, ends "_GOOS", "_GOARCH" or "_GOOS_GOARCH" (go/build's
// goodOSArchFile). Such a file is a variant as a //go:build line makes one.
func hasPlatformSuffix(relative string) bool {
	stem := strings.TrimSuffix(path.Base(relative), ".go")
	underscore := strings.Index(stem, "_")
	if underscore < 0 {
		return false
	}
	parts := strings.Split(stem[underscore:], "_")
	if last := len(parts) - 1; last > 0 && parts[last] == "test" {
		parts = parts[:last]
	}
	return len(parts) > 1 && platformNames[parts[len(parts)-1]]
}

// globSlice is the sorted globs, nil when there are none, so a unit without
// one keeps its graph digest.
func globSlice(globs map[string]bool) []string {
	if len(globs) == 0 {
		return nil
	}
	return sortedKeys(globs)
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
func pathTokens(body []byte, modulePath string, imports []*ast.ImportSpec) fileScan {
	aliases, dotImported := rootLocatorImports(imports)
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
		scan.tokens = append(scan.tokens, literalTokens(value, modulePath)...)
	}
}

// literalTokens is the AFP-V0-021 lexicon over one unquoted string literal.
func literalTokens(value, modulePath string) []string {
	value = parentBackslash.Replace(printfVerb.ReplaceAllString(value, verbMark))
	var tokens []string
	for _, bounds := range pathToken.FindAllStringIndex(value, -1) {
		start, end := bounds[0], bounds[1]
		// A `/` directly after `<` closes a markup tag (`</script>`), so it
		// anchors nothing: the tag name is a lone name token (AFP-V0-505).
		// The HTML-escaped `&lt;/script&gt;` closes one too (AFP-V0-840).
		if value[start] == '/' && (start > 0 && value[start-1] == '<' || closesEscapedTag(value, start)) {
			start++
			if start == end {
				continue
			}
		}
		name := rootAnchored(value[start:end], modulePath)
		tokens = append(tokens, wholeShortEnds(name, value, start, end))
	}
	return tokens
}

// closesEscapedTag reports a `/` at index directly after the HTML entity
// `&lt;`, in any letter case: the escaped closing tag an HTML test asserts
// (AFP-V0-840, V1-3207).
func closesEscapedTag(value string, index int) bool {
	return index >= len("&lt;") && strings.EqualFold(value[index-len("&lt;"):index], "&lt;")
}

// parentBackslash rewrites a backslash after a parent component (`..\x`) to
// the slash the lexicon reads, so the token is the whole climbing path and
// not a bare `..` that rule (d) reads as the repository root (AFP-V0-506).
var parentBackslash = strings.NewReplacer(`..\`, `../`)

// verbMark stands in for a removed printf verb: like the literal's own ends, a
// verb is where another value joins the literal.
const verbMark = "\x00"

// wholeShortEnds marks an outer component of one or two characters of a
// multi-component token as whole when the literal itself delimits it with an
// ASCII byte (V1-1818): "./" before a first one and "/" after a last one, so
// the `s/a` of `"sed 's/a b'"` becomes `./s/a/` and names only a path with `s`
// and `a` as whole components. A component at the literal's start or end, or
// beside a printf verb, may join another value (`"docs/a" + "lpha.go"`), and one
// beside a non-ASCII byte may continue the same name, so either stays a
// fragment that matches by suffix or prefix.
func wholeShortEnds(name, value string, start, end int) string {
	components := strings.Split(name, "/")
	last := len(components) - 1
	if last < 1 {
		return name
	}
	if short(components[0]) && runComponent(components[1]) && delimited(value, start-1) {
		name = "./" + name
	}
	if short(components[last]) && runComponent(components[last-1]) && delimited(value, end) {
		name += "/"
	}
	return name
}

// runComponent reports a component the matcher keeps in a component run.
func runComponent(component string) bool {
	return component != "" && component != "." && component != ".."
}

func short(component string) bool {
	return runComponent(component) && len(component) < 3
}

// delimited reports an ASCII byte of value at index that is not a verb mark.
func delimited(value string, index int) bool {
	return index >= 0 && index < len(value) && value[index] < utf8.RuneSelf && value[index] != verbMark[0]
}

// FilePathTokens is the AFP-V0-021 lexicon over one Go file, as
// observeDirectory reads it: the path tokens of every string literal after the
// file's import declarations, with an import path under modulePath rewritten to
// a "/"-anchored directory. A file that does not parse or lex is an error.
func FilePathTokens(name string, body []byte, modulePath string) ([]string, error) {
	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, name, body, parser.ImportsOnly)
	if err != nil {
		return nil, err
	}
	scan := pathTokens(body[importsEnd(fileSet, file):], modulePath, nil)
	return scan.tokens, scan.err
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
// the root module, or the go.work use set when the root declares one; the
// packages of a go.mod outside that set are dropped from the graph rather than
// attributed to the module above them, and that module is a frontier unless
// its manifest shows it cannot import an observed module (V1-0867). The
// per-module evidence names every nested manifest read and why it stays open.
func observeModules(root *affected.Source, manifests []string, frontier map[string]bool) (map[string]module, []nestedModule, error) {
	listed, err := workspaceDirectories(root, frontier)
	if err != nil {
		return nil, nil, err
	}
	modules := make(map[string]module, len(listed)+len(manifests))
	for _, directory := range listed {
		modulePath, err := readModuleSourcePath(root, directory)
		if err != nil {
			frontier[FrontierModulePath] = true
		}
		modules[directory] = module{dir: directory, path: modulePath, listed: true}
	}
	nested := make([]string, 0, len(manifests))
	for _, manifest := range manifests {
		directory := path.Dir(manifest)
		if _, known := modules[directory]; known {
			continue
		}
		if fixtureOfObservedModule(directory, modules) {
			continue
		}
		nested = append(nested, directory)
		modules[directory] = module{dir: directory}
	}
	evidence := nestedModules(root, nested, modules)
	for _, judged := range evidence {
		if judged.Open != "" && !judged.Dependent {
			frontier[FrontierNestedModule] = true
		}
	}
	return modules, evidence, nil
}

// workspaceDirectories lists the module directories the root's go.work uses,
// or the root alone when there is no go.work. The go.work is read with the
// strict manifest reader the nested-module decision uses (V1-0874), so the two
// never disagree about the use set: a go.work that reader refuses (over-size,
// unreadable, or outside the grammar it admits) observes the root alone and
// raises FrontierWorkspaceUnparsed. A directory outside the root cannot be
// observed; it is skipped and raised as a frontier.
func workspaceDirectories(root *affected.Source, frontier map[string]bool) ([]string, error) {
	_, err := root.Stat("go.work")
	if errors.Is(err, fs.ErrNotExist) {
		return []string{"."}, nil
	}
	if err != nil {
		return nil, err
	}
	work, reason := readManifest(root, "go.work", true)
	if reason != "" {
		frontier[FrontierWorkspaceUnparsed] = true
		return []string{"."}, nil
	}
	directories := make([]string, 0, len(work.uses))
	for _, use := range work.uses {
		directories = appendUse(directories, use, frontier)
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
	cleaned := path.Clean(filepath.ToSlash(value))
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
	return readModuleSourcePath(affected.DiskSource(root), ".")
}

// ReadModulePath is readModulePath: the root module path an attribution over
// this graph's import paths needs (STR-V0-002).
func ReadModulePath(root string) (string, error) { return readModulePath(root) }

// GoverningWorkspace names the go.work the go tool uses from directory, a
// repository-relative directory: the nearest one at or above it inside root,
// as a repository-relative path, with the repository-relative directories its
// use directives list (AFP-V0-469). workspace is "" when there is none; reason
// says why an existing one could not be read within the manifest bound.
func GoverningWorkspace(root, directory string) (workspace string, uses []string, reason string) {
	source := affected.DiskSource(root)
	for current := path.Clean(directory); ; current = path.Dir(current) {
		name := path.Join(current, "go.work")
		if _, err := source.Stat(name); !errors.Is(err, fs.ErrNotExist) {
			work, why := readManifest(source, name, true)
			if why != "" {
				return name, nil, why
			}
			for _, use := range work.uses {
				uses = append(uses, path.Join(current, filepath.ToSlash(use)))
			}
			return name, uses, ""
		}
		if current == "." || current == "/" {
			return "", nil, ""
		}
	}
}

func readModuleSourcePath(root *affected.Source, directory string) (string, error) {
	body, err := root.Read(path.Join(directory, "go.mod"))
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(body), "\n") {
		fields := strings.Fields(stripComment(line))
		if len(fields) != 2 || fields[0] != "module" {
			continue
		}
		value := fields[1]
		if strings.HasPrefix(value, "\"") || strings.HasPrefix(value, "`") {
			// A quoted path follows Go string-literal rules, as modfile reads
			// it; a raw string (which the go tool refuses) or an escape this
			// cannot interpret is uncertainty (V1-0873).
			unquoted, err := strconv.Unquote(value)
			if err != nil || value[0] == '`' {
				return "", fmt.Errorf("%w: go.mod module path %s is not a Go string literal", affected.ErrInvalidLanguage, value)
			}
			value = unquoted
		}
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
