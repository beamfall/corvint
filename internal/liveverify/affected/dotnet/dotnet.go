// Package dotnet is the C# implementation of the affected-selection language
// seam.
//
// A test unit is one C# source file. Its identity carries the exact VSTest
// FullyQualifiedName filter for the statically discovered test methods in that
// file, so it is addressable despite VSTest having no source-file filter.
// ProjectReference edges connect each test file to its compilation closure.
package dotnet

import (
	"encoding/xml"
	"fmt"
	"path"
	"regexp"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// Frontier reasons this plugin can raise.
const (
	FrontierAmbiguousAttribute = "dotnet:ambiguous-test-attribute"
	FrontierDynamicDiscovery   = "dotnet:dynamic-test-discovery"
	FrontierFeatureOwnership   = "dotnet:gherkin-ownership-unresolved"
	FrontierGeneratedSource    = "dotnet:generated-test-source"
	FrontierProjectConfig      = "dotnet:project-configuration-unresolved"
	FrontierProjectReference   = "dotnet:project-reference-unresolved"
	FrontierRunner             = "dotnet:test-runner-filter-unresolved"
	FrontierUnparsedSource     = "dotnet:unparsed-source"
	FrontierUnassignedSource   = "dotnet:source-project-unresolved"
	FrontierUnsupportedRunner  = "dotnet:unsupported-test-framework"
)

const unitSeparator = "::"

var (
	namespacePattern        = regexp.MustCompile(`(?m)^\s*namespace\s+([A-Za-z_][A-Za-z0-9_.]*)\s*[;{]`)
	namespaceTokenPattern   = regexp.MustCompile(`\bnamespace\s+`)
	classPattern            = regexp.MustCompile(`\b((?:partial\s+)?)class\s+([A-Za-z_][A-Za-z0-9_]*)(\s*<[^>{}]*>)?[^;{]*\{`)
	classDeclarationPattern = regexp.MustCompile(`(?s)^\s*(?:(?:public|protected|internal|private|static|abstract|sealed|partial|new)\s+)*class\s+`)
	methodPattern           = regexp.MustCompile(`(?s)^\s*(?:(?:public|protected|internal|private|static|virtual|override|sealed|new|async|extern|unsafe|partial)\s+)+[A-Za-z_][A-Za-z0-9_<>,.?\[\]\s]*?\s+([A-Za-z_][A-Za-z0-9_]*)\s*(?:<[^>{}()]*>)?\s*\(`)
	looseMethodPattern      = regexp.MustCompile(`(?s)^\s*(?:(?:public|protected|internal|private|static|virtual|override|sealed|new|async|extern|unsafe|partial)\s+)*[A-Za-z_@][A-Za-z0-9_@<>,.?\[\]\s]*?\s+[A-Za-z_@][A-Za-z0-9_]*\s*(?:<[^>{}()]*>)?\s*\(`)
	aliasPattern            = regexp.MustCompile(`(?m)^\s*(?:global\s+)?using\s+[A-Za-z_][A-Za-z0-9_]*\s*=`)
	fixturePattern          = regexp.MustCompile(`\b(IClassFixture|ICollectionFixture|CollectionDefinition|SetUp|OneTimeSetUp|SetUpFixture|TestInitialize|ClassInitialize|AssemblyInitialize|TestCleanup|ClassCleanup|AssemblyCleanup)(?:Attribute)?\b`)
	dynamicPattern          = regexp.MustCompile(`\b(MemberData|ClassData|TestCaseSource|TestFixtureSource|ValueSource|DynamicData|DataSource|ITestDataSource)(?:Attribute)?\b`)
	customTestPattern       = regexp.MustCompile(`\bclass\s+(Fact|Theory|Test|TestCase|TestMethod|DataTestMethod)Attribute\b`)
	inheritancePattern      = regexp.MustCompile(`\bclass\s+[A-Za-z_][A-Za-z0-9_]*(?:\s*<[^>{}]*>)?\s*:`)
)

// Language observes SDK-style C# projects under one repository root.
type Language struct{}

// New returns the .NET language plugin.
func New() Language { return Language{} }

// Name is the plugin namespace.
func (Language) Name() string { return "dotnet" }

// Owns reports whether a path is C# source text. Gherkin and YAML ownership is
// deliberately not claimed by a host-language plugin.
func (Language) Owns(relative string) bool {
	return strings.HasSuffix(strings.ToLower(relative), ".cs")
}

// Address decodes a test unit identity into the project and VSTest filter used
// to run it: dotnet test <project> --filter <filter>.
func Address(id string) (project, filter string, ok bool) {
	value, ok := strings.CutPrefix(id, "dotnet:")
	if !ok {
		return "", "", false
	}
	project, filter, ok = strings.Cut(value, unitSeparator)
	return project, filter, ok && project != "" && filter != ""
}

// Units observes C# compilation projects and statically discoverable tests.
func (Language) Units(root string) (affected.Result, error) {
	files, err := affected.SourceFiles(root, func(name string) bool {
		lower := strings.ToLower(name)
		return strings.HasSuffix(lower, ".cs") || strings.HasSuffix(lower, ".csproj") ||
			strings.HasSuffix(lower, ".fsproj") || strings.HasSuffix(lower, ".vbproj") ||
			lower == "directory.build.props" || lower == "directory.build.targets" ||
			strings.HasSuffix(lower, ".feature")
	})
	if err != nil {
		return affected.Result{}, err
	}
	frontier := map[string]bool{}
	projects := make([]project, 0, 32)
	csFiles := make([]string, 0, len(files))
	for _, relative := range files {
		lower := strings.ToLower(relative)
		switch {
		case strings.HasSuffix(lower, ".csproj"):
			observed, readErr := readProject(root, relative)
			if readErr != nil {
				frontier[FrontierProjectConfig] = true
				continue
			}
			projects = append(projects, observed)
		case strings.HasSuffix(lower, ".cs"):
			if !generatedPath(relative) {
				csFiles = append(csFiles, relative)
			}
		case strings.HasSuffix(lower, ".feature"):
			continue
		case strings.HasSuffix(lower, ".fsproj") || strings.HasSuffix(lower, ".vbproj"):
			// An F# or Visual Basic project's ProjectReference edges into C#
			// projects are unread, so its tests could depend on any of them.
			frontier[FrontierProjectReference] = true
		default:
			frontier[FrontierProjectConfig] = true
		}
	}
	sort.Slice(projects, func(left, right int) bool { return projects[left].path < projects[right].path })
	if hasNestedProjects(projects) {
		frontier[FrontierProjectConfig] = true
	}
	byPath := make(map[string]*project, len(projects))
	for index := range projects {
		byPath[projects[index].path] = &projects[index]
		if projects[index].uncertain {
			frontier[FrontierProjectConfig] = true
		}
		if projects[index].unsupported {
			frontier[FrontierUnsupportedRunner] = true
		}
		if projects[index].mtp {
			frontier[FrontierRunner] = true
		}
	}
	for _, relative := range csFiles {
		owner := owningProject(relative, projects)
		if owner == nil {
			frontier[FrontierUnassignedSource] = true
			continue
		}
		owner.files = append(owner.files, relative)
	}
	if hasGherkinProject(projects) {
		frontier[FrontierFeatureOwnership] = true
	}

	units := make([]affected.Unit, 0, len(projects)+len(csFiles))
	for index := range projects {
		observed, flags := observeProject(root, &projects[index], byPath)
		units = append(units, observed...)
		for flag := range flags {
			frontier[flag] = true
		}
	}
	sort.Slice(units, func(left, right int) bool { return units[left].ID < units[right].ID })
	return affected.Result{Units: units, Frontier: sortedKeys(frontier)}, nil
}

type project struct {
	path        string
	directory   string
	packages    map[string]bool
	references  []string
	files       []string
	frameworks  map[string]bool
	test        bool
	gherkin     bool
	e2e         bool
	mtp         bool
	uncertain   bool
	unsupported bool
}

type projectXML struct {
	XMLName    xml.Name `xml:"Project"`
	Sdk        string   `xml:"Sdk,attr"`
	Properties []struct {
		IsTestProject             string `xml:"IsTestProject"`
		EnableDefaultItems        string `xml:"EnableDefaultItems"`
		EnableDefaultCompileItems string `xml:"EnableDefaultCompileItems"`
	} `xml:"PropertyGroup"`
	Items []struct {
		Packages []struct {
			Include   string `xml:"Include,attr"`
			Update    string `xml:"Update,attr"`
			Condition string `xml:"Condition,attr"`
		} `xml:"PackageReference"`
		Projects []struct {
			Include   string `xml:"Include,attr"`
			Condition string `xml:"Condition,attr"`
		} `xml:"ProjectReference"`
		Compile []struct {
			Include string `xml:"Include,attr"`
			Remove  string `xml:"Remove,attr"`
			Update  string `xml:"Update,attr"`
		} `xml:"Compile"`
	} `xml:"ItemGroup"`
	Imports []struct{} `xml:"Import"`
}

func readProject(root, relative string) (project, error) {
	body, err := affected.ReadSource(root, relative)
	if err != nil {
		return project{}, err
	}
	var document projectXML
	if err := xml.Unmarshal(body, &document); err != nil {
		return project{}, err
	}
	if document.XMLName.Local != "Project" {
		return project{}, fmt.Errorf("%w: %s has no Project root", affected.ErrInvalidLanguage, relative)
	}
	result := project{
		path:       relative,
		directory:  path.Dir(relative),
		packages:   map[string]bool{},
		frameworks: map[string]bool{},
	}
	if strings.Contains(string(body), "Condition=") || strings.Contains(string(body), "Condition =") {
		result.uncertain = true
	}
	sdk := strings.ToLower(document.Sdk)
	if strings.HasPrefix(sdk, "mstest.sdk") {
		result.frameworks["mstest"] = true
		result.test = true
		result.mtp = true
	}
	if len(document.Imports) != 0 {
		result.uncertain = true
	}
	for _, group := range document.Properties {
		if strings.EqualFold(strings.TrimSpace(group.IsTestProject), "true") {
			result.test = true
		}
		if strings.EqualFold(strings.TrimSpace(group.EnableDefaultItems), "false") ||
			strings.EqualFold(strings.TrimSpace(group.EnableDefaultCompileItems), "false") {
			result.uncertain = true
		}
	}
	for _, group := range document.Items {
		if len(group.Compile) != 0 {
			result.uncertain = true
		}
		for _, reference := range group.Packages {
			name := strings.ToLower(strings.TrimSpace(reference.Include))
			if name == "" {
				name = strings.ToLower(strings.TrimSpace(reference.Update))
			}
			if name == "" || reference.Condition != "" {
				result.uncertain = true
				continue
			}
			result.packages[name] = true
		}
		for _, reference := range group.Projects {
			if reference.Condition != "" {
				result.uncertain = true
			}
			result.references = append(result.references, reference.Include)
		}
	}
	classifyPackages(&result)
	return result, nil
}

func classifyPackages(value *project) {
	for name := range value.packages {
		switch name {
		case "xunit", "xunit.v3":
			value.frameworks["xunit"] = true
			value.test = true
		case "nunit":
			value.frameworks["nunit"] = true
			value.test = true
		case "mstest", "mstest.testframework":
			value.frameworks["mstest"] = true
			value.test = true
		case "microsoft.net.test.sdk":
			value.test = true
		case "microsoft.testing.platform":
			value.test = true
			value.mtp = true
		case "microsoft.playwright", "selenium.webdriver":
			value.test = true
			value.e2e = true
		case "microsoft.playwright.nunit":
			value.frameworks["nunit"] = true
			value.test = true
			value.e2e = true
		case "microsoft.playwright.mstest":
			value.frameworks["mstest"] = true
			value.test = true
			value.e2e = true
		case "microsoft.playwright.xunit", "microsoft.playwright.xunit.v3":
			value.frameworks["xunit"] = true
			value.test = true
			value.e2e = true
		case "specflow", "reqnroll":
			value.gherkin = true
			value.test = true
		case "specflow.xunit", "reqnroll.xunit":
			value.frameworks["xunit"] = true
			value.gherkin = true
			value.test = true
		case "specflow.nunit", "reqnroll.nunit":
			value.frameworks["nunit"] = true
			value.gherkin = true
			value.test = true
		case "specflow.mstest", "reqnroll.mstest":
			value.frameworks["mstest"] = true
			value.gherkin = true
			value.test = true
		case "tunit", "gdunit4":
			value.test = true
			value.unsupported = true
		}
		if strings.HasPrefix(name, "gdunit4.") || strings.HasPrefix(name, "tunit.") {
			value.test = true
			value.unsupported = true
		}
		if strings.HasPrefix(name, "selenium.") {
			value.test = true
			value.e2e = true
		}
	}
}

func observeProject(root string, value *project, projects map[string]*project) ([]affected.Unit, map[string]bool) {
	flags := map[string]bool{}
	projectID := "dotnet:" + value.path
	imports := make([]string, 0, len(value.references))
	for _, reference := range value.references {
		resolved := path.Clean(path.Join(value.directory, strings.ReplaceAll(reference, `\`, "/")))
		if strings.HasPrefix(resolved, "../") || projects[resolved] == nil {
			flags[FrontierProjectReference] = true
			continue
		}
		imports = append(imports, "dotnet:"+resolved)
	}
	imports = sortedUnique(imports)
	sources := make([]string, 0, len(value.files))
	tests := make([]affected.Unit, 0)
	staticTests := 0
	methodOwners := map[string]string{}
	for _, relative := range value.files {
		body, err := affected.ReadSource(root, relative)
		if err != nil {
			flags[FrontierUnparsedSource] = true
			continue
		}
		methods, fileFlags := discoverMethods(string(body), value)
		for flag := range fileFlags {
			flags[flag] = true
		}
		if len(methods) == 0 {
			sources = append(sources, relative)
			continue
		}
		staticTests += len(methods)
		predicates := make([]string, 0, len(methods))
		for _, method := range methods {
			if previous := methodOwners[method]; previous != "" && previous != relative {
				flags[FrontierDynamicDiscovery] = true
			}
			methodOwners[method] = relative
			predicates = append(predicates, "FullyQualifiedName="+method)
		}
		filter := strings.Join(sortedUnique(predicates), "|")
		tests = append(tests, affected.Unit{
			ID:      projectID + unitSeparator + filter,
			Tests:   []string{relative},
			Imports: []string{projectID},
		})
	}
	if value.test && len(value.frameworks) != 0 && staticTests == 0 {
		flags[FrontierDynamicDiscovery] = true
	}
	if value.test && len(value.frameworks) == 0 {
		flags[FrontierRunner] = true
	}
	if runnerUnresolved(value) {
		flags[FrontierRunner] = true
	}
	for _, unit := range tests {
		imports = append(imports, unit.ID)
	}
	result := []affected.Unit{{ID: projectID, Sources: sortedUnique(sources), Imports: sortedUnique(imports)}}
	return append(result, tests...), flags
}

type classSpan struct {
	name      string
	start     int
	end       int
	testClass bool
	ambiguous bool
}

func discoverMethods(source string, project *project) ([]string, map[string]bool) {
	flags := map[string]bool{}
	if !project.test || len(project.frameworks) == 0 {
		return nil, flags
	}
	if aliasPattern.MatchString(source) {
		flags[FrontierAmbiguousAttribute] = true
	}
	if fixturePattern.MatchString(source) || dynamicPattern.MatchString(source) || inheritancePattern.MatchString(source) {
		flags[FrontierDynamicDiscovery] = true
	}
	if customTestPattern.MatchString(source) {
		flags[FrontierAmbiguousAttribute] = true
		return nil, flags
	}
	if strings.Contains(source, "GeneratedCode") || strings.Contains(source, "SourceGenerator") {
		flags[FrontierGeneratedSource] = true
	}
	if project.unsupported && (strings.Contains(source, "using GdUnit4") || strings.Contains(source, "GdUnit4.") ||
		strings.Contains(source, "using TUnit") || strings.Contains(source, "TUnit.")) {
		flags[FrontierUnsupportedRunner] = true
		return nil, flags
	}
	namespaces := namespacePattern.FindAllStringSubmatch(source, -1)
	if len(namespaces) > 1 || len(namespaces) != len(namespaceTokenPattern.FindAllString(source, -1)) {
		flags[FrontierUnparsedSource] = true
		return nil, flags
	}
	namespace := ""
	if len(namespaces) == 1 {
		namespace = namespaces[0][1]
	}
	classes := classSpans(source)
	attributes := attributeGroups(source)
	methods := make(map[string]bool)
	for _, attribute := range attributes {
		framework, testAttribute, ambiguous := frameworkForAttribute(attribute.names, project.frameworks)
		if ambiguous {
			flags[FrontierAmbiguousAttribute] = true
			continue
		}
		if !testAttribute {
			if containsTestName(attribute.names) {
				flags[FrontierAmbiguousAttribute] = true
			} else if !knownSupportAttribute(attribute.names) && attributeCanDiscoverTest(source, attribute.end, classes) {
				flags[FrontierDynamicDiscovery] = true
			}
			continue
		}
		declarationStart := skipAttributeGroups(source, attribute.end)
		declaration := source[declarationStart:]
		match := methodPattern.FindStringSubmatchIndex(declaration)
		if match == nil {
			flags[FrontierUnparsedSource] = true
			continue
		}
		methodPosition := declarationStart + match[0]
		owner := containingClass(classes, methodPosition)
		if owner == nil || owner.ambiguous || (framework == "mstest" && !owner.testClass) {
			flags[FrontierUnparsedSource] = true
			continue
		}
		method := declaration[match[2]:match[3]]
		qualified := owner.name + "." + method
		if namespace != "" {
			qualified = namespace + "." + qualified
		}
		methods[qualified] = true
	}
	return sortedKeys(methods), flags
}

func knownSupportAttribute(names []string) bool {
	for _, name := range names {
		switch attributeBase(name) {
		case "TestClass", "TestFixture", "Category", "Trait", "Property", "Ignore", "Explicit", "Timeout",
			"Apartment", "Parallelizable", "NonParallelizable", "Collection", "CollectionDefinition":
			continue
		default:
			return false
		}
	}
	return len(names) != 0
}

func attributeCanDiscoverTest(source string, end int, classes []classSpan) bool {
	declarationStart := skipAttributeGroups(source, end)
	declaration := source[declarationStart:]
	if classDeclarationPattern.MatchString(declaration) {
		return true
	}
	match := methodPattern.FindStringSubmatchIndex(declaration)
	if match == nil {
		return looseMethodPattern.MatchString(declaration)
	}
	return containingClass(classes, declarationStart+match[0]) != nil
}

func skipAttributeGroups(source string, start int) int {
	for start < len(source) {
		for start < len(source) && (source[start] == ' ' || source[start] == '\t' || source[start] == '\r' || source[start] == '\n') {
			start++
		}
		if start >= len(source) || source[start] != '[' {
			return start
		}
		end := strings.IndexByte(source[start+1:], ']')
		if end < 0 {
			return start
		}
		start += end + 2
	}
	return start
}

type attributeGroup struct {
	names []string
	end   int
}

func attributeGroups(source string) []attributeGroup {
	groups := make([]attributeGroup, 0, 32)
	for index := 0; index < len(source); index++ {
		if source[index] != '[' {
			continue
		}
		end := strings.IndexByte(source[index+1:], ']')
		if end < 0 {
			break
		}
		end += index + 2
		content := source[index+1 : end-1]
		groups = append(groups, attributeGroup{names: attributeNames(content), end: end})
		index = end - 1
	}
	return groups
}

func attributeNames(content string) []string {
	names := make([]string, 0, 2)
	depth := 0
	start := 0
	for index := 0; index <= len(content); index++ {
		atEnd := index == len(content)
		if !atEnd {
			switch content[index] {
			case '(':
				depth++
			case ')':
				if depth > 0 {
					depth--
				}
			}
		}
		if !atEnd && (content[index] != ',' || depth != 0) {
			continue
		}
		clause := strings.TrimSpace(content[start:index])
		if cut := strings.IndexAny(clause, "(:"); cut >= 0 {
			clause = clause[:cut]
		}
		clause = strings.TrimSpace(strings.TrimPrefix(clause, "global::"))
		clause = strings.TrimSuffix(clause, "Attribute")
		if clause != "" {
			names = append(names, clause)
		}
		start = index + 1
	}
	return names
}

func classSpans(source string) []classSpan {
	clean := structuralText(source)
	matches := classPattern.FindAllStringSubmatchIndex(clean, -1)
	spans := make([]classSpan, 0, len(matches))
	for _, match := range matches {
		open := strings.IndexByte(clean[match[0]:match[1]], '{') + match[0]
		close := matchingBrace(clean, open)
		if close < 0 {
			continue
		}
		spans = append(spans, classSpan{
			name:      source[match[4]:match[5]],
			start:     open,
			end:       close,
			testClass: hasAdjacentAttribute(source, match[0], "TestClass"),
			ambiguous: (match[2] >= 0 && match[3] > match[2]) || (match[6] >= 0 && match[7] > match[6]),
		})
	}
	for index := range spans {
		for other := range spans {
			if index != other && spans[other].start > spans[index].start && spans[other].end < spans[index].end {
				spans[other].ambiguous = true
			}
		}
	}
	return spans
}

func hasAdjacentAttribute(source string, position int, want string) bool {
	prefix := strings.TrimSpace(source[:position])
	for {
		fields := strings.Fields(prefix)
		if len(fields) == 0 || !classModifier(fields[len(fields)-1]) {
			break
		}
		prefix = strings.TrimSpace(strings.TrimSuffix(prefix, fields[len(fields)-1]))
	}
	for strings.HasSuffix(prefix, "]") {
		open := strings.LastIndexByte(prefix, '[')
		if open < 0 {
			return false
		}
		for _, name := range attributeNames(prefix[open+1 : len(prefix)-1]) {
			if attributeBase(name) == want {
				return true
			}
		}
		prefix = strings.TrimSpace(prefix[:open])
	}
	return false
}

func classModifier(value string) bool {
	switch value {
	case "public", "protected", "internal", "private", "static", "abstract", "sealed", "partial", "new":
		return true
	default:
		return false
	}
}

func structuralText(source string) string {
	result := []byte(source)
	for index := 0; index < len(result); index++ {
		if result[index] == '/' && index+1 < len(result) && result[index+1] == '/' {
			for index < len(result) && result[index] != '\n' {
				result[index] = ' '
				index++
			}
		}
		if index+1 < len(result) && result[index] == '/' && result[index+1] == '*' {
			result[index], result[index+1] = ' ', ' '
			index += 2
			for index+1 < len(result) && !(result[index] == '*' && result[index+1] == '/') {
				if result[index] != '\n' {
					result[index] = ' '
				}
				index++
			}
		}
		if result[index] == '"' || result[index] == '\'' {
			quote := result[index]
			result[index] = ' '
			for index++; index < len(result); index++ {
				if result[index] == '\\' {
					result[index] = ' '
					if index+1 < len(result) {
						index++
						result[index] = ' '
					}
					continue
				}
				if result[index] == quote {
					result[index] = ' '
					break
				}
				if result[index] != '\n' {
					result[index] = ' '
				}
			}
		}
	}
	return string(result)
}

func matchingBrace(source string, open int) int {
	depth := 0
	for index := open; index < len(source); index++ {
		switch source[index] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return index
			}
		}
	}
	return -1
}

func containingClass(classes []classSpan, position int) *classSpan {
	var owner *classSpan
	for index := range classes {
		candidate := &classes[index]
		if candidate.start >= position || candidate.end <= position {
			continue
		}
		if owner == nil || candidate.start > owner.start {
			owner = candidate
		}
	}
	return owner
}

func frameworkForAttribute(names []string, frameworks map[string]bool) (string, bool, bool) {
	for _, qualified := range names {
		name := attributeBase(qualified)
		explicit := explicitFramework(qualified)
		if explicit == "unsupported" {
			return "", false, containsTestName([]string{name})
		}
		switch name {
		case "Fact", "Theory":
			return "xunit", frameworks["xunit"] && (explicit == "" || explicit == "xunit"), explicit != "" && explicit != "xunit"
		case "Test", "TestCase", "TestCaseSource":
			return "nunit", frameworks["nunit"] && (explicit == "" || explicit == "nunit"), explicit != "" && explicit != "nunit"
		case "TestMethod", "DataTestMethod":
			return "mstest", frameworks["mstest"] && (explicit == "" || explicit == "mstest"), explicit != "" && explicit != "mstest"
		}
	}
	return "", false, false
}

func containsTestName(names []string) bool {
	_, ok, _ := frameworkForAttribute(names, map[string]bool{"xunit": true, "nunit": true, "mstest": true})
	return ok
}

func attributeBase(name string) string {
	if cut := strings.LastIndex(name, "."); cut >= 0 {
		return name[cut+1:]
	}
	return name
}

func explicitFramework(name string) string {
	if !strings.Contains(name, ".") {
		return ""
	}
	lower := strings.ToLower(name)
	switch {
	case strings.HasPrefix(lower, "xunit."):
		return "xunit"
	case strings.HasPrefix(lower, "nunit.framework."):
		return "nunit"
	case strings.HasPrefix(lower, "microsoft.visualstudio.testtools.unittesting."):
		return "mstest"
	default:
		return "unsupported"
	}
}

func owningProject(relative string, projects []project) *project {
	var owner *project
	best := -1
	ambiguous := false
	for index := range projects {
		directory := projects[index].directory
		if directory == "." {
			directory = ""
		}
		if directory != "" && !strings.HasPrefix(relative, directory+"/") {
			continue
		}
		if len(directory) < best {
			continue
		}
		if len(directory) == best {
			ambiguous = true
			continue
		}
		owner = &projects[index]
		best = len(directory)
		ambiguous = false
	}
	if ambiguous {
		return nil
	}
	return owner
}

func runnerUnresolved(value *project) bool {
	if value.mtp {
		return true
	}
	if len(value.frameworks) != 0 && !value.packages["microsoft.net.test.sdk"] {
		return true
	}
	if value.frameworks["xunit"] && !value.packages["xunit.runner.visualstudio"] {
		return true
	}
	if value.frameworks["nunit"] && !value.packages["nunit3testadapter"] {
		return true
	}
	if value.frameworks["mstest"] && !value.packages["mstest"] && !value.packages["mstest.testadapter"] {
		return true
	}
	return false
}

func hasNestedProjects(projects []project) bool {
	for outer := range projects {
		directory := projects[outer].directory
		if directory == "." {
			directory = ""
		}
		for inner := range projects {
			if outer == inner || projects[inner].directory == directory {
				continue
			}
			if directory == "" || strings.HasPrefix(projects[inner].directory, directory+"/") {
				return true
			}
		}
	}
	return false
}

func hasGherkinProject(projects []project) bool {
	for _, value := range projects {
		if value.gherkin {
			return true
		}
	}
	return false
}

func generatedPath(relative string) bool {
	for _, component := range strings.Split(relative, "/") {
		if component == "bin" || component == "obj" {
			return true
		}
	}
	return false
}

func sortedUnique(values []string) []string {
	set := make(map[string]bool, len(values))
	for _, value := range values {
		if value != "" {
			set[value] = true
		}
	}
	return sortedKeys(set)
}

func sortedKeys[Value any](values map[string]Value) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
