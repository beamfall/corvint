package analyzerdotnet

import (
	"path"
	"strings"
)

// projectFacts is the evaluated state of one project-shaped input. Every field
// is decided by that input alone: this candidate models no import chain, no
// property inheritance, and no condition, because the closed schema rejects
// Import elements, Condition attributes, and `$(...)` expansion outright.
type projectFacts struct {
	frameworks   []string
	runtimes     []string
	language     string
	isTest       bool
	defaultItems bool
	declared     map[string]bool
	compiled     []string
	references   []string
}

// parseProject reads a .csproj/.fsproj/.vbproj or a Directory.Build file. Both
// use one closed schema; which facts each may witness is decided by the fact
// matrix, not by the grammar, and is applied at emission below.
func parseProject(input Input, body []byte, collector *factCollector) string {
	root, why := parseClosedXML(body, "", msbuildNamespace)
	if why != "" {
		return why
	}
	state, why := evaluateProject(root, input.Path)
	if why != "" {
		return why
	}
	return emitProject(input, state, collector)
}

func evaluateProject(root element, inputPath string) (projectFacts, string) {
	if root.name != "Project" {
		return projectFacts{}, "UNSUPPORTED_SCHEMA"
	}
	if !root.blank() {
		return projectFacts{}, "MALFORMED_INPUT"
	}
	if why := projectRootAttributes(root); why != "" {
		return projectFacts{}, why
	}
	// EnableDefaultItems defaults to true in the SDK, which is exactly the
	// state in which the compile set is glob-determined and therefore not
	// knowable here.
	state := projectFacts{defaultItems: true, declared: map[string]bool{}}
	base := baseDirectory(inputPath)
	for _, group := range root.children {
		// The element name is checked before its shape, so an element this
		// schema does not list -- Import and Target above all -- rejects as an
		// unsupported schema rather than as an unknown attribute on it.
		if group.name != "PropertyGroup" && group.name != "ItemGroup" {
			return projectFacts{}, "UNSUPPORTED_SCHEMA"
		}
		if why := requirePlainGroup(group); why != "" {
			return projectFacts{}, why
		}
		var why string
		if group.name == "PropertyGroup" {
			why = readProperties(group, &state)
		} else {
			why = readItems(group, base, &state)
		}
		if why != "" {
			return projectFacts{}, why
		}
	}
	return state, ""
}

func projectRootAttributes(root element) string {
	if len(root.attributes) > 1 {
		return "UNKNOWN_FIELD"
	}
	if len(root.attributes) == 0 {
		return ""
	}
	if root.attributes[0].name != "Sdk" {
		return "UNKNOWN_FIELD"
	}
	if !dotnetName(root.attributes[0].value) {
		return "UNSUPPORTED_SCHEMA"
	}
	return ""
}

func readProperties(group element, state *projectFacts) string {
	for _, property := range group.children {
		if why := readScalar(property, state); why != "" {
			return why
		}
	}
	return ""
}

// scalarProperties is the closed named-scalar set, dispatched by name so that
// an unlisted property is an unsupported schema rather than a shape failure.
var scalarProperties = map[string]func(string, *projectFacts) string{
	"TargetFramework":    func(text string, s *projectFacts) string { return assignOne(&s.frameworks, text, dotnetTFM) },
	"TargetFrameworks":   func(text string, s *projectFacts) string { return assignMany(&s.frameworks, text, dotnetTFM) },
	"RuntimeIdentifier":  func(text string, s *projectFacts) string { return assignOne(&s.runtimes, text, dotnetRID) },
	"RuntimeIdentifiers": func(text string, s *projectFacts) string { return assignMany(&s.runtimes, text, dotnetRID) },
	"LangVersion":        func(text string, s *projectFacts) string { return assignLanguage(&s.language, text) },
	"IsTestProject":      func(text string, s *projectFacts) string { return assignBoolean(&s.isTest, text) },
	"EnableDefaultItems": func(text string, s *projectFacts) string { return assignBoolean(&s.defaultItems, text) },
}

// readScalar assigns one named scalar property. A property spelled twice is a
// duplicate; a singular property beside its plural sibling is a conflict, since
// both spell the same declaration and MSBuild's preference between them is an
// evaluation this profile does not perform.
func readScalar(property element, state *projectFacts) string {
	assign, listed := scalarProperties[property.name]
	if !listed {
		return "UNSUPPORTED_SCHEMA"
	}
	if why := requireNoAttributes(property); why != "" {
		return why
	}
	if len(property.children) != 0 {
		return "UNSUPPORTED_SCHEMA"
	}
	if why := noExpansion(property.text); why != "" {
		return why
	}
	if state.declared[property.name] {
		return "DUPLICATE_VALUE"
	}
	state.declared[property.name] = true
	return assign(property.text, state)
}

func assignOne(target *[]string, raw string, admits func(string) bool) string {
	if len(*target) != 0 {
		return "CONFLICTING_VALUE"
	}
	if !admits(raw) {
		return "UNSUPPORTED_SCHEMA"
	}
	*target = []string{raw}
	return ""
}

func assignMany(target *[]string, raw string, admits func(string) bool) string {
	if len(*target) != 0 {
		return "CONFLICTING_VALUE"
	}
	values, ok := semicolonList(raw, admits)
	if !ok {
		return "UNSUPPORTED_SCHEMA"
	}
	*target = values
	return ""
}

func assignLanguage(target *string, raw string) string {
	if !dotnetLang(raw) {
		return "UNSUPPORTED_SCHEMA"
	}
	*target = raw
	return ""
}

func assignBoolean(target *bool, raw string) string {
	if !dotnetBoolean(raw) {
		return "UNSUPPORTED_SCHEMA"
	}
	*target = raw == "true"
	return ""
}

func readItems(group element, base []string, state *projectFacts) string {
	for _, item := range group.children {
		if !listedItem(item.name) {
			return "UNSUPPORTED_SCHEMA"
		}
		if why := requireEmpty(item); why != "" {
			return why
		}
		for _, held := range item.attributes {
			if why := noExpansion(held.value); why != "" {
				return why
			}
		}
		var why string
		switch item.name {
		case "PackageReference":
			why = readPackageReference(item)
		case "ProjectReference":
			why = readProjectReference(item, base, state)
		default:
			why = readFileItem(item, base, state)
		}
		if why != "" {
			return why
		}
	}
	return ""
}

// readPackageReference validates the element and emits nothing. The fact
// matrix carries no package-reference tuple: only `dotnet.package.central`,
// witnessed by Directory.Packages.props, states a package version. Validating
// without emitting is the matrix's own shape, not an omission.
func readPackageReference(item element) string {
	include, present := item.attribute("Include")
	if !present || !dotnetName(include) {
		return "UNSUPPORTED_SCHEMA"
	}
	for _, held := range item.attributes {
		var ok bool
		switch held.name {
		case "Include", "PrivateAssets":
			ok = dotnetName(held.value)
		case "Version":
			ok = coreVersion(held.value)
		case "GeneratePathProperty":
			ok = dotnetBoolean(held.value)
		default:
			return "UNKNOWN_FIELD"
		}
		if !ok {
			return "UNSUPPORTED_SCHEMA"
		}
	}
	return ""
}

func readProjectReference(item element, base []string, state *projectFacts) string {
	if len(item.attributes) != 1 || item.attributes[0].name != "Include" {
		return "UNKNOWN_FIELD"
	}
	resolved, why := resolveRelative(base, item.attributes[0].value)
	if why != "" {
		return why
	}
	for _, prior := range state.references {
		if prior == resolved {
			return "DUPLICATE_VALUE"
		}
	}
	state.references = append(state.references, resolved)
	return ""
}

// readFileItem applies one Compile or None item in document order.
//
// Only Compile contributes to the compile set. Include adds, Remove takes
// away, and Update is metadata on an item that must already exist -- an Update
// naming an unknown path targets a default-glob item this candidate cannot see,
// so it changes nothing.
func readFileItem(item element, base []string, state *projectFacts) string {
	if len(item.attributes) != 1 {
		return "UNKNOWN_FIELD"
	}
	operation := item.attributes[0].name
	if operation != "Include" && operation != "Update" && operation != "Remove" {
		return "UNKNOWN_FIELD"
	}
	resolved, why := resolveRelative(base, item.attributes[0].value)
	if why != "" {
		return why
	}
	if item.name != "Compile" {
		return ""
	}
	switch operation {
	case "Include":
		if indexOf(state.compiled, resolved) >= 0 {
			return "DUPLICATE_VALUE"
		}
		state.compiled = append(state.compiled, resolved)
	case "Remove":
		if at := indexOf(state.compiled, resolved); at >= 0 {
			state.compiled = append(state.compiled[:at], state.compiled[at+1:]...)
		}
	}
	return ""
}

func listedItem(name string) bool {
	switch name {
	case "PackageReference", "ProjectReference", "Compile", "None":
		return true
	}
	return false
}

func indexOf(values []string, wanted string) int {
	for index, value := range values {
		if value == wanted {
			return index
		}
	}
	return -1
}

// noExpansion rejects MSBuild's evaluation syntax wherever a value is read.
// `$(Prop)`, `@(Item)`, and `%(Meta)` all require evaluating state this
// candidate does not hold, so they are rejected rather than passed through as
// opaque literals that would read as exact values downstream.
//
// All three sigils are checked, not just `$`. The reference implementation's
// guard originally matched a bare `$` only, so an item list or a metadata
// reference -- neither of which contains one -- passed through and could be
// adopted as a literal value; widening it was one of its conservative-planning
// repairs. Here the guard runs over every attribute value and every scalar
// text, so no property escapes it.
func noExpansion(value string) string {
	if strings.ContainsAny(value, "$%") || strings.Contains(value, "@(") {
		return "DYNAMIC_INPUT"
	}
	return ""
}

// emitProject writes the facts this input family is permitted to witness. The
// matrix binds framework, runtime, language, project-reference, and source
// classification tuples to `dotnet.project`. A Directory.Build file is parsed
// under the same grammar to validate its schema, but witnesses nothing: a
// Compile item there is evaluated by MSBuild against the importing project's
// directory, which this candidate never learns (it does not walk the import
// chain), so any path it resolved from the props file's own directory would be
// an invented certainty, not a fact (decision 0174).
func emitProject(input Input, state projectFacts, collector *factCollector) string {
	if input.Family != "dotnet.project" {
		return ""
	}
	unit := collector.request.CompilationUnitID
	for _, framework := range state.frameworks {
		if why := collector.add(input, "dotnet.target.framework", input.Path, "targets", framework, unit); why != "" {
			return why
		}
	}
	for _, runtime := range state.runtimes {
		if why := collector.add(input, "dotnet.runtime.identifier", input.Path, "targets-runtime", runtime, unit); why != "" {
			return why
		}
	}
	if state.language != "" {
		if why := collector.add(input, "dotnet.language.version", input.Path, "declares-language", state.language, unit); why != "" {
			return why
		}
	}
	for _, reference := range state.references {
		if why := collector.add(input, "dotnet.project.reference", input.Path, "references-project", reference, reference); why != "" {
			return why
		}
	}
	return emitSourceClassification(input, state, collector)
}

// emitSourceClassification is deliberately conservative.
//
// A `dotnet.source` fact asserts that a named path is compiled into this unit.
// That is knowable only when the input turns the SDK's default item globs off:
// with EnableDefaultItems true -- its default -- the compile set is `**/*.cs`
// minus removals, and expanding a glob would mean reading the repository, which
// this candidate never does. Emitting the explicitly listed items anyway would
// state a partial set as if it were the whole one, and an explicit Compile
// Include alongside the default globs is a duplicate-item error in the real
// SDK, so those facts would frequently describe a project that cannot build.
// Withholding the claim is the honest output.
func emitSourceClassification(input Input, state projectFacts, collector *factCollector) string {
	if state.defaultItems {
		return ""
	}
	for _, compiled := range state.compiled {
		if why := collector.add(input, "dotnet.source", compiled, "classifies",
			classify(compiled, state.isTest), compiled); why != "" {
			return why
		}
	}
	return ""
}

// classify picks one of the three closed classifications for a compiled path.
//
// Generated wins over test: it is a property of the file itself, while
// IsTestProject is a property of the project that contains it, and one path
// carries exactly one classification.
//
// Test identity comes only from the declared IsTestProject property. The
// reference implementation also infers it from the presence of an xunit/nunit
// package reference, which is a heuristic over a value this matrix does not
// even make a fact; a name in a dependency list is not a declaration that a
// project is a test project.
func classify(compiled string, isTest bool) string {
	if generated(compiled) {
		return "generated"
	}
	if isTest {
		return "test"
	}
	return "source"
}

// generatedDirectories is the closed set of path segments whose contents are
// tool output. `build` is deliberately absent: it is the conventional home for
// hand-written and NuGet-shipped MSBuild extensions, and treating it as
// generated is the exact defect the reference implementation's conservative
// planning repair removed from its own set.
var generatedDirectories = [...]string{"bin", "generated", "obj"}

func generated(compiled string) bool {
	base := path.Base(compiled)
	if strings.HasSuffix(base, ".g.cs") || strings.HasSuffix(base, ".Designer.cs") {
		return true
	}
	for _, segment := range strings.Split(compiled, "/") {
		for _, marker := range generatedDirectories {
			if segment == marker {
				return true
			}
		}
	}
	return false
}
