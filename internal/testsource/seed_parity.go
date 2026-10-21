package testsource

import (
	"errors"
	"fmt"
	"io/fs"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/testvalidity"
)

// The seed-parity reading (TSA-V0-100..109): for a new test linked to a legacy test by an explicit
// reference, extract the setup each side performs into a normalised shape (entity kinds, counts
// per kind, explicitly set attributes), compare the shapes, and report divergences and
// single-member aggregate claims. Every finding is advisory. Parity is reported only when both
// sides were analysed completely.

const (
	// maxSeedFixtures bounds the YAML fixture files one side reads (TSA-V0-101).
	maxSeedFixtures = 16
	// seedExpr is the normalised value of an attribute set to a non-literal expression.
	seedExpr = "<expr>"
)

// Seed-parity abstention reasons (TSA-V0-107).
const (
	seedReasonFileUnsupported     = "seed-file-unsupported"
	seedReasonSourceUncertain     = "seed-source-uncertain"
	seedReasonExampleNotFound     = "seed-example-not-found"
	seedReasonFormUnsupported     = "seed-form-unsupported"
	seedReasonScopeUnresolved     = "seed-scope-unresolved"
	seedReasonKindNotLiteral      = "seed-kind-not-literal"
	seedReasonCountNotLiteral     = "seed-count-not-literal"
	seedReasonAttributesNotRead   = "seed-attributes-not-literal"
	seedReasonFixtureUnavailable  = "seed-fixture-unavailable"
	seedReasonFixtureUnsupported  = "seed-fixture-unsupported"
	seedReasonFixtureBoundReached = "seed-fixture-bound-exceeded"
	seedReasonExecutionUnresolved = "seed-execution-unresolved"
	// seedReasonHelperUnresolved is a call to a first-party helper the reading cannot follow
	// into its factory calls (TSA-V0-644).
	seedReasonHelperUnresolved = "seed-helper-unresolved"
)

// SeedLink names a new test and the legacy test it claims to preserve, each `path:line` of the
// example's declaring line (TSA-V0-100).
type SeedLink struct {
	New    string
	Legacy string
}

// SeedParity is one link's comparison (TSA-V0-103).
type SeedParity struct {
	New      string                       `json:"new"`
	Legacy   string                       `json:"legacy"`
	Status   string                       `json:"status"`
	Sides    []SeedSide                   `json:"sides"`
	Findings []testvalidity.SourceFinding `json:"findings"`
}

// SeedSide is one side's normalised setup shape, or why it was not analysed (TSA-V0-102).
type SeedSide struct {
	Side     string `json:"side"`
	Test     string `json:"test"`
	Analysed bool   `json:"analysed"`
	Reason   string `json:"reason,omitempty"`
	Location string `json:"location,omitempty"`
	// Helper names the helper a seed-helper-unresolved side calls at Location (TSA-V0-644).
	Helper string     `json:"helper,omitempty"`
	Kinds  []SeedKind `json:"kinds"`
}

// SeedKind is one entity kind a side seeds: the total count and each explicitly set attribute
// with its distinct normalised values.
type SeedKind struct {
	Kind       string          `json:"kind"`
	Count      int             `json:"count"`
	Attributes []SeedAttribute `json:"attributes"`
}

// SeedAttribute is one explicitly set attribute and its sorted distinct values; a non-literal
// value is "<expr>".
type SeedAttribute struct {
	Name   string   `json:"name"`
	Values []string `json:"values"`
}

// seedCall is one recognised setup call or fixture file.
type seedCall struct {
	kind  string
	count int
	attrs map[string]string
	at    string   // path:line
	bound []string // the names bound to the records
}

// seedAssert is one asserting line and the snake_case identifiers it reads.
type seedAssert struct {
	at     string
	idents map[string]bool
}

// seedExtract is one side's raw extraction.
type seedExtract struct {
	test    string
	reason  string
	at      string
	helper  string
	calls   []seedCall
	asserts []seedAssert
}

func (x *seedExtract) abstain(reason, at string) {
	if x.reason == "" {
		x.reason, x.at = reason, at
	}
}

// abstainHelper abstains on a call to helper at the line at (TSA-V0-644).
func (x *seedExtract) abstainHelper(helper, at string) {
	if x.reason == "" {
		x.reason, x.at, x.helper = seedReasonHelperUnresolved, at, helper
	}
}

// fixtureReader reads one committed fixture file: its bytes, whether it exists, or an error.
type fixtureReader func(name string) ([]byte, bool, error)

// A fixture file that is committed but over the bound, or not a regular file, is never replaced by
// another directory's file: the side abstains (TSA-V0-107).
var (
	errSeedFixtureTooLarge   = errors.New("seed fixture over the bound")
	errSeedFixtureUnreadable = errors.New("seed fixture not a regular file")
)

// parseSeedRef splits a `path:line` reference (TSA-V0-100).
func parseSeedRef(ref string) (string, int, bool) {
	i := strings.LastIndexByte(ref, ':')
	if i <= 0 {
		return "", 0, false
	}
	name, digits := ref[:i], ref[i+1:]
	line, err := strconv.Atoi(digits)
	if err != nil || line <= 0 || strconv.Itoa(line) != digits {
		return "", 0, false
	}
	if !fs.ValidPath(name) || name == "." || name == ".git" || strings.HasPrefix(name, ".git/") || strings.ContainsAny(name, "\x00\n\r\\") {
		return "", 0, false
	}
	return name, line, true
}

// seedParity reads both sides of one link at the revision. A file that is not committed refuses
// tests-source-unavailable.
func (r Repo) seedParity(commit string, link SeedLink) (SeedParity, error) {
	sides := [2]seedExtract{}
	for k, ref := range []string{link.Legacy, link.New} {
		name, line, _ := parseSeedRef(ref)
		data, found, err := r.readOne(commit, name, MaxFileBytes)
		if err != nil {
			return SeedParity{}, err
		}
		if !found {
			return SeedParity{}, refuse("tests-source-unavailable", "%s is not a committed file at the revision", name)
		}
		read := func(fixture string) ([]byte, bool, error) {
			data, found, err := r.readOne(commit, fixture, MaxFileBytes)
			var refusal *Error
			switch {
			case errors.As(err, &refusal) && refusal.Code == "tests-bound-exceeded":
				return nil, false, errSeedFixtureTooLarge
			case errors.As(err, &refusal):
				return nil, false, errSeedFixtureUnreadable
			}
			return data, found, err
		}
		x, err := extractSeed(name, string(data), line, read)
		if err != nil {
			return SeedParity{}, err
		}
		sides[k] = x
	}
	return compareSeeds(link, sides[0], sides[1]), nil
}

// extractSeed routes one side to the RSpec or JS/TS extraction (TSA-V0-101).
func extractSeed(name, text string, line int, read fixtureReader) (seedExtract, error) {
	test := fmt.Sprintf("%s:%d", name, line)
	switch {
	case strings.HasSuffix(name, "_spec.rb"):
		return rspecSeed(name, text, line, read)
	case IsTestFile(name):
		return jsSeed(name, text, line), nil
	}
	x := seedExtract{test: test}
	x.abstain(seedReasonFileUnsupported, test)
	return x, nil
}

// shape normalises a side's calls into sorted kinds (TSA-V0-102).
func (x seedExtract) shape() []SeedKind {
	byKind := map[string]*SeedKind{}
	values := map[string]map[string]map[string]bool{}
	order := []string{}
	for _, c := range x.calls {
		k, ok := byKind[c.kind]
		if !ok {
			k = &SeedKind{Kind: c.kind, Attributes: []SeedAttribute{}}
			byKind[c.kind], values[c.kind] = k, map[string]map[string]bool{}
			order = append(order, c.kind)
		}
		k.Count += c.count
		for name, value := range c.attrs {
			if values[c.kind][name] == nil {
				values[c.kind][name] = map[string]bool{}
			}
			values[c.kind][name][value] = true
		}
	}
	slices.Sort(order)
	out := make([]SeedKind, 0, len(order))
	for _, kind := range order {
		k := byKind[kind]
		names := make([]string, 0, len(values[kind]))
		for name := range values[kind] {
			names = append(names, name)
		}
		slices.Sort(names)
		for _, name := range names {
			vs := make([]string, 0, len(values[kind][name]))
			for v := range values[kind][name] {
				vs = append(vs, v)
			}
			slices.Sort(vs)
			k.Attributes = append(k.Attributes, SeedAttribute{Name: name, Values: vs})
		}
		out = append(out, *k)
	}
	return out
}

// firstSeed is the location of the side's first call of kind, or "".
func (x seedExtract) firstSeed(kind string) string {
	for _, c := range x.calls {
		if c.kind == kind {
			return c.at
		}
	}
	return ""
}

// readsKind reports an assertion that reads kind: an identifier that is the kind, its plural
// or a name bound to its records (TSA-V0-104).
func (x seedExtract) readsKind(a seedAssert, kind string) bool {
	for ident := range a.idents {
		if ident == kind || seedSingular(ident) == kind {
			return true
		}
	}
	for _, c := range x.calls {
		if c.kind != kind {
			continue
		}
		for _, name := range c.bound {
			if a.idents[name] {
				return true
			}
		}
	}
	return false
}

// readsGroup reports an assertion that reads a group of kind: a plural identifier of the kind or
// a plural name bound to its records (TSA-V0-106).
func (x seedExtract) readsGroup(a seedAssert, kind string) bool {
	for ident := range a.idents {
		if ident != kind && seedSingular(ident) == kind {
			return true
		}
	}
	for _, c := range x.calls {
		for _, name := range c.bound {
			if c.kind == kind && a.idents[name] && seedSingular(name) != name {
				return true
			}
		}
	}
	return false
}

// readers lists the side's asserting lines that read kind.
func (x seedExtract) readers(kind string) []string {
	out := []string{}
	for _, a := range x.asserts {
		if x.readsKind(a, kind) {
			out = append(out, a.at)
		}
	}
	return out
}

// seedDiff is one divergence before it becomes a finding.
type seedDiff struct {
	kind, attr string
	read       bool
	// uncertain marks an attribute compared against an unknown value (TSA-V0-105).
	uncertain bool
	message   string
}

// compareSeeds compares the two sides (TSA-V0-103..107).
func compareSeeds(link SeedLink, legacy, current seedExtract) SeedParity {
	out := SeedParity{New: link.New, Legacy: link.Legacy, Findings: []testvalidity.SourceFinding{}}
	pairs := []struct {
		name string
		x    seedExtract
	}{{"legacy", legacy}, {"new", current}}
	for _, p := range pairs {
		side := SeedSide{Side: p.name, Test: p.x.test, Analysed: p.x.reason == "", Reason: p.x.reason, Location: p.x.at, Helper: p.x.helper, Kinds: []SeedKind{}}
		if side.Analysed {
			side.Kinds = p.x.shape()
		}
		out.Sides = append(out.Sides, side)
	}
	if legacy.reason != "" || current.reason != "" {
		out.Status = testvalidity.SeedParityStatusNotAnalysed
		for k, p := range pairs {
			if p.x.reason == "" {
				continue
			}
			other := pairs[1-k]
			reason := p.x.reason
			if p.x.helper != "" {
				reason += " calling " + p.x.helper
			}
			out.Findings = append(out.Findings, testvalidity.SourceFinding{
				Rule: testvalidity.RuleSeedParityNotAnalysed, Severity: testvalidity.SeverityAdvisory,
				Test: link.New, Location: p.x.at, Cause: other.x.test,
				Message: fmt.Sprintf("the %s side's setup was not analysed (%s at %s), so seed parity with %s is not reported", p.name, reason, p.x.at, other.x.test),
			})
		}
		return out
	}
	diffs := seedDiffs(legacy, current, out.Sides[0].Kinds, out.Sides[1].Kinds)
	diverges, uncertain := false, false
	for _, d := range diffs {
		location, cause := current.firstSeed(d.kind), legacy.firstSeed(d.kind)
		if location == "" {
			location = current.test
		}
		if cause == "" {
			cause = legacy.test
		}
		rule := testvalidity.RuleSeedParityDiverges
		if d.uncertain {
			rule, uncertain = testvalidity.RuleSeedParityUncertain, true
		} else {
			diverges = true
		}
		out.Findings = append(out.Findings, testvalidity.SourceFinding{
			Rule: rule, Severity: testvalidity.SeverityAdvisory,
			Test: link.New, Location: location, Cause: cause, Message: d.message,
		})
	}
	switch {
	case diverges:
		out.Status = testvalidity.SeedParityStatusDiverges
	case uncertain:
		out.Status = testvalidity.SeedParityStatusUncertain
	default:
		out.Status = testvalidity.SeedParityStatusParity
	}
	out.Findings = append(out.Findings, aggregateFindings(link, legacy, current)...)
	return out
}

// seedDiffs lists the kind, count and attribute differences, those an asserting line reads
// first, then by kind and attribute (TSA-V0-104, TSA-V0-105).
func seedDiffs(legacy, current seedExtract, legacyKinds, currentKinds []SeedKind) []seedDiff {
	index := func(kinds []SeedKind) map[string]SeedKind {
		out := map[string]SeedKind{}
		for _, k := range kinds {
			out[k.Kind] = k
		}
		return out
	}
	l, c := index(legacyKinds), index(currentKinds)
	kinds := []string{}
	for kind := range l {
		kinds = append(kinds, kind)
	}
	for kind := range c {
		if _, ok := l[kind]; !ok {
			kinds = append(kinds, kind)
		}
	}
	slices.Sort(kinds)
	diffs := []seedDiff{}
	for _, kind := range kinds {
		lr, cr := legacy.readers(kind), current.readers(kind)
		reading := fmt.Sprintf("asserting lines reading %s: legacy %s; new %s", kind, listOrNone(lr), listOrNone(cr))
		if l[kind].Count != c[kind].Count {
			diffs = append(diffs, seedDiff{kind: kind, read: len(lr)+len(cr) > 0,
				message: fmt.Sprintf("legacy seeds %d %s, new seeds %d; %s", l[kind].Count, kind, c[kind].Count, reading)})
		}
		if l[kind].Count == 0 || c[kind].Count == 0 {
			continue
		}
		la, ca := attrIndex(l[kind]), attrIndex(c[kind])
		names := []string{}
		for name := range la {
			names = append(names, name)
		}
		for name := range ca {
			if _, ok := la[name]; !ok {
				names = append(names, name)
			}
		}
		slices.Sort(names)
		for _, name := range names {
			lv, lok := la[name]
			cv, cok := ca[name]
			message := ""
			switch {
			case lok && !cok:
				message = fmt.Sprintf("%s.%s is set explicitly on the legacy side (%s) and left at its default on the new side", kind, name, strings.Join(lv, ", "))
			case cok && !lok:
				message = fmt.Sprintf("%s.%s is set explicitly on the new side (%s) and left at its default on the legacy side", kind, name, strings.Join(cv, ", "))
			case literalOnly(lv) && literalOnly(cv) && !slices.Equal(lv, cv):
				message = fmt.Sprintf("%s.%s differs: legacy %s, new %s", kind, name, strings.Join(lv, ", "), strings.Join(cv, ", "))
			case !slices.Equal(lv, cv):
				// An unknown value against a different value set is neither a divergence nor
				// parity (TSA-V0-105).
				diffs = append(diffs, seedDiff{kind: kind, attr: name, uncertain: true, read: legacy.readsAttr(name) || current.readsAttr(name),
					message: fmt.Sprintf("%s.%s cannot be compared: legacy %s, new %s, and %s is a value the reading does not know; %s", kind, name, strings.Join(lv, ", "), strings.Join(cv, ", "), seedExpr, reading)})
				continue
			default:
				continue
			}
			diffs = append(diffs, seedDiff{kind: kind, attr: name, read: legacy.readsAttr(name) || current.readsAttr(name),
				message: message + "; " + reading})
		}
	}
	slices.SortStableFunc(diffs, func(a, b seedDiff) int {
		if a.read != b.read {
			if a.read {
				return -1
			}
			return 1
		}
		if a.kind != b.kind {
			return strings.Compare(a.kind, b.kind)
		}
		return strings.Compare(a.attr, b.attr)
	})
	return diffs
}

// readsAttr reports an asserting line that names the attribute.
func (x seedExtract) readsAttr(name string) bool {
	for _, a := range x.asserts {
		if a.idents[name] {
			return true
		}
	}
	return false
}

func attrIndex(k SeedKind) map[string][]string {
	out := map[string][]string{}
	for _, a := range k.Attributes {
		out[a.Name] = a.Values
	}
	return out
}

func literalOnly(values []string) bool { return !slices.Contains(values, seedExpr) }

func listOrNone(locations []string) string {
	if len(locations) == 0 {
		return "none"
	}
	return strings.Join(locations, ", ")
}

// aggregateWords are the identifier words that make an assertion a group-aggregate claim
// (TSA-V0-106).
var aggregateWords = map[string]bool{"total": true, "sum": true, "all": true, "every": true, "reduce": true,
	"inject": true, "average": true, "balance": true, "aggregate": true}

func (a seedAssert) aggregate() bool {
	for ident := range a.idents {
		for _, word := range strings.Split(ident, "_") {
			if aggregateWords[word] {
				return true
			}
		}
	}
	return false
}

// aggregateFindings reports, on each side, every group-aggregate assertion that reads a group of a
// kind either side seeds with exactly one member (TSA-V0-106).
func aggregateFindings(link SeedLink, legacy, current seedExtract) []testvalidity.SourceFinding {
	out := []testvalidity.SourceFinding{}
	counts := [2]map[string]int{{}, {}}
	for k, x := range []seedExtract{legacy, current} {
		for _, c := range x.calls {
			counts[k][c.kind] += c.count
		}
	}
	kinds := []string{}
	for kind := range counts[0] {
		kinds = append(kinds, kind)
	}
	for kind := range counts[1] {
		if _, ok := counts[0][kind]; !ok {
			kinds = append(kinds, kind)
		}
	}
	slices.Sort(kinds)
	for k, x := range []seedExtract{legacy, current} {
		name := [2]string{"legacy", "new"}[k]
		for _, a := range x.asserts {
			if !a.aggregate() {
				continue
			}
			for _, kind := range kinds {
				single := []string{}
				cause := ""
				for j, side := range []seedExtract{legacy, current} {
					if counts[j][kind] == 1 {
						single = append(single, [2]string{"legacy", "new"}[j])
						if cause == "" {
							cause = side.firstSeed(kind)
						}
					}
				}
				if len(single) == 0 || !x.readsGroup(a, kind) {
					continue
				}
				out = append(out, testvalidity.SourceFinding{
					Rule: testvalidity.RuleSeedAggregateSingleMember, Severity: testvalidity.SeverityAdvisory,
					Test: link.New, Location: a.at, Cause: cause,
					Message: fmt.Sprintf("the %s side asserts a group aggregate over %s at %s, but the %s side seeds only one %s, so the aggregate is not proven across a group", name, kind, a.at, strings.Join(single, " and "), kind),
				})
			}
		}
	}
	return out
}

var (
	camelBoundary = regexp.MustCompile(`([a-z0-9])([A-Z])`)
	acronymEnd    = regexp.MustCompile(`([A-Z]+)([A-Z][a-z])`)
)

// snake normalises an identifier, kind or attribute name to lower snake_case.
func snake(name string) string {
	name = strings.TrimRight(name, "?!")
	name = acronymEnd.ReplaceAllString(name, "${1}_${2}")
	name = camelBoundary.ReplaceAllString(name, "${1}_${2}")
	return strings.ToLower(strings.ReplaceAll(name, "-", "_"))
}

// seedSingular strips one English plural suffix from a snake_case name: ies to y, a sibilant es,
// or a final s that does not follow s.
func seedSingular(name string) string {
	switch {
	case strings.HasSuffix(name, "ies") && len(name) > 3:
		return strings.TrimSuffix(name, "ies") + "y"
	case strings.HasSuffix(name, "sses"), strings.HasSuffix(name, "xes"), strings.HasSuffix(name, "ches"), strings.HasSuffix(name, "shes"):
		return strings.TrimSuffix(name, "es")
	case strings.HasSuffix(name, "s") && !strings.HasSuffix(name, "ss") && len(name) > 1:
		return strings.TrimSuffix(name, "s")
	}
	return name
}

// seedWords are the identifier words that make a call look like setup (TSA-V0-101).
var seedWords = map[string]bool{"create": true, "build": true, "insert": true, "upsert": true, "seed": true,
	"fixture": true, "fixtures": true, "factory": true, "populate": true}

func seedLike(name string) bool {
	for _, word := range strings.Split(snake(name), "_") {
		if seedWords[word] {
			return true
		}
	}
	return false
}
