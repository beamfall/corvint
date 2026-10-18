package testsource

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
)

const (
	// DuplicatesSchema names the `corvint tests duplicates` report (TSA-V0-020).
	DuplicatesSchema = "corvint-tests-duplicates/0"
	// DecisionsSchema names the project-owned decision file (TSA-V0-022).
	DecisionsSchema = "corvint-test-duplicate-decisions/0"
	// DecisionsPath is the committed decision file, read at the analysed revision.
	DecisionsPath = ".corvint/test-duplicates.json"
	// CandidateNote is stated in every report (TSA-V0-023).
	CandidateNote    = "pairs are candidates for review, not verdicts: shared assertion lines suggest, but do not prove, that two tests check the same behaviour"
	maxDecisionBytes = 1 << 20
	maxDecisions     = 10000
	maxTests         = 50000
)

// maxPairs bounds each of the unresolved and resolved lists; a variable so a test can lower it.
var maxPairs = 1000

// DuplicatesOptions select the revision, the gate and the thresholds (TSA-V0-020, TSA-V0-021).
type DuplicatesOptions struct {
	Revision string
	// Base and Gate select gate mode: only pairs with a test added or changed since Base count.
	Base, Gate string
	// Threshold is the minimum Jaccard score; MaxLineTests excludes an assertion line that appears
	// in more tests than this.
	Threshold    float64
	MaxLineTests int
}

// Pair is one candidate duplicate.
type Pair struct {
	Test        string   `json:"test"`
	Other       string   `json:"other"`
	Score       float64  `json:"score"`
	SharedLines []string `json:"sharedLines"`
	Decision    string   `json:"decision,omitempty"`
	Reason      string   `json:"reason,omitempty"`
	Obligation  string   `json:"obligation,omitempty"`
}

// Decision is one recorded review of a pair (TSA-V0-022).
type Decision struct {
	Decision   string `json:"decision"`
	Test       string `json:"test"`
	Other      string `json:"other"`
	Reason     string `json:"reason,omitempty"`
	Obligation string `json:"obligation,omitempty"`
}

// IgnoredDecision is a decision that resolves nothing, with why.
type IgnoredDecision struct {
	Decision
	Problem string `json:"problem"`
}

// DuplicatesReport is the deterministic `corvint tests duplicates` result. UnresolvedCount and
// ResolvedCount count every pair, including any beyond the output bound of the two lists.
type DuplicatesReport struct {
	Schema           string            `json:"schema"`
	Note             string            `json:"note"`
	Revision         string            `json:"revision"`
	Base             string            `json:"base,omitempty"`
	Gate             string            `json:"gate,omitempty"`
	Threshold        float64           `json:"threshold"`
	MaxLineTests     int               `json:"maxLineTests"`
	Tests            int               `json:"tests"`
	ChangedTests     []string          `json:"changedTests,omitempty"`
	ExcludedLines    int               `json:"excludedLines"`
	UnresolvedCount  int               `json:"unresolvedCount"`
	ResolvedCount    int               `json:"resolvedCount"`
	Unresolved       []Pair            `json:"unresolved"`
	Resolved         []Pair            `json:"resolved"`
	IgnoredDecisions []IgnoredDecision `json:"ignoredDecisions"`
	Truncated        bool              `json:"truncated"`
	Uncertain        []string          `json:"uncertain"`
}

// testCase is one declared test and its normalised assertion lines.
type testCase struct {
	id         string
	path       string
	line, end  int
	assertions []string
}

// extractTests lists the non-skipped tests of a source with their normalised `expect` chains
// (TSA-V0-020). A test whose id repeats in its file gets a ` [n]` suffix.
func extractTests(s *source) []testCase {
	out, seen := []testCase{}, map[string]int{}
	for _, d := range s.decls {
		if d.kind != "test" || d.modifiers["skip"] || d.modifiers["fixme"] || !s.callback(d.body) {
			continue
		}
		titles := append(s.describes(d), d.title)
		id := s.path + " :: " + strings.Join(titles, " > ")
		if seen[id]++; seen[id] > 1 {
			id += fmt.Sprintf(" [%d]", seen[id])
		}
		lines, unique := []string{}, map[string]bool{}
		for i := d.body[0]; i < d.body[1]; i++ {
			chain, ok := s.readExpect(i)
			if !ok {
				continue
			}
			line := s.render(i, chain.end+1)
			if !unique[line] {
				unique[line] = true
				lines = append(lines, line)
			}
			i = chain.end
		}
		sort.Strings(lines)
		out = append(out, testCase{id: id, path: s.path, line: d.line, end: d.endLine, assertions: lines})
	}
	return out
}

// Duplicates reports candidate duplicate test pairs at a revision (TSA-V0-020..023).
func Duplicates(r Repo, options DuplicatesOptions) (DuplicatesReport, error) {
	if options.Threshold <= 0 || options.Threshold > 1 || options.MaxLineTests < 2 || options.MaxLineTests > 100 {
		return DuplicatesReport{}, refuse("tests-invalid-option", "threshold must be in (0,1] and the line cutoff in [2,100]")
	}
	if (options.Gate == "") != (options.Base == "") {
		return DuplicatesReport{}, refuse("tests-invalid-option", "gate mode needs both --gate and --base")
	}
	revision, err := r.Resolve(options.Revision)
	if err != nil {
		return DuplicatesReport{}, err
	}
	report := DuplicatesReport{Schema: DuplicatesSchema, Note: CandidateNote, Revision: revision, Gate: options.Gate,
		Threshold: options.Threshold, MaxLineTests: options.MaxLineTests, Unresolved: []Pair{}, Resolved: []Pair{},
		IgnoredDecisions: []IgnoredDecision{}, Uncertain: []string{}}
	var changed map[string]*lineChanges
	if options.Gate != "" {
		if report.Base, err = r.Resolve(options.Base); err != nil {
			return DuplicatesReport{}, err
		}
		if changed, err = r.changedLines(report.Base, revision, IsTestFile); err != nil {
			return DuplicatesReport{}, err
		}
	}
	texts, _, err := r.readFiles(revision, IsTestFile)
	if err != nil {
		return DuplicatesReport{}, err
	}
	decisions, err := r.readDecisions(revision)
	if err != nil {
		return DuplicatesReport{}, err
	}
	paths := make([]string, 0, len(texts))
	for name := range texts {
		paths = append(paths, name)
	}
	sort.Strings(paths)
	tests := []testCase{}
	for _, name := range paths {
		s := parseSource(name, texts[name])
		if s.uncertain {
			report.Uncertain = append(report.Uncertain, name)
		}
		tests = append(tests, extractTests(s)...)
		if len(tests) > maxTests {
			return DuplicatesReport{}, refuse("tests-bound-exceeded", "more than %d tests", maxTests)
		}
	}
	report.Tests = len(tests)
	var focus map[int]bool
	if changed != nil {
		focus = map[int]bool{}
		for i, tc := range tests {
			if changed[tc.path].touches(tc.line, tc.end) {
				focus[i] = true
				report.ChangedTests = append(report.ChangedTests, tc.id)
			}
		}
		if report.ChangedTests == nil {
			report.ChangedTests = []string{}
		}
	}
	pairs, excluded := scorePairs(tests, focus, options)
	report.ExcludedLines = excluded
	resolveDecisions(&report, pairs, decisions)
	// Counts and the gate are taken before the lists are truncated, so a pair beyond the output
	// bound still fails the gate (TSA-V0-020, TSA-V0-021).
	report.UnresolvedCount, report.ResolvedCount = len(report.Unresolved), len(report.Resolved)
	if len(report.Unresolved) > maxPairs {
		report.Unresolved, report.Truncated = report.Unresolved[:maxPairs], true
	}
	if len(report.Resolved) > maxPairs {
		report.Resolved, report.Truncated = report.Resolved[:maxPairs], true
	}
	return report, nil
}

// scorePairs scores every pair of tests in different files that share at least two assertion
// lines that are not frequent; with a focus set, only pairs touching it. Every pair is returned;
// the caller bounds the output after resolving decisions.
func scorePairs(tests []testCase, focus map[int]bool, options DuplicatesOptions) ([]Pair, int) {
	holders := map[string][]int{}
	for i, tc := range tests {
		for _, line := range tc.assertions {
			holders[line] = append(holders[line], i)
		}
	}
	excluded, kept := 0, map[string]bool{}
	for line, ids := range holders {
		if len(ids) > options.MaxLineTests {
			excluded++
		} else {
			kept[line] = true
		}
	}
	size := func(i int) int {
		n := 0
		for _, line := range tests[i].assertions {
			if kept[line] {
				n++
			}
		}
		return n
	}
	shared := map[[2]int][]string{}
	for line := range kept {
		ids := holders[line]
		for a := 0; a < len(ids); a++ {
			for b := a + 1; b < len(ids); b++ {
				x, y := ids[a], ids[b]
				if tests[x].path == tests[y].path || (focus != nil && !focus[x] && !focus[y]) {
					continue
				}
				shared[[2]int{x, y}] = append(shared[[2]int{x, y}], line)
			}
		}
	}
	pairs := []Pair{}
	for key, lines := range shared {
		if len(lines) < 2 {
			continue
		}
		union := size(key[0]) + size(key[1]) - len(lines)
		score := math.Round(float64(len(lines))/float64(union)*10000) / 10000
		if score < options.Threshold {
			continue
		}
		sort.Strings(lines)
		first, second := tests[key[0]].id, tests[key[1]].id
		if second < first {
			first, second = second, first
		}
		pairs = append(pairs, Pair{Test: first, Other: second, Score: score, SharedLines: lines})
	}
	sort.Slice(pairs, func(a, b int) bool {
		if pairs[a].Score != pairs[b].Score {
			return pairs[a].Score > pairs[b].Score
		}
		if pairs[a].Test != pairs[b].Test {
			return pairs[a].Test < pairs[b].Test
		}
		return pairs[a].Other < pairs[b].Other
	})
	return pairs, excluded
}

// resolveDecisions files every pair as resolved or unresolved (TSA-V0-022). A distinct-from
// decision resolves its pair only with a reason; a covered-by decision only with an obligation.
func resolveDecisions(report *DuplicatesReport, pairs []Pair, decisions []Decision) {
	type key struct{ a, b string }
	valid := map[key]Decision{}
	for _, d := range decisions {
		problem := ""
		switch {
		case d.Decision != "distinct-from" && d.Decision != "covered-by":
			problem = "unknown-decision"
		case d.Test == "" || d.Other == "" || d.Test == d.Other:
			problem = "invalid-pair"
		case d.Decision == "distinct-from" && strings.TrimSpace(d.Reason) == "":
			problem = "missing-reason"
		case d.Decision == "covered-by" && strings.TrimSpace(d.Obligation) == "":
			problem = "missing-obligation"
		}
		if problem != "" {
			report.IgnoredDecisions = append(report.IgnoredDecisions, IgnoredDecision{Decision: d, Problem: problem})
			continue
		}
		a, b := d.Test, d.Other
		if b < a {
			a, b = b, a
		}
		if _, ok := valid[key{a, b}]; !ok {
			valid[key{a, b}] = d
		}
	}
	for _, p := range pairs {
		if d, ok := valid[key{p.Test, p.Other}]; ok {
			p.Decision, p.Reason, p.Obligation = d.Decision, d.Reason, d.Obligation
			report.Resolved = append(report.Resolved, p)
		} else {
			report.Unresolved = append(report.Unresolved, p)
		}
	}
}

// readDecisions reads the committed decision file at revision; an absent file is no decisions and
// a malformed one refuses (TSA-V0-022).
func (r Repo) readDecisions(revision string) ([]Decision, error) {
	data, ok, err := r.readOne(revision, DecisionsPath, maxDecisionBytes)
	if err != nil {
		return nil, err
	}
	if !ok {
		return []Decision{}, nil
	}
	return ParseDecisions(data)
}

// ParseDecisions strictly decodes a decision file.
func ParseDecisions(data []byte) ([]Decision, error) {
	var file struct {
		Schema    string     `json:"schema"`
		Decisions []Decision `json:"decisions"`
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&file); err != nil || decoder.More() {
		return nil, refuse("tests-decisions-invalid", "%s is not a valid decision file", DecisionsPath)
	}
	if file.Schema != DecisionsSchema {
		return nil, refuse("tests-decisions-invalid", "%s must declare schema %s", DecisionsPath, DecisionsSchema)
	}
	if len(file.Decisions) > maxDecisions {
		return nil, refuse("tests-bound-exceeded", "more than %d decisions", maxDecisions)
	}
	if file.Decisions == nil {
		file.Decisions = []Decision{}
	}
	return file.Decisions, nil
}
