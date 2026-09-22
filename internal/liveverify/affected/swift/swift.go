// Package swift is the Swift implementation of the affected-selection
// language seam.
//
// A unit is one SwiftPM or Xcode target. That is the smallest common unit both
// XCTest and Swift Testing can address without runtime discovery. XCUITest
// bundles remain separate Xcode targets, so UI tests are never folded into a
// unit-test bundle. The plugin reads manifests, project files, schemes, and
// Swift imports as source text; it never invokes SwiftPM or Xcode.
package swift

import (
	"io/fs"
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
	FrontierUnreadableSource       = "swift:unreadable-source"
	FrontierManifestUnresolved     = "swift:package-manifest-unresolved"
	FrontierXcodeProjectUnresolved = "swift:xcode-project-unresolved"
	FrontierXcodeSchemeUnresolved  = "swift:xcode-test-scheme-unresolved"
	FrontierXcodeUIHostUnresolved  = "swift:xcuitest-app-host-unresolved"
	FrontierConditionalCompilation = "swift:conditional-compilation"
	FrontierDynamicTestDiscovery   = "swift:dynamic-test-discovery"
	FrontierFrameworkUnresolved    = "swift:test-framework-unresolved"
	FrontierUnmappedSource         = "swift:unmapped-source"
	FrontierOverlappingMembership  = "swift:overlapping-target-membership"
	FrontierAmbiguousImport        = "swift:module-import-ambiguous"
	FrontierAppiumExternal         = "swift:appium-external-test-body"
	FrontierMaestroExternal        = "swift:maestro-yaml-flow"
	// FrontierExclusionContract reports that the frozen Unit/Exclusion seam
	// cannot carry LPCV-V0-014's provider and evidence identities.
	FrontierExclusionContract = "swift:exclusion-evidence-unrepresentable"
)

var (
	stringPattern      = regexp.MustCompile(`"((?:\\.|[^"\\])*)"`)
	importPattern      = regexp.MustCompile(`(?m)^\s*(?:@[A-Za-z_][A-Za-z0-9_]*(?:\([^\n]*\))?\s+)*(?:(?:private|fileprivate|internal|package|public|open)\s+)?import\s+(?:(?:typealias|struct|class|enum|protocol|let|var|func)\s+)?([A-Za-z_][A-Za-z0-9_]*)`)
	testingPattern     = regexp.MustCompile(`(?m)@(?:Testing\.)?Test\b`)
	xctestPattern      = regexp.MustCompile(`(?m)(?:\bXCTestCase\b|\bXCT[A-Za-z0-9_]*\s*\()`)
	xctestDeclPattern  = regexp.MustCompile(`(?m)(?:\b(?:class|final\s+class)\s+[A-Za-z_][A-Za-z0-9_]*[^\n{]*:\s*[^\n{]*\bXCTestCase\b|\bfunc\s+test[A-Za-z0-9_]*)`)
	dynamicTestPattern = regexp.MustCompile(`(?m)\b(?:defaultTestSuite|testInvocations|allTests|XCTMain|XCTestSuite\s*\(|addTest\s*\()`)
	objectStartPattern = regexp.MustCompile(`(?m)^\s*([A-Fa-f0-9]{8,32})\s*(?:/\*.*?\*/\s*)?=\s*\{`)
	identifierPattern  = regexp.MustCompile(`[A-Fa-f0-9]{24}`)
	attributePattern   = regexp.MustCompile(`([A-Za-z][A-Za-z0-9]*)\s*=\s*"([^"]*)"`)
)

// Language observes Swift targets under one repository root.
type Language struct{}

// New returns the Swift language plugin.
func New() Language { return Language{} }

// Name is the plugin namespace.
func (Language) Name() string { return "swift" }

// Owns reports whether a path is Swift source text. YAML flows and other
// external E2E bodies are observed as frontier evidence but never claimed.
func (Language) Owns(relative string) bool { return strings.HasSuffix(relative, ".swift") }

// Units observes every statically resolvable SwiftPM and Xcode target.
func (Language) Units(root string) (affected.Result, error) {
	files, err := affected.SourceFiles(root, func(name string) bool { return strings.HasSuffix(name, ".swift") })
	if err != nil {
		return affected.Result{}, err
	}
	metadata, err := affected.SourceFiles(root, metadataFile)
	if err != nil {
		return affected.Result{}, err
	}
	frontier := map[string]bool{}
	candidates := make([]candidate, 0, 64)
	manifestPaths := filterPaths(metadata, func(value string) bool { return path.Base(value) == "Package.swift" })
	for _, manifest := range manifestPaths {
		observed, observeErr := observePackage(root, manifest, files, frontier)
		if observeErr != nil {
			return affected.Result{}, observeErr
		}
		candidates = append(candidates, observed...)
	}
	projectPaths := filterPaths(metadata, func(value string) bool { return path.Base(value) == "project.pbxproj" })
	for _, project := range projectPaths {
		observed, observeErr := observeProject(root, project, files, metadata, frontier)
		if observeErr != nil {
			return affected.Result{}, observeErr
		}
		candidates = append(candidates, observed...)
	}
	detectExternalHarnesses(root, metadata, frontier)
	units, claimed := materialize(candidates, frontier)
	for _, unit := range units {
		if len(unit.Tests) != 0 {
			frontier[FrontierExclusionContract] = true
			break
		}
	}
	manifestSet := make(map[string]bool, len(manifestPaths))
	for _, manifest := range manifestPaths {
		manifestSet[manifest] = true
	}
	for _, file := range files {
		if !claimed[file] && !manifestSet[file] {
			frontier[FrontierUnmappedSource] = true
		}
	}
	return affected.Result{Units: units, Frontier: sortedKeys(frontier)}, nil
}

type candidate struct {
	unit       affected.Unit
	module     string
	rawImports map[string]bool
	priority   int
}

type packageTarget struct {
	kind         string
	name         string
	path         string
	dependencies []string
	sources      []string
	exclude      []string
}

func observePackage(root, manifest string, swiftFiles []string, frontier map[string]bool) ([]candidate, error) {
	body, err := affected.ReadSource(root, manifest)
	if err != nil {
		frontier[FrontierManifestUnresolved] = true
		return nil, nil
	}
	manifestCode := swiftCodeMask(string(body))
	if strings.Contains(manifestCode, "#if") || strings.Contains(manifestCode, ".when(") || strings.Contains(manifestCode, "condition:") {
		frontier[FrontierConditionalCompilation] = true
	}
	if strings.Contains(manifestCode, ".plugin(") || strings.Contains(manifestCode, "plugins:") {
		frontier[FrontierManifestUnresolved] = true
	}
	targets, complete := scanPackageTargets(string(body))
	if !complete {
		frontier[FrontierManifestUnresolved] = true
	}
	packageDirectory := path.Dir(manifest)
	candidates := make([]candidate, 0, len(targets))
	for _, target := range targets {
		targetFiles := packageTargetFiles(packageDirectory, target, swiftFiles)
		if len(targetFiles) == 0 {
			continue
		}
		id := "swift:spm:" + packageDirectory + "/" + target.name
		unit := affected.Unit{ID: id}
		if target.kind == "testTarget" {
			unit.Tests, unit.Sources = splitTestFiles(root, targetFiles, false, frontier)
		} else {
			unit.Sources = targetFiles
		}
		rawImports := make(map[string]bool, len(target.dependencies)+8)
		for _, dependency := range target.dependencies {
			rawImports[dependency] = true
		}
		inspectSwiftFiles(root, targetFiles, rawImports, frontier)
		candidates = append(candidates, candidate{unit: unit, module: target.name, rawImports: rawImports, priority: 0})
	}
	return candidates, nil
}

func scanPackageTargets(text string) ([]packageTarget, bool) {
	kinds := map[string]bool{"target": true, "executableTarget": true, "testTarget": true}
	targets := make([]packageTarget, 0, 16)
	complete := true
	mask := swiftCodeMask(text)
	for offset := 0; offset < len(text); {
		kind, start, ok := nextTargetCall(mask, offset, kinds)
		if !ok {
			break
		}
		body, end, balanced := balancedCall(text, start)
		if !balanced {
			return targets, false
		}
		target, parsed := parsePackageTarget(kind, body)
		if parsed {
			targets = append(targets, target)
		} else {
			complete = false
		}
		offset = end
	}
	if strings.Contains(text, ".testTarget") && len(targets) == 0 {
		complete = false
	}
	return targets, complete
}

func swiftCodeMask(text string) string {
	return swiftLexicalMask(text, true)
}

func swiftWithoutComments(text string) string {
	return swiftLexicalMask(text, false)
}

func swiftLexicalMask(text string, maskStrings bool) string {
	masked := []byte(text)
	inString := false
	escaped := false
	lineComment := false
	blockDepth := 0
	for index := 0; index < len(masked); index++ {
		char := masked[index]
		if lineComment {
			if char == '\n' {
				lineComment = false
			} else {
				masked[index] = ' '
			}
			continue
		}
		if blockDepth > 0 {
			masked[index] = ' '
			if index+1 < len(masked) && text[index:index+2] == "/*" {
				blockDepth++
				masked[index+1] = ' '
				index++
			} else if index+1 < len(masked) && text[index:index+2] == "*/" {
				blockDepth--
				masked[index+1] = ' '
				index++
			}
			continue
		}
		if inString {
			if maskStrings {
				masked[index] = ' '
			}
			if escaped {
				escaped = false
			} else if char == '\\' {
				escaped = true
			} else if char == '"' {
				inString = false
			}
			continue
		}
		if index+1 < len(masked) && text[index:index+2] == "//" {
			masked[index], masked[index+1] = ' ', ' '
			lineComment = true
			index++
			continue
		}
		if index+1 < len(masked) && text[index:index+2] == "/*" {
			masked[index], masked[index+1] = ' ', ' '
			blockDepth = 1
			index++
			continue
		}
		if char == '"' {
			if maskStrings {
				masked[index] = ' '
			}
			inString = true
		}
	}
	return string(masked)
}

func nextTargetCall(text string, offset int, kinds map[string]bool) (string, int, bool) {
	for offset < len(text) {
		cut := strings.IndexByte(text[offset:], '.')
		if cut < 0 {
			return "", 0, false
		}
		start := offset + cut + 1
		end := start
		for end < len(text) && (text[end] == '_' || text[end] >= 'A' && text[end] <= 'Z' || text[end] >= 'a' && text[end] <= 'z') {
			end++
		}
		kind := text[start:end]
		cursor := end
		for cursor < len(text) && (text[cursor] == ' ' || text[cursor] == '\t' || text[cursor] == '\n' || text[cursor] == '\r') {
			cursor++
		}
		if kinds[kind] && cursor < len(text) && text[cursor] == '(' {
			return kind, cursor, true
		}
		offset = end
	}
	return "", 0, false
}

func balancedCall(text string, open int) (string, int, bool) {
	depth := 0
	inString := false
	escaped := false
	lineComment := false
	blockDepth := 0
	for index := open; index < len(text); index++ {
		char := text[index]
		if lineComment {
			if char == '\n' {
				lineComment = false
			}
			continue
		}
		if blockDepth > 0 {
			if index+1 < len(text) && text[index:index+2] == "/*" {
				blockDepth++
				index++
			} else if index+1 < len(text) && text[index:index+2] == "*/" {
				blockDepth--
				index++
			}
			continue
		}
		if inString {
			if escaped {
				escaped = false
			} else if char == '\\' {
				escaped = true
			} else if char == '"' {
				inString = false
			}
			continue
		}
		if index+1 < len(text) && text[index:index+2] == "//" {
			lineComment = true
			index++
			continue
		}
		if index+1 < len(text) && text[index:index+2] == "/*" {
			blockDepth = 1
			index++
			continue
		}
		switch char {
		case '"':
			inString = true
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return text[open+1 : index], index + 1, true
			}
		}
	}
	return "", len(text), false
}

func parsePackageTarget(kind, body string) (packageTarget, bool) {
	body = swiftWithoutComments(body)
	target := packageTarget{kind: kind}
	name, present, literal := stringScalarField(body, "name")
	if !present || !literal || name == "" {
		return packageTarget{}, false
	}
	target.name = name
	if value, pathPresent, pathLiteral := stringScalarField(body, "path"); pathPresent {
		if !pathLiteral {
			return packageTarget{}, false
		}
		target.path = value
	}
	complete := true
	for _, field := range []string{"dependencies", "sources", "exclude"} {
		values, present, literal := stringListField(body, field)
		if present && !literal {
			complete = false
		}
		switch field {
		case "dependencies":
			target.dependencies = values
		case "sources":
			target.sources = values
		case "exclude":
			target.exclude = values
		}
	}
	return target, complete
}

func stringScalarField(text, name string) (string, bool, bool) {
	pattern := regexp.MustCompile(`^` + regexp.QuoteMeta(name) + `\s*:`)
	for _, field := range splitTopLevel(text) {
		field = strings.TrimSpace(field)
		location := pattern.FindStringIndex(field)
		if location == nil {
			continue
		}
		cursor := skipSwiftSpace(field, location[1])
		if cursor >= len(field) || field[cursor] != '"' {
			return "", true, false
		}
		match := stringPattern.FindStringSubmatchIndex(field[cursor:])
		if match == nil || match[0] != 0 {
			return "", true, false
		}
		end := cursor + match[1]
		if skipSwiftSpace(field, end) != len(field) {
			return "", true, false
		}
		value, err := strconv.Unquote(field[cursor:end])
		return value, true, err == nil
	}
	return "", false, true
}

func skipSwiftSpace(text string, cursor int) int {
	for cursor < len(text) && (text[cursor] == ' ' || text[cursor] == '\t' || text[cursor] == '\n' || text[cursor] == '\r') {
		cursor++
	}
	return cursor
}

func stringListField(text, name string) ([]string, bool, bool) {
	pattern := regexp.MustCompile(`\b` + regexp.QuoteMeta(name) + `\s*:`)
	location := pattern.FindStringIndex(text)
	if location == nil {
		return nil, false, true
	}
	cursor := location[1]
	for cursor < len(text) && (text[cursor] == ' ' || text[cursor] == '\t' || text[cursor] == '\n' || text[cursor] == '\r') {
		cursor++
	}
	if cursor >= len(text) || text[cursor] != '[' {
		return nil, true, false
	}
	end, ok := balancedSquare(text, cursor)
	if !ok {
		return nil, true, false
	}
	tail := end
	for tail < len(text) && (text[tail] == ' ' || text[tail] == '\t' || text[tail] == '\n' || text[tail] == '\r') {
		tail++
	}
	if tail < len(text) && text[tail] != ',' {
		return nil, true, false
	}
	content := text[cursor+1 : end-1]
	literal := staticStringList(content)
	if name == "dependencies" {
		literal = staticDependencyList(content)
	}
	return stringLiterals(content), true, literal
}

func staticStringList(text string) bool {
	remaining := strings.Trim(swiftCodeMask(text), " \t\r\n,")
	if remaining != "" {
		return false
	}
	for _, literal := range stringPattern.FindAllString(text, -1) {
		if _, err := strconv.Unquote(literal); err != nil || strings.Contains(literal, `\(`) {
			return false
		}
	}
	return true
}

func staticDependencyList(text string) bool {
	for _, entry := range splitTopLevel(text) {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		if entry[0] == '"' {
			match := stringPattern.FindStringIndex(entry)
			if match == nil || match[0] != 0 || strings.TrimSpace(entry[match[1]:]) != "" {
				return false
			}
			if _, err := strconv.Unquote(entry[:match[1]]); err != nil || strings.Contains(entry[:match[1]], `\(`) {
				return false
			}
			continue
		}
		if entry[0] != '.' {
			return false
		}
		open := strings.IndexByte(entry, '(')
		if open < 0 {
			return false
		}
		callee := strings.TrimSpace(entry[:open])
		if callee != ".target" && callee != ".product" && callee != ".byName" {
			return false
		}
		_, end, ok := balancedCall(entry, open)
		if !ok || strings.TrimSpace(entry[end:]) != "" {
			return false
		}
		for _, label := range []string{"name", "package"} {
			pattern := regexp.MustCompile(`\b` + label + `\s*:`)
			for _, location := range pattern.FindAllStringIndex(entry, -1) {
				cursor := skipSwiftSpace(entry, location[1])
				if cursor >= len(entry) || entry[cursor] != '"' {
					return false
				}
				match := stringPattern.FindStringIndex(entry[cursor:])
				if match == nil || match[0] != 0 {
					return false
				}
				literal := entry[cursor : cursor+match[1]]
				if _, err := strconv.Unquote(literal); err != nil || strings.Contains(literal, `\(`) {
					return false
				}
				tail := skipSwiftSpace(entry, cursor+match[1])
				if tail >= len(entry) || entry[tail] != ',' && entry[tail] != ')' {
					return false
				}
			}
		}
	}
	return true
}

func splitTopLevel(text string) []string {
	parts := make([]string, 0, 8)
	start := 0
	parenDepth := 0
	bracketDepth := 0
	inString := false
	escaped := false
	for index := 0; index < len(text); index++ {
		char := text[index]
		if inString {
			if escaped {
				escaped = false
			} else if char == '\\' {
				escaped = true
			} else if char == '"' {
				inString = false
			}
			continue
		}
		switch char {
		case '"':
			inString = true
		case '(':
			parenDepth++
		case ')':
			parenDepth--
		case '[':
			bracketDepth++
		case ']':
			bracketDepth--
		case ',':
			if parenDepth == 0 && bracketDepth == 0 {
				parts = append(parts, text[start:index])
				start = index + 1
			}
		}
	}
	return append(parts, text[start:])
}

func balancedSquare(text string, open int) (int, bool) {
	depth := 0
	inString := false
	escaped := false
	for index := open; index < len(text); index++ {
		char := text[index]
		if inString {
			if escaped {
				escaped = false
			} else if char == '\\' {
				escaped = true
			} else if char == '"' {
				inString = false
			}
			continue
		}
		if char == '"' {
			inString = true
			continue
		}
		if char == '[' {
			depth++
		} else if char == ']' {
			depth--
			if depth == 0 {
				return index + 1, true
			}
		}
	}
	return len(text), false
}

func stringLiterals(text string) []string {
	values := make([]string, 0, 8)
	for _, match := range stringPattern.FindAllStringSubmatch(text, -1) {
		value, err := strconv.Unquote(`"` + match[1] + `"`)
		if err == nil {
			values = append(values, value)
		}
	}
	return values
}

func packageTargetFiles(packageDirectory string, target packageTarget, swiftFiles []string) []string {
	targetRoot := target.path
	if targetRoot == "" {
		base := "Sources"
		if target.kind == "testTarget" {
			base = "Tests"
		}
		targetRoot = path.Join(base, target.name)
	}
	targetRoot = joinRelative(packageDirectory, targetRoot)
	wanted := make([]string, 0, len(target.sources))
	for _, source := range target.sources {
		wanted = append(wanted, joinRelative(targetRoot, source))
	}
	excluded := make([]string, 0, len(target.exclude))
	for _, value := range target.exclude {
		excluded = append(excluded, joinRelative(targetRoot, value))
	}
	matched := make([]string, 0, 32)
	for _, file := range swiftFiles {
		if len(wanted) != 0 && !hasPathPrefix(file, wanted) {
			continue
		}
		if len(wanted) == 0 && file != targetRoot && !strings.HasPrefix(file, targetRoot+"/") {
			continue
		}
		if hasPathPrefix(file, excluded) {
			continue
		}
		matched = append(matched, file)
	}
	return matched
}

func observeProject(root, project string, swiftFiles, metadata []string, frontier map[string]bool) ([]candidate, error) {
	body, err := affected.ReadSource(root, project)
	if err != nil {
		frontier[FrontierXcodeProjectUnresolved] = true
		return nil, nil
	}
	objects, complete := parsePBXObjects(string(body))
	if !complete {
		frontier[FrontierXcodeProjectUnresolved] = true
	}
	projectBundle := path.Dir(project)
	projectRoot := path.Dir(projectBundle)
	schemes := projectSchemes(root, projectBundle, metadata, objects, frontier)
	fileRefs := map[string]pbxObject{}
	buildRefs := map[string]string{}
	sourcePhases := map[string][]string{}
	dependencies := map[string]string{}
	groupParents := map[string]string{}
	mainGroups := map[string]bool{}
	for id, object := range objects {
		switch object.field("isa") {
		case "PBXFileReference":
			fileRefs[id] = object
		case "PBXBuildFile":
			buildRefs[id] = object.fieldID("fileRef")
		case "PBXSourcesBuildPhase":
			sourcePhases[id] = object.listIDs("files")
		case "PBXTargetDependency":
			dependencies[id] = object.fieldID("target")
		case "PBXGroup", "PBXVariantGroup":
			for _, child := range object.listIDs("children") {
				if previous, exists := groupParents[child]; exists && previous != id {
					frontier[FrontierXcodeProjectUnresolved] = true
					continue
				}
				groupParents[child] = id
			}
		case "PBXProject":
			if mainGroup := object.fieldID("mainGroup"); mainGroup != "" {
				mainGroups[mainGroup] = true
			}
		}
	}
	candidates := make([]candidate, 0, 32)
	for id, object := range objects {
		if object.field("isa") != "PBXNativeTarget" {
			continue
		}
		name := object.field("name")
		if name == "" {
			frontier[FrontierXcodeProjectUnresolved] = true
			continue
		}
		paths := make([]string, 0, 32)
		hasShellPhase := false
		for _, phase := range object.listIDs("buildPhases") {
			if objects[phase].field("isa") == "PBXShellScriptBuildPhase" {
				hasShellPhase = true
			}
			for _, build := range sourcePhases[phase] {
				fileRef := buildRefs[build]
				declared := fileRefs[fileRef].field("path")
				if declared == "" {
					declared = fileRefs[fileRef].field("name")
				}
				if declared == "" {
					frontier[FrontierXcodeProjectUnresolved] = true
					continue
				}
				if !strings.HasSuffix(strings.Trim(declared, `"`), ".swift") {
					continue
				}
				resolved, ok := resolveProjectFile(projectRoot, fileRef, fileRefs, groupParents, mainGroups, objects, swiftFiles)
				if !ok {
					frontier[FrontierXcodeProjectUnresolved] = true
					continue
				}
				paths = append(paths, resolved)
			}
		}
		paths = uniqueSorted(paths)
		if len(object.listIDs("fileSystemSynchronizedGroups")) != 0 {
			frontier[FrontierXcodeProjectUnresolved] = true
		}
		if len(paths) == 0 {
			continue
		}
		productType := strings.Trim(object.field("productType"), `"`)
		isUI := strings.HasSuffix(productType, ".bundle.ui-testing")
		isTest := isUI || strings.HasSuffix(productType, ".bundle.unit-test") || strings.HasSuffix(productType, ".bundle.ocunit-test")
		if isTest && hasShellPhase {
			frontier[FrontierDynamicTestDiscovery] = true
		}
		scheme := ""
		if values := schemes[id]; len(values) != 0 {
			scheme = values[0]
		}
		if isTest && scheme == "" {
			frontier[FrontierXcodeSchemeUnresolved] = true
		}
		identity := "swift:xcode:" + projectBundle + "/"
		if scheme != "" {
			identity += scheme + "/"
		}
		identity += name
		unit := affected.Unit{ID: identity}
		if isTest {
			unit.Tests, unit.Sources = splitTestFiles(root, paths, isUI, frontier)
		} else {
			unit.Sources = paths
		}
		rawImports := make(map[string]bool, 8)
		uiHostResolved := false
		for _, dependency := range object.listIDs("dependencies") {
			if targetID := dependencies[dependency]; targetID != "" {
				if target, known := objects[targetID]; known {
					rawImports[target.field("name")] = true
					for _, phase := range target.listIDs("buildPhases") {
						if objects[phase].field("isa") == "PBXShellScriptBuildPhase" {
							frontier[FrontierDynamicTestDiscovery] = true
						}
					}
					if strings.HasSuffix(strings.Trim(target.field("productType"), `"`), ".application") {
						uiHostResolved = true
					}
				}
			}
		}
		if isUI && !uiHostResolved {
			frontier[FrontierXcodeUIHostUnresolved] = true
		}
		inspectSwiftFiles(root, paths, rawImports, frontier)
		candidates = append(candidates, candidate{unit: unit, module: name, rawImports: rawImports, priority: 1})
	}
	return candidates, nil
}

type pbxObject string

func parsePBXObjects(text string) (map[string]pbxObject, bool) {
	objects := make(map[string]pbxObject, 256)
	complete := true
	for _, location := range objectStartPattern.FindAllStringSubmatchIndex(text, -1) {
		id := text[location[2]:location[3]]
		open := strings.IndexByte(text[location[0]:location[1]], '{') + location[0]
		end, ok := balancedBraces(text, open)
		if !ok {
			complete = false
			continue
		}
		observed := pbxObject(text[open+1 : end-1])
		previous, exists := objects[id]
		if !exists || previous.field("isa") == "" && observed.field("isa") != "" {
			objects[id] = observed
		}
	}
	return objects, complete
}

func balancedBraces(text string, open int) (int, bool) {
	depth := 0
	inString := false
	escaped := false
	for index := open; index < len(text); index++ {
		char := text[index]
		if inString {
			if escaped {
				escaped = false
			} else if char == '\\' {
				escaped = true
			} else if char == '"' {
				inString = false
			}
			continue
		}
		if char == '"' {
			inString = true
			continue
		}
		if char == '{' {
			depth++
		} else if char == '}' {
			depth--
			if depth == 0 {
				return index + 1, true
			}
		}
	}
	return len(text), false
}

func (object pbxObject) field(name string) string {
	pattern := regexp.MustCompile(`\b` + regexp.QuoteMeta(name) + `\s*=\s*("(?:\\.|[^"\\])*"|[^;]+);`)
	match := pattern.FindStringSubmatch(string(object))
	if len(match) == 0 {
		return ""
	}
	value := strings.TrimSpace(match[1])
	if strings.HasPrefix(value, `"`) {
		if decoded, err := strconv.Unquote(value); err == nil {
			return decoded
		}
	}
	return value
}

func (object pbxObject) fieldID(name string) string {
	match := identifierPattern.FindString(object.field(name))
	return match
}

func (object pbxObject) listIDs(name string) []string {
	pattern := regexp.MustCompile(`(?s)\b` + regexp.QuoteMeta(name) + `\s*=\s*\((.*?)\);`)
	match := pattern.FindStringSubmatch(string(object))
	if len(match) == 0 {
		return nil
	}
	return identifierPattern.FindAllString(match[1], -1)
}

func projectSchemes(root, projectBundle string, metadata []string, objects map[string]pbxObject, frontier map[string]bool) map[string][]string {
	result := make(map[string][]string)
	for _, relative := range metadata {
		if !strings.HasSuffix(relative, ".xcscheme") {
			continue
		}
		if !strings.Contains(relative, "/xcshareddata/xcschemes/") {
			continue
		}
		body, err := affected.ReadSource(root, relative)
		if err != nil {
			frontier[FrontierXcodeSchemeUnresolved] = true
			continue
		}
		if !strings.Contains(string(body), "container:"+projectBundle) && !strings.Contains(string(body), "container:"+path.Base(projectBundle)) {
			continue
		}
		scheme := strings.TrimSuffix(path.Base(relative), ".xcscheme")
		for _, targetID := range testableBlueprints(string(body)) {
			if _, known := objects[targetID]; known {
				result[targetID] = append(result[targetID], scheme)
			}
		}
	}
	for target := range result {
		result[target] = uniqueSorted(result[target])
	}
	return result
}

func testableBlueprints(text string) []string {
	values := make([]string, 0, 8)
	for offset := 0; ; {
		start := strings.Index(text[offset:], "<TestableReference")
		if start < 0 {
			break
		}
		start += offset
		end := strings.Index(text[start:], "</TestableReference>")
		if end < 0 {
			break
		}
		block := text[start : start+end]
		headerEnd := strings.IndexByte(block, '>')
		if headerEnd < 0 {
			break
		}
		header := block[:headerEnd]
		skipped := false
		for _, match := range attributePattern.FindAllStringSubmatch(header, -1) {
			if match[1] == "skipped" && strings.EqualFold(match[2], "YES") {
				skipped = true
			}
		}
		if skipped {
			offset = start + end + len("</TestableReference>")
			continue
		}
		for _, match := range attributePattern.FindAllStringSubmatch(block, -1) {
			if match[1] == "BlueprintIdentifier" {
				values = append(values, match[2])
				break
			}
		}
		offset = start + end + len("</TestableReference>")
	}
	return uniqueSorted(values)
}

func resolveProjectFile(projectRoot, fileRef string, fileRefs map[string]pbxObject, groupParents map[string]string, mainGroups map[string]bool, objects map[string]pbxObject, swiftFiles []string) (string, bool) {
	file, known := fileRefs[fileRef]
	if !known {
		return "", false
	}
	declared := file.field("path")
	if declared == "" {
		declared = file.field("name")
	}
	if declared == "" || !strings.HasSuffix(declared, ".swift") {
		return "", false
	}
	components := []string{declared}
	sourceTree := file.field("sourceTree")
	current := fileRef
	seen := map[string]bool{fileRef: true}
	for sourceTree == "" || sourceTree == "<group>" {
		parent, exists := groupParents[current]
		if !exists {
			if !mainGroups[current] {
				return "", false
			}
			break
		}
		if seen[parent] {
			return "", false
		}
		seen[parent] = true
		group := objects[parent]
		if component := group.field("path"); component != "" {
			components = append([]string{component}, components...)
		}
		sourceTree = group.field("sourceTree")
		current = parent
	}
	if sourceTree != "" && sourceTree != "<group>" && sourceTree != "SOURCE_ROOT" {
		return "", false
	}
	resolved := joinRelative(projectRoot, path.Join(components...))
	for _, observed := range swiftFiles {
		if observed == resolved {
			return observed, true
		}
	}
	return "", false
}

func inspectSwiftFiles(root string, files []string, imports map[string]bool, frontier map[string]bool) {
	for _, relative := range files {
		body, err := affected.ReadSource(root, relative)
		if err != nil {
			frontier[FrontierUnreadableSource] = true
			continue
		}
		text := string(body)
		code := swiftCodeMask(text)
		for _, match := range importPattern.FindAllStringSubmatch(text, -1) {
			imports[match[1]] = true
			if strings.Contains(strings.ToLower(match[1]), "appium") {
				frontier[FrontierAppiumExternal] = true
			}
		}
		if strings.Contains(code, "#if") {
			frontier[FrontierConditionalCompilation] = true
		}
		if dynamicTestPattern.MatchString(code) || hasDynamicParameterizedTest(text) {
			frontier[FrontierDynamicTestDiscovery] = true
		}
	}
}

func hasDynamicParameterizedTest(text string) bool {
	mask := swiftCodeMask(text)
	for offset := 0; offset < len(mask); {
		location := testingPattern.FindStringIndex(mask[offset:])
		if location == nil {
			return false
		}
		open := offset + location[1]
		for open < len(mask) && (mask[open] == ' ' || mask[open] == '\t' || mask[open] == '\n' || mask[open] == '\r') {
			open++
		}
		if open >= len(mask) || mask[open] != '(' {
			offset = open
			continue
		}
		body, end, ok := balancedCall(text, open)
		if !ok {
			return true
		}
		argument := strings.Index(body, "arguments:")
		if argument >= 0 {
			value := strings.TrimSpace(body[argument+len("arguments:"):])
			if !literalArgumentArray(value) {
				return true
			}
		}
		offset = end
	}
	return false
}

func literalArgumentArray(value string) bool {
	if value == "" || value[0] != '[' {
		return false
	}
	end, ok := balancedSquare(value, 0)
	if !ok {
		return false
	}
	inside := value[1 : end-1]
	if strings.Contains(inside, `\(`) {
		return false
	}
	mask := swiftCodeMask(inside)
	identifier := regexp.MustCompile(`[A-Za-z_][A-Za-z0-9_]*`)
	for _, location := range identifier.FindAllStringIndex(mask, -1) {
		token := mask[location[0]:location[1]]
		if token == "true" || token == "false" || token == "nil" {
			continue
		}
		if location[0] > 0 && mask[location[0]-1] == '.' {
			continue
		}
		return false
	}
	remainder := strings.TrimSpace(value[end:])
	if remainder == "" {
		return true
	}
	if !strings.HasPrefix(remainder, ",") {
		return false
	}
	remainder = strings.TrimSpace(remainder[1:])
	if strings.HasPrefix(remainder, "[") {
		return literalArgumentArray(remainder)
	}
	return strings.HasPrefix(remainder, ".")
}

func splitTestFiles(root string, files []string, ui bool, frontier map[string]bool) ([]string, []string) {
	frameworkKnown := ui
	tests := make([]string, 0, len(files))
	sources := make([]string, 0, len(files))
	for _, relative := range files {
		body, err := affected.ReadSource(root, relative)
		if err != nil {
			sources = append(sources, relative)
			continue
		}
		text := string(body)
		swiftTesting := strings.Contains(text, "import Testing") && testingPattern.MatchString(text)
		xctest := strings.Contains(text, "import XCTest") && xctestDeclPattern.MatchString(text)
		if swiftTesting {
			frameworkKnown = true
		}
		if strings.Contains(text, "import XCTest") || xctestPattern.MatchString(text) {
			frameworkKnown = true
		}
		if swiftTesting || xctest || dynamicTestPattern.MatchString(text) {
			tests = append(tests, relative)
		} else {
			sources = append(sources, relative)
		}
	}
	if !frameworkKnown || len(tests) == 0 {
		frontier[FrontierFrameworkUnresolved] = true
	}
	return tests, sources
}

func materialize(candidates []candidate, frontier map[string]bool) ([]affected.Unit, map[string]bool) {
	sort.Slice(candidates, func(left, right int) bool {
		if candidates[left].priority != candidates[right].priority {
			return candidates[left].priority < candidates[right].priority
		}
		return candidates[left].unit.ID < candidates[right].unit.ID
	})
	claimed := make(map[string]bool, 1024)
	kept := make([]candidate, 0, len(candidates))
	for _, candidate := range candidates {
		candidate.unit.Sources = claimPaths(candidate.unit.Sources, claimed, frontier)
		candidate.unit.Tests = claimPaths(candidate.unit.Tests, claimed, frontier)
		if len(candidate.unit.Sources) == 0 && len(candidate.unit.Tests) == 0 {
			continue
		}
		kept = append(kept, candidate)
	}
	modules := make(map[string][]string, len(kept))
	for _, candidate := range kept {
		modules[candidate.module] = append(modules[candidate.module], candidate.unit.ID)
	}
	units := make([]affected.Unit, 0, len(kept))
	for _, candidate := range kept {
		edges := make(map[string]bool, len(candidate.rawImports))
		for module := range candidate.rawImports {
			targets := modules[module]
			if len(targets) > 1 {
				frontier[FrontierAmbiguousImport] = true
			}
			for _, target := range targets {
				if target != candidate.unit.ID {
					edges[target] = true
				}
			}
		}
		candidate.unit.Imports = sortedKeys(edges)
		units = append(units, candidate.unit)
	}
	sort.Slice(units, func(left, right int) bool { return units[left].ID < units[right].ID })
	return units, claimed
}

func claimPaths(paths []string, claimed map[string]bool, frontier map[string]bool) []string {
	kept := make([]string, 0, len(paths))
	for _, value := range uniqueSorted(paths) {
		if claimed[value] {
			frontier[FrontierOverlappingMembership] = true
			continue
		}
		claimed[value] = true
		kept = append(kept, value)
	}
	return kept
}

func detectExternalHarnesses(root string, metadata []string, frontier map[string]bool) {
	for _, relative := range metadata {
		base := strings.ToLower(path.Base(relative))
		if strings.HasSuffix(base, ".yaml") || strings.HasSuffix(base, ".yml") {
			if strings.Contains(strings.ToLower(relative), "maestro") {
				frontier[FrontierMaestroExternal] = true
			}
			continue
		}
		if !appiumManifest(base) {
			continue
		}
		body, err := affected.ReadSource(root, relative)
		if err == nil && strings.Contains(strings.ToLower(string(body)), "appium") {
			frontier[FrontierAppiumExternal] = true
		}
	}
	maestroRoot := filepath.Join(root, ".maestro")
	entries := 0
	_ = filepath.WalkDir(maestroRoot, func(current string, entry fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		entries++
		if entries > affected.MaxWalkEntries {
			frontier[FrontierMaestroExternal] = true
			return fs.SkipAll
		}
		if entry.Type().IsRegular() && (strings.HasSuffix(entry.Name(), ".yaml") || strings.HasSuffix(entry.Name(), ".yml")) {
			frontier[FrontierMaestroExternal] = true
		}
		return nil
	})
}

func metadataFile(name string) bool {
	lower := strings.ToLower(name)
	return name == "Package.swift" || name == "project.pbxproj" || strings.HasSuffix(name, ".xcscheme") ||
		strings.HasSuffix(lower, ".yaml") || strings.HasSuffix(lower, ".yml") || appiumManifest(lower)
}

func appiumManifest(name string) bool {
	switch name {
	case "package.json", "pyproject.toml", "requirements.txt", "pom.xml", "build.gradle", "build.gradle.kts", "gemfile", "packages.config", "composer.json", "appium.yml", "appium.yaml", ".appiumrc":
		return true
	default:
		return strings.HasSuffix(name, ".csproj") || strings.HasSuffix(name, ".fsproj")
	}
}

func joinRelative(base, value string) string {
	joined := path.Clean(path.Join(base, value))
	if joined == "." {
		return "."
	}
	return strings.TrimPrefix(joined, "./")
}

func hasPathPrefix(value string, prefixes []string) bool {
	for _, prefix := range prefixes {
		if value == prefix || strings.HasPrefix(value, prefix+"/") {
			return true
		}
	}
	return false
}

func filterPaths(values []string, keep func(string) bool) []string {
	filtered := make([]string, 0, len(values))
	for _, value := range values {
		if keep(value) {
			filtered = append(filtered, value)
		}
	}
	return filtered
}

func uniqueSorted(values []string) []string {
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
