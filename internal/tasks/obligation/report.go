// Package obligation reads a Playwright json-reporter document and computes
// which obligation-ledger ids it credits (TOL-V0-009..012). It is pure: the
// caller supplies the report bytes and the spec-file blobs at the declared
// commit, and nothing here runs Node, Playwright or Git. The report itself is
// never retained; only its digest and the derived credits leave this package.
package obligation

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// MaxReportBytes bounds one report (TOL-V0-009, owner bound 4).
const MaxReportBytes = 64 << 20

// MaxListIDs bounds each reported id list; only `unknown` can exceed the
// 256-entry ledger bound, and it is truncated with a flag.
const MaxListIDs = wire.ObligationsMaxEntries

// Unbound reasons (TOL-V0-012).
const (
	UnboundOutside = "OUTSIDE_REPOSITORY"
	UnboundAbsent  = "ABSENT_AT_COMMIT"
	UnboundNoID    = "ID_NOT_IN_SOURCE"
)

// qualifiedVersions is the TOL-V0-009 qualified Playwright version list. It
// stays empty until the live fixture in the spec's Acceptance evidence passes
// on a 1.63.x PWP-V0-008 tuple, so every report refuses
// OBLIGATION_REPORT_VERSION_UNQUALIFIED until then.
var qualifiedVersions = []string{}

// QualifiedVersions returns a copy of the qualified version list.
func QualifiedVersions() []string { return append([]string(nil), qualifiedVersions...) }

// Report is one admitted report: its digest, version and root directory and
// every retry-0 match the classifier reads.
type Report struct {
	Sha256  wire.Digest
	Version string
	// RootDir is config.rootDir; a caller may replace it with its resolved
	// form so it compares with a resolved repository root.
	RootDir string
	suites  []jsonSuite
}

type jsonReport struct {
	Config struct {
		Version string `json:"version"`
		RootDir string `json:"rootDir"`
	} `json:"config"`
	Suites []jsonSuite `json:"suites"`
}

type jsonSuite struct {
	Title  string      `json:"title"`
	Specs  []jsonSpec  `json:"specs"`
	Suites []jsonSuite `json:"suites"`
}

type jsonSpec struct {
	Title string     `json:"title"`
	ID    string     `json:"id"`
	File  string     `json:"file"`
	Tests []jsonTest `json:"tests"`
}

type jsonTest struct {
	ExpectedStatus string           `json:"expectedStatus"`
	ProjectName    string           `json:"projectName"`
	Annotations    []jsonAnnotation `json:"annotations"`
	Results        []jsonResult     `json:"results"`
}

type jsonResult struct {
	Status      string           `json:"status"`
	Retry       *int             `json:"retry"`
	Steps       []jsonStep       `json:"steps"`
	Annotations []jsonAnnotation `json:"annotations"`
	Error       *jsonError       `json:"error"`
}

// jsonAnnotation is a test annotation; test.fail(...) adds {type: "fail",
// description} at test level (and, in newer reporters, per result).
type jsonAnnotation struct {
	Type        string `json:"type"`
	Description string `json:"description"`
}

type jsonError struct {
	Message string `json:"message"`
}

type jsonStep struct {
	Title string          `json:"title"`
	Error json.RawMessage `json:"error"`
	Steps []jsonStep      `json:"steps"`
}

func reportErr(code, f string, args ...any) error {
	return wire.Errorf(code, "--from-playwright-report", ticket.ObligationReportDetail+" "+f, args...)
}

// ReadReport reads at most MaxReportBytes from file and admits it with
// ParseReport against the given qualified versions.
func ReadReport(file string, qualified []string) (*Report, error) {
	f, err := os.Open(file)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, reportErr(wire.CodeMissingEvidence, "the report file does not exist")
		}
		return nil, reportErr(wire.CodeMissingEvidence, "the report file is unreadable")
	}
	defer f.Close()
	raw, err := io.ReadAll(io.LimitReader(f, MaxReportBytes+1))
	if err != nil {
		return nil, reportErr(wire.CodeMissingEvidence, "the report file is unreadable")
	}
	return ParseReport(raw, qualified)
}

// ParseReport admits one json-reporter document: at most MaxReportBytes, one
// JSON value whose top level is exactly {config, errors, stats, suites}, and
// a config.version in qualified. Nothing of the report but the parsed match
// structure is kept.
func ParseReport(raw []byte, qualified []string) (*Report, error) {
	if len(raw) > MaxReportBytes {
		return nil, reportErr(wire.CodeMalformed, "the report exceeds %d bytes", MaxReportBytes)
	}
	var top map[string]json.RawMessage
	dec := json.NewDecoder(bytes.NewReader(raw))
	if err := dec.Decode(&top); err != nil || top == nil {
		return nil, reportErr(wire.CodeMalformed, "the report is not one JSON object")
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, reportErr(wire.CodeMalformed, "the report holds more than one JSON value")
	}
	want := []string{"config", "errors", "stats", "suites"}
	if len(top) != len(want) {
		return nil, reportErr(wire.CodeMalformed, "the report's top level must be exactly config, errors, stats and suites")
	}
	for _, k := range want {
		if _, ok := top[k]; !ok {
			return nil, reportErr(wire.CodeMalformed, "the report's top level must be exactly config, errors, stats and suites")
		}
	}
	var doc jsonReport
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, reportErr(wire.CodeMalformed, "the report does not have the json-reporter shape")
	}
	v := doc.Config.Version
	if v == "" {
		return nil, reportErr(wire.CodeMalformed, "config.version is missing")
	}
	if _, err := wire.ParseLabel("config.version", v); err != nil {
		return nil, reportErr(wire.CodeMalformed, "config.version is not a version label")
	}
	ok := false
	for _, q := range qualified {
		ok = ok || q == v
	}
	if !ok {
		return nil, wire.Errorf(wire.CodeUnsupportedVersion, "--from-playwright-report", "%s Playwright %s is not in the qualified version list %v", ticket.ObligationVersionDetail, v, qualified)
	}
	return &Report{Sha256: wire.Sum(raw), Version: v, RootDir: doc.Config.RootDir, suites: doc.Suites}, nil
}

// Match kinds (TOL-V0-022, TOL-V0-023). An ordinary match passes or fails;
// a match inside an expected-failure (test.fail) test is either a confirmed
// expected failure or a mixed one, and neither ever credits.
const (
	kindOrdinary = iota
	kindDefect
	kindMixed
)

// match is one occurrence of an id in a retry-0 result.
type match struct {
	file   string // spec.file, relative to config.rootDir
	m      ticket.ObligationMatch
	passed bool
	kind   int
	defect string // kindDefect: the defect id, "" when none is named
	errMsg string // kindDefect: the failure's first line, "" when none
}

// failAnnotation reports whether the test is an expected failure
// (test.fail) and the fail annotation's description.
func failAnnotation(t jsonTest, res jsonResult) (bool, string) {
	for _, list := range [][]jsonAnnotation{t.Annotations, res.Annotations} {
		for _, a := range list {
			if a.Type == "fail" {
				return true, a.Description
			}
		}
	}
	return t.ExpectedStatus == "failed", ""
}

// errorMessage is a step error's message, "" when it has none.
func errorMessage(raw json.RawMessage) string {
	var e jsonError
	if json.Unmarshal(raw, &e) != nil {
		return ""
	}
	return e.Message
}

// expectedFailMatch classifies one match inside an expected-failure test
// (TOL-V0-022). exp says the match is an expected-fail obligation, failed
// that its own step (or, for a title id, the result) failed, and msg is the
// failure message.
func expectedFailMatch(m match, exp, failed bool, msg, desc, title, prefix string) match {
	switch {
	case !exp:
		m.kind = kindMixed
	case failed:
		m.kind = kindDefect
		allowed := DefectIDs(desc, prefix)
		m.defect = firstOf(definedDefects(DefectIDs(msg, prefix), allowed), DefectIDs(title, prefix), allowed)
		m.errMsg = Excerpt(msg)
	}
	return m
}

// definedDefects keeps the ids of found that the fail description names,
// or all of them when it names none.
func definedDefects(found, allowed []string) []string {
	if len(allowed) == 0 {
		return found
	}
	var out []string
	for _, id := range found {
		for _, a := range allowed {
			if id == a {
				out = append(out, id)
			}
		}
	}
	return out
}

func firstOf(lists ...[]string) string {
	for _, l := range lists {
		if len(l) > 0 {
			return l[0]
		}
	}
	return ""
}

// collect returns every retry-0 match of an id carrying prefix, by id
// (TOL-V0-010). Results of tests whose expectedStatus is not `passed` and
// results with status timedOut, interrupted or skipped never pass. A match
// in an expected-failure test is classified by TOL-V0-022 and never passes.
func (r *Report) collect(prefix string) map[string][]match {
	out := map[string][]match{}
	var walk func(s jsonSuite, describes []string, top bool)
	walk = func(s jsonSuite, describes []string, top bool) {
		if !top {
			describes = append(append([]string{}, describes...), s.Title)
		}
		for _, sp := range s.Specs {
			titlePath := append(append([]string{}, describes...), sp.Title)
			for _, t := range sp.Tests {
				testID := sp.ID + "@" + t.ProjectName
				for _, res := range t.Results {
					if res.Retry == nil || *res.Retry != 0 {
						continue
					}
					eligible := t.ExpectedStatus == "passed"
					ran := res.Status != "timedOut" && res.Status != "interrupted" && res.Status != "skipped"
					xfail, desc := failAnnotation(t, res)
					resMsg := ""
					if res.Error != nil {
						resMsg = res.Error.Message
					}
					for _, title := range titlePath {
						for _, id := range FindIDs(title, prefix) {
							m := match{file: sp.File, passed: eligible && !xfail && res.Status == "passed",
								m: ticket.ObligationMatch{TestID: testID, TitlePath: titlePath, StepPath: []string{}}}
							if xfail {
								exp := SaysExpectedFail(title) || len(DefectIDs(desc, prefix)) > 0 || len(DefectIDs(title, prefix)) > 0
								m = expectedFailMatch(m, exp, res.Status == "failed", resMsg, desc, title, prefix)
							}
							out[id] = append(out[id], m)
						}
					}
					var steps func(list []jsonStep, parents []string)
					steps = func(list []jsonStep, parents []string) {
						for _, st := range list {
							stepPath := append(append([]string{}, parents...), st.Title)
							failed := len(st.Error) != 0 && string(st.Error) != "null"
							for _, id := range FindIDs(st.Title, prefix) {
								title := st.Title
								m := match{file: sp.File, passed: eligible && !xfail && ran && !failed,
									m: ticket.ObligationMatch{TestID: testID, TitlePath: titlePath, StepTitle: &title, StepPath: stepPath}}
								if xfail {
									msg := ""
									if failed {
										msg = errorMessage(st.Error)
									}
									named := definedDefects(DefectIDs(msg, prefix), DefectIDs(desc, prefix))
									exp := SaysExpectedFail(st.Title) || (failed && len(named) > 0)
									m = expectedFailMatch(m, exp, failed, msg, desc, st.Title, prefix)
								}
								out[id] = append(out[id], m)
							}
							steps(st.Steps, stepPath)
						}
					}
					steps(res.Steps, nil)
				}
			}
		}
		for _, c := range s.Suites {
			walk(c, describes, false)
		}
	}
	for _, s := range r.suites {
		walk(s, nil, true)
	}
	return out
}

// repoPath maps a spec file to its repository-relative path, or "" when it
// lies outside repoRoot (TOL-V0-012).
func (r *Report) repoPath(repoRoot, file string) string {
	if !filepath.IsAbs(r.RootDir) || file == "" {
		return ""
	}
	rel, err := filepath.Rel(repoRoot, filepath.Join(r.RootDir, filepath.FromSlash(file)))
	if err != nil {
		return ""
	}
	p := filepath.ToSlash(rel)
	if p == "." || p == ".." || strings.HasPrefix(p, "../") || path.IsAbs(p) {
		return ""
	}
	if _, err := wire.ParsePath("path", p); err != nil {
		return ""
	}
	return p
}

// SourcePaths returns the sorted repository paths of every passing match of
// a prefix id, which the caller reads at the declared commit.
func (r *Report) SourcePaths(prefix, repoRoot string) []string {
	seen := map[string]bool{}
	for _, ms := range r.collect(prefix) {
		for _, m := range ms {
			if p := r.repoPath(repoRoot, m.file); m.passed && p != "" {
				seen[p] = true
			}
		}
	}
	out := make([]string, 0, len(seen))
	for p := range seen {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

// Source is the declared commit's view of the spec files: Present holds every
// path that is a blob there, and Content its bytes when readable (a blob too
// large to read is present without content and cannot bind an id).
type Source struct {
	Present map[string]bool
	Content map[string][]byte
}

// UnboundID is one id reported unbound with its reason (TOL-V0-012).
type UnboundID struct {
	ID     string
	Reason string
}

// DefectConfirmed is one id an expected-failure test confirmed blocked by a
// known defect (TOL-V0-022): Defect is the named defect id, Error the
// failure's first line, each "" when absent.
type DefectConfirmed struct {
	ID, Defect, Error string
}

// MixedExpectedFail is one ordinary id named inside an expected-failure
// test, with the sorted test ids that mixed it (TOL-V0-023).
type MixedExpectedFail struct {
	ID    string
	Tests []string
}

// MixedRemedy is the TOL-V0-023 remedy for a mixed expected-failure test.
const MixedRemedy = "test.fail covers the whole test, so this obligation can never be credited there; move the expected-fail obligation into its own test"

// Result is one classification (TOL-V0-011, TOL-V0-013, TOL-V0-022,
// TOL-V0-023). Eligible holds every credited-or-creditable id with its
// passing, source-bound matches; Credited is the subset the ledger admits
// now. Every list is sorted.
type Result struct {
	DefectConfirmed   []DefectConfirmed
	MixedExpectedFail []MixedExpectedFail
	Eligible          map[string][]ticket.ObligationMatch
	Credited          []string
	AlreadyWitnessed  []string
	Conflicting       []string
	Failed            []string
	Unknown           []string
	UnknownTruncated  bool
	Unbound           []UnboundID
	Unmatched         []string
}

// Classify computes TOL-V0-010..012 for the ledger l from the report. ids,
// when non-nil, is the --ids subset that may be credited.
func Classify(r *Report, l *ticket.ObligationLedger, repoRoot string, src Source, ids []string) (*Result, error) {
	res := &Result{Eligible: map[string][]ticket.ObligationMatch{}}
	subset := map[string]bool{}
	for _, id := range ids {
		subset[id] = true
	}
	inSubset := func(id string) bool { return ids == nil || subset[id] }
	found := r.collect(l.Prefix)
	for id, ms := range found {
		e := l.Entry(id)
		if e == nil {
			res.Unknown = append(res.Unknown, id)
			continue
		}
		var mixed []string
		defects := 0
		for _, m := range ms {
			switch m.kind {
			case kindMixed:
				mixed = append(mixed, m.m.TestID)
			case kindDefect:
				defects++
			}
		}
		if len(mixed) > 0 {
			res.MixedExpectedFail = append(res.MixedExpectedFail, MixedExpectedFail{ID: id, Tests: sortedUnique(mixed)})
			continue
		}
		if defects == len(ms) {
			first := ms[0]
			for _, m := range ms {
				if m.defect != "" {
					first = m
					break
				}
			}
			res.DefectConfirmed = append(res.DefectConfirmed, DefectConfirmed{ID: id, Defect: first.defect, Error: first.errMsg})
			continue
		}
		pass, fail := false, false
		for _, m := range ms {
			pass, fail = pass || m.passed, fail || !m.passed
		}
		if defects > 0 {
			res.Conflicting = append(res.Conflicting, id)
			continue
		}
		if fail {
			if pass {
				res.Conflicting = append(res.Conflicting, id)
			} else {
				res.Failed = append(res.Failed, id)
			}
			continue
		}
		bound, reason := bindMatches(r, repoRoot, src, id, ms)
		if reason != "" {
			res.Unbound = append(res.Unbound, UnboundID{ID: id, Reason: reason})
			continue
		}
		switch e.State {
		case ticket.ObligationWitnessed:
			res.AlreadyWitnessed = append(res.AlreadyWitnessed, id)
			continue
		case ticket.ObligationDeferred:
			continue
		}
		// An id outside --ids is never credited, so its match count cannot
		// refuse a witness of the subset (TOL-V0-009).
		if !inSubset(id) {
			continue
		}
		if len(bound) > ticket.MaxObligationMatches {
			return nil, wire.Errorf(wire.CodeLimitExceeded, "/payload/credits", "%s %s has %d matches; an event holds at most %d per id", ticket.ObligationEventTooLargeDetail, id, len(bound), ticket.MaxObligationMatches)
		}
		res.Eligible[id] = bound
		res.Credited = append(res.Credited, id)
	}
	for _, e := range l.Entries {
		if _, ok := found[e.ID]; ok || !inSubset(e.ID) {
			continue
		}
		if e.State == ticket.ObligationOpen || e.State == ticket.ObligationDefect || e.State == ticket.ObligationBlocked {
			res.Unmatched = append(res.Unmatched, e.ID)
		}
	}
	for _, list := range [][]string{res.Credited, res.AlreadyWitnessed, res.Conflicting, res.Failed, res.Unknown, res.Unmatched} {
		sort.Strings(list)
	}
	sort.Slice(res.Unbound, func(i, j int) bool { return res.Unbound[i].ID < res.Unbound[j].ID })
	sort.Slice(res.DefectConfirmed, func(i, j int) bool { return res.DefectConfirmed[i].ID < res.DefectConfirmed[j].ID })
	sort.Slice(res.MixedExpectedFail, func(i, j int) bool { return res.MixedExpectedFail[i].ID < res.MixedExpectedFail[j].ID })
	if len(res.Unknown) > MaxListIDs {
		res.Unknown, res.UnknownTruncated = res.Unknown[:MaxListIDs], true
	}
	return res, nil
}

// bindMatches source-binds every match of id and returns the distinct
// matches in canonical order, or the first unbound reason.
func bindMatches(r *Report, repoRoot string, src Source, id string, ms []match) ([]ticket.ObligationMatch, string) {
	var out []ticket.ObligationMatch
	seen := map[string]bool{}
	for _, m := range ms {
		p := r.repoPath(repoRoot, m.file)
		switch {
		case p == "":
			return nil, UnboundOutside
		case !src.Present[p]:
			return nil, UnboundAbsent
		case !ContainsID(src.Content[p], id):
			return nil, UnboundNoID
		}
		bound := m.m
		bound.Path = p
		key := string(wire.EncodeFile(bound.Value()))
		if !seen[key] {
			seen[key] = true
			out = append(out, bound)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return string(wire.EncodeFile(out[i].Value())) < string(wire.EncodeFile(out[j].Value()))
	})
	return out, ""
}

func idByte(c byte) bool {
	return c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-'
}

// FindIDs returns the distinct valid ids of prefix that occur in s as whole
// tokens, bounded by a byte outside [A-Za-z0-9-] or the string edge
// (TOL-V0-010), in order of first occurrence.
func FindIDs(s, prefix string) []string {
	var out []string
	seen := map[string]bool{}
	lead := prefix + "-"
	for i := 0; i+len(lead) <= len(s); i++ {
		if s[i:i+len(lead)] != lead || (i > 0 && idByte(s[i-1])) {
			continue
		}
		j := i + len(lead)
		for j < len(s) && idByte(s[j]) {
			j++
		}
		tok := s[i:j]
		if _, err := wire.ParseObligationID("id", tok); err == nil && !seen[tok] {
			seen[tok] = true
			out = append(out, tok)
		}
	}
	return out
}

// ContainsID reports whether content holds id as a whole token (TOL-V0-012).
func ContainsID(content []byte, id string) bool {
	for i := 0; ; {
		k := bytes.Index(content[i:], []byte(id))
		if k < 0 {
			return false
		}
		at := i + k
		end := at + len(id)
		if (at == 0 || !idByte(content[at-1])) && (end == len(content) || !idByte(content[end])) {
			return true
		}
		i = at + 1
	}
}
