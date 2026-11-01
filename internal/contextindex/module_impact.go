package contextindex

import (
	"fmt"
	"path"
	"regexp"
	"sort"
	"strings"
	"testing/fstest"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/swift"
)

// This file holds the module-membership reverse-reference readings of path
// impact for Swift, Kotlin and C# (NGI-V0-980 to NGI-V0-987). None of them is
// a `GPK-V0-027` reverse-import rule: the frozen oracle has no arm for these
// suffixes, so `ImpactRuleNamed` stays false for them and the receipt's
// capability listing names them separately. Each reading is syntax evidence
// over pinned index text; it compiles nothing, reads no build graph beyond the
// SwiftPM manifests the affected Swift model already parses, and names what it
// leaves unresolved as coverage uncertainty rather than as an empty answer.

// Coverage lines a request naming a path of each language carries
// (NGI-V0-985). They name the precision limits, never a closure.
const (
	swiftModuleUncertainty  = "Swift reverse references follow SwiftPM target membership and import declarations only: Xcode project membership, @_exported re-exports, conditional compilation and dependents beyond one import are not resolved"
	kotlinModuleUncertainty = "Kotlin reverse references follow package declarations, imports and declared names only: Gradle module visibility, Java importers, fully qualified references without an import, wildcard-only extension calls and reflection are not resolved"
	csharpModuleUncertainty = "C# reverse references follow namespace declarations, using directives and declared type names only: project references, global using reach, extension-method calls, fully qualified references without a using and reflection are not resolved"
)

// ModuleRuleLanguage names the module-membership reading for a changed path's
// suffix, or "" when none applies. Like rule (a) it tests the raw suffix, so
// the listing reports it case-sensitive. `.kts` is a build or script file that
// nothing imports, so it carries no reading and keeps the profile gap.
func ModuleRuleLanguage(changedPath string) string {
	switch path.Ext(changedPath) {
	case ".swift":
		return "swift"
	case ".kt":
		return "kotlin"
	case ".cs":
		return "csharp"
	}
	return ""
}

// moduleReading is one impact request's view of the three readings. It parses
// each source at most once and builds the SwiftPM target map only when a
// Swift path asks for it.
type moduleReading struct {
	index     *Index
	files     map[string]*moduleFile
	swift     *swiftTargetMap
	requested map[string]bool
	mapped    map[string]struct{}
	notes     map[string]struct{}
}

type moduleFile struct {
	loaded     bool
	lines      []string
	pkg        string
	namespaces []string
	imports    []moduleImport
}

type moduleImport struct {
	name          string
	line          int
	star, typeRef bool
	global        bool
}

func newModuleReading(index *Index) *moduleReading {
	return &moduleReading{index: index, files: map[string]*moduleFile{}, requested: map[string]bool{},
		mapped: map[string]struct{}{}, notes: map[string]struct{}{}}
}

var (
	kotlinPackagePattern = regexp.MustCompile(`(?m)^[ \t]*package[ \t]+([A-Za-z_][A-Za-z0-9_]*(?:\.[A-Za-z_][A-Za-z0-9_]*)*)`)
	kotlinImportPattern  = regexp.MustCompile(`(?m)^[ \t]*import[ \t]+([A-Za-z_][A-Za-z0-9_]*(?:\.[A-Za-z_][A-Za-z0-9_]*)*)(\.\*)?`)
	csharpNamespace      = regexp.MustCompile(`\bnamespace[ \t\r\n]+([A-Za-z_][A-Za-z0-9_]*(?:\.[A-Za-z_][A-Za-z0-9_]*)*)`)
	csharpUsingPattern   = regexp.MustCompile(`(?m)^[ \t]*(global[ \t]+)?using[ \t]+(static[ \t]+)?(?:([A-Za-z_][A-Za-z0-9_]*)[ \t]*=[ \t]*)?(?:global::)?([A-Za-z_][A-Za-z0-9_]*(?:\.[A-Za-z_][A-Za-z0-9_]*)*)[ \t]*;`)
)

// codeOnlyText blanks every comment and string byte of text, keeping offsets
// and newlines, with the comment syntax the marker scan reads for the suffix.
func codeOnlyText(filePath, text string) string {
	syntax, ok := markerComments[pathExt(filePath)]
	if !ok {
		return text
	}
	out := blankedBytes(text)
	for i := 0; i < len(text); {
		end, comment := syntax.next(text, i)
		if !comment && end == i+1 {
			out[i] = text[i]
		}
		i = end
	}
	return string(out)
}

func lineOf(text string, offset int) int { return strings.Count(text[:offset], "\n") + 1 }

// file parses one source's declarations once per request.
func (reading *moduleReading) file(name string) *moduleFile {
	if parsed, ok := reading.files[name]; ok {
		return parsed
	}
	parsed := &moduleFile{}
	reading.files[name] = parsed
	text, valid, loaded := reading.index.Sources[name].Text()
	if !valid || !loaded {
		return parsed
	}
	parsed.loaded = true
	code := codeOnlyText(name, text)
	// The directives are blanked from the lines a reference is read on, so a
	// directive naming a package or namespace is never its own corroboration.
	body := []byte(code)
	directive := func(match []int) {
		copy(body[match[0]:match[1]], blankedBytes(code[match[0]:match[1]]))
	}
	switch ModuleRuleLanguage(name) {
	case "kotlin":
		if match := kotlinPackagePattern.FindStringSubmatchIndex(code); match != nil {
			parsed.pkg = code[match[2]:match[3]]
			directive(match)
		}
		for _, match := range kotlinImportPattern.FindAllStringSubmatchIndex(code, -1) {
			parsed.imports = append(parsed.imports, moduleImport{name: code[match[2]:match[3]], line: lineOf(code, match[2]), star: match[4] >= 0})
			directive(match)
		}
	case "csharp":
		for _, match := range csharpNamespace.FindAllStringSubmatchIndex(code, -1) {
			parsed.namespaces = append(parsed.namespaces, code[match[2]:match[3]])
			directive(match)
		}
		for _, match := range csharpUsingPattern.FindAllStringSubmatchIndex(code, -1) {
			parsed.imports = append(parsed.imports, moduleImport{name: code[match[8]:match[9]], line: lineOf(code, match[8]),
				typeRef: match[4] >= 0 || match[6] >= 0, global: match[2] >= 0})
			directive(match)
		}
	}
	parsed.lines = strings.Split(string(body), "\n")
	return parsed
}

// swiftTargetMap is the affected Swift model's SwiftPM target membership over
// the index's pinned Swift text (NGI-V0-980).
type swiftTargetMap struct {
	unit    map[string]string
	members map[string][]string
	module  map[string]string
	units   map[string][]string
	failed  bool
}

func buildSwiftTargetMap(index *Index) *swiftTargetMap {
	targets := &swiftTargetMap{unit: map[string]string{}, members: map[string][]string{}, module: map[string]string{}, units: map[string][]string{}}
	fsys := fstest.MapFS{}
	for name, source := range index.Sources {
		if !strings.HasSuffix(name, ".swift") {
			continue
		}
		if text, valid, loaded := source.Text(); valid && loaded {
			fsys[name] = &fstest.MapFile{Data: []byte(text), Mode: 0o644}
		}
	}
	result, err := swift.New().UnitsSource(affected.FSSource(fsys))
	if err != nil {
		targets.failed = true
		return targets
	}
	for _, unit := range result.Units {
		module, ok := swift.TargetModule(unit.ID)
		if !ok {
			continue
		}
		targets.module[unit.ID] = module
		targets.units[module] = append(targets.units[module], unit.ID)
		for _, member := range append(append([]string(nil), unit.Sources...), unit.Tests...) {
			targets.unit[member] = unit.ID
			targets.members[unit.ID] = append(targets.members[unit.ID], member)
		}
	}
	return targets
}

// declaredNames are the changed path's extracted symbol names: every name for
// matching an import that spells it, and the names of at least four runes for
// matching a bare reference, as the Go same-package reading uses.
func (reading *moduleReading) declaredNames(changedPath string) (all map[string]bool, words []string) {
	all = map[string]bool{}
	for _, symbol := range reading.index.Symbols {
		if symbol.Path == changedPath {
			all[symbol.Name] = true
		}
	}
	for name := range all {
		if utf8.RuneCountInString(name) >= 4 {
			words = append(words, name)
		}
	}
	sort.Strings(words)
	return all, words
}

// references finds the code lines of a parsed source that name a declared word.
func references(name string, parsed *moduleFile, words []string) packageReference {
	matches := make([]nameMatch, 0)
	for lineIndex, line := range parsed.lines {
		for _, word := range words {
			if containsPythonWord(line, word) {
				matches = append(matches, nameMatch{lineIndex + 1, word})
			}
		}
	}
	count := len(matches)
	if len(matches) > maxEvidence {
		matches = matches[:maxEvidence]
	}
	return packageReference{name, matches, count}
}

// impact returns the reading's rows for one changed non-test path and records
// whether the path's module was resolved (NGI-V0-984).
func (reading *moduleReading) impact(changedPath string) []rankedResult {
	language := ModuleRuleLanguage(changedPath)
	if language == "" || reading.index.isTest(changedPath) {
		return nil
	}
	reading.requested[language] = true
	switch language {
	case "swift":
		return reading.swiftImpact(changedPath)
	case "kotlin":
		return reading.kotlinImpact(changedPath)
	}
	return reading.csharpImpact(changedPath)
}

func (reading *moduleReading) sortedSources(language string) []string {
	names := make([]string, 0)
	for name := range reading.index.Sources {
		if ModuleRuleLanguage(name) == language {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

func (reading *moduleReading) importRow(changedPath, importerPath string, line int, imported, summary, confidence string) rankedResult {
	score := 700
	if reading.index.isTest(importerPath) {
		score = 650
	}
	if confidence != "high" {
		score -= 50
	}
	source := reading.index.Sources[importerPath]
	return rankedResult{score: score, order: 1, key: importerPath, result: map[string]any{
		"kind": "reverse-import", "id": importerPath, "score": score, "summary": summary,
		"evidence": []any{evidence(importerPath, line, source.BlobHash, "imports "+imported, confidence, "syntax")},
	}}
}

// referenceRow is a same-module reference: the module's members see the
// changed path's declarations without an import, so a named declaration is the
// only syntax evidence of a dependency (NGI-V0-983).
func (reading *moduleReading) referenceRow(changedPath string, reference packageReference, scope string) rankedResult {
	kind, score := "reference", 775
	if reading.index.isTest(reference.path) {
		kind, score = "test", 650
	}
	result := map[string]any{
		"kind": kind, "id": reference.path, "score": score,
		"summary":  "same-" + scope + " code references declarations from " + changedPath,
		"evidence": packageReferenceEvidence(reading.index, changedPath, reference),
	}
	noteEvidenceTruncated(result, reference.count-len(reference.matches))
	return rankedResult{score: score, order: 1, key: reference.path, result: result}
}

func (reading *moduleReading) swiftImpact(changedPath string) []rankedResult {
	if reading.swift == nil {
		reading.swift = buildSwiftTargetMap(reading.index)
	}
	targets := reading.swift
	unitID, ok := targets.unit[changedPath]
	if !ok {
		return nil
	}
	reading.mapped[changedPath] = struct{}{}
	module := targets.module[unitID]
	if len(targets.units[module]) > 1 {
		reading.notes[fmt.Sprintf("Swift module %s names %d SwiftPM targets, so its importers are reported for each", module, len(targets.units[module]))] = struct{}{}
	}
	_, words := reading.declaredNames(changedPath)
	rows := make([]rankedResult, 0)
	members := map[string]bool{}
	for _, member := range targets.members[unitID] {
		members[member] = true
		if member == changedPath {
			continue
		}
		parsed := reading.file(member)
		if !parsed.loaded {
			continue
		}
		if reference := references(member, parsed, words); reference.count != 0 {
			rows = append(rows, reading.referenceRow(changedPath, reference, "target"))
		}
	}
	for _, name := range reading.sortedSources("swift") {
		if members[name] || path.Base(name) == "Package.swift" {
			continue
		}
		text, valid, loaded := reading.index.Sources[name].Text()
		if !valid || !loaded {
			continue
		}
		for _, imported := range swift.ImportedModules(text) {
			if imported.Module == module {
				rows = append(rows, reading.importRow(changedPath, name, imported.Line, module,
					"imports SwiftPM target module "+module+" containing "+changedPath, "high"))
				break
			}
		}
	}
	return rows
}

func (reading *moduleReading) kotlinImpact(changedPath string) []rankedResult {
	changed := reading.file(changedPath)
	if !changed.loaded {
		return nil
	}
	reading.mapped[changedPath] = struct{}{}
	all, words := reading.declaredNames(changedPath)
	rows := make([]rankedResult, 0)
	for _, name := range reading.sortedSources("kotlin") {
		if name == changedPath {
			continue
		}
		parsed := reading.file(name)
		if !parsed.loaded {
			continue
		}
		if row, ok := reading.kotlinImport(changedPath, changed.pkg, name, parsed, all, words); ok {
			rows = append(rows, row)
			continue
		}
		if parsed.pkg == changed.pkg {
			if reference := references(name, parsed, words); reference.count != 0 {
				rows = append(rows, reading.referenceRow(changedPath, reference, "package"))
			}
		}
	}
	return rows
}

// kotlinImport resolves an explicit import of a declaration the changed path
// declares, else a wildcard import of its package corroborated by a named
// declaration (NGI-V0-981). The default package cannot be imported.
func (reading *moduleReading) kotlinImport(changedPath, pkg, name string, parsed *moduleFile, all map[string]bool, words []string) (rankedResult, bool) {
	if pkg == "" || parsed.pkg == pkg {
		return rankedResult{}, false
	}
	for _, imported := range parsed.imports {
		if imported.star || !strings.HasPrefix(imported.name, pkg+".") {
			continue
		}
		declaration, _, _ := strings.Cut(strings.TrimPrefix(imported.name, pkg+"."), ".")
		if all[declaration] {
			return reading.importRow(changedPath, name, imported.line, imported.name,
				"imports "+declaration+" declared by "+changedPath, "high"), true
		}
	}
	for _, imported := range parsed.imports {
		if imported.star && imported.name == pkg {
			if reference := references(name, parsed, words); reference.count != 0 {
				return reading.importRow(changedPath, name, imported.line, imported.name+".*",
					"wildcard-imports package "+pkg+" of "+changedPath+" and names "+reference.matches[0].name, "medium"), true
			}
		}
	}
	return rankedResult{}, false
}

func (reading *moduleReading) csharpImpact(changedPath string) []rankedResult {
	changed := reading.file(changedPath)
	if !changed.loaded {
		return nil
	}
	reading.mapped[changedPath] = struct{}{}
	namespaces := changed.namespaces
	if len(namespaces) == 0 {
		namespaces = []string{""}
	}
	all, words := reading.declaredNames(changedPath)
	rows := make([]rankedResult, 0)
	for _, name := range reading.sortedSources("csharp") {
		if name == changedPath {
			continue
		}
		parsed := reading.file(name)
		if !parsed.loaded {
			continue
		}
		if csharpEncloses(namespaces, parsed.namespaces) {
			if reference := references(name, parsed, words); reference.count != 0 {
				rows = append(rows, reading.referenceRow(changedPath, reference, "namespace"))
			}
			continue
		}
		if row, ok := reading.csharpUsing(changedPath, namespaces, name, parsed, all, words); ok {
			rows = append(rows, row)
		}
	}
	return rows
}

// csharpEncloses reports whether code in one of the importer's namespaces
// sees the changed path's types without a using directive: C# name lookup
// searches the enclosing namespaces outward, and the global namespace encloses
// every other.
func csharpEncloses(declared, importer []string) bool {
	if len(importer) == 0 {
		importer = []string{""}
	}
	for _, outer := range declared {
		for _, inner := range importer {
			if outer == "" || inner == outer || strings.HasPrefix(inner, outer+".") {
				return true
			}
		}
	}
	return false
}

// csharpUsing resolves a `using static` or alias directive naming a declared
// type, else a namespace using directive corroborated by a named declaration
// (NGI-V0-982). A global using reaches every file of its project, which the
// index cannot enumerate, so its reach is disclosed rather than inferred.
func (reading *moduleReading) csharpUsing(changedPath string, namespaces []string, name string, parsed *moduleFile, all map[string]bool, words []string) (rankedResult, bool) {
	for _, imported := range parsed.imports {
		if !imported.typeRef {
			continue
		}
		for _, namespace := range namespaces {
			declaration := imported.name
			if namespace != "" {
				if !strings.HasPrefix(declaration, namespace+".") {
					continue
				}
				declaration = strings.TrimPrefix(declaration, namespace+".")
			}
			if all[declaration] {
				return reading.importRow(changedPath, name, imported.line, imported.name,
					"imports type "+declaration+" declared by "+changedPath, "high"), true
			}
		}
	}
	for _, imported := range parsed.imports {
		if imported.typeRef || !stringIn(namespaces, imported.name) || imported.name == "" {
			continue
		}
		if imported.global {
			reading.notes["a global using of namespace "+imported.name+" reaches every file of its project, which C# reverse references do not enumerate"] = struct{}{}
		}
		if reference := references(name, parsed, words); reference.count != 0 {
			return reading.importRow(changedPath, name, imported.line, imported.name,
				"uses namespace "+imported.name+" of "+changedPath+" and names "+reference.matches[0].name, "medium"), true
		}
	}
	return rankedResult{}, false
}

// disclosures are the coverage lines for the languages the request named,
// then the request's observed notes, each in a stable order.
func (reading *moduleReading) disclosures() []string {
	if reading == nil {
		return nil
	}
	lines := make([]string, 0, 3+len(reading.notes))
	for _, item := range []struct{ language, line string }{
		{"csharp", csharpModuleUncertainty}, {"kotlin", kotlinModuleUncertainty}, {"swift", swiftModuleUncertainty},
	} {
		if reading.requested[item.language] {
			lines = append(lines, item.line)
		}
	}
	if reading.swift != nil && reading.swift.failed {
		lines = append(lines, "the SwiftPM target map could not be read, so Swift paths have no target membership")
	}
	notes := make([]string, 0, len(reading.notes))
	for note := range reading.notes {
		notes = append(notes, note)
	}
	sort.Strings(notes)
	return append(lines, notes...)
}

// withModuleMapped returns a view of index naming the paths a module reading
// resolved, so their coverage withholds the profile gap; like withRubyMapped
// it never writes the shared index.
func (index *Index) withModuleMapped(reading *moduleReading) *Index {
	if reading == nil || len(reading.mapped) == 0 {
		return index
	}
	view := *index
	view.moduleMapped = reading.mapped
	return &view
}
