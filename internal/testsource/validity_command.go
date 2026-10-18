package testsource

import (
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

// SourceValidityOptions selects the revision ("" reads HEAD) and the repository-relative test files
// to read.
type SourceValidityOptions struct {
	Revision string
	Files    []string
}

// SourceValidityReport is the corvint-test-source-validity/0 report.
type SourceValidityReport struct {
	Schema   string               `json:"schema"`
	Revision string               `json:"revision"`
	Files    []SourceValidityFile `json:"files"`
	Tests    []SourceValidityTest `json:"tests"`
}

// SourceValidityFile is one file's findings and whether the reader abstained on part of it.
type SourceValidityFile struct {
	Path      string                       `json:"path"`
	Uncertain bool                         `json:"uncertain"`
	Reason    string                       `json:"reason,omitempty"`
	Findings  []testvalidity.SourceFinding `json:"findings"`
}

// SourceValidityTest is the hygiene axis a test's findings give it: ELIGIBLE with only advisory
// findings, INELIGIBLE with any invalid one (TSA-V0-051).
type SourceValidityTest struct {
	Test    string            `json:"test"`
	Hygiene testvalidity.Axis `json:"hygiene"`
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
// for a `_spec.rb` file, the RSpec example rules (TSA-V0-062..066). It reads committed Git content
// only. A path that is not a valid relative path refuses tests-invalid-arguments; a path that is
// not a committed regular file refuses tests-source-unavailable; more than MaxValidityFiles files
// or a file over MaxFileBytes refuses tests-bound-exceeded.
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
	if len(files) == 0 {
		return SourceValidityReport{}, refuse("tests-invalid-arguments", "no test file was named")
	}
	if len(files) > MaxValidityFiles {
		return SourceValidityReport{}, refuse("tests-bound-exceeded", "more than %d files", MaxValidityFiles)
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
	byTest := map[string][]testvalidity.SourceFinding{}
	order := []string{}
	for _, name := range files {
		data, found, err := r.readOne(commit, name, MaxFileBytes)
		if err != nil {
			return SourceValidityReport{}, err
		}
		if !found {
			return SourceValidityReport{}, refuse("tests-source-unavailable", "%s is not a committed file at the revision", name)
		}
		file := readSourceValidity(name, string(data))
		for _, f := range file.Findings {
			if _, ok := byTest[f.Test]; !ok {
				order = append(order, f.Test)
			}
			byTest[f.Test] = append(byTest[f.Test], f)
		}
		report.Files = append(report.Files, file)
	}
	for _, test := range order {
		baseline := testvalidity.Projection{Hygiene: testvalidity.Axis{State: testvalidity.HygieneEligible, Reason: hygieneReasonSourceRead, Anchors: []string{test}}}
		report.Tests = append(report.Tests, SourceValidityTest{Test: test, Hygiene: testvalidity.ApplySourceFindings(baseline, byTest[test]).Hygiene})
	}
	return report, nil
}

// readSourceValidity applies the rules to one file's text. A `_spec.rb` file's findings are every
// example's findings in source order.
func readSourceValidity(name, text string) SourceValidityFile {
	if !strings.HasSuffix(name, "_spec.rb") {
		base := ValidityFindings(name, text)
		return SourceValidityFile{Path: name, Uncertain: base.Uncertain, Reason: base.Reason, Findings: base.Findings}
	}
	file := SourceValidityFile{Path: name, Findings: []testvalidity.SourceFinding{}}
	examples, reason := RSpecExampleSources(name, text)
	if reason != "" {
		file.Uncertain, file.Reason = true, reason
		return file
	}
	lines := make([]int, 0, len(examples))
	for line := range examples {
		lines = append(lines, line)
	}
	slices.Sort(lines)
	for _, line := range lines {
		file.Findings = append(file.Findings, examples[line].Findings...)
	}
	return file
}
