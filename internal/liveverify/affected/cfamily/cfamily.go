// Package cfamily is the C, C++, Objective-C, Objective-C++ and JNI
// implementation of the affected-selection language seam (NSA-V0-001..005).
//
// A unit is one file, because these languages share headers and no single
// compilation unit owns one. Edges are read from #include and #import
// directives in source text; no compiler, preprocessor or build system is run,
// so the graph can be rebuilt on a dirty worktree with no toolchain present.
//
// Which objects a test binary links is decided by a build system this plugin
// does not read, so every plan cfamily takes part in stays UNKNOWN through
// FrontierLinkGraph even when it selects tests.
package cfamily

import (
	"bytes"
	"context"
	"errors"
	"path"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// Frontier reasons this plugin can raise.
const (
	// FrontierAmbiguousInclude reports a quoted include that resolved to more
	// than one repository file. Every candidate edge is kept.
	FrontierAmbiguousInclude = "cfamily:ambiguous-include"
	// FrontierUnresolvedInclude reports a quoted include that resolved to no
	// repository file.
	FrontierUnresolvedInclude = "cfamily:unresolved-include"
	// FrontierMacroInclude reports an include whose target is a macro, which
	// only the preprocessor can expand.
	FrontierMacroInclude = "cfamily:macro-include"
	// FrontierUnreadableSource reports a file that could not be read or decoded.
	FrontierUnreadableSource = "cfamily:unreadable-source"
	// FrontierJNIBoundary is a unit frontier on a file declaring a JNIEXPORT
	// function: the JVM tests that load it reach it through no include.
	FrontierJNIBoundary = "cfamily:jni-boundary"
	// FrontierLinkGraph is the permanent frontier: the objects a test binary
	// links are decided by a build system (CMake, Make, Bazel, Xcode, SwiftPM,
	// cgo, Gradle, a Cargo build script) that this plugin does not read.
	FrontierLinkGraph = "cfamily:link-graph-unmodelled"
)

// headerSuffixes name files that are included rather than compiled alone. A
// header is never a test.
var headerSuffixes = []string{".h", ".hh", ".hpp", ".hxx", ".h++", ".inl", ".ipp", ".tpp"}

// implementationSuffixes name files a compiler takes as a translation unit.
var implementationSuffixes = []string{".c", ".cc", ".cpp", ".cxx", ".c++", ".m", ".mm"}

// testDirectories are the directory components that mark a conventional test
// path (NSA-V0-002).
var testDirectories = map[string]bool{"test": true, "tests": true, "Tests": true, "UnitTests": true, "spec": true}

// testStemSuffixes are the file-stem suffixes that mark a conventional test path.
var testStemSuffixes = []string{"_test", "_tests", "_unittest", "Test", "Tests"}

// maxFallbackCandidates bounds the suffix matches kept for a quoted include the
// specified search could not resolve; past it only the frontier is raised.
const maxFallbackCandidates = 64

var (
	directivePattern = regexp.MustCompile(`(?m)^[ \t]*#[ \t]*(?:include_next|include|import)\b`)
	testSignals      = []*regexp.Regexp{
		// GoogleTest, and CppUTest's TEST.
		regexp.MustCompile(`\b(?:TEST|TEST_F|TEST_P|TYPED_TEST|TYPED_TEST_P)\s*\(`),
		// Catch2 and doctest.
		regexp.MustCompile(`\b(?:TEST_CASE|TEST_CASE_METHOD|TEMPLATE_TEST_CASE|SCENARIO)\s*\(`),
		// Boost.Test.
		regexp.MustCompile(`\bBOOST_(?:AUTO|FIXTURE|DATA)_TEST_CASE\s*\(`),
		// CppUTest groups.
		regexp.MustCompile(`\bTEST_GROUP\s*\(`),
		// Unity.
		regexp.MustCompile(`\bRUN_TEST\s*\(`),
		// Check.
		regexp.MustCompile(`\bSTART_TEST\s*\(`),
		// CUnit.
		regexp.MustCompile(`\bCU_add_test\s*\(`),
		// cmocka.
		regexp.MustCompile(`\bcmocka_unit_test\w*\s*\(`),
		// A main function: a CTest executable in a conventional test path.
		regexp.MustCompile(`\b(?:int|void|auto)\s+main\s*\(`),
	}
	// xctestSubclass is an Objective-C interface whose superclass is XCTestCase.
	xctestSubclass = regexp.MustCompile(`@interface\s+\w+\s*:\s*XCTestCase\b`)
	jniExport      = regexp.MustCompile(`\bJNIEXPORT\b`)
)

// Language observes C-family source and header files under one repository root.
type Language struct{}

// New returns the C-family language plugin.
func New() Language { return Language{} }

// Name is the plugin namespace.
func (Language) Name() string { return "cfamily" }

// Owns reports whether a path is C, C++, Objective-C or Objective-C++ source
// text. Shader sources are deliberately left unclaimed.
func (Language) Owns(relative string) bool { return owned(relative) }

func owned(name string) bool {
	return isHeader(name) || hasAnySuffix(name, implementationSuffixes)
}

func isHeader(name string) bool { return hasAnySuffix(name, headerSuffixes) }

func hasAnySuffix(name string, suffixes []string) bool {
	for _, suffix := range suffixes {
		if strings.HasSuffix(name, suffix) && len(name) > len(suffix) {
			return true
		}
	}
	return false
}

// observation is what one file's text says before includes are resolved.
type observation struct {
	relative string
	test     bool
	jni      bool
	quoted   []string
	angled   []string
}

// Units observes every C-family file in the repository rooted at root.
func (language Language) Units(ctx context.Context, root string) (affected.Result, error) {
	return language.UnitsSource(affected.DiskSourceContext(ctx, root))
}

// UnitsSource reads only the explicitly supplied source universe.
func (language Language) UnitsSource(root *affected.Source) (affected.Result, error) {
	files, err := root.Files(affected.ByName(owned))
	if err != nil {
		return affected.Result{}, err
	}
	if len(files) == 0 {
		return affected.Result{Units: []affected.Unit{}}, nil
	}
	// Every repository file, so an include of a file no plugin owns (an X-macro
	// .def, a generated .inc) counts as resolved rather than unresolved. A dirty
	// path there is unowned and widens the plan on its own.
	everything, err := root.Files(func(_, _ string) bool { return true })
	if err != nil {
		return affected.Result{}, err
	}
	frontier := map[string]bool{FrontierLinkGraph: true}
	observed := make([]observation, 0, len(files))
	for _, relative := range files {
		body, readErr := root.Read(relative)
		if errors.Is(readErr, context.Canceled) || errors.Is(readErr, context.DeadlineExceeded) {
			return affected.Result{}, readErr
		}
		if readErr != nil {
			frontier[FrontierUnreadableSource] = true
			observed = append(observed, observation{relative: relative})
			continue
		}
		if !utf8.Valid(body) {
			// Directives are ASCII, so the bytes are still scanned for edges.
			frontier[FrontierUnreadableSource] = true
		}
		item, flags := observe(relative, body)
		for flag := range flags {
			frontier[flag] = true
		}
		observed = append(observed, item)
	}
	index := newPathIndex(files, everything)
	units := make([]affected.Unit, 0, len(observed))
	for _, item := range observed {
		unit := affected.Unit{ID: unitID(item.relative)}
		if item.test {
			unit.Tests = []string{item.relative}
		} else {
			unit.Sources = []string{item.relative}
		}
		if item.jni {
			unit.Frontier = []string{FrontierJNIBoundary}
		}
		edges := index.resolve(item, frontier)
		delete(edges, item.relative)
		unit.Imports = make([]string, 0, len(edges))
		for _, target := range sortedKeys(edges) {
			unit.Imports = append(unit.Imports, unitID(target))
		}
		units = append(units, unit)
	}
	sort.Slice(units, func(left, right int) bool { return units[left].ID < units[right].ID })
	return affected.Result{Units: units, Frontier: sortedKeys(frontier)}, nil
}

func unitID(relative string) string { return "cfamily:" + relative }

// observe classifies one file and lists its include targets.
func observe(relative string, body []byte) (observation, map[string]bool) {
	// Splice continued lines first, as translation phase 2 does, so a
	// directive continued with a backslash is read whole.
	body = bytes.ReplaceAll(bytes.ReplaceAll(body, []byte("\\\r\n"), nil), []byte("\\\n"), nil)
	code, masked := scrub(body)
	item := observation{relative: relative, jni: jniExport.Match(masked)}
	item.test = !isHeader(relative) && testPath(relative) && hasTestSignal(relative, masked)
	flags := map[string]bool{}
	for _, match := range directivePattern.FindAllIndex(masked, -1) {
		end := match[1]
		for end < len(code) && code[end] != '\n' {
			end++
		}
		rest := strings.TrimLeft(string(code[match[1]:end]), " \t")
		switch {
		case strings.HasPrefix(rest, `"`):
			closing := strings.IndexByte(rest[1:], '"')
			if closing < 0 {
				flags[FrontierUnresolvedInclude] = true
				continue
			}
			item.quoted = append(item.quoted, rest[1:1+closing])
		case strings.HasPrefix(rest, "<"):
			if closing := strings.IndexByte(rest, '>'); closing > 1 {
				item.angled = append(item.angled, rest[1:closing])
			}
		case rest != "" && (rest[0] == '_' || isLetter(rest[0])):
			flags[FrontierMacroInclude] = true
		}
	}
	return item, flags
}

// testPath applies the conventional test-path rule of NSA-V0-002.
func testPath(relative string) bool {
	for _, component := range strings.Split(path.Dir(relative), "/") {
		if testDirectories[component] {
			return true
		}
	}
	base := path.Base(relative)
	stem := strings.TrimSuffix(base, path.Ext(base))
	if strings.HasPrefix(stem, "test_") {
		return true
	}
	for _, suffix := range testStemSuffixes {
		if strings.HasSuffix(stem, suffix) && len(stem) > len(suffix) {
			return true
		}
	}
	return false
}

// hasTestSignal reports a runnable-test signal in comment- and string-free
// text. An XCTestCase subclass counts only in Objective-C sources.
func hasTestSignal(relative string, masked []byte) bool {
	for _, signal := range testSignals {
		if signal.Match(masked) {
			return true
		}
	}
	objectiveC := strings.HasSuffix(relative, ".m") || strings.HasSuffix(relative, ".mm")
	return objectiveC && xctestSubclass.Match(masked)
}

// pathIndex resolves include targets against the source universe.
type pathIndex struct {
	units map[string]bool
	all   map[string]bool
	// byBase lists the unit files under each base name, for suffix matching.
	byBase map[string][]string
}

func newPathIndex(files, everything []string) pathIndex {
	index := pathIndex{
		units:  make(map[string]bool, len(files)),
		all:    make(map[string]bool, len(everything)),
		byBase: make(map[string][]string, len(files)),
	}
	for _, file := range files {
		index.units[file] = true
		index.byBase[path.Base(file)] = append(index.byBase[path.Base(file)], file)
	}
	for _, file := range everything {
		index.all[file] = true
	}
	return index
}

// resolve returns the unit files one observation imports: its resolved
// includes and, for a header, its implementation counterparts (NSA-V0-003).
func (index pathIndex) resolve(item observation, frontier map[string]bool) map[string]bool {
	edges := map[string]bool{}
	add := func(candidate string) {
		if index.units[candidate] {
			edges[candidate] = true
		}
	}
	for _, target := range item.quoted {
		for _, candidate := range index.quoted(item.relative, target, frontier) {
			add(candidate)
		}
	}
	for _, target := range item.angled {
		// An angle include is a repository file only when exactly one file has
		// that path suffix; otherwise it is external.
		if matches := index.suffixMatches(target); len(matches) == 1 {
			add(matches[0])
		}
	}
	if isHeader(item.relative) {
		for _, candidate := range counterparts(item.relative) {
			add(candidate)
		}
	}
	return edges
}

// quoted resolves one quoted include: relative to the including file's
// directory first, then relative to the repository root and to each ancestor
// include or src directory. A target found by none of those keeps every unique
// suffix match as an edge, because a missed edge narrows selection, and still
// raises the unresolved frontier.
func (index pathIndex) quoted(owner, target string, frontier map[string]bool) []string {
	target = strings.ReplaceAll(target, `\`, "/")
	if target == "" || strings.HasPrefix(target, "/") {
		frontier[FrontierUnresolvedInclude] = true
		return nil
	}
	directory := path.Dir(owner)
	if beside := path.Join(directory, target); index.all[beside] {
		return []string{beside}
	}
	found := map[string]bool{}
	consider := func(candidate string) {
		if affected.ValidRelativePath(candidate) && index.all[candidate] {
			found[candidate] = true
		}
	}
	consider(path.Clean(target))
	for ancestor := directory; ; ancestor = path.Dir(ancestor) {
		if base := path.Base(ancestor); base == "include" || base == "src" {
			consider(path.Join(ancestor, target))
		}
		consider(path.Join(ancestor, "include", target))
		consider(path.Join(ancestor, "src", target))
		if ancestor == "." {
			break
		}
	}
	switch len(found) {
	case 0:
		frontier[FrontierUnresolvedInclude] = true
		if matches := index.suffixMatches(target); len(matches) <= maxFallbackCandidates {
			return matches
		}
		return nil
	case 1:
	default:
		frontier[FrontierAmbiguousInclude] = true
	}
	return sortedKeys(found)
}

// suffixMatches lists the unit files equal to target or ending in "/"+target.
func (index pathIndex) suffixMatches(target string) []string {
	cleaned := path.Clean(strings.ReplaceAll(target, `\`, "/"))
	if !affected.ValidRelativePath(cleaned) {
		return nil
	}
	matches := make([]string, 0, 1)
	for _, file := range index.byBase[path.Base(cleaned)] {
		if file == cleaned || strings.HasSuffix(file, "/"+cleaned) {
			matches = append(matches, file)
		}
	}
	return matches
}

// counterparts lists the implementation files a header stands for: the same
// stem with an implementation suffix in the same directory, or in the sibling
// src directory of an enclosing include directory, mirrored or flat.
func counterparts(header string) []string {
	directory, base := path.Split(header)
	stem := strings.TrimSuffix(base, path.Ext(base))
	directories := []string{strings.TrimSuffix(directory, "/")}
	components := strings.Split(strings.TrimSuffix(directory, "/"), "/")
	for position, component := range components {
		if component != "include" {
			continue
		}
		mirrored := append(append(append([]string(nil), components[:position]...), "src"), components[position+1:]...)
		flat := append(append([]string(nil), components[:position]...), "src")
		directories = append(directories, strings.Join(mirrored, "/"), strings.Join(flat, "/"))
	}
	candidates := make([]string, 0, len(directories)*len(implementationSuffixes))
	for _, candidateDirectory := range directories {
		for _, suffix := range implementationSuffixes {
			candidates = append(candidates, path.Join(candidateDirectory, stem+suffix))
		}
	}
	return candidates
}

// scrub returns two views of body with equal length and line structure: code,
// with comments blanked, and masked, with string and character literal
// contents blanked as well. Directives are found in masked, so one inside a
// comment or a string is ignored, and their targets are read from code.
func scrub(body []byte) (code, masked []byte) {
	code = append([]byte(nil), body...)
	masked = append([]byte(nil), body...)
	blank := func(views [][]byte, from, to int) {
		for _, view := range views {
			for position := from; position < to && position < len(view); position++ {
				if view[position] != '\n' {
					view[position] = ' '
				}
			}
		}
	}
	both := [][]byte{code, masked}
	literal := [][]byte{masked}
	for position := 0; position < len(body); {
		current := body[position]
		next := byte(0)
		if position+1 < len(body) {
			next = body[position+1]
		}
		switch {
		case current == '/' && next == '/':
			end := lineCommentEnd(body, position)
			blank(both, position, end)
			position = end
		case current == '/' && next == '*':
			end := bytes.Index(body[position+2:], []byte("*/"))
			if end < 0 {
				end = len(body)
			} else {
				end += position + 4
			}
			blank(both, position, end)
			position = end
		case current == '"':
			end := literalEnd(body, position)
			blank(literal, position+1, end-1)
			position = end
		case current == '\'' && !digitSeparator(body, position):
			end := quotedEnd(body, position, '\'')
			blank(literal, position+1, end-1)
			position = end
		default:
			position++
		}
	}
	return code, masked
}

// lineCommentEnd is the newline ending a // comment, honouring a trailing
// backslash continuation.
func lineCommentEnd(body []byte, start int) int {
	for position := start; position < len(body); position++ {
		if body[position] != '\n' {
			continue
		}
		previous := position - 1
		if previous >= start && body[previous] == '\r' {
			previous--
		}
		if previous >= start && body[previous] == '\\' {
			continue
		}
		return position
	}
	return len(body)
}

// literalEnd is the offset just past the string literal opening at start,
// recognising C++ raw strings (R"delim(...)delim").
func literalEnd(body []byte, start int) int {
	prefixStart := start
	for prefixStart > 0 && isIdentifier(body[prefixStart-1]) {
		prefixStart--
	}
	switch string(body[prefixStart:start]) {
	case "R", "LR", "uR", "UR", "u8R":
		open := start + 1
		for open < len(body) && open-start <= 17 && body[open] != '(' && body[open] != '"' && body[open] != '\n' {
			open++
		}
		if open < len(body) && body[open] == '(' {
			closing := []byte(")" + string(body[start+1:open]) + `"`)
			if end := bytes.Index(body[open+1:], closing); end >= 0 {
				return open + 1 + end + len(closing)
			}
			return len(body)
		}
	}
	return quotedEnd(body, start, '"')
}

// quotedEnd is the offset just past an ordinary literal closed by quote. An
// unterminated literal ends at the newline, as the compiler would reject it.
func quotedEnd(body []byte, start int, quote byte) int {
	for position := start + 1; position < len(body); position++ {
		switch body[position] {
		case '\\':
			position++
		case quote:
			return position + 1
		case '\n':
			return position + 1
		}
	}
	return len(body)
}

// digitSeparator reports a C++14 digit separator (1'000) rather than a
// character literal.
func digitSeparator(body []byte, position int) bool {
	start := position
	for start > 0 && isIdentifier(body[start-1]) {
		start--
	}
	return start < position && body[start] >= '0' && body[start] <= '9'
}

func isLetter(value byte) bool {
	return (value >= 'a' && value <= 'z') || (value >= 'A' && value <= 'Z')
}

func isIdentifier(value byte) bool {
	return value == '_' || isLetter(value) || (value >= '0' && value <= '9')
}

func sortedKeys[Value any](values map[string]Value) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
