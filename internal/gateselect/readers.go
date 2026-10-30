package gateselect

// The repository index behind AFP-V0-012 (decision 0131): which root-module
// package a dirty path can reach, by import edges and by the path literals its
// files carry. Every rule over-approximates; a package whose reads cannot be
// attributed from its literals is `unresolved` and selected on any dirty path.

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/scanner"
	"go/token"
	"io"
	"io/fs"
	"os"
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
)

const (
	maxIndexEntries = 200000
	maxSourceBytes  = 8 << 20
)

// rootLocatorFuncs are the calls that find the repository from the working
// directory at run time, so no literal bounds what follows: import path to the
// one function name on it that locates the root. Runtime frame APIs
// (runtime.Caller, Callers, CallersFrames, FuncForPC) return metadata, not
// file contents, and are excluded as the planner excludes them (AFP-V0-810).
var rootLocatorFuncs = map[string]string{"os": "Getwd"}

// rootLocatorToken is the Git option with the same effect, split so that this
// package's own source does not carry it.
var rootLocatorToken = "--show-" + "toplevel"

var (
	printfVerb = regexp.MustCompile(`%[-+# 0-9.*]*[a-zA-Z%]`)
	pathToken  = regexp.MustCompile(`[A-Za-z0-9._~@+/-]+`)
)

type goPackage struct {
	embeds      bool
	unresolved  string // why the package's reads are not bounded by its literals
	locatesRoot bool   // a non-test file is unresolved, so every dependent is too
}

type repositoryIndex struct {
	module         string
	packages       map[string]*goPackage // by repository-relative directory, "" for the root
	nested         []string              // directories holding a nested go.mod
	holders        map[string][]string   // path token -> directories whose files carry it
	nonTestHolders map[string][]string   // path token -> directories whose non-test files carry it
	importers      map[string][]string   // directory -> directories whose non-test files import it
	namers         map[string][]string   // directory -> directories whose non-test files name it by a path token
	testImporters  map[string][]string   // directory -> directories whose _test.go files alone import or name it
}

func indexRepository(root, module string) (*repositoryIndex, error) {
	index := &repositoryIndex{module: module, packages: map[string]*goPackage{}, holders: map[string][]string{}, nonTestHolders: map[string][]string{}, importers: map[string][]string{}, namers: map[string][]string{}, testImporters: map[string][]string{}}
	var files []indexedFile
	entries := 0
	walkErr := filepath.WalkDir(root, func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		entries++
		if entries > maxIndexEntries {
			return fmt.Errorf("more than %d entries", maxIndexEntries)
		}
		relative, err := filepath.Rel(root, name)
		if err != nil {
			return err
		}
		if relative = filepath.ToSlash(relative); relative == "." {
			relative = ""
		}
		if entry.IsDir() {
			return index.enterDirectory(root, relative)
		}
		if !strings.HasSuffix(relative, ".go") {
			return nil
		}
		if !entry.Type().IsRegular() {
			// go build/test reads a symlinked .go file like any other; an
			// index that silently skipped it would under-select the file's
			// own dependents. Fail closed to the full run instead.
			return fmt.Errorf("%s is a Go source but not a regular file", relative)
		}
		expected, err := entry.Info()
		if err != nil {
			return err
		}
		files = append(files, indexedFile{relative: relative, expected: expected})
		return nil
	})
	// V1-2169: the files are read, parsed and lexed by a bounded pool, then
	// recorded in walk order, so the index and the first error are those of
	// the serial pass: a file's scan error precedes any walk error after it.
	imports := map[string]map[string]bool{}
	for _, scanned := range scanFiles(root, module, files) {
		if scanned.err != nil {
			return nil, scanned.err
		}
		index.record(scanned, imports)
	}
	if walkErr != nil {
		return nil, walkErr
	}
	index.link(imports)
	return index, nil
}

// indexedFile is one Go source the walk found, with the entry it observed.
type indexedFile struct {
	relative string
	expected fs.FileInfo
}

// scannedFile is what one Go source contributes to the index, computed
// without touching it.
type scannedFile struct {
	relative, directory string
	test                bool
	imports             []string
	scan                sourceScan
	tokens              [][]string // pathTokens of each scan.literals entry
	err                 error
}

// scanWorkers bounds the Go sources read and lexed at once.
const scanWorkers = 8

// scanFiles scans every file, each result at its file's offset.
func scanFiles(root, module string, files []indexedFile) []scannedFile {
	results := make([]scannedFile, len(files))
	workers := min(runtime.GOMAXPROCS(0), scanWorkers, len(files))
	if workers < 2 {
		for offset, file := range files {
			results[offset] = scanFile(root, module, file)
		}
		return results
	}
	var next atomic.Int64
	var pending sync.WaitGroup
	for range workers {
		pending.Add(1)
		go func() {
			defer pending.Done()
			for offset := int(next.Add(1)) - 1; offset < len(files); offset = int(next.Add(1)) - 1 {
				results[offset] = scanFile(root, module, files[offset])
			}
		}()
	}
	pending.Wait()
	return results
}

func (index *repositoryIndex) enterDirectory(root, relative string) error {
	if relative == "" {
		return nil
	}
	if hiddenDirectory(path.Base(relative)) {
		return filepath.SkipDir
	}
	if exists(filepath.Join(root, relative, "go.mod")) {
		index.nested = append(index.nested, relative)
		return filepath.SkipDir
	}
	return nil
}

func scanFile(root, module string, source indexedFile) scannedFile {
	relative := source.relative
	scanned := scannedFile{relative: relative, directory: parentDirectory(relative), test: strings.HasSuffix(relative, "_test.go")}
	body, err := readBounded(filepath.Join(root, filepath.FromSlash(relative)), source.expected)
	if err != nil {
		scanned.err = err
		return scanned
	}
	file, err := parser.ParseFile(token.NewFileSet(), relative, body, parser.ImportsOnly)
	if err != nil {
		scanned.err = fmt.Errorf("%s: imports do not parse", relative)
		return scanned
	}
	for _, spec := range file.Imports {
		value, _ := strconv.Unquote(spec.Path.Value)
		scanned.imports = append(scanned.imports, value)
	}
	aliases, dotImported := rootLocatorImports(file.Imports)
	scanned.scan = scanSource(body, aliases, dotImported)
	scanned.tokens = make([][]string, len(scanned.scan.literals))
	for offset, literal := range scanned.scan.literals {
		scanned.tokens[offset] = pathTokens(literal, module)
	}
	return scanned
}

// record adds one scanned file to the index.
func (index *repositoryIndex) record(scanned scannedFile, imports map[string]map[string]bool) {
	relative, directory, test := scanned.relative, scanned.directory, scanned.test
	pkg := index.packages[directory]
	if pkg == nil {
		pkg = &goPackage{}
		index.packages[directory] = pkg
		imports[directory] = map[string]bool{}
	}
	for _, value := range scanned.imports {
		imports[directory][value] = imports[directory][value] || !test
	}
	scan := scanned.scan
	if scan.err != nil {
		pkg.markUnresolved(relative+" does not tokenize", test)
	}
	for _, call := range scan.calls {
		pkg.markUnresolved(relative+" calls "+call, test)
	}
	pkg.embeds = pkg.embeds || (scan.embeds && !test)
	for _, values := range scanned.tokens {
		for _, value := range values {
			index.holders[value] = appendOnce(index.holders[value], directory)
			if !test {
				index.nonTestHolders[value] = appendOnce(index.nonTestHolders[value], directory)
			}
			if reason := escapesPackage(directory, value, test); reason != "" {
				pkg.markUnresolved(relative+" "+reason, test)
			}
		}
	}
}

func (pkg *goPackage) markUnresolved(reason string, test bool) {
	if pkg.unresolved == "" || (!test && !pkg.locatesRoot) {
		pkg.unresolved = reason
	}
	pkg.locatesRoot = pkg.locatesRoot || !test
}

func readBounded(name string, expected fs.FileInfo) ([]byte, error) {
	file, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil {
		return nil, err
	}
	named, err := os.Lstat(name)
	if err != nil {
		return nil, err
	}
	if !expected.Mode().IsRegular() || !opened.Mode().IsRegular() || !named.Mode().IsRegular() ||
		!os.SameFile(opened, expected) || !os.SameFile(opened, named) {
		return nil, errors.New("a Go source changed before its bounded read")
	}
	body, err := io.ReadAll(io.LimitReader(file, maxSourceBytes+1))
	if err == nil && len(body) > maxSourceBytes {
		err = fmt.Errorf("a Go file exceeds %d bytes", maxSourceBytes)
	}
	return body, err
}

type sourceScan struct {
	literals []string
	calls    []string // root-locator functions the file calls
	embeds   bool
	err      error
}

// rootLocatorImports resolves a file's own import specs against
// rootLocatorFuncs: aliases maps a local qualifier (the import's default
// name, or its alias) to the import path it names; dotImported marks an
// import path pulled in with `import . "path"`, whose exported names appear
// unqualified. A blank (`_`) import brings no identifier into scope.
func rootLocatorImports(specs []*ast.ImportSpec) (aliases map[string]string, dotImported map[string]bool) {
	aliases = map[string]string{}
	dotImported = map[string]bool{}
	for _, spec := range specs {
		value, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			continue
		}
		if _, ok := rootLocatorFuncs[value]; !ok {
			continue
		}
		switch {
		case spec.Name == nil:
			aliases[value] = value
		case spec.Name.Name == ".":
			dotImported[value] = true
		case spec.Name.Name == "_":
			// no identifier reaches source
		default:
			aliases[spec.Name.Name] = value
		}
	}
	return aliases, dotImported
}

// scanSource lexes one Go file for its string literal values outside import
// declarations, its root-locating calls (`os.Getwd`, under
// whatever local name the file's own imports gave them — a plain import, an
// alias, or a dot import), and any //go:embed directive.
func scanSource(body []byte, aliases map[string]string, dotImported map[string]bool) sourceScan {
	var scan sourceScan
	var lexer scanner.Scanner
	lexer.Init(token.NewFileSet().AddFile("", -1, len(body)), body, func(_ token.Position, message string) { scan.err = errors.New(message) }, scanner.ScanComments)
	recent := [3]string{}
	importing := importOutside
	for {
		_, kind, text := lexer.Scan()
		if kind == token.EOF {
			return scan
		}
		inImport := importing != importOutside
		if importing = importState(importing, kind); inImport {
			continue
		}
		recent = [3]string{recent[1], recent[2], kind.String() + text}
		if kind == token.IDENT && recent[1] == "." {
			qualifier := strings.TrimPrefix(recent[0], "IDENT")
			if importPath, ok := aliases[qualifier]; ok && rootLocatorFuncs[importPath] == text {
				scan.calls = append(scan.calls, importPath+"."+text)
			}
		} else if kind == token.IDENT {
			for importPath := range dotImported {
				if rootLocatorFuncs[importPath] == text {
					scan.calls = append(scan.calls, importPath+"."+text+" (dot import)")
				}
			}
		}
		scan.embeds = scan.embeds || (kind == token.COMMENT && strings.HasPrefix(text, "//go:embed"))
		if kind != token.STRING {
			continue
		}
		if value, err := strconv.Unquote(text); err == nil {
			scan.literals = append(scan.literals, value)
		}
	}
}

const (
	importOutside = iota
	importSpec    // after `import`, before its path or `(`
	importBlock   // inside `import ( ... )`
)

// importState advances a recognizer over import declarations, so that an
// import path is an edge and never a path token.
func importState(state int, kind token.Token) int {
	switch {
	case kind == token.IMPORT:
		return importSpec
	case kind == token.COMMENT:
		return state
	case state == importSpec && kind == token.LPAREN:
		return importBlock
	case state == importSpec && kind == token.STRING:
		return importOutside
	case state == importBlock && kind == token.RPAREN:
		return importOutside
	}
	return state
}

// pathTokens splits a literal into path-shaped tokens with printf verbs
// removed; an import path under the module becomes a root-anchored path.
func pathTokens(literal, module string) []string {
	var tokens []string
	for _, value := range pathToken.FindAllString(printfVerb.ReplaceAllString(literal, " "), -1) {
		if value == module {
			value = "/"
		} else if rest, ok := strings.CutPrefix(value, module+"/"); ok {
			value = "/" + rest
		}
		tokens = append(tokens, value)
	}
	return tokens
}

// escapesPackage names why a token read from directory's files cannot be
// bounded by its components. Any file: it names the root lookup. A test file
// (run with its package directory as the working directory): it is a package
// pattern, it is made only of `..` components (which compose to any ancestor),
// or it resolves to the repository root or above. A non-test file: it climbs
// with `..` to a named component, which resolves against whichever package's
// test runs it. Production code spells a bare `..` or `../` to reject a path.
func escapesPackage(directory, value string, test bool) string {
	if value == rootLocatorToken || (test && strings.HasSuffix(value, "/...")) {
		return fmt.Sprintf("carries the literal %q", value)
	}
	climbs, parents := false, true
	for _, component := range strings.Split(value, "/") {
		climbs = climbs || component == ".."
		parents = parents && (component == ".." || component == "." || component == "")
	}
	if strings.HasPrefix(value, "/") || !climbs || (parents && !test) {
		return ""
	}
	if !test {
		return fmt.Sprintf("carries the literal %q, which climbs from the working directory", value)
	}
	resolved := path.Join(directory, value)
	if parents || resolved == "." || resolved == ".." || strings.HasPrefix(resolved, "../") {
		return fmt.Sprintf("carries the literal %q, which reaches the repository root", value)
	}
	return ""
}

func appendOnce(values []string, value string) []string {
	if len(values) > 0 && values[len(values)-1] == value {
		return values
	}
	return append(values, value)
}

// link records the three edge classes that reach a package. importers are
// module imports in non-test files; namers are non-test files carrying a
// root-module literal ("/") or a path token whose component run names another
// package directory (a non-test file that builds `./cmd/corvint` depends on
// that package as surely as an importer does); testImporters are packages
// whose _test.go files alone import or name it. A test file is never
// imported, so nothing reaches a package's own importers through it: a
// test-only edge selects the package holding the test and stops there, as the
// planner's testUsersOf does (V1-0059, V1-2727). readers() and
// resolvingReaders() still see every holder, test or not, and a component-run
// token that names a package directory is also a literal readers() can match
// against a dirty path in that same directory, so a package whose test merely
// names a dirty path remains selected directly.
func (index *repositoryIndex) link(imports map[string]map[string]bool) {
	for directory, set := range imports {
		for value, nonTest := range set {
			if target, ok := index.directoryOf(value); ok && target != directory {
				if nonTest {
					index.importers[target] = append(index.importers[target], directory)
				} else {
					index.testImporters[target] = append(index.testImporters[target], directory)
				}
			}
		}
	}
	for _, directory := range index.holders["/"] {
		if directory == "" {
			continue
		}
		if slices.Contains(index.nonTestHolders["/"], directory) {
			index.namers[""] = append(index.namers[""], directory)
		} else {
			index.testImporters[""] = append(index.testImporters[""], directory)
		}
	}
	for value, directories := range index.nonTestHolders {
		if value == "/" {
			continue
		}
		for _, run := range componentRuns(value) {
			target := strings.Join(run.components, "/")
			if index.packages[target] == nil {
				continue
			}
			for _, directory := range directories {
				if directory != target {
					index.namers[target] = append(index.namers[target], directory)
				}
			}
		}
	}
}

func (index *repositoryIndex) directoryOf(importPath string) (string, bool) {
	if importPath == index.module {
		return "", true
	}
	return strings.CutPrefix(importPath, index.module+"/")
}

// dependents is the reverse closure of directories over importers and
// namers, then one level of testImporters of every directory reached, seeds
// included, excluding a seed unless an edge returns to it. It is the reach of
// rules (a) and (b).
func (index *repositoryIndex) dependents(directories ...string) []string {
	return index.reach(true, directories)
}

// reach is the planner's traverse-then-testUsersOf walk (internal/liveverify/
// affected, Graph.traverse and Graph.testUsersOf) over this index's edges:
// the closure over non-test edges, namers too when withNamers, and then the
// packages whose tests alone import or name a reached package, never expanded
// further because their importers do not compile their tests (V1-2727).
func (index *repositoryIndex) reach(withNamers bool, directories []string) []string {
	seen := map[string]bool{}
	queue := append([]string(nil), directories...)
	for len(queue) > 0 {
		next := queue[0]
		queue = queue[1:]
		edges := index.importers[next]
		if withNamers {
			edges = slices.Concat(edges, index.namers[next])
		}
		for _, importer := range edges {
			if !seen[importer] {
				seen[importer] = true
				queue = append(queue, importer)
			}
		}
	}
	reached := slices.Concat(directories, sortedKeys(seen))
	for _, directory := range reached {
		for _, user := range index.testImporters[directory] {
			seen[user] = true
		}
	}
	return sortedKeys(seen)
}

// readers are the packages whose own files carry a path token naming
// dirtyPath. A dependency's literal is resolved against a root or directory its
// caller hands it; a dependency that finds the root itself is `locatesRoot`.
func (index *repositoryIndex) readers(dirtyPath string) []string {
	readers := map[string]bool{}
	for value, directories := range index.holders {
		if !namesPath(value, dirtyPath) {
			continue
		}
		for _, directory := range directories {
			readers[directory] = true
		}
	}
	return sortedKeys(readers)
}

// pathReaders is readers for every path at once: the same namesPath relation
// evaluated token by token through one pathMatcher, so the paths are split
// once and each token's runs once rather than once per path.
func (index *repositoryIndex) pathReaders(paths []string) [][]string {
	sets := make([]map[string]bool, len(paths))
	matcher := newPathMatcher(paths)
	for value, directories := range index.holders {
		for _, i := range matcher.named(value) {
			if sets[i] == nil {
				sets[i] = map[string]bool{}
			}
			for _, directory := range directories {
				sets[i][directory] = true
			}
		}
	}
	readers := make([][]string, len(paths))
	for i := range paths {
		readers[i] = sortedKeys(sets[i])
	}
	return readers
}

// resolvingReaders are the readers of dirtyPath whose token, resolved against
// the holder's own directory (a root-anchored token against the root), can
// form dirtyPath or one of its ancestor directories. A separate root anchor
// in the same package may compose with a partial naming token; the package is
// the narrowest relation the literal-only index can prove. Naming is the
// planner's (namesResolvedPath, AFP-V0-931): a lone component names only the
// file name, so a module-path `/` anchor beside a bare `.corvint` token does
// not make its holder a reader of every sealed CEM.
func (index *repositoryIndex) resolvingReaders(dirtyPath string) []string {
	readers := map[string]bool{}
	anchors := index.rootAnchors(dirtyPath)
	climbs := index.climbingAnchors()
	for value, directories := range index.holders {
		if !namesPath(value, dirtyPath) {
			continue
		}
		// A lone directory component composes with the root only beside a
		// parent-only token, which can climb there from the package
		// directory; a `/` anchor alone (the module path) cannot
		// (AFP-V0-931).
		named := namesResolvedPath(value, dirtyPath)
		for _, directory := range directories {
			if resolvesWithin(directory, value, dirtyPath) ||
				(anchors[directory] && resolvesWithin("", value, dirtyPath) && (named || climbs[directory])) {
				readers[directory] = true
			}
		}
	}
	return sortedKeys(readers)
}

// holderReaders are the packages holding a literal that, resolved against
// the holder's own directory, is dirtyPath or one of its ancestor directories
// other than the repository root: the readers a plan's reader selection must
// keep before SelectStructural takes it (ADN-V0-010, ADN-V0-011).
func (index *repositoryIndex) holderReaders(dirtyPath string) []string {
	readers := map[string]bool{}
	for value, directories := range index.holders {
		if strings.HasPrefix(value, "/") || !namesPath(value, dirtyPath) {
			continue
		}
		for _, directory := range directories {
			resolved := path.Join(directory, value)
			if resolved == "." || resolved == ".." || strings.HasPrefix(resolved, "../") {
				continue
			}
			if resolved == dirtyPath || strings.HasPrefix(dirtyPath, resolved+"/") {
				readers[directory] = true
			}
		}
	}
	return sortedKeys(readers)
}

func resolvesWithin(directory, value, dirtyPath string) bool {
	resolved := path.Join(directory, value)
	if rest, ok := strings.CutPrefix(value, "/"); ok {
		resolved = path.Clean(rest)
	}
	if resolved == "." {
		resolved = ""
	}
	if resolved == "" || resolved == dirtyPath || strings.HasPrefix(dirtyPath, resolved+"/") {
		return true
	}

	// componentRuns permits the outer components of a multi-component token
	// to be fragments because an adjacent expression may complete them. Keep
	// that contract after resolution while requiring every component position
	// to line up from the repository root.
	resolvedParts := strings.Split(resolved, "/")
	dirtyParts := strings.Split(dirtyPath, "/")
	if len(resolvedParts) < 2 || len(resolvedParts) != len(dirtyParts) {
		return false
	}
	for position, component := range resolvedParts {
		switch {
		case position == 0:
			if !strings.HasSuffix(dirtyParts[position], component) {
				return false
			}
		case position == len(resolvedParts)-1:
			if !strings.HasPrefix(dirtyParts[position], component) {
				return false
			}
		case dirtyParts[position] != component:
			return false
		}
	}
	return true
}

// rootAnchors reports packages where a token could put an adjacent path
// fragment at the repository root. Parent-only tokens are kept conservative
// because the scanner intentionally does not reconstruct the expression that
// may repeat or combine them.
func (index *repositoryIndex) rootAnchors(dirtyPath string) map[string]bool {
	anchors := map[string]bool{}
	for value, directories := range index.holders {
		rooted := parentOnly(value)
		if rest, ok := strings.CutPrefix(value, "/"); ok {
			rooted = rest == "" || strings.HasPrefix(dirtyPath, path.Clean(rest))
		}
		if rooted {
			for _, directory := range directories {
				anchors[directory] = true
			}
		}
	}
	return anchors
}

// climbingAnchors reports the packages holding a parent-only token, the
// rootAnchors that can climb from the package directory to the root.
func (index *repositoryIndex) climbingAnchors() map[string]bool {
	climbs := map[string]bool{}
	for value, directories := range index.holders {
		if parentOnly(value) {
			for _, directory := range directories {
				climbs[directory] = true
			}
		}
	}
	return climbs
}

func parentOnly(value string) bool {
	parent := false
	for _, component := range strings.Split(value, "/") {
		switch component {
		case "", ".":
		case "..":
			parent = true
		default:
			return false
		}
	}
	return parent
}

// nestedModule reports whether dirtyPath lies under a nested go.mod, outside
// the root module that `go test ./...` covers.
func (index *repositoryIndex) nestedModule(dirtyPath string) bool {
	for _, directory := range index.nested {
		if strings.HasPrefix(dirtyPath, directory+"/") {
			return true
		}
	}
	return false
}

// enclosing lists every package directory that is an ancestor of dirtyPath,
// through testdata and hidden components, nearest first.
func (index *repositoryIndex) enclosing(dirtyPath string) []string {
	var directories []string
	for directory := parentDirectory(dirtyPath); ; directory = parentDirectory(directory) {
		if index.packages[directory] != nil {
			directories = append(directories, directory)
		}
		if directory == "" {
			return directories
		}
	}
}

// unresolved maps each package whose reads no literal bounds to the reason:
// its own root-locating file, or a dependency whose non-test code locates the
// root and so reads from it when the package's tests call it. The dependency
// is reached as the planner's Graph.unboundedReaders reaches it: over non-test
// import edges, then one level of test importers; a path token naming the
// locator's directory links no code to it, so namers are not followed
// (V1-2727).
func (index *repositoryIndex) unresolved() map[string]string {
	reasons := map[string]string{}
	var locators []string
	for directory, pkg := range index.packages {
		if pkg.unresolved != "" {
			reasons[directory] = pkg.unresolved
		}
		if pkg.locatesRoot {
			locators = append(locators, directory)
		}
	}
	sort.Strings(locators)
	for _, locator := range locators {
		for _, dependent := range index.reach(false, []string{locator}) {
			if reasons[dependent] == "" {
				reasons[dependent] = "depends on " + importPath(index.module, locator) + ", whose " + index.packages[locator].unresolved
			}
		}
	}
	return reasons
}

type run struct {
	components                []string
	partialFirst, partialLast bool
}

// componentRuns splits a token at empty, `.`, and `..` components. A run of two
// or more components may begin or end mid-name where the token does (a literal
// concatenated with a variable), so its outer components match by suffix and
// prefix; a lone component must match a whole path component.
func componentRuns(value string) []run {
	components := strings.Split(value, "/")
	var runs []run
	start := 0
	for end := 0; end <= len(components); end++ {
		if end < len(components) && components[end] != "" && components[end] != "." && components[end] != ".." {
			continue
		}
		if end > start {
			long := end-start >= 2
			runs = append(runs, run{components: components[start:end], partialFirst: long && start == 0, partialLast: long && end == len(components)})
		}
		start = end + 1
	}
	return runs
}

// namesResolvedPath is namesPath with the planner's lone-component rule
// (internal/liveverify/affected namesPath, V1-0290), which resolvingReaders
// mirrors (AFP-V0-931, V1-3146): a single-component run of a token that is
// neither anchored nor climbing is tried against the file name only, never a
// directory component, so the `.corvint` of a fixture-root join does not name
// `.corvint/changes/<commit>.cem.json`. A reader that composes such a
// component with a located repository root reads through a root locator,
// which rule (d) selects whatever this returns.
func namesResolvedPath(value, dirtyPath string) bool {
	parts := strings.Split(dirtyPath, "/")
	nameOnly := !strings.HasPrefix(value, "/") && !strings.Contains(value, "..")
	for _, candidate := range componentRuns(value) {
		first := 0
		if len(candidate.components) == 1 && nameOnly {
			first = len(parts) - 1
		}
		for offset := first; offset+len(candidate.components) <= len(parts); offset++ {
			if candidate.matchesAt(parts[offset:]) {
				return true
			}
		}
	}
	return false
}

func namesPath(value, dirtyPath string) bool {
	parts := strings.Split(dirtyPath, "/")
	for _, candidate := range componentRuns(value) {
		for offset := 0; offset+len(candidate.components) <= len(parts); offset++ {
			if candidate.matchesAt(parts[offset:]) {
				return true
			}
		}
	}
	return false
}

func (candidate run) matchesAt(parts []string) bool {
	last := len(candidate.components) - 1
	for position, component := range candidate.components {
		switch {
		case position == 0 && candidate.partialFirst && position != last:
			if !strings.HasSuffix(parts[position], component) {
				return false
			}
		case position == last && candidate.partialLast:
			if !strings.HasPrefix(parts[position], component) {
				return false
			}
		default:
			if parts[position] != component {
				return false
			}
		}
	}
	return true
}

// pathMatcher answers namesPath for one token against many paths at once:
// the paths are split once, and a run with an exact component (one matched
// whole, not by suffix or prefix) is tried only at the positions carrying that
// component, so the relation stays namesPath's while the work per token is
// proportional to the paths that can match.
type pathMatcher struct {
	parts    [][]string
	occurred map[string][]occurrence // component -> where it occurs
}

type occurrence struct {
	path, position int
}

func newPathMatcher(paths []string) *pathMatcher {
	m := &pathMatcher{occurred: map[string][]occurrence{}}
	for i, p := range paths {
		parts := strings.Split(p, "/")
		m.parts = append(m.parts, parts)
		for j, component := range parts {
			m.occurred[component] = append(m.occurred[component], occurrence{i, j})
		}
	}
	return m
}

// named returns the indices, ascending, of the paths namesPath(value, path)
// holds for.
func (m *pathMatcher) named(value string) []int {
	matched := map[int]bool{}
	for _, candidate := range componentRuns(value) {
		exact := candidate.exactPosition()
		if exact < 0 {
			for i, parts := range m.parts {
				if !matched[i] && candidate.matchesAnywhere(parts) {
					matched[i] = true
				}
			}
			continue
		}
		for _, at := range m.occurred[candidate.components[exact]] {
			offset := at.position - exact
			if matched[at.path] || offset < 0 || offset+len(candidate.components) > len(m.parts[at.path]) {
				continue
			}
			if candidate.matchesAt(m.parts[at.path][offset:]) {
				matched[at.path] = true
			}
		}
	}
	indices := make([]int, 0, len(matched))
	for i := range matched {
		indices = append(indices, i)
	}
	sort.Ints(indices)
	return indices
}

// exactPosition is a position matchesAt compares whole, or -1 when the run is
// two components matched by suffix and prefix only.
func (candidate run) exactPosition() int {
	last := len(candidate.components) - 1
	for position := range candidate.components {
		if (position == 0 && candidate.partialFirst && position != last) || (position == last && candidate.partialLast) {
			continue
		}
		return position
	}
	return -1
}

func (candidate run) matchesAnywhere(parts []string) bool {
	for offset := 0; offset+len(candidate.components) <= len(parts); offset++ {
		if candidate.matchesAt(parts[offset:]) {
			return true
		}
	}
	return false
}
