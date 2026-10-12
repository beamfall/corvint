// Package java is the Java implementation of the affected-selection language
// seam.
//
// A unit is one .java file, because a file is where Java resolves imports and
// where JUnit and TestNG discover test classes. Edges are read from package,
// import, and type-name text; no compiler, build tool, or annotation processor
// runs, so the graph can be rebuilt on a dirty worktree with no JDK present.
package java

import (
	"context"
	"path"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// Frontier and unbounded-read reasons this plugin can raise.
const (
	// FrontierUnparsedSource reports a file that could not be read, decoded,
	// or lexed (an unterminated comment, string, or text block).
	FrontierUnparsedSource = "java:unparsed-source"
	// FrontierAmbiguousType reports a reference that resolved to more than one
	// declaring file. Every candidate edge is kept.
	FrontierAmbiguousType = "java:ambiguous-type"
	// FrontierKotlinDependent is a unit frontier on a Java file whose types
	// Kotlin source references: the Kotlin plugin owns those dependents, and no
	// edge crosses the two namespaces.
	FrontierKotlinDependent = "java:kotlin-dependent"
	// UnresolvedImport marks a unit importing a first-party name that no
	// observed file declares: generated or annotation-processed source.
	UnresolvedImport = "java:unresolved-import"
	// Reflection marks a unit loading classes or members by computed name.
	Reflection = "java:reflection"
)

// testAnnotations are the JUnit 4, JUnit Jupiter, and TestNG markers that make
// a class discoverable by a runner.
var testAnnotations = []string{
	"Test", "ParameterizedTest", "RepeatedTest", "TestFactory", "TestTemplate", "RunWith", "Nested",
}

// reflectionMarkers are calls whose class or member target is a runtime value.
var reflectionMarkers = []string{
	"ServiceLoader", ".loadClass(", ".getMethod(", ".getDeclaredMethod(", ".getField(",
	".getDeclaredField(", ".newInstance(", "MethodHandles.", ".getConstructor(", ".getDeclaredConstructor(",
}

// Language observes Java source under one repository root.
type Language struct{}

// New returns the Java language plugin.
func New() Language { return Language{} }

// Name is the plugin namespace.
func (Language) Name() string { return "java" }

// Owns reports whether a path is Java source text. Build files (pom.xml,
// build.gradle) and resources stay unowned, so a change to one keeps the plan
// UNKNOWN rather than being treated as a source with no dependents.
func (Language) Owns(relative string) bool { return strings.HasSuffix(relative, ".java") }

type observation struct {
	relative    string
	packageName string
	types       []string
	imports     []importRef
	words       map[string]bool
	qualified   []string
	literals    []string
	unit        affected.Unit
}

type importRef struct {
	name     string
	wildcard bool
	static   bool
}

// Units observes every Java file in the repository rooted at root.
func (language Language) Units(ctx context.Context, root string) (affected.Result, error) {
	return language.UnitsSource(affected.DiskSourceContext(ctx, root))
}

// UnitsSource reads only the explicitly supplied source universe.
func (language Language) UnitsSource(root *affected.Source) (affected.Result, error) {
	files, err := root.Files(func(name string) bool {
		return strings.HasSuffix(name, ".java") || strings.HasSuffix(name, ".kt") || strings.HasSuffix(name, ".kts")
	})
	if err != nil {
		return affected.Result{}, err
	}
	frontier := map[string]bool{}
	observations := make([]observation, 0, len(files))
	kotlin := make([]jvmReferrer, 0)
	for _, relative := range files {
		body, readErr := root.Read(relative)
		if !strings.HasSuffix(relative, ".java") {
			if readErr == nil {
				kotlin = append(kotlin, kotlinReferrer(string(body)))
			}
			continue
		}
		if readErr != nil {
			frontier[FrontierUnparsedSource] = true
			continue
		}
		clean, literals, ok := sourceText(strings.TrimPrefix(string(body), "\ufeff"))
		if !ok {
			frontier[FrontierUnparsedSource] = true
			continue
		}
		observations = append(observations, observe(relative, clean, literals))
	}
	resolve(observations, frontier)
	for index := range observations {
		markKotlinDependents(&observations[index], kotlin)
	}
	units := make([]affected.Unit, 0, len(observations))
	for _, observed := range observations {
		units = append(units, observed.unit)
	}
	sort.Slice(units, func(left, right int) bool { return units[left].ID < units[right].ID })
	return affected.Result{Units: units, Frontier: sortedKeys(frontier)}, nil
}

func observe(relative, clean string, literals []string) observation {
	observed := observation{relative: relative, words: identifierWords(clean), literals: literals}
	for _, statement := range strings.Split(clean, ";") {
		trimmed := strings.Join(strings.Fields(statement), " ")
		// A statement may begin after an annotation or a closing brace; only
		// header statements carry package and import keywords at their start.
		switch {
		case strings.HasPrefix(trimmed, "package "):
			if observed.packageName == "" {
				observed.packageName = dottedName(strings.TrimPrefix(trimmed, "package "))
			}
		case strings.HasPrefix(trimmed, "import "):
			if ref, ok := parseImport(strings.TrimPrefix(trimmed, "import ")); ok {
				observed.imports = append(observed.imports, ref)
			}
		}
	}
	observed.types = declaredTypes(clean)
	observed.qualified = qualifiedReferences(clean)
	id := "java:" + relative
	if isTestFile(relative, observed.words) {
		observed.unit = affected.Unit{ID: id, Tests: []string{relative}}
	} else {
		observed.unit = affected.Unit{ID: id, Sources: []string{relative}}
	}
	if reflective(clean, literals) {
		markUnbounded(&observed.unit, Reflection)
	}
	return observed
}

// parseImport reads the body of one import statement.
func parseImport(body string) (importRef, bool) {
	ref := importRef{}
	if strings.HasPrefix(body, "static ") {
		ref.static = true
		body = strings.TrimPrefix(body, "static ")
	}
	body = strings.ReplaceAll(body, " ", "")
	if strings.HasSuffix(body, ".*") {
		ref.wildcard = true
		body = strings.TrimSuffix(body, ".*")
	}
	ref.name = dottedName(body)
	if ref.name == "" || ref.name != body {
		return importRef{}, false
	}
	if ref.static && !ref.wildcard {
		// import static p.C.member names a member; the declaring type is p.C.
		cut := strings.LastIndex(ref.name, ".")
		if cut < 0 {
			return importRef{}, false
		}
		ref.name = ref.name[:cut]
	}
	return ref, true
}

// resolve turns imports, same-package type mentions, qualified references and
// literal Class.forName targets into unit edges.
func resolve(observations []observation, frontier map[string]bool) {
	types := make(map[string]map[string]bool, len(observations)*2)
	packages := make(map[string]map[string]bool, len(observations))
	for _, observed := range observations {
		add(packages, observed.packageName, observed.unit.ID)
		for _, name := range observed.types {
			add(types, qualify(observed.packageName, name), observed.unit.ID)
		}
	}
	firstParty := sortedKeys(packages)
	byPackage := make(map[string][]int, len(packages))
	for index, observed := range observations {
		byPackage[observed.packageName] = append(byPackage[observed.packageName], index)
	}
	for index := range observations {
		observed := &observations[index]
		edges := map[string]bool{}
		link := func(targets map[string]bool) {
			if len(targets) > 1 {
				frontier[FrontierAmbiguousType] = true
			}
			merge(edges, targets)
		}
		// Same-package types need no import; a mention by simple name is the
		// edge, so an unrelated class in the same package is not one.
		mentioned(observed, observed.packageName, types, observations, byPackage, link)
		for _, ref := range observed.imports {
			if ref.wildcard && !ref.static {
				if len(packages[ref.name]) != 0 {
					mentioned(observed, ref.name, types, observations, byPackage, link)
					continue
				}
			}
			targets := lookup(ref.name, types)
			if len(targets) == 0 && underFirstParty(packageOf(ref, packages), firstParty) {
				markUnbounded(&observed.unit, UnresolvedImport)
			}
			link(targets)
		}
		for _, name := range append(append([]string(nil), observed.qualified...), observed.literals...) {
			link(lookup(name, types))
		}
		delete(edges, observed.unit.ID)
		observed.unit.Imports = sortedKeys(edges)
	}
}

// mentioned links every type declared in packageName whose simple name the
// observed file uses as an identifier.
func mentioned(observed *observation, packageName string, types map[string]map[string]bool, observations []observation, byPackage map[string][]int, link func(map[string]bool)) {
	for _, index := range byPackage[packageName] {
		other := observations[index]
		if other.unit.ID == observed.unit.ID {
			continue
		}
		for _, name := range other.types {
			if observed.words[topLevel(name)] {
				link(types[qualify(packageName, name)])
			}
		}
	}
}

// lookup resolves a qualified name to its declaring files by the longest
// prefix that names a declared type, so p.Outer.Inner reaches p.Outer.
func lookup(name string, types map[string]map[string]bool) map[string]bool {
	for name != "" {
		if targets, ok := types[name]; ok {
			return targets
		}
		cut := strings.LastIndex(name, ".")
		if cut < 0 {
			return nil
		}
		name = name[:cut]
	}
	return nil
}

// packageOf is the package part of an unresolved import: the name itself for a
// non-static wildcard, otherwise everything before the type's simple name.
func packageOf(ref importRef, packages map[string]map[string]bool) string {
	if ref.wildcard && !ref.static {
		return ref.name
	}
	name := ref.name
	for name != "" {
		cut := strings.LastIndex(name, ".")
		if cut < 0 {
			return ""
		}
		name = name[:cut]
		if len(packages[name]) != 0 || lowerInitial(name[strings.LastIndex(name, ".")+1:]) {
			return name
		}
	}
	return ""
}

// underFirstParty reports whether a package is, contains, or lies under a
// package some observed file declares, or shares its first two segments with
// one (com.ex.gen beside com.ex.app). An unresolved name there is first-party
// code the graph cannot see, such as generated or annotation-processed source.
func underFirstParty(packageName string, firstParty []string) bool {
	if packageName == "" {
		return false
	}
	for _, owned := range firstParty {
		if owned == "" {
			continue
		}
		if packageName == owned || strings.HasPrefix(packageName, owned+".") || strings.HasPrefix(owned, packageName+".") {
			return true
		}
		if group := leadingSegments(owned, 2); group != "" && strings.HasPrefix(packageName, group+".") {
			return true
		}
	}
	return false
}

// leadingSegments is the first count dotted segments of a name that has more
// than count segments, and empty otherwise.
func leadingSegments(name string, count int) string {
	parts := strings.Split(name, ".")
	if len(parts) <= count {
		return ""
	}
	return strings.Join(parts[:count], ".")
}

func reflective(clean string, literals []string) bool {
	for _, marker := range reflectionMarkers {
		if strings.Contains(clean, marker) {
			return true
		}
	}
	// Every forName call must have been a literal sourceText could read.
	return strings.Count(clean, "forName") > len(literals)
}

func markUnbounded(unit *affected.Unit, reason string) {
	if unit.UnboundedReads == "" || reason < unit.UnboundedReads {
		unit.UnboundedReads = reason
	}
	// A test that reads unboundedly is selected on any dirty path by itself; a
	// non-test file must also locate the root so every test reaching it is.
	unit.LocatesRoot = len(unit.Tests) == 0
}

// isTestFile requires both a runner signal and a conventional test location,
// so a helper under src/test and a main class mentioning @Test stay sources.
func isTestFile(relative string, words map[string]bool) bool {
	signal := words["TestCase"] || words["testng"]
	for _, annotation := range testAnnotations {
		signal = signal || words["@"+annotation]
	}
	if !signal {
		return false
	}
	for _, component := range strings.Split(path.Dir(relative), "/") {
		if component == "test" || component == "tests" || strings.HasSuffix(component, "Test") {
			return true
		}
	}
	stem := strings.TrimSuffix(path.Base(relative), ".java")
	for _, suffix := range []string{"Test", "Tests", "IT", "TestCase"} {
		if strings.HasSuffix(stem, suffix) {
			return true
		}
	}
	return strings.HasPrefix(stem, "Test")
}

// declaredTypes lists every class, interface, enum, record, and annotation
// type name, nested ones as Outer.Inner by brace depth.
func declaredTypes(clean string) []string {
	tokens := tokenize(clean)
	type open struct {
		name  string
		depth int
	}
	stack := make([]open, 0, 4)
	pending := ""
	depth := 0
	names := make([]string, 0, 4)
	for index, token := range tokens {
		switch token {
		case "{":
			depth++
			if pending != "" {
				stack = append(stack, open{name: pending, depth: depth})
				pending = ""
			}
		case "}":
			if len(stack) != 0 && stack[len(stack)-1].depth == depth {
				stack = stack[:len(stack)-1]
			}
			depth--
		case "class", "interface", "enum", "record":
			if index+1 >= len(tokens) || !identifier(tokens[index+1]) {
				continue
			}
			// x.class is a literal, not a declaration.
			if index > 0 && tokens[index-1] == "." {
				continue
			}
			name := tokens[index+1]
			if len(stack) != 0 {
				name = stack[len(stack)-1].name + "." + name
			}
			names = append(names, name)
			pending = name
		}
	}
	return compactSorted(names)
}

// qualifiedReferences lists dotted names in code with a lowercase first segment
// and at least three segments, the shape of a fully qualified type reference.
func qualifiedReferences(clean string) []string {
	values := make([]string, 0)
	for _, field := range strings.FieldsFunc(clean, func(r rune) bool { return !identifierRune(r) && r != '.' }) {
		parts := strings.Split(strings.Trim(field, "."), ".")
		if len(parts) < 3 || !lowerInitial(parts[0]) {
			continue
		}
		values = append(values, strings.Join(parts, "."))
	}
	return compactSorted(values)
}

func identifierWords(clean string) map[string]bool {
	words := map[string]bool{}
	for _, token := range tokenize(clean) {
		if identifier(token) {
			words[token] = true
		}
	}
	// Annotations are recorded with their marker so a method named test is
	// not mistaken for @Test.
	for _, field := range strings.FieldsFunc(clean, func(r rune) bool { return !identifierRune(r) && r != '@' && r != '.' }) {
		if strings.HasPrefix(field, "@") {
			name := field[1:]
			words["@"+name[strings.LastIndex(name, ".")+1:]] = true
		}
		for _, part := range strings.Split(field, ".") {
			if part == "testng" {
				words[part] = true
			}
		}
	}
	return words
}

func tokenize(clean string) []string {
	tokens := make([]string, 0, len(clean)/4)
	start := -1
	for index := 0; index <= len(clean); index++ {
		if index < len(clean) && identifierRune(rune(clean[index])) {
			if start < 0 {
				start = index
			}
			continue
		}
		if start >= 0 {
			tokens = append(tokens, clean[start:index])
			start = -1
		}
		if index < len(clean) {
			switch clean[index] {
			case '{', '}', '.':
				tokens = append(tokens, clean[index:index+1])
			}
		}
	}
	return tokens
}

func dottedName(value string) string {
	value = strings.TrimSpace(value)
	for _, part := range strings.Split(value, ".") {
		if !identifier(part) {
			return ""
		}
	}
	return value
}

func identifier(value string) bool {
	if value == "" || value[0] >= '0' && value[0] <= '9' {
		return false
	}
	for _, r := range value {
		if !identifierRune(r) {
			return false
		}
	}
	return true
}

func identifierRune(r rune) bool {
	return r == '_' || r == '$' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r > 0x7f
}

func lowerInitial(value string) bool { return value != "" && value[0] >= 'a' && value[0] <= 'z' }

func topLevel(name string) string {
	if cut := strings.Index(name, "."); cut >= 0 {
		return name[:cut]
	}
	return name
}

func qualify(packageName, name string) string {
	if packageName == "" {
		return name
	}
	return packageName + "." + name
}

func add(values map[string]map[string]bool, key, value string) {
	if values[key] == nil {
		values[key] = map[string]bool{}
	}
	values[key][value] = true
}

func merge(target, source map[string]bool) {
	for key := range source {
		target[key] = true
	}
}

func compactSorted(values []string) []string {
	sort.Strings(values)
	out := values[:0]
	for index, value := range values {
		if index == 0 || value != values[index-1] {
			out = append(out, value)
		}
	}
	return out
}

func sortedKeys[Value any](values map[string]Value) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
