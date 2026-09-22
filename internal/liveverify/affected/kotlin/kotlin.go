// Package kotlin is the Kotlin/JVM implementation of the affected-selection
// language seam.
//
// A source unit is one .kt or .kts file. A test unit is the complete set of
// statically addressable top-level test classes in one file: Gradle can run
// that set with repeated --tests filters, while AndroidJUnitRunner accepts the
// same set as its comma-separated class argument. Nothing is executed and no
// build tool is loaded while the graph is observed.
package kotlin

import (
	"path"
	"sort"
	"strings"
	"unicode"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// Frontier reasons this plugin can raise.
const (
	FrontierUnparsedSource       = "kotlin:unparsed-source"
	FrontierDynamicDiscovery     = "kotlin:dynamic-test-discovery"
	FrontierTestDiscovery        = "kotlin:test-discovery-unresolved"
	FrontierTestAddress          = "kotlin:test-address-unresolved"
	FrontierBuildTarget          = "kotlin:build-target-unresolved"
	FrontierCustomSourceSet      = "kotlin:custom-source-set"
	FrontierRunnerMismatch       = "kotlin:test-runner-source-set-mismatch"
	FrontierAmbiguousDeclaration = "kotlin:ambiguous-declaration"
	FrontierScriptContext        = "kotlin:script-context-unresolved"
	FrontierJVMSiblingSource     = "kotlin:jvm-sibling-source-present"
)

// Framework names returned by detectFrameworks. MockK is deliberately an
// auxiliary marker: it never makes a file a test by itself.
const (
	frameworkJUnit5      = "junit5"
	frameworkJUnit4      = "junit4"
	frameworkKotlinTest  = "kotlin-test"
	frameworkKotest      = "kotest"
	frameworkMockK       = "mockk"
	frameworkEspresso    = "espresso"
	frameworkUIAutomator = "ui-automator"
	frameworkRobolectric = "robolectric"
	frameworkAppium      = "appium"
)

// Language observes Kotlin source under one repository root.
type Language struct{}

// New returns the Kotlin language plugin.
func New() Language { return Language{} }

// Name is the plugin namespace.
func (Language) Name() string { return "kotlin" }

// Owns reports whether a path is Kotlin source text. Cross-language Maestro
// YAML and Gherkin feature files intentionally remain unclaimed.
func (Language) Owns(relative string) bool {
	return strings.HasSuffix(relative, ".kt") || strings.HasSuffix(relative, ".kts")
}

type importRef struct {
	name     string
	wildcard bool
}

type sourceObservation struct {
	relative    string
	packageName string
	aliases     []string
	imports     []importRef
	unit        affected.Unit
	config      bool
}

type sourceScan struct {
	packageName string
	imports     []importRef
	aliases     []string
	classes     []string
	frameworks  map[string]bool
	test        bool
	dynamic     bool
	ambiguous   bool
}

type buildLayout struct {
	gradle        bool
	maven         bool
	projectMapped bool
	composite     bool
}

// Units observes every Kotlin source and resolves static first-party imports.
func (language Language) Units(root string) (affected.Result, error) {
	files, err := affected.SourceFiles(root, func(name string) bool {
		return language.Owns(name) || strings.HasSuffix(name, ".java")
	})
	if err != nil {
		return affected.Result{}, err
	}
	layout, err := observeBuildLayout(root)
	if err != nil {
		return affected.Result{}, err
	}
	frontier := map[string]bool{}
	observations := make([]sourceObservation, 0, len(files))
	seenIDs := make(map[string]bool, len(files))
	for _, relative := range files {
		if strings.HasSuffix(relative, ".java") {
			frontier[FrontierJVMSiblingSource] = true
			continue
		}
		body, readErr := affected.ReadSource(root, relative)
		if readErr != nil {
			frontier[FrontierUnparsedSource] = true
			continue
		}
		clean, ok := sourceText(strings.TrimPrefix(string(body), "\ufeff"))
		if !ok {
			frontier[FrontierUnparsedSource] = true
			continue
		}
		scan := scanSource(relative, clean)
		observation := makeObservation(relative, scan, layout, frontier)
		if seenIDs[observation.unit.ID] {
			frontier[FrontierAmbiguousDeclaration] = true
			observation.unit.ID += ":" + relative
		}
		seenIDs[observation.unit.ID] = true
		observations = append(observations, observation)
	}
	resolve(observations, frontier)
	units := make([]affected.Unit, 0, len(observations))
	for _, observation := range observations {
		units = append(units, observation.unit)
	}
	sort.Slice(units, func(left, right int) bool { return units[left].ID < units[right].ID })
	return affected.Result{Units: units, Frontier: sortedKeys(frontier)}, nil
}

func observeBuildLayout(root string) (buildLayout, error) {
	manifests, err := affected.SourceFiles(root, func(name string) bool {
		switch name {
		case "build.gradle", "build.gradle.kts", "settings.gradle", "settings.gradle.kts", "pom.xml":
			return true
		default:
			return false
		}
	})
	if err != nil {
		return buildLayout{}, err
	}
	layout := buildLayout{}
	for _, manifest := range manifests {
		name := path.Base(manifest)
		layout.gradle = layout.gradle || strings.HasSuffix(name, ".gradle") || strings.HasSuffix(name, ".gradle.kts")
		layout.maven = layout.maven || name == "pom.xml"
		body, readErr := affected.ReadSource(root, manifest)
		if readErr != nil {
			layout.projectMapped = true
			continue
		}
		text := string(body)
		layout.projectMapped = layout.projectMapped || strings.Contains(text, "projectDir")
		layout.composite = layout.composite || strings.Contains(text, "includeBuild(") || strings.Contains(text, "includeBuild ")
	}
	return layout, nil
}

func makeObservation(relative string, scan sourceScan, layout buildLayout, frontier map[string]bool) sourceObservation {
	observation := sourceObservation{
		relative:    relative,
		packageName: scan.packageName,
		aliases:     scan.aliases,
		imports:     scan.imports,
	}
	if isBuildConfiguration(relative) {
		observation.config = true
		observation.unit = affected.Unit{ID: "kotlin:config:" + relative, Sources: []string{relative}}
		return observation
	}
	if strings.HasSuffix(relative, ".kts") {
		frontier[FrontierScriptContext] = true
	}
	if scan.dynamic {
		frontier[FrontierDynamicDiscovery] = true
	}
	if scan.ambiguous {
		frontier[FrontierTestDiscovery] = true
	}
	if !scan.test {
		observation.unit = affected.Unit{ID: "kotlin:source:" + relative, Sources: []string{relative}}
		return observation
	}
	kind, module, task := testTarget(relative, layout, scan.frameworks, frontier)
	classes := qualifiedClasses(scan.packageName, scan.classes)
	if len(classes) == 0 {
		frontier[FrontierTestAddress] = true
		classes = []string{relative}
	}
	if len(classes) > 1 {
		frontier[FrontierTestAddress] = true
	}
	id := "kotlin:" + kind + ":" + module + ":" + task + ":" + strings.Join(classes, "+")
	observation.unit = affected.Unit{ID: id, Tests: []string{relative}}
	return observation
}

func testTarget(relative string, layout buildLayout, frameworks map[string]bool, frontier map[string]bool) (string, string, string) {
	module, sourceSet, found := sourceLocation(relative)
	if !found {
		frontier[FrontierBuildTarget] = true
		return "unknown", ".", "unknown"
	}
	lower := strings.ToLower(sourceSet)
	instrumented := strings.HasPrefix(lower, "androidtest")
	kind := "jvm"
	if instrumented {
		kind = "instrumentation"
	} else if sourceSet != "test" && sourceSet != "jvmTest" && !strings.HasPrefix(sourceSet, "test") {
		kind = "unknown"
	}
	if layout.gradle && layout.maven || layout.projectMapped {
		frontier[FrontierBuildTarget] = true
		return kind, module, "unknown"
	}
	if layout.composite {
		frontier[FrontierBuildTarget] = true
	}
	if instrumented && frameworks[frameworkRobolectric] {
		frontier[FrontierRunnerMismatch] = true
	}
	if !instrumented && frameworks[frameworkUIAutomator] {
		frontier[FrontierRunnerMismatch] = true
	}
	if !instrumented && frameworks[frameworkEspresso] && !frameworks[frameworkRobolectric] {
		frontier[FrontierRunnerMismatch] = true
	}
	if layout.gradle {
		if instrumented {
			if sourceSet == "androidTest" {
				return "instrumentation", module, instrumentationTask(sourceSet)
			}
			frontier[FrontierCustomSourceSet] = true
			return "instrumentation", module, "unknown"
		}
		if sourceSet == "test" || sourceSet == "jvmTest" {
			return "jvm", module, sourceSet
		}
		frontier[FrontierCustomSourceSet] = true
		return kind, module, "unknown"
	}
	if layout.maven && sourceSet == "test" {
		return "maven", module, "test"
	}
	frontier[FrontierBuildTarget] = true
	return "unknown", module, sourceSet
}

func instrumentationTask(sourceSet string) string {
	variant := strings.TrimPrefix(sourceSet, "androidTest")
	if variant == "" {
		return "connectedAndroidTest"
	}
	return "connected" + variant + "AndroidTest"
}

func sourceLocation(relative string) (string, string, bool) {
	components := strings.Split(relative, "/")
	for index := 0; index+1 < len(components); index++ {
		if components[index] != "src" {
			continue
		}
		module := strings.Join(components[:index], "/")
		if module == "" {
			module = "."
		}
		return module, components[index+1], true
	}
	return ".", "", false
}

func qualifiedClasses(packageName string, classes []string) []string {
	values := make([]string, 0, len(classes))
	for _, className := range classes {
		if packageName == "" {
			values = append(values, className)
			continue
		}
		values = append(values, packageName+"."+className)
	}
	sort.Strings(values)
	return compact(values)
}

func scanSource(relative, text string) sourceScan {
	lines := strings.Split(text, "\n")
	scan := sourceScan{frameworks: detectFrameworks(text)}
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if value, ok := headerValue(trimmed, "package"); ok && scan.packageName == "" {
			scan.packageName = kotlinName(value)
			continue
		}
		if value, ok := headerValue(trimmed, "import"); ok {
			if ref, ok := parseImport(value); ok {
				scan.imports = append(scan.imports, ref)
			}
			continue
		}
		scan.imports = append(scan.imports, qualifiedBodyRefs(trimmed)...)
	}
	declarations, classes, _, excludedClasses := declarations(text)
	scan.classes = kotestClassNames(text, classes)
	scan.aliases = qualify(scan.packageName, declarations)
	junit := hasJUnitEntry(text, scan.frameworks)
	if junit && len(classes) != 0 {
		candidates := likelyTestClasses(classes)
		if len(candidates) == 0 && len(classes) == 1 {
			candidates = classes
		}
		if len(candidates) == 0 || len(candidates) != len(classes) && len(classes) > 1 {
			scan.ambiguous = true
		}
		scan.classes = append(scan.classes, candidates...)
	}
	scan.classes = compactSorted(scan.classes)
	conventional := conventionalTestSource(relative)
	nameCandidate := testNameCandidate(path.Base(relative), classes)
	scan.test = len(scan.classes) != 0
	if !scan.test && conventional && nameCandidate && len(classes) != 0 {
		scan.test = true
		scan.classes = likelyTestClasses(classes)
		scan.ambiguous = true
	}
	if (junit || scan.frameworks[frameworkKotest]) && len(excludedClasses) != 0 {
		scan.ambiguous = true
	}
	if !scan.test && conventional && len(excludedClasses) == 0 && unknownTestShape(text) {
		scan.ambiguous = true
	}
	scan.dynamic = dynamicDiscovery(text, scan.frameworks)
	return scan
}

func qualifiedBodyRefs(line string) []importRef {
	refs := make([]importRef, 0, 2)
	start := -1
	flush := func(end int) {
		if start < 0 {
			return
		}
		value := strings.Trim(line[start:end], ".`")
		start = -1
		if strings.Contains(value, ".") {
			refs = append(refs, importRef{name: value})
		}
	}
	for index, r := range line {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '.' || r == '`' {
			if start < 0 {
				start = index
			}
			continue
		}
		flush(index)
	}
	flush(len(line))
	return refs
}

// headerValue returns the text after a package or import keyword that is
// followed by any Kotlin whitespace, such as a tab.
func headerValue(line, keyword string) (string, bool) {
	rest, found := strings.CutPrefix(line, keyword)
	if !found || rest == "" || !unicode.IsSpace(rune(rest[0])) {
		return "", false
	}
	return strings.TrimSpace(rest), true
}

func parseImport(value string) (importRef, bool) {
	if cut := strings.Index(value, " as "); cut >= 0 {
		value = strings.TrimSpace(value[:cut])
	}
	value = kotlinName(value)
	if value == "" {
		return importRef{}, false
	}
	ref := importRef{name: value}
	if strings.HasSuffix(value, ".*") {
		ref.name = strings.TrimSuffix(value, ".*")
		ref.wildcard = true
	}
	return ref, ref.name != ""
}

func kotlinName(value string) string {
	value = strings.TrimSpace(strings.TrimSuffix(value, ";"))
	value = strings.ReplaceAll(value, "`", "")
	for index, r := range value {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '.' || r == '*' {
			continue
		}
		return value[:index]
	}
	return value
}

func declarations(text string) ([]string, []string, []string, []string) {
	declared := make([]string, 0, 16)
	classes := make([]string, 0, 4)
	kotest := make([]string, 0, 2)
	abstract := make([]string, 0, 1)
	for _, line := range strings.Split(text, "\n") {
		if len(line) != len(strings.TrimLeftFunc(line, unicode.IsSpace)) {
			continue
		}
		words := strings.Fields(line)
		for index, word := range words {
			word = strings.Trim(word, "(){}:,<>")
			switch word {
			case "class", "object", "interface", "typealias":
				if index+1 >= len(words) {
					continue
				}
				name := identifier(words[index+1])
				if name == "" {
					continue
				}
				declared = append(declared, name)
				if word == "class" || word == "object" {
					if word == "class" && (containsWord(words[:index], "abstract") || containsWord(words[:index], "annotation")) {
						abstract = append(abstract, name)
						continue
					}
					classes = append(classes, name)
					if containsSpecStyle(line) {
						kotest = append(kotest, name)
					}
				}
			case "fun", "val", "var":
				if index+1 < len(words) {
					if name := identifier(words[index+1]); name != "" {
						declared = append(declared, name)
					}
				}
			}
		}
	}
	return compactSorted(declared), compactSorted(classes), compactSorted(kotest), compactSorted(abstract)
}

func containsWord(words []string, want string) bool {
	for _, word := range words {
		if strings.Trim(word, "(){}:,<>") == want {
			return true
		}
	}
	return false
}

func identifier(value string) string {
	value = strings.Trim(value, "`(){}[]:,<>")
	for index, r := range value {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
			continue
		}
		value = value[:index]
		break
	}
	return value
}

func containsSpecStyle(line string) bool {
	for _, style := range []string{
		"StringSpec", "FunSpec", "BehaviorSpec", "DescribeSpec", "ShouldSpec",
		"FeatureSpec", "FreeSpec", "WordSpec", "ExpectSpec", "AnnotationSpec",
	} {
		if containsIdentifier(line, style) {
			return true
		}
	}
	return false
}

func kotestClassNames(text string, classes []string) []string {
	values := make([]string, 0, len(classes))
	for _, className := range classes {
		if extendsSpecStyle(classHeader(text, className)) {
			values = append(values, className)
		}
	}
	return values
}

func extendsSpecStyle(header string) bool {
	colon := topLevelColon(header)
	if colon < 0 {
		return false
	}
	header = header[colon+1:]
	for _, style := range []string{
		"StringSpec", "FunSpec", "BehaviorSpec", "DescribeSpec", "ShouldSpec",
		"FeatureSpec", "FreeSpec", "WordSpec", "ExpectSpec", "AnnotationSpec",
	} {
		start := strings.Index(header, style)
		for start >= 0 {
			beforeOK := start == 0 || !identifierRune(rune(header[start-1]))
			after := start + len(style)
			afterOK := after == len(header) || !identifierRune(rune(header[after]))
			for after < len(header) && unicode.IsSpace(rune(header[after])) {
				after++
			}
			if beforeOK && afterOK && parenthesisDepth(header[:start]) == 0 && after < len(header) && header[after] == '(' {
				return true
			}
			next := strings.Index(header[start+len(style):], style)
			if next < 0 {
				break
			}
			start += len(style) + next
		}
	}
	return false
}

func topLevelColon(text string) int {
	depth := 0
	for index, r := range text {
		switch r {
		case '(':
			depth++
		case ')':
			if depth > 0 {
				depth--
			}
		case ':':
			if depth == 0 {
				return index
			}
		}
	}
	return -1
}

func parenthesisDepth(text string) int {
	depth := 0
	for _, r := range text {
		switch r {
		case '(':
			depth++
		case ')':
			if depth > 0 {
				depth--
			}
		}
	}
	return depth
}

func identifierRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'
}

func classHeader(text, className string) string {
	flat := strings.Join(strings.Fields(text), " ")
	marker := "class " + className
	start := strings.Index(flat, marker)
	if start < 0 {
		return ""
	}
	header := flat[start:]
	if next := strings.Index(header[len(marker):], " class "); next >= 0 {
		header = header[:len(marker)+next]
	}
	if end := strings.Index(header, "{"); end >= 0 {
		header = header[:end]
	}
	return header
}

func containsIdentifier(text, want string) bool {
	words := strings.FieldsFunc(text, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_'
	})
	for _, word := range words {
		if word == want {
			return true
		}
	}
	return false
}

func unknownTestShape(text string) bool {
	_, classes, _, _ := declarations(text)
	for _, className := range classes {
		if strings.Contains(classHeader(text, className), ":") {
			return true
		}
	}
	for start := strings.Index(text, "@"); start >= 0; {
		if !strings.HasPrefix(text[start:], "@file:") {
			return true
		}
		next := strings.Index(text[start+1:], "@")
		if next < 0 {
			break
		}
		start += next + 1
	}
	return false
}

func detectFrameworks(text string) map[string]bool {
	frameworks := map[string]bool{}
	if strings.Contains(text, "org.junit.jupiter.") {
		frameworks[frameworkJUnit5] = true
	} else if strings.Contains(text, "org.junit.") {
		frameworks[frameworkJUnit4] = true
	}
	markers := map[string]string{
		"kotlin.test.":               frameworkKotlinTest,
		"io.kotest.":                 frameworkKotest,
		"io.mockk.":                  frameworkMockK,
		"androidx.test.espresso.":    frameworkEspresso,
		"androidx.test.uiautomator.": frameworkUIAutomator,
		"org.robolectric.":           frameworkRobolectric,
		"io.appium.java_client.":     frameworkAppium,
	}
	for marker, framework := range markers {
		if strings.Contains(text, marker) {
			frameworks[framework] = true
		}
	}
	if containsSpecStyle(text) {
		frameworks[frameworkKotest] = true
	}
	return frameworks
}

func hasJUnitEntry(text string, frameworks map[string]bool) bool {
	if !frameworks[frameworkJUnit5] && !frameworks[frameworkJUnit4] && !frameworks[frameworkKotlinTest] {
		return false
	}
	annotations := map[string]bool{
		"Test": true, "ParameterizedTest": true, "RepeatedTest": true,
		"TestFactory": true, "TestTemplate": true,
	}
	annotations = importedAnnotationNames(text, annotations)
	return hasAnnotation(text, annotations)
}

func importedAnnotationNames(text string, base map[string]bool) map[string]bool {
	names := make(map[string]bool, len(base)+2)
	for name := range base {
		names[name] = true
	}
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "import ") {
			continue
		}
		imported := strings.TrimSpace(strings.TrimPrefix(trimmed, "import "))
		alias := ""
		if cut := strings.Index(imported, " as "); cut >= 0 {
			alias = strings.TrimSpace(imported[cut+4:])
			imported = strings.TrimSpace(imported[:cut])
		}
		name := imported
		if cut := strings.LastIndex(name, "."); cut >= 0 {
			name = name[cut+1:]
		}
		if names[name] && alias != "" {
			names[alias] = true
		} else if alias != "" {
			delete(names, alias)
		}
	}
	return names
}

func hasAnnotation(text string, names map[string]bool) bool {
	for index := 0; index < len(text); index++ {
		if text[index] != '@' {
			continue
		}
		end := index + 1
		for end < len(text) {
			r := rune(text[end])
			if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '.' || r == '`' {
				end++
				continue
			}
			break
		}
		value := strings.ReplaceAll(text[index+1:end], "`", "")
		if cut := strings.LastIndex(value, "."); cut >= 0 {
			value = value[cut+1:]
		}
		if names[value] {
			return true
		}
	}
	return false
}

func likelyTestClasses(classes []string) []string {
	values := make([]string, 0, len(classes))
	for _, className := range classes {
		if strings.HasSuffix(className, "Test") || strings.HasSuffix(className, "Tests") || strings.HasSuffix(className, "Spec") {
			values = append(values, className)
		}
	}
	return values
}

func testNameCandidate(filename string, classes []string) bool {
	stem := strings.TrimSuffix(strings.TrimSuffix(filename, ".kts"), ".kt")
	if strings.HasSuffix(stem, "Test") || strings.HasSuffix(stem, "Tests") || strings.HasSuffix(stem, "Spec") {
		return true
	}
	return len(likelyTestClasses(classes)) != 0
}

func conventionalTestSource(relative string) bool {
	_, sourceSet, found := sourceLocation(relative)
	if !found {
		return false
	}
	lower := strings.ToLower(sourceSet)
	return strings.Contains(lower, "test") && !strings.Contains(lower, "fixture")
}

func dynamicDiscovery(text string, frameworks map[string]bool) bool {
	compactText := strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return r
	}, text)
	for _, marker := range []string{"Class.forName(", "ServiceLoader.load(", ".loadClass("} {
		if strings.Contains(compactText, marker) {
			return true
		}
	}
	if frameworks[frameworkKotest] && (strings.Contains(compactText, "include(") || strings.Contains(compactText, "withData(")) {
		return true
	}
	dynamicAnnotations := importedAnnotationNames(text, map[string]bool{
		"TestFactory": true, "TestTemplate": true, "SelectClasses": true,
		"SelectPackages": true, "IncludeEngines": true, "ExcludeEngines": true,
	})
	return hasAnnotation(text, dynamicAnnotations)
}

func qualify(packageName string, declarations []string) []string {
	values := make([]string, 0, len(declarations))
	for _, declaration := range declarations {
		if packageName == "" {
			values = append(values, declaration)
			continue
		}
		values = append(values, packageName+"."+declaration)
	}
	return compactSorted(values)
}

func resolve(observations []sourceObservation, frontier map[string]bool) {
	aliases := make(map[string]map[string]bool, len(observations)*2)
	packages := make(map[string]map[string]bool, len(observations))
	configs := make([]sourceObservation, 0)
	for _, observation := range observations {
		if observation.config {
			configs = append(configs, observation)
		}
		if !observation.config {
			add(packages, observation.packageName, observation.unit.ID)
		}
		for _, alias := range observation.aliases {
			add(aliases, alias, observation.unit.ID)
		}
	}
	for index := range observations {
		observation := &observations[index]
		edges := map[string]bool{}
		if !observation.config {
			merge(edges, packages[observation.packageName])
		}
		for _, ref := range observation.imports {
			// A star import of a name that is no package imports the members of
			// a class, object, or enum, so it resolves like that declaration.
			if len(packages[ref.name]) == 0 {
				ref.wildcard = false
			}
			targets := resolveImport(ref, aliases, packages)
			if len(targets) > 1 && !ref.wildcard {
				frontier[FrontierAmbiguousDeclaration] = true
			}
			merge(edges, targets)
		}
		for _, config := range configs {
			if configApplies(config.relative, observation.relative) {
				edges[config.unit.ID] = true
			}
		}
		delete(edges, observation.unit.ID)
		observation.unit.Imports = sortedKeys(edges)
	}
}

func resolveImport(ref importRef, aliases, packages map[string]map[string]bool) map[string]bool {
	if ref.wildcard {
		return packages[ref.name]
	}
	name := ref.name
	for name != "" {
		// A declaration and a package can share one qualified name, so both
		// stay candidates rather than the declaration shadowing the package.
		targets := map[string]bool{}
		merge(targets, aliases[name])
		merge(targets, packages[name])
		if len(targets) != 0 {
			return targets
		}
		cut := strings.LastIndex(name, ".")
		if cut < 0 {
			break
		}
		name = name[:cut]
	}
	return nil
}

func isBuildConfiguration(relative string) bool {
	name := path.Base(relative)
	return name == "build.gradle.kts" || name == "settings.gradle.kts" || strings.HasSuffix(name, ".gradle.kts") ||
		strings.HasPrefix(relative, "buildSrc/") || strings.HasPrefix(relative, "build-logic/") ||
		strings.Contains(relative, "/build-logic/")
}

func configApplies(config, source string) bool {
	name := path.Base(config)
	directory := path.Dir(config)
	if strings.HasPrefix(config, "buildSrc/") || strings.HasPrefix(config, "build-logic/") || strings.Contains(config, "/build-logic/") {
		return true
	}
	if name != "build.gradle.kts" && strings.HasSuffix(name, ".gradle.kts") {
		return true
	}
	if name == "settings.gradle.kts" || directory == "." {
		return true
	}
	return strings.HasPrefix(source, directory+"/")
}

func add(values map[string]map[string]bool, key, value string) {
	if values[key] == nil {
		values[key] = map[string]bool{}
	}
	values[key][value] = true
}

func merge(target, source map[string]bool) {
	for value := range source {
		target[value] = true
	}
}

func compactSorted(values []string) []string {
	sort.Strings(values)
	return compact(values)
}

func compact(values []string) []string {
	if len(values) == 0 {
		return values
	}
	output := values[:1]
	for _, value := range values[1:] {
		if value != output[len(output)-1] {
			output = append(output, value)
		}
	}
	return output
}

func sortedKeys[Value any](values map[string]Value) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// sourceText removes comments and string/character contents while preserving
// identifiers and newlines. An unterminated lexical construct is not guessed.
func sourceText(text string) (string, bool) {
	const (
		code = iota
		lineComment
		blockComment
		quoted
		tripleQuoted
		character
		backticked
	)
	state := code
	depth := 0
	escaped := false
	var output strings.Builder
	output.Grow(len(text))
	for index := 0; index < len(text); index++ {
		current := text[index]
		next := byte(0)
		if index+1 < len(text) {
			next = text[index+1]
		}
		switch state {
		case code:
			switch {
			case current == '/' && next == '/':
				state = lineComment
				output.WriteString("  ")
				index++
			case current == '/' && next == '*':
				state = blockComment
				depth = 1
				output.WriteString("  ")
				index++
			case current == '"' && index+2 < len(text) && text[index:index+3] == "\"\"\"":
				state = tripleQuoted
				output.WriteString("   ")
				index += 2
			case current == '"':
				state = quoted
				output.WriteByte(' ')
			case current == '\'':
				state = character
				output.WriteByte(' ')
			case current == '`':
				state = backticked
				output.WriteByte(current)
			default:
				output.WriteByte(current)
			}
		case lineComment:
			if current == '\n' {
				state = code
				output.WriteByte('\n')
			} else {
				output.WriteByte(' ')
			}
		case blockComment:
			switch {
			case current == '/' && next == '*':
				depth++
				output.WriteString("  ")
				index++
			case current == '*' && next == '/':
				depth--
				output.WriteString("  ")
				index++
				if depth == 0 {
					state = code
				}
			case current == '\n':
				output.WriteByte('\n')
			default:
				output.WriteByte(' ')
			}
		case quoted, character:
			if current == '\n' {
				return "", false
			}
			output.WriteByte(' ')
			if escaped {
				escaped = false
				continue
			}
			if current == '\\' {
				escaped = true
				continue
			}
			if state == quoted && current == '"' || state == character && current == '\'' {
				state = code
			}
		case backticked:
			// A backticked name is copied verbatim: a quote or apostrophe in it
			// is part of the identifier, not the start of a literal.
			if current == '\n' {
				return "", false
			}
			output.WriteByte(current)
			if current == '`' {
				state = code
			}
		case tripleQuoted:
			if current == '"' && index+2 < len(text) && text[index:index+3] == "\"\"\"" {
				state = code
				output.WriteString("   ")
				index += 2
			} else if current == '\n' {
				output.WriteByte('\n')
			} else {
				output.WriteByte(' ')
			}
		}
	}
	return output.String(), state == code || state == lineComment
}
