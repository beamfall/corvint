package testsource

import (
	"fmt"
	"maps"
	"path"
	"slices"
	"strings"

	"github.com/Beamfall/corvint/internal/appmap"
	"github.com/Beamfall/corvint/internal/testvalidity"
)

// The `corvint tests unselected` reading (TSA-V0-140..146): every committed test-looking JS/TS file
// that no configured runner selects under its configuration, with the reason, and the refusal of
// witness credit to such a file's tests.

const (
	// UnselectedSchema names the report.
	UnselectedSchema = "corvint-test-file-selection/0"
	// CodeTestFileUnselected is the finding code and the hygiene reason of a test whose file no
	// configured runner selects (TSA-V0-144, TSA-V0-146).
	CodeTestFileUnselected = "TEST-FILE-UNSELECTED"
)

// Reasons a test-looking file is not selected (TSA-V0-144).
const (
	ReasonNoRunnerForFramework = "no-runner-for-framework"
	ReasonOutsideInclude       = "outside-include"
	ReasonExcluded             = "excluded"
	ReasonSuffixMismatch       = "suffix-mismatch"
	// ReasonPatternRootDependent: a Playwright regular expression decides the path only through
	// the directories above the repository, so the file abstains (TSA-V0-654).
	ReasonPatternRootDependent = "runner-pattern-root-dependent"
)

// Selection states of one test-looking file.
const (
	SelectionSelected   = "selected"
	SelectionUnselected = "unselected"
	SelectionUncertain  = "uncertain"
)

// Framework attributions (TSA-V0-143).
const (
	frameworkGlobals   = "globals"
	frameworkIndirect  = "indirect"
	frameworkAmbiguous = "ambiguous"
	frameworkOther     = "unmodelled"
)

// UnselectedOptions selects the revision ("" reads HEAD).
type UnselectedOptions struct {
	Revision string
}

// UnselectedReport is the corvint-test-file-selection/0 report: the runners read, the unmodelled
// runner evidence, the selection of every test-looking file and one finding per unselected file.
type UnselectedReport struct {
	Schema     string              `json:"schema"`
	Revision   string              `json:"revision"`
	Runners    []SelectionRunner   `json:"runners"`
	Unmodelled []string            `json:"unmodelledRunners"`
	Files      []SelectionFile     `json:"files"`
	Findings   []UnselectedFinding `json:"findings"`
}

// SelectionFile is one test-looking file's selection. Runner, Config and Pattern name the runner
// whose verdict gave an unselected reason, or the runner an uncertain file abstained on.
type SelectionFile struct {
	Path      string `json:"path"`
	Framework string `json:"framework"`
	Selection string `json:"selection"`
	Reason    string `json:"reason,omitempty"`
	Runner    string `json:"runner,omitempty"`
	Config    string `json:"config,omitempty"`
	Pattern   string `json:"pattern,omitempty"`
	// ConfigLine is the line of Config that made its runner uncertain, when known (TSA-V0-648).
	ConfigLine int `json:"configLine,omitempty"`
}

// UnselectedFinding is one TEST-FILE-UNSELECTED finding.
type UnselectedFinding struct {
	Code      string `json:"code"`
	Path      string `json:"path"`
	Framework string `json:"framework"`
	Reason    string `json:"reason"`
	Runner    string `json:"runner,omitempty"`
	Config    string `json:"config,omitempty"`
	Pattern   string `json:"pattern,omitempty"`
}

var (
	// testFileSuffixes and testDirectories are the path prefilter of a test-looking file (TSA-V0-140).
	testFileSuffixes = []string{".spec", ".test", "_spec", "_test", "-spec", "-test", ".specs", ".tests", ".e2e", ".cy"}
	testDirectories  = map[string]bool{"__tests__": true, "__test__": true, "test": true, "tests": true, "spec": true,
		"specs": true, "e2e": true, "integration": true, "cypress": true}
	frameworkModules = map[string]string{"@playwright/test": RunnerPlaywright, "playwright/test": RunnerPlaywright,
		"vitest": RunnerVitest, "@jest/globals": RunnerJest, "mocha": RunnerMocha}
	// otherFrameworkModules are test frameworks this reading does not model.
	otherFrameworkModules = map[string]bool{"node:test": true, "bun:test": true, "ava": true, "tap": true, "uvu": true,
		"@japa/runner": true, "cypress": true, "@jest/test-globals": true}
	globalRunners = []string{RunnerJest, RunnerVitest, RunnerMocha}
)

// isTestCandidatePath is the path prefilter: a JS/TS file outside node_modules whose stem ends with
// a test suffix or that sits under a conventional test directory.
func isTestCandidatePath(name string) bool {
	extension := path.Ext(name)
	if !webExtensions[extension] || inNodeModules(name) || strings.HasSuffix(name, ".d.ts") {
		return false
	}
	stem := strings.TrimSuffix(path.Base(name), extension)
	for _, suffix := range testFileSuffixes {
		if strings.HasSuffix(stem, suffix) {
			return true
		}
	}
	parts := strings.Split(name, "/")
	for _, part := range parts[:len(parts)-1] {
		if testDirectories[part] {
			return true
		}
	}
	return false
}

// UnselectedFiles reads the committed runner configuration and test-looking files at the revision
// and classifies every test-looking file (TSA-V0-140..145). It reads committed Git content only,
// within the TSA-V0-012 bounds.
func UnselectedFiles(r Repo, options UnselectedOptions) (UnselectedReport, error) {
	revision := options.Revision
	if revision == "" {
		revision = "HEAD"
	}
	commit, err := r.Resolve(revision)
	if err != nil {
		return UnselectedReport{}, err
	}
	texts, _, err := r.readFiles(commit, func(name string) bool { return isRunnerConfigPath(name) || isTestCandidatePath(name) })
	if err != nil {
		return UnselectedReport{}, err
	}
	configs := map[string]string{}
	for name, text := range texts {
		if isRunnerConfigPath(name) {
			configs[name] = text
		}
	}
	set := discoverRunners(configs)
	report := UnselectedReport{Schema: UnselectedSchema, Revision: commit, Runners: set.runners, Unmodelled: set.unmodelled,
		Files: []SelectionFile{}, Findings: []UnselectedFinding{}}
	if report.Unmodelled == nil {
		report.Unmodelled = []string{}
	}
	names := []string{}
	for name := range texts {
		if isTestCandidatePath(name) {
			names = append(names, name)
		}
	}
	slices.Sort(names)
	resolver := newRepoResolver(r, commit)
	for _, name := range names {
		file, ok := classifyFile(name, parseSource(name, texts[name]), set, resolver)
		if !ok {
			continue
		}
		report.Files = append(report.Files, file)
		if file.Selection == SelectionUnselected {
			report.Findings = append(report.Findings, UnselectedFinding{Code: CodeTestFileUnselected, Path: name, Framework: file.Framework,
				Reason: file.Reason, Runner: file.Runner, Config: file.Config, Pattern: file.Pattern})
		}
	}
	return report, nil
}

// classifyFile classifies one candidate; ok is false when it declares no test or describe, so it
// is not test-looking (TSA-V0-140, TSA-V0-144).
func classifyFile(name string, s *source, set runnerSet, resolver *moduleResolver) (SelectionFile, bool) {
	if !isTestCandidatePath(name) || !slices.ContainsFunc(s.decls, func(d decl) bool { return d.kind == "test" || d.kind == "describe" }) {
		return SelectionFile{}, false
	}
	file := SelectionFile{Path: name, Framework: s.framework(resolver)}
	uncertain := func(reason string) (SelectionFile, bool) {
		file.Selection, file.Reason = SelectionUncertain, reason
		return file, true
	}
	switch {
	case s.uncertain:
		return uncertain("source-unparsed")
	case file.Framework == frameworkAmbiguous:
		return uncertain("framework-ambiguous")
	case file.Framework == frameworkOther:
		return uncertain("framework-unmodelled")
	case file.Framework == frameworkUnresolved:
		return uncertain(ReasonFrameworkUnresolved)
	case len(set.runners) == 0 && len(set.unmodelled) == 0:
		return uncertain("no-runner-configuration")
	}
	compatible := []SelectionRunner{}
	for _, runner := range set.runners {
		if runnerRuns(runner.Runner, file.Framework) {
			compatible = append(compatible, runner)
		}
	}
	verdicts := make([][3]string, len(compatible))
	for i := range compatible {
		if compatible[i].Status == RunnerUncertain {
			continue
		}
		selected, reason, pattern := compatible[i].verdict(name)
		if selected {
			file.Selection, file.Runner, file.Config = SelectionSelected, compatible[i].Runner, compatible[i].Config
			return file, true
		}
		verdicts[i] = [3]string{reason, pattern}
	}
	explicit := file.Framework != frameworkGlobals && file.Framework != frameworkIndirect
	if len(set.unmodelled) > 0 && !explicit {
		// An unmodelled runner may run a file that uses globals or an indirect import.
		return uncertain("runner-unmodelled")
	}
	for _, runner := range compatible {
		if runner.Status == RunnerUncertain {
			file.Runner, file.Config, file.ConfigLine = runner.Runner, runner.Config, runner.Line
			return uncertain(runner.Reason)
		}
	}
	for i, verdict := range verdicts {
		if verdict[0] == ReasonPatternRootDependent {
			file.Runner, file.Config, file.Pattern = compatible[i].Runner, compatible[i].Config, verdict[1]
			file.ConfigLine = compatible[i].patternLine(verdict[1])
			return uncertain(ReasonPatternRootDependent)
		}
	}
	file.Selection = SelectionUnselected
	if len(compatible) == 0 {
		file.Reason = ReasonNoRunnerForFramework
		return file, true
	}
	best := -1
	rank := map[string]int{ReasonExcluded: 0, ReasonSuffixMismatch: 1, ReasonOutsideInclude: 2}
	for i, verdict := range verdicts {
		if best < 0 || rank[verdict[0]] < rank[verdicts[best][0]] {
			best = i
		}
	}
	file.Reason, file.Pattern = verdicts[best][0], verdicts[best][1]
	file.Runner, file.Config = compatible[best].Runner, compatible[best].Config
	return file, true
}

// runnerRuns reports whether runner can run a file of framework (TSA-V0-143).
func runnerRuns(runner, framework string) bool {
	switch framework {
	case frameworkIndirect:
		return true
	case frameworkGlobals:
		return slices.Contains(globalRunners, runner)
	}
	return runner == framework
}

// framework attributes the file from its imports and requires (TSA-V0-143, TSA-V0-147): the one
// modelled framework module it imports, "unmodelled" for another test framework module, "ambiguous"
// for more than one, and "globals" otherwise. A test or describe root it calls (or expect) is
// followed through local aliases (`const test = base`, `base.extend(…)`); when it ends on a binding
// from another module, resolver follows that binding through committed relative modules, re-exports
// and extend-derived bindings to its framework module, and the file is "indirect" when some binding
// cannot be followed. A test or describe root declared in the file that ends on no import is
// "unresolved". A nil resolver follows no module.
func (s *source) framework(resolver *moduleResolver) string {
	found := map[string]bool{}
	for i, t := range s.tokens {
		if t.Kind != appmap.JSString || !t.Literal {
			continue
		}
		if _, ok := s.moduleBinding(i); !ok {
			continue
		}
		switch module := t.Text; {
		case frameworkModules[module] != "":
			found[frameworkModules[module]] = true
		case otherFrameworkModules[module]:
			found[frameworkOther] = true
		}
	}
	table := s.moduleTable()
	bound, resolved, followed, unresolved := []importedName{}, map[string]bool{}, true, false
	roots := map[string]bool{"expect": false}
	for _, d := range s.decls {
		roots[d.base] = true
	}
	for _, root := range slices.Sorted(maps.Keys(roots)) {
		declares, name := roots[root], table.rooted(root)
		b, ok := table.imports[name]
		switch {
		case ok && frameworkModules[b.spec] == "" && !otherFrameworkModules[b.spec]:
			bound = append(bound, b)
		case !ok && declares && (table.declared[name] || name != root):
			unresolved = true
		}
	}
	for _, b := range bound {
		framework := resolver.follow(s.path, b, 0)
		if framework == "" {
			followed = false
			continue
		}
		resolved[framework] = true
	}
	direct := len(found) > 0
	for framework := range resolved {
		found[framework] = true
	}
	switch {
	case len(found) > 1:
		return frameworkAmbiguous
	case len(found) == 1 && (direct || followed):
		for framework := range found {
			return framework
		}
	case len(bound) > 0:
		return frameworkIndirect
	case unresolved:
		return frameworkUnresolved
	}
	return frameworkGlobals
}

// moduleBinding reports whether the string at i is a module specifier (`from "m"`, `import "m"`,
// `require("m")`, `import("m")`) and returns the token range of the names it binds.
func (s *source) moduleBinding(i int) ([2]int, bool) {
	switch {
	case s.ident(i-1, "from"):
		for j := i - 2; j >= 0 && j > i-200; j-- {
			if s.ident(j, "import") || s.ident(j, "export") {
				return [2]int{j + 1, i - 1}, true
			}
		}
		return [2]int{}, true
	case s.ident(i-1, "import"):
		return [2]int{}, true
	case s.punct(i-1, "(") && s.punct(i+1, ")") && (s.ident(i-2, "require") || s.ident(i-2, "import")):
		for j := i - 3; j >= 0 && j > i-200; j-- {
			if s.ident(j, "const") || s.ident(j, "let") || s.ident(j, "var") {
				return [2]int{j + 1, i - 2}, true
			}
			if s.punct(j, ";") || s.punct(j, "}") && s.match[j] < 0 {
				break
			}
		}
		return [2]int{}, true
	}
	return [2]int{}, false
}

// SelectedSourceValidity is SourceValidity with witness credit refused to the tests of every named
// JS/TS file that no configured runner selects (TSA-V0-146): each such test gets an invalid
// TEST-FILE-UNSELECTED finding, so its hygiene axis is INELIGIBLE. An uncertain selection changes
// nothing.
func SelectedSourceValidity(r Repo, options SourceValidityOptions) (SourceValidityReport, error) {
	report, err := SourceValidity(r, options)
	if err != nil {
		return report, err
	}
	var set *runnerSet
	resolver := newRepoResolver(r, report.Revision)
	for fi := range report.Files {
		file := &report.Files[fi]
		if !isTestCandidatePath(file.Path) {
			continue
		}
		if set == nil {
			configs, _, err := r.readFiles(report.Revision, isRunnerConfigPath)
			if err != nil {
				return SourceValidityReport{}, err
			}
			discovered := discoverRunners(configs)
			set = &discovered
		}
		data, _, err := r.readOne(report.Revision, file.Path, MaxFileBytes)
		if err != nil {
			return SourceValidityReport{}, err
		}
		s := parseSource(file.Path, string(data))
		selection, ok := classifyFile(file.Path, s, *set, resolver)
		if !ok || selection.Selection != SelectionUnselected {
			continue
		}
		refuseUnselectedTests(&report, file, s, selection)
	}
	return report, nil
}

// refuseUnselectedTests adds one invalid finding per test of the unselected file and applies it.
func refuseUnselectedTests(report *SourceValidityReport, file *SourceValidityFile, s *source, selection SelectionFile) {
	cause := selection.Reason
	if selection.Pattern != "" {
		cause += ": " + selection.Pattern
	}
	finding := func(test string) testvalidity.SourceFinding {
		return testvalidity.SourceFinding{Rule: CodeTestFileUnselected, Severity: testvalidity.SeverityInvalid, Test: test, Location: test,
			Cause: cause, Message: fmt.Sprintf("no configured runner selects %s (%s), so its tests cannot witness a claim", file.Path, cause)}
	}
	seen := map[string]bool{}
	for ti := range report.Tests {
		test := &report.Tests[ti]
		if !strings.HasPrefix(test.Test, file.Path+":") {
			continue
		}
		seen[test.Test] = true
		f := finding(test.Test)
		file.Findings = append(file.Findings, f)
		test.Hygiene = testvalidity.ApplySourceFindings(testvalidity.Projection{Hygiene: test.Hygiene}, []testvalidity.SourceFinding{f}).Hygiene
	}
	for _, d := range s.decls {
		test := fmt.Sprintf("%s:%d", file.Path, d.line)
		if d.kind != "test" || seen[test] {
			continue
		}
		seen[test] = true
		f := finding(test)
		file.Findings = append(file.Findings, f)
		report.Tests = append(report.Tests, sourceTest(test, []testvalidity.SourceFinding{f}))
	}
}
