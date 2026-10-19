package testsource

import (
	"fmt"
	"io/fs"
	"slices"
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
}

// SourceValidityReport is the corvint-test-source-validity/0 report.
type SourceValidityReport struct {
	Schema   string               `json:"schema"`
	Revision string               `json:"revision"`
	Files    []SourceValidityFile `json:"files"`
	Tests    []SourceValidityTest `json:"tests"`
	// SeedParity is one advisory comparison per link, in argument order (TSA-V0-103).
	SeedParity []SeedParity `json:"seedParity,omitempty"`
}

// SourceValidityFile is one file's findings and whether the reader abstained on part of it.
type SourceValidityFile struct {
	Path      string                       `json:"path"`
	Uncertain bool                         `json:"uncertain"`
	Reason    string                       `json:"reason,omitempty"`
	Findings  []testvalidity.SourceFinding `json:"findings"`
}

// SourceValidityTest is the hygiene axis a test's findings give it: ELIGIBLE with only advisory
// findings, INELIGIBLE with any invalid one (TSA-V0-051). A located JS/TS example also carries its
// NOT_MEASURED strength axis (TSA-V0-081).
type SourceValidityTest struct {
	Test     string             `json:"test"`
	Hygiene  testvalidity.Axis  `json:"hygiene"`
	Strength *testvalidity.Axis `json:"strength,omitempty"`
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
	revision := options.Revision
	if revision == "" {
		revision = "HEAD"
	}
	commit, err := r.Resolve(revision)
	if err != nil {
		return SourceValidityReport{}, err
	}
	report := SourceValidityReport{Schema: SourceValiditySchema, Revision: commit, Files: []SourceValidityFile{}, Tests: []SourceValidityTest{}}
	for _, name := range files {
		data, found, err := r.readOne(commit, name, MaxFileBytes)
		if err != nil {
			return SourceValidityReport{}, err
		}
		if !found {
			return SourceValidityReport{}, refuse("tests-source-unavailable", "%s is not a committed file at the revision", name)
		}
		file, tests := readSourceValidity(name, string(data))
		report.Files = append(report.Files, file)
		report.Tests = append(report.Tests, tests...)
	}
	for _, link := range options.Links {
		parity, err := r.seedParity(commit, link)
		if err != nil {
			return SourceValidityReport{}, err
		}
		report.SeedParity = append(report.SeedParity, parity)
	}
	return report, nil
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
func readSourceValidity(name, text string) (SourceValidityFile, []SourceValidityTest) {
	if strings.HasSuffix(name, "_spec.rb") {
		file := SourceValidityFile{Path: name, Findings: []testvalidity.SourceFinding{}}
		examples, reason := RSpecExampleSources(name, text)
		if reason != "" {
			file.Uncertain, file.Reason = true, reason
			return file, findingTests(file.Findings, nil)
		}
		for _, line := range exampleLines(examples) {
			file.Findings = append(file.Findings, examples[line].Findings...)
		}
		return file, findingTests(file.Findings, nil)
	}
	base := ValidityFindings(name, text)
	file := SourceValidityFile{Path: name, Uncertain: base.Uncertain, Reason: base.Reason, Findings: base.Findings}
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

func exampleLines(examples map[int]testvalidity.ExampleSource) []int {
	lines := make([]int, 0, len(examples))
	for line := range examples {
		lines = append(lines, line)
	}
	slices.Sort(lines)
	return lines
}
