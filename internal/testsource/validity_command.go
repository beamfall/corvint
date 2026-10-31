package testsource

import (
	"fmt"
	"io/fs"
	"slices"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/testvalidity"
)

// The `corvint tests validity` reading (TSA-V0-071): the static test-validity rules over named
// committed test files, with the hygiene verdict each finding gives the test it names.

const (
	// SourceValiditySchema names the report.
	SourceValiditySchema = "corvint-test-source-validity/0"
	// MaxValidityFiles bounds the files one report reads.
	MaxValidityFiles = 256
	// hygieneReasonSourceRead is the baseline hygiene reason of a test the rules read.
	hygieneReasonSourceRead = "source-rules-read"
)

// SourceValidityOptions selects the revision ("" reads HEAD), the repository-relative test files
// to read and the new>legacy test links whose setup is compared (TSA-V0-100).
type SourceValidityOptions struct {
	Revision string
	Files    []string
	Links    []SeedLink
	// Profile, when set, adds the named environment profile's warnings on the files (TSA-V0-363).
	Profile string
}

// SourceValidityReport is the corvint-test-source-validity/0 report.
type SourceValidityReport struct {
	Schema   string               `json:"schema"`
	Revision string               `json:"revision"`
	Files    []SourceValidityFile `json:"files"`
	Tests    []SourceValidityTest `json:"tests"`
	// SeedParity is one advisory comparison per link, in argument order (TSA-V0-103).
	SeedParity []SeedParity `json:"seedParity,omitempty"`
	// AssertionParity is one advisory comparison per link, in argument order (TSA-V0-673).
	AssertionParity []AssertionParity  `json:"assertionParity,omitempty"`
	Env             *SourceValidityEnv `json:"env,omitempty"`
}

// SourceValidityFile is one file's findings and whether the reader abstained on part of it.
type SourceValidityFile struct {
	Path      string                       `json:"path"`
	Uncertain bool                         `json:"uncertain"`
	Reason    string                       `json:"reason,omitempty"`
	Findings  []testvalidity.SourceFinding `json:"findings"`
	// Abstentions are the advisory readings withheld as cross-file (TSA-V0-300).
	Abstentions []testvalidity.RuleAbstention `json:"abstentions,omitempty"`
	// Exemptions are the readings a justified annotation exempted, with its reason (TSA-V0-870).
	Exemptions []testvalidity.RuleExemption `json:"exemptions,omitempty"`
}

// SourceValidityTest is the hygiene axis a test's findings give it: ELIGIBLE with only advisory
// findings, INELIGIBLE with any invalid one (TSA-V0-051). A located JS/TS example also carries its
// NOT_MEASURED strength axis (TSA-V0-081). Conditions are the flag values an RSpec example forces,
// each with its "conditional on flag KEY=VALUE" qualifier (TSA-V0-440).
type SourceValidityTest struct {
	Test       string                       `json:"test"`
	Hygiene    testvalidity.Axis            `json:"hygiene"`
	Strength   *testvalidity.Axis           `json:"strength,omitempty"`
	Conditions []testvalidity.FlagCondition `json:"conditions,omitempty"`
}

// Refused reports whether any test is refused as proof.
func (r SourceValidityReport) Refused() bool {
	for _, t := range r.Tests {
		if t.Hygiene.State == testvalidity.HygieneIneligible {
			return true
		}
	}
	return false
}

// SourceValidity reads each named committed file at the revision and applies ValidityFindings and,
// for a `_spec.rb` file, the RSpec example rules (TSA-V0-062..066) or, for a JS/TS test file, the
// JS/TS example rules with one test per example in line order (TSA-V0-073..081). It reads committed Git content
// only. A path that is not a valid relative path refuses tests-invalid-arguments; a path that is
// not a committed regular file refuses tests-source-unavailable; more than MaxValidityFiles files
// or a file over MaxFileBytes refuses tests-bound-exceeded. Each link then adds its advisory
// seed-parity comparison (TSA-V0-100..109); a link reference that is not `path:line` refuses
// tests-invalid-arguments.
func SourceValidity(r Repo, options SourceValidityOptions) (SourceValidityReport, error) {
	return sourceValidity(r, options, newReadBudget())
}

// sourceValidity is SourceValidity with apps, the one application-read budget every spec prefix of
// the command draws on (TSA-V0-448).
func sourceValidity(r Repo, options SourceValidityOptions, apps *readBudget) (SourceValidityReport, error) {
	files := []string{}
	seen := map[string]bool{}
	for _, name := range options.Files {
		if !fs.ValidPath(name) || name == "." || name == ".git" || strings.HasPrefix(name, ".git/") || strings.ContainsAny(name, "\x00\n\r\\") {
			return SourceValidityReport{}, refuse("tests-invalid-arguments", "%q is not a repository-relative file path", name)
		}
		if !seen[name] {
			seen[name] = true
			files = append(files, name)
		}
	}
	for _, link := range options.Links {
		for _, ref := range []string{link.New, link.Legacy} {
			if _, _, ok := parseSeedRef(ref); !ok {
				return SourceValidityReport{}, refuse("tests-invalid-arguments", "%q is not a repository-relative path:line test reference", ref)
			}
		}
	}
	if len(files) == 0 && len(options.Links) == 0 {
		return SourceValidityReport{}, refuse("tests-invalid-arguments", "no test file was named")
	}
	if len(files) > MaxValidityFiles || len(options.Links) > MaxValidityFiles {
		return SourceValidityReport{}, refuse("tests-bound-exceeded", "more than %d files or links", MaxValidityFiles)
	}
	if options.Profile != "" && !ValidProfileName(options.Profile) {
		return SourceValidityReport{}, refuse("tests-invalid-arguments", "%q is not a profile name", options.Profile)
	}
	revision := options.Revision
	if revision == "" {
		revision = "HEAD"
	}
	commit, err := r.Resolve(revision)
	if err != nil {
		return SourceValidityReport{}, err
	}
	report := SourceValidityReport{Schema: SourceValiditySchema, Revision: commit, Files: []SourceValidityFile{}, Tests: []SourceValidityTest{}}
	supports := map[string]testvalidity.RubySupport{}
	var seeds func(name, text string) []testvalidity.SourceFinding
	var flags *testvalidity.FlagLookups
	for _, name := range files {
		if m := RSpecQualifiedRef.FindStringSubmatch(name); m != nil {
			file, test, err := r.qualifiedValidity(commit, m, supports, apps)
			if err != nil {
				return SourceValidityReport{}, err
			}
			report.Files = append(report.Files, file)
			report.Tests = append(report.Tests, test)
			continue
		}
		data, err := r.committedFile(commit, name)
		if err != nil {
			return SourceValidityReport{}, err
		}
		support := testvalidity.RubySupport{}
		if strings.HasSuffix(name, "_spec.rb") {
			support = r.rubySupport(commit, name, supports, apps)
			if flags == nil {
				read, err := r.flagLookups(commit)
				if err != nil {
					return SourceValidityReport{}, err
				}
				flags = &read
			}
			support.Flags = flags
		}
		file, tests := readSourceValidity(name, string(data), support)
		if IsTestFile(name) && !strings.HasSuffix(name, "_spec.rb") && !file.Uncertain {
			if seeds == nil {
				if seeds, err = r.seedRules(commit); err != nil {
					return SourceValidityReport{}, err
				}
			}
			file.Findings = append(file.Findings, seeds(name, string(data))...)
		}
		report.Files = append(report.Files, file)
		report.Tests = append(report.Tests, tests...)
	}
	for _, link := range options.Links {
		parity, err := r.seedParity(commit, link)
		if err != nil {
			return SourceValidityReport{}, err
		}
		report.SeedParity = append(report.SeedParity, parity)
		assertions, err := r.assertionParity(commit, link)
		if err != nil {
			return SourceValidityReport{}, err
		}
		report.AssertionParity = append(report.AssertionParity, assertions)
	}
	if options.Profile != "" {
		if report.Env, err = envValidity(r, commit, options.Profile, files); err != nil {
			return SourceValidityReport{}, err
		}
	}
	return report, nil
}

// seedRules reads the committed seed rule file at commit (TSA-V0-240) and returns the seed
// findings of one JS/TS test file under it: none when the file is absent, and one file-level
// SEED-VALIDATION-UNRESOLVED abstention when it does not decode.
func (r Repo) seedRules(commit string) (func(name, text string) []testvalidity.SourceFinding, error) {
	data, found, err := r.readOne(commit, SeedRulesPath, MaxFileBytes)
	if err != nil || !found {
		return func(string, string) []testvalidity.SourceFinding { return nil }, err
	}
	rules, err := DecodeSeedRules(data)
	if err != nil {
		reason := err.Error()
		return func(name, _ string) []testvalidity.SourceFinding {
			return []testvalidity.SourceFinding{{Rule: testvalidity.RuleSeedValidationUnresolved, Severity: testvalidity.SeverityAdvisory,
				Test: name, Location: SeedRulesPath, Cause: SeedRulesPath,
				Message: "abstained: " + SeedRulesPath + " is not a valid " + SeedRulesProfile + " file (" + reason + ")"}}
		}, nil
	}
	return func(name, text string) []testvalidity.SourceFinding { return SeedFindings(name, text, rules) }, nil
}

// committedFile reads one committed file at commit, refusing tests-source-unavailable when it is
// not a committed regular file.
func (r Repo) committedFile(commit, name string) ([]byte, error) {
	data, found, err := r.readOne(commit, name, MaxFileBytes)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, refuse("tests-source-unavailable", "%s is not a committed file at the revision", name)
	}
	return data, nil
}

// qualifiedValidity reads one qualified reference `SPEC:INCLUDE_LINE>SHARED:EXAMPLE_LINE`: the
// shared example evaluated through that one inclusion, reported under the reference. A reference
// the reader cannot bind is one ABSTAINED test with the refusal reason (TSA-V0-089..091).
func (r Repo) qualifiedValidity(commit string, m []string, supports map[string]testvalidity.RubySupport, apps *readBudget) (SourceValidityFile, SourceValidityTest, error) {
	q := testvalidity.RSpecInclusion{SpecPath: m[1], SharedPath: m[3]}
	fmt.Sscanf(m[2], "%d", &q.IncludeLine)
	fmt.Sscanf(m[4], "%d", &q.ExampleLine)
	if q.SharedPath == "same" {
		q.SharedPath = q.SpecPath
	}
	for _, path := range []string{q.SpecPath, q.SharedPath} {
		if !fs.ValidPath(path) || path == ".git" || strings.HasPrefix(path, ".git/") {
			return SourceValidityFile{}, SourceValidityTest{}, refuse("tests-invalid-arguments", "%q is not a repository-relative file path", path)
		}
	}
	spec, err := r.committedFile(commit, q.SpecPath)
	if err != nil {
		return SourceValidityFile{}, SourceValidityTest{}, err
	}
	shared := spec
	if q.SharedPath != q.SpecPath {
		if shared, err = r.committedFile(commit, q.SharedPath); err != nil {
			return SourceValidityFile{}, SourceValidityTest{}, err
		}
	}
	q.SpecText, q.SharedText = string(spec), string(shared)
	q.Support = r.rubySupport(commit, q.SpecPath, supports, apps)
	source, ref := RSpecInclusionSource(q)
	file := SourceValidityFile{Path: ref, Findings: source.Findings}
	if source.Read != testvalidity.ExampleSourceLocated {
		file.Uncertain, file.Reason = true, source.Read
		anchors := []string{ref}
		if source.Cause != "" {
			anchors = append(anchors, source.Cause)
		}
		return file, SourceValidityTest{Test: ref, Hygiene: testvalidity.Axis{State: testvalidity.HygieneAbstained, Reason: source.Read, Anchors: anchors}}, nil
	}
	return file, sourceTest(ref, source.Findings), nil
}

// sourceTest is the hygiene the findings give one test over the source-rules-read baseline.
func sourceTest(test string, findings []testvalidity.SourceFinding) SourceValidityTest {
	baseline := testvalidity.Projection{Hygiene: testvalidity.Axis{State: testvalidity.HygieneEligible, Reason: hygieneReasonSourceRead, Anchors: []string{test}}}
	return SourceValidityTest{Test: test, Hygiene: testvalidity.ApplySourceFindings(baseline, findings).Hygiene}
}

// findingTests is one test per test the findings name, in first-finding order.
func findingTests(findings []testvalidity.SourceFinding, skip map[string]bool) []SourceValidityTest {
	byTest := map[string][]testvalidity.SourceFinding{}
	order := []string{}
	for _, f := range findings {
		if skip[f.Test] {
			continue
		}
		if _, ok := byTest[f.Test]; !ok {
			order = append(order, f.Test)
		}
		byTest[f.Test] = append(byTest[f.Test], f)
	}
	out := make([]SourceValidityTest, 0, len(order))
	for _, test := range order {
		out = append(out, sourceTest(test, byTest[test]))
	}
	return out
}

// readSourceValidity applies the rules to one file's text and returns its tests. A `_spec.rb` file's
// findings are every example's findings in source order. A JS/TS test file's findings are the
// ValidityFindings followed by the example rules' findings in line order, and its tests are one per
// example in line order: a located example takes its findings and strength, an unlocated one
// abstains with the reader's reason unless a finding already refuses it (TSA-V0-081).
func readSourceValidity(name, text string, support testvalidity.RubySupport) (SourceValidityFile, []SourceValidityTest) {
	if strings.HasSuffix(name, "_spec.rb") {
		file := SourceValidityFile{Path: name, Findings: []testvalidity.SourceFinding{}}
		examples, reason := RSpecExampleSourcesWithSupport(name, text, support)
		if reason != "" {
			file.Uncertain, file.Reason = true, reason
			return file, findingTests(file.Findings, nil)
		}
		for _, line := range exampleLines(examples) {
			file.Findings = append(file.Findings, examples[line].Findings...)
		}
		tests := findingTests(file.Findings, nil)
		for i, t := range tests {
			if line, err := strconv.Atoi(t.Test[strings.LastIndex(t.Test, ":")+1:]); err == nil {
				tests[i].Conditions = examples[line].Conditions
			}
		}
		return file, tests
	}
	base := ValidityFindings(name, text)
	file := SourceValidityFile{Path: name, Uncertain: base.Uncertain, Reason: base.Reason, Findings: base.Findings, Abstentions: base.Abstentions, Exemptions: base.Exemptions}
	if !IsTestFile(name) || base.Uncertain {
		return file, findingTests(file.Findings, nil)
	}
	examples, reason := JSExampleSources(name, text)
	if reason != "" {
		file.Uncertain, file.Reason = true, reason
		return file, findingTests(file.Findings, nil)
	}
	file.Findings = append([]testvalidity.SourceFinding{}, base.Findings...)
	baseByTest := map[string][]testvalidity.SourceFinding{}
	for _, f := range base.Findings {
		baseByTest[f.Test] = append(baseByTest[f.Test], f)
	}
	tests, covered := []SourceValidityTest{}, map[string]bool{}
	for _, line := range exampleLines(examples) {
		example := examples[line]
		test := fmt.Sprintf("%s:%d", name, line)
		covered[test] = true
		if example.Read != testvalidity.ExampleSourceLocated {
			t := sourceTest(test, baseByTest[test])
			if t.Hygiene.State != testvalidity.HygieneIneligible {
				t.Hygiene = testvalidity.Axis{State: testvalidity.HygieneAbstained, Reason: example.Read, Anchors: []string{test}}
			}
			tests = append(tests, t)
			continue
		}
		for _, f := range example.Findings {
			if !slices.Contains(base.Findings, f) {
				file.Findings = append(file.Findings, f)
			}
		}
		t := sourceTest(test, example.Findings)
		strength := testvalidity.JSExampleStrength(example, test)
		t.Strength = &strength
		tests = append(tests, t)
	}
	return file, append(tests, findingTests(base.Findings, covered)...)
}

// RubySupportPaths reports whether path is in the support set of a spec under prefix (the text
// before `spec/`): spec/spec_helper.rb, spec/rails_helper.rb or spec/support/**/*.rb (TSA-V0-088).
func RubySupportPaths(prefix string) func(string) bool {
	return func(path string) bool {
		rest, ok := strings.CutPrefix(path, prefix+"spec/")
		return ok && (rest == "spec_helper.rb" || rest == "rails_helper.rb" || strings.HasPrefix(rest, "support/") && strings.HasSuffix(rest, ".rb"))
	}
}

// specPrefix is the text of a spec path before its `spec/` directory, or "" when it has none.
func specPrefix(name string) string {
	if strings.HasPrefix(name, "spec/") {
		return ""
	}
	if k := strings.LastIndex(name, "/spec/"); k >= 0 {
		return name[:k+1]
	}
	return ""
}

// rubySupport reads the committed support set of a spec at commit once per prefix. A read that
// fails or exceeds the bounds leaves the set unbound with a named reason, so helper resolution
// abstains instead of guessing (TSA-V0-088). The application source it binds draws on apps.
func (r Repo) rubySupport(commit, name string, cache map[string]testvalidity.RubySupport, apps *readBudget) testvalidity.RubySupport {
	prefix := specPrefix(name)
	if support, ok := cache[prefix]; ok {
		return support
	}
	files, _, err := r.readFiles(commit, RubySupportPaths(prefix))
	support := testvalidity.RubySupport{Files: files}
	if err != nil {
		support = testvalidity.RubySupport{Unbound: "rspec-support-unreadable"}
	}
	support.App = r.rubyApp(commit, prefix, apps)
	cache[prefix] = support
	return support
}

// maxAppFiles bounds the application files one prefix lists for TSA-V0-448.
const maxAppFiles = 20000

// rubyApp lists the committed application source under prefix that a never-written assertion
// target resolves in (TSA-V0-448): the .rb files under app/, lib/ and config/, and db/schema.rb.
// Each file is read on first use, at most MaxFileBytes, and cached; ReadAll reads the rest at
// once. Every read draws on budget, which the command shares across all spec prefixes, so a file
// over a bound or past the budget reads as absent and ReadAll fails. A listing that fails or
// exceeds maxAppFiles lists none, so no target resolves.
func (r Repo) rubyApp(commit, prefix string, budget *readBudget) *testvalidity.RubyApp {
	entries, err := r.tree(commit, func(path string) bool {
		rest, ok := strings.CutPrefix(path, prefix)
		return ok && (rest == "db/schema.rb" || (strings.HasPrefix(rest, "app/") || strings.HasPrefix(rest, "lib/") || strings.HasPrefix(rest, "config/")) && strings.HasSuffix(rest, ".rb"))
	}, maxAppFiles)
	if err != nil {
		return nil
	}
	listed := map[string]entry{}
	paths := make([]string, 0, len(entries))
	for _, e := range entries {
		listed[e.path] = e
		paths = append(paths, e.path)
	}
	slices.Sort(paths)
	read := map[string]string{}
	failed := map[string]bool{}
	// load reads the given unread entries in one batch within budget, or none.
	load := func(batch []entry) bool {
		if len(batch) > budget.files {
			return false
		}
		data, used, err := r.blobs(batch, budget.bytes)
		if err != nil {
			return false
		}
		budget.files -= len(batch)
		budget.bytes -= used
		for _, e := range batch {
			read[e.path] = string(data[e.oid])
		}
		return true
	}
	app := &testvalidity.RubyApp{Paths: paths}
	app.Read = func(path string) (string, bool) {
		if text, ok := read[path]; ok {
			return text, true
		}
		e, ok := listed[path]
		if !ok || failed[path] {
			return "", false
		}
		if !load([]entry{e}) {
			failed[path] = true
			return "", false
		}
		return read[path], true
	}
	complete, tried := false, false
	app.ReadAll = func() bool {
		if tried {
			return complete
		}
		tried = true
		batch := []entry{}
		for _, path := range paths {
			if _, ok := read[path]; ok {
				continue
			}
			if failed[path] {
				return false
			}
			batch = append(batch, listed[path])
		}
		complete = load(batch)
		return complete
	}
	return app
}

func exampleLines(examples map[int]testvalidity.ExampleSource) []int {
	lines := make([]int, 0, len(examples))
	for line := range examples {
		lines = append(lines, line)
	}
	slices.Sort(lines)
	return lines
}
