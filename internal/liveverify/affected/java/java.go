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
	// UnresolvedName marks a unit naming a type that no observed file, import,
	// or java.lang declares, where a first-party package could supply it.
	UnresolvedName = "java:unresolved-name"
	// Reflection marks a unit loading classes or members by computed name.
	Reflection = "java:reflection"
)

// testAnnotations are the JUnit 4, JUnit Jupiter, and TestNG markers that make
// a class discoverable by a runner.
var testAnnotations = []string{
	"Test", "ParameterizedTest", "RepeatedTest", "TestFactory", "TestTemplate", "RunWith", "Nested",
}

// reflectionMarkers are calls whose class or member target is a runtime value,
// matched against code with all whitespace removed.
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
	names       []string
	supertypes  []string
	signal      bool
	location    bool
	unit        affected.Unit
}

type importRef struct {
	name     string
	simple   string
	wildcard bool
	static   bool
}

// Units observes every Java file in the repository rooted at root.
func (language Language) Units(ctx context.Context, root string) (affected.Result, error) {
	return language.UnitsSource(affected.DiskSourceContext(ctx, root))
}

// UnitsSource reads only the explicitly supplied source universe.
func (language Language) UnitsSource(root *affected.Source) (affected.Result, error) {
	files, err := root.Files(func(_, name string) bool {
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
				kotlin = append(kotlin, kotlinReferrer(relative, string(body)))
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
	resolve(observations, frontier, kotlin)
	inheritTests(observations)
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
				observed.packageName = dottedName(strings.ReplaceAll(strings.TrimPrefix(trimmed, "package "), " ", ""))
			}
		case strings.HasPrefix(trimmed, "import "):
			if ref, ok := parseImport(strings.TrimPrefix(trimmed, "import ")); ok {
				observed.imports = append(observed.imports, ref)
			}
		}
	}
	tokens := tokenize(clean)
	observed.types = declaredTypes(tokens)
	observed.supertypes = supertypes(tokens)
	found := chains(clean)
	observed.qualified = qualifiedReferences(found)
	observed.names = typeNames(found)
	observed.signal = runnerSignal(observed.words)
	observed.location = testLocation(relative)
	id := "java:" + relative
	if observed.signal && observed.location {
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
	if !ref.wildcard {
		ref.simple = ref.name[strings.LastIndex(ref.name, ".")+1:]
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
func resolve(observations []observation, frontier map[string]bool, kotlin []jvmReferrer) {
	types := make(map[string]map[string]bool, len(observations)*2)
	packages := make(map[string]map[string]bool, len(observations))
	// known holds the simple names first-party Java and Kotlin files can
	// supply, by package and by declaring Java file, so a name a file cannot
	// see from either is generated, external, or missing.
	known := visibility{byPackage: map[string]map[string]bool{}, byUnit: map[string]map[string]bool{}, types: types}
	for _, observed := range observations {
		add(packages, observed.packageName, observed.unit.ID)
		for _, name := range observed.types {
			add(types, qualify(observed.packageName, name), observed.unit.ID)
			for _, part := range strings.Split(name, ".") {
				add(known.byPackage, observed.packageName, part)
				add(known.byUnit, observed.unit.ID, part)
			}
		}
	}
	for _, referrer := range kotlin {
		for word := range referrer.words {
			add(known.byPackage, referrer.packageName, word)
		}
		add(known.byPackage, referrer.packageName, referrer.facade)
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
		if unresolvedName(observed, known, packages, firstParty) {
			markUnbounded(&observed.unit, UnresolvedName)
		}
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
		for _, name := range observed.qualified {
			link(lookup(name, types))
		}
		// Class.forName takes a binary name, so p.Outer$Inner names p.Outer.Inner.
		for _, name := range observed.literals {
			link(lookup(name, types))
			if strings.Contains(name, "$") {
				link(lookup(strings.ReplaceAll(name, "$", "."), types))
			}
		}
		delete(edges, observed.unit.ID)
		observed.unit.Imports = sortedKeys(edges)
	}
}

// unresolvedName reports a capitalised simple name that nothing observed
// declares or imports when a first-party package could supply it: the file
// imports one on demand, or no external on-demand import could explain it, so
// it is a same-package type the graph cannot see (generated source). Behind an
// external on-demand import the name is attributed to that package.
func unresolvedName(observed *observation, known visibility, packages map[string]map[string]bool, firstParty []string) bool {
	external := false
	firstPartyWildcard := false
	local := map[string]bool{}
	for _, ref := range observed.imports {
		if ref.simple != "" {
			local[ref.simple] = true
		}
		if !ref.wildcard {
			continue
		}
		owner := ref.name
		if ref.static {
			owner = packageOf(importRef{name: ref.name}, packages)
		}
		if len(packages[owner]) != 0 || underFirstParty(owner, firstParty) {
			firstPartyWildcard = true
		} else {
			external = true
		}
	}
	if external && !firstPartyWildcard {
		return false
	}
	for _, name := range observed.names {
		if !local[name] && !javaLang[name] && !known.visible(observed, name) {
			return true
		}
	}
	return false
}

// visibility holds the simple names first-party source declares, by package
// (Java type names and their nested parts, Kotlin words and facades) and by
// declaring Java file.
type visibility struct {
	byPackage map[string]map[string]bool
	byUnit    map[string]map[string]bool
	types     map[string]map[string]bool
}

// visible reports whether a file can use name unqualified from first-party
// source: its own package declares it, a package it imports on demand does,
// or the file declaring a type it imports or names qualified does (a nested
// or inherited member type). A same-named type in a package the file neither
// imports nor names cannot supply it.
func (known visibility) visible(observed *observation, name string) bool {
	if known.byPackage[observed.packageName][name] {
		return true
	}
	declares := func(qualified string) bool {
		for id := range lookup(qualified, known.types) {
			if known.byUnit[id][name] {
				return true
			}
		}
		return false
	}
	for _, ref := range observed.imports {
		if ref.wildcard && !ref.static && known.byPackage[ref.name][name] || declares(ref.name) {
			return true
		}
	}
	for _, qualified := range observed.qualified {
		if declares(qualified) {
			return true
		}
	}
	return false
}

// inheritTests marks a file in a test location as a test when it extends or
// implements a type whose declaring file, reached by an edge, carries a runner
// signal, directly or by inheritance: JUnit and TestNG run inherited test
// methods. A base type outside the observed sources is not seen.
func inheritTests(observations []observation) {
	declaring := map[string][]int{}
	for index, observed := range observations {
		for _, name := range observed.types {
			simple := name[strings.LastIndex(name, ".")+1:]
			declaring[simple] = append(declaring[simple], index)
		}
	}
	for changed := true; changed; {
		changed = false
		for index := range observations {
			observed := &observations[index]
			if observed.signal {
				continue
			}
			for _, super := range observed.supertypes {
				for _, base := range declaring[super] {
					if base != index && observations[base].signal && hasEdge(observed.unit.Imports, observations[base].unit.ID) {
						observed.signal = true
						changed = true
					}
				}
			}
		}
	}
	for index := range observations {
		observed := &observations[index]
		if observed.signal && observed.location && len(observed.unit.Tests) == 0 {
			observed.unit.Tests = []string{observed.relative}
			observed.unit.Sources = nil
			observed.unit.LocatesRoot = false
		}
	}
}

func hasEdge(imports []string, id string) bool {
	at := sort.SearchStrings(imports, id)
	return at < len(imports) && imports[at] == id
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
	compact := strings.Join(strings.Fields(clean), "")
	for _, marker := range reflectionMarkers {
		if strings.Contains(compact, marker) {
			return true
		}
	}
	// Every forName call must have been a literal sourceText could read.
	return strings.Count(compact, "forName") > len(literals)
}

func markUnbounded(unit *affected.Unit, reason string) {
	if unit.UnboundedReads == "" || reason < unit.UnboundedReads {
		unit.UnboundedReads = reason
	}
	// A test that reads unboundedly is selected on any dirty path by itself; a
	// non-test file must also locate the root so every test reaching it is.
	unit.LocatesRoot = len(unit.Tests) == 0
}

// runnerSignal reports a JUnit or TestNG marker. A file is a test only with a
// signal and a conventional test location, so a helper under src/test and a
// main class mentioning @Test stay sources.
func runnerSignal(words map[string]bool) bool {
	signal := words["TestCase"] || words["testng"]
	for _, annotation := range testAnnotations {
		signal = signal || words["@"+annotation]
	}
	return signal
}

func testLocation(relative string) bool {
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
func declaredTypes(tokens []string) []string {
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

// supertypes lists the simple names after extends and implements up to the
// class body; type arguments are included, which can only add a match.
func supertypes(tokens []string) []string {
	names := make([]string, 0)
	for index := 0; index < len(tokens); index++ {
		if tokens[index] != "extends" && tokens[index] != "implements" {
			continue
		}
		for index++; index < len(tokens) && tokens[index] != "{"; index++ {
			token := tokens[index]
			following := index+1 < len(tokens) && tokens[index+1] == "."
			if identifier(token) && !following && token != "extends" && token != "implements" {
				names = append(names, token)
			}
		}
	}
	return compactSorted(names)
}

// chain is a dotted name in code, read across whitespace and blanked
// comments. An annotation chain followed @.
type chain struct {
	parts      []string
	annotation bool
}

// chains reads every dotted name that does not continue a member access, so
// p . Core, p./*c*/Core and @ org.junit.Test are each one name.
func chains(clean string) []chain {
	found := make([]chain, 0, len(clean)/16)
	previous := byte(0)
	for index := 0; index < len(clean); {
		current := clean[index]
		switch {
		case current == ' ' || current == '\t' || current == '\n' || current == '\r':
			index++
		case current == '@':
			parts, end := readChain(clean, skipSpace(clean, index+1))
			if len(parts) == 0 {
				previous = current
				index++
				continue
			}
			found = append(found, chain{parts: parts, annotation: true})
			previous, index = 'a', end
		case identifierRune(rune(current)):
			parts, end := readChain(clean, index)
			if previous != '.' && len(parts) != 0 {
				found = append(found, chain{parts: parts})
			}
			if end == index {
				end++
			}
			previous, index = 'a', end
		default:
			previous = current
			index++
		}
	}
	return found
}

func readChain(clean string, start int) ([]string, int) {
	word := wordAt(clean, start)
	if !identifier(word) {
		return nil, start + len(word)
	}
	parts := []string{word}
	end := start + len(word)
	for {
		dot := skipSpace(clean, end)
		if dot >= len(clean) || clean[dot] != '.' {
			return parts, end
		}
		next := skipSpace(clean, dot+1)
		word := wordAt(clean, next)
		if !identifier(word) {
			return parts, end
		}
		parts = append(parts, word)
		end = next + len(word)
	}
}

func wordAt(clean string, start int) string {
	end := start
	for end < len(clean) && identifierRune(rune(clean[end])) {
		end++
	}
	return clean[start:end]
}

// qualifiedReferences lists dotted names with a lowercase first segment, the
// shape of a qualified type reference (p.Core, com.ex.core.Core), including
// an annotation's (@p.Core).
func qualifiedReferences(found []chain) []string {
	values := make([]string, 0)
	for _, name := range found {
		if len(name.parts) >= 2 && lowerInitial(name.parts[0]) {
			values = append(values, strings.Join(name.parts, "."))
		}
	}
	return compactSorted(values)
}

// typeNames lists the first segment of every name that looks like a type: an
// uppercase initial and at least one lowercase letter, so constants and type
// parameters such as T are not candidates.
func typeNames(found []chain) []string {
	values := make([]string, 0)
	for _, name := range found {
		first := name.parts[0]
		if first[0] >= 'A' && first[0] <= 'Z' && strings.ToUpper(first) != first {
			values = append(values, first)
		}
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
	for _, name := range chains(clean) {
		if name.annotation {
			words["@"+name.parts[len(name.parts)-1]] = true
		}
		for _, part := range name.parts {
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

// javaLang is the java.lang package, imported implicitly by every file.
var javaLang = setOf(
	"AbstractMethodError", "Appendable", "ArithmeticException", "ArrayIndexOutOfBoundsException",
	"ArrayStoreException", "AssertionError", "AutoCloseable", "BootstrapMethodError", "Boolean", "Byte",
	"CharSequence", "Character", "Class", "ClassCastException", "ClassCircularityError",
	"ClassFormatError", "ClassLoader", "ClassNotFoundException", "ClassValue", "CloneNotSupportedException",
	"Cloneable", "Comparable", "Deprecated", "Double", "Enum", "EnumConstantNotPresentException", "Error",
	"Exception", "ExceptionInInitializerError", "Float", "FunctionalInterface", "IllegalAccessError",
	"IllegalAccessException", "IllegalArgumentException", "IllegalCallerException",
	"IllegalMonitorStateException", "IllegalStateException", "IllegalThreadStateException",
	"IncompatibleClassChangeError", "IndexOutOfBoundsException", "InheritableThreadLocal",
	"InstantiationError", "InstantiationException", "Integer", "InternalError", "InterruptedException",
	"Iterable", "LayerInstantiationException", "LinkageError", "Long", "MatchException", "Math", "Module",
	"ModuleLayer", "NegativeArraySizeException", "NoClassDefFoundError", "NoSuchFieldError",
	"NoSuchFieldException", "NoSuchMethodError", "NoSuchMethodException", "NullPointerException", "Number",
	"NumberFormatException", "Object", "OutOfMemoryError", "Override", "Package", "Process",
	"ProcessBuilder", "ProcessHandle", "Readable", "Record", "ReflectiveOperationException", "Runnable",
	"Runtime", "RuntimeException", "RuntimePermission", "SafeVarargs", "ScopedValue", "SecurityException",
	"SecurityManager", "Short", "StackOverflowError", "StackTraceElement", "StackWalker", "StrictMath",
	"String", "StringBuffer", "StringBuilder", "StringIndexOutOfBoundsException", "SuppressWarnings",
	"System", "Thread", "ThreadDeath", "ThreadGroup", "ThreadLocal", "Throwable", "TypeNotPresentException",
	"UnsatisfiedLinkError", "UnsupportedClassVersionError", "UnsupportedOperationException", "VerifyError",
	"VirtualMachineError", "Void", "WrongThreadException",
)

func setOf(values ...string) map[string]bool {
	set := make(map[string]bool, len(values))
	for _, value := range values {
		set[value] = true
	}
	return set
}
