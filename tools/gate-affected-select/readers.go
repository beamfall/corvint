package main

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
	"sort"
	"strconv"
	"strings"
)

const (
	maxIndexEntries = 200000
	maxSourceBytes  = 8 << 20
)

// rootLocatorFuncs are the calls that find the repository from a source file
// or the working directory at run time, so no literal bounds what follows:
// import path to the one function name on it that locates the root.
var rootLocatorFuncs = map[string]string{"runtime": "Caller", "os": "Getwd"}

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
	module    string
	packages  map[string]*goPackage // by repository-relative directory, "" for the root
	nested    []string              // directories holding a nested go.mod
	holders   map[string][]string   // path token -> directories whose files carry it
	importers map[string][]string   // directory -> directories importing or naming it
}

func indexRepository(root, module string) (*repositoryIndex, error) {
	index := &repositoryIndex{module: module, packages: map[string]*goPackage{}, holders: map[string][]string{}, importers: map[string][]string{}}
	imports := map[string]map[string]bool{}
	entries := 0
	err := filepath.WalkDir(root, func(name string, entry fs.DirEntry, err error) error {
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
		return index.scanFile(root, relative, expected, imports)
	})
	if err != nil {
		return nil, err
	}
	index.link(imports)
	return index, nil
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

func (index *repositoryIndex) scanFile(root, relative string, expected fs.FileInfo, imports map[string]map[string]bool) error {
	body, err := readBounded(filepath.Join(root, filepath.FromSlash(relative)), expected)
	if err != nil {
		return err
	}
	file, err := parser.ParseFile(token.NewFileSet(), relative, body, parser.ImportsOnly)
	if err != nil {
		return fmt.Errorf("%s: imports do not parse", relative)
	}
	directory := parentDirectory(relative)
	pkg := index.packages[directory]
	if pkg == nil {
		pkg = &goPackage{}
		index.packages[directory] = pkg
		imports[directory] = map[string]bool{}
	}
	for _, spec := range file.Imports {
		value, _ := strconv.Unquote(spec.Path.Value)
		imports[directory][value] = true
	}
	test := strings.HasSuffix(relative, "_test.go")
	aliases, dotImported := rootLocatorImports(file.Imports)
	scan := scanSource(body, aliases, dotImported)
	if scan.err != nil {
		pkg.markUnresolved(relative+" does not tokenize", test)
	}
	for _, call := range scan.calls {
		pkg.markUnresolved(relative+" calls "+call, test)
	}
	pkg.embeds = pkg.embeds || (scan.embeds && !test)
	for _, literal := range scan.literals {
		for _, value := range pathTokens(literal, index.module) {
			index.holders[value] = appendOnce(index.holders[value], directory)
			if reason := escapesPackage(directory, value, test); reason != "" {
				pkg.markUnresolved(relative+" "+reason, test)
			}
		}
	}
	return nil
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
// declarations, its root-locating calls (`runtime.Caller`, `os.Getwd`, under
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

// link records each package's importers: a module import, or a path token
// whose component run names another package directory (a test that builds
// `./cmd/corvint` depends on that package as surely as an importer does).
func (index *repositoryIndex) link(imports map[string]map[string]bool) {
	for directory, set := range imports {
		for value := range set {
			if target, ok := index.directoryOf(value); ok && target != directory {
				index.importers[target] = append(index.importers[target], directory)
			}
		}
	}
	for value, directories := range index.holders {
		if value == "/" {
			for _, directory := range directories {
				if directory != "" {
					index.importers[""] = append(index.importers[""], directory)
				}
			}
			continue
		}
		for _, run := range componentRuns(value) {
			target := strings.Join(run.components, "/")
			if index.packages[target] == nil {
				continue
			}
			for _, directory := range directories {
				if directory != target {
					index.importers[target] = append(index.importers[target], directory)
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

// dependents is the reverse closure of directory over importers, excluding
// directory itself unless a cycle returns to it.
func (index *repositoryIndex) dependents(directories ...string) []string {
	seen := map[string]bool{}
	queue := append([]string(nil), directories...)
	for len(queue) > 0 {
		next := queue[0]
		queue = queue[1:]
		for _, importer := range index.importers[next] {
			if !seen[importer] {
				seen[importer] = true
				queue = append(queue, importer)
			}
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

// resolvingReaders are the readers of dirtyPath whose token, resolved against
// the holder's own directory (a root-anchored token against the root), is
// dirtyPath or one of its ancestor directories: the literal a package's test
// opens in the repository under test rather than in a fixture root it builds.
func (index *repositoryIndex) resolvingReaders(dirtyPath string) []string {
	readers := map[string]bool{}
	for value, directories := range index.holders {
		if !namesPath(value, dirtyPath) {
			continue
		}
		for _, directory := range directories {
			if resolvesWithin(directory, value, dirtyPath) {
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
	return resolved == dirtyPath || strings.HasPrefix(dirtyPath, resolved+"/")
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
// root and so reads from it when the package's tests call it.
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
		for _, dependent := range index.dependents(locator) {
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
