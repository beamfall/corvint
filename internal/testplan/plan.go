package testplan

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/testvalidity"
	"github.com/Beamfall/corvint/internal/testvaliditydoc"
)

const (
	// PlanSchema names the JSON plan (TCN-V0-009).
	PlanSchema = "test-consolidation-plan/0"
	// DefaultMaxSteps and the 2..32 range bound steps per test (TCN-V0-008).
	DefaultMaxSteps = 8
	MinMaxSteps     = 2
	MaxMaxSteps     = 32
	// none stands for an absent digest or revision.
	none = "none"
)

// Abstention, isolation and witness reason codes (TCN-V0-003, TCN-V0-004, TCN-V0-006).
const (
	ReasonMissingAnchor    = "missing-anchor"
	ReasonUnresolvedScreen = "unresolved-screen"
	ReasonRouteMismatch    = "route-mismatch"
	ReasonStaleAnchor      = "stale-anchor"
	ReasonAnchorUnknown    = "anchor-unknown"
	ReasonDestructive      = "destructive-change"
	ReasonConflictingState = "conflicting-state"
	ReasonDifferentUser    = "different-user-or-org"
	ReasonUniqueContext    = "unique-context"
	ReasonGrouped          = "-"
	WitnessNotFound        = "witness-not-found"
	WitnessNotPassed       = "witness-not-passed"
	WitnessStale           = "witness-stale"
	WitnessUnbound         = "witness-unbound"
	WitnessNotAssociated   = "witness-not-associated"
	WitnessIneligible      = "witness-ineligible"
	WitnessConflicting     = "witness-conflicting"
	WitnessPreview         = "witness-preview"
	anchorsValidated       = "VALIDATED"
	anchorsNotRun          = "NOT_RUN"
	statusComplete         = "COMPLETE"
	statusIncomplete       = "INCOMPLETE"
	jsProviderSource       = "corvint-js-test-provider"
	jsEndToEndKind         = "e2e"
)

// MapScreen is one AMAP-V0 screen as the planner validates it: resolved and concrete, its route
// template, and its lineage freshness at the evaluated revision (FRESH, STALE or UNKNOWN).
type MapScreen struct {
	Resolved bool
	Template string
	Lineage  string
}

// MapView is one map's validation evidence (TCN-V0-003).
type MapView struct {
	App     string
	Digest  string
	Screens map[string]MapScreen
}

// Evidence is everything the plan is computed from besides the input. Documents are already
// projected with freshness recomputed at the evaluated revision, in file-digest order.
type Evidence struct {
	Revision    string // the evaluated full object ID, or "none"
	TestsDigest string // "sha256:..." or "none"
	Documents   []testvaliditydoc.Document
	Maps        []MapView // nil: anchors NOT_RUN
}

// Plan is one closed test-consolidation-plan/0 document (TCN-V0-009) in member order.
type Plan struct {
	Schema            string       `json:"schema"`
	InputDigest       string       `json:"input_digest"`
	TestsDigest       string       `json:"tests_digest"`
	MapsDigest        string       `json:"maps_digest"`
	Maps              []PlanMap    `json:"maps"`
	EvaluatedRevision string       `json:"evaluated_revision"`
	AnchorValidation  string       `json:"anchor_validation"`
	Authority         string       `json:"authority"`
	Status            string       `json:"status"`
	MaxSteps          int          `json:"max_steps"`
	Counts            Counts       `json:"counts"`
	Tests             []Test       `json:"tests"`
	Reused            []Reuse      `json:"reused"`
	Duplicates        []Duplicate  `json:"duplicates"`
	Abstained         []Abstention `json:"abstained"`
	WitnessRejections []Rejection  `json:"witness_rejections"`
	TableDigest       string       `json:"table_digest"`

	table []byte
	specs map[string]string // variation ID -> spec, for the table's non-test rows
}

// PlanMap names one validated map.
type PlanMap struct {
	App       string `json:"app"`
	MapDigest string `json:"map_digest"`
}

// Counts accounts for every input variation exactly once.
type Counts struct {
	Variations        int `json:"variations"`
	NewTests          int `json:"new_tests"`
	Grouped           int `json:"grouped"`
	Isolated          int `json:"isolated"`
	Reused            int `json:"reused"`
	Duplicates        int `json:"duplicates"`
	Abstained         int `json:"abstained"`
	BaselineOnePerRow int `json:"baseline_one_per_row"`
}

// Context is the tuple a test shares.
type Context struct {
	Spec   string `json:"spec"`
	App    string `json:"app"`
	Setup  string `json:"setup"`
	Screen string `json:"screen"`
	User   string `json:"user"`
	Org    string `json:"org"`
}

// Test is one proposed new test.
type Test struct {
	Test    string  `json:"test"`
	Spec    string  `json:"spec"`
	Context Context `json:"context"`
	Steps   []Step  `json:"steps"`
	Reason  string  `json:"reason"`
}

// Step is one variation placed in a test.
type Step struct {
	Order       int    `json:"order"`
	VariationID string `json:"variation_id"`
}

// Reuse is one variation an existing passing test witnesses.
type Reuse struct {
	VariationID string         `json:"variation_id"`
	Witnesses   []ReuseWitness `json:"witnesses"`
}

// ReuseWitness is one qualifying witness with its unchanged five-axis projection. Project is the
// declared project, empty when the declaration names none.
type ReuseWitness struct {
	TestID     string                  `json:"test_id"`
	Project    string                  `json:"project"`
	Projection testvalidity.Projection `json:"projection"`
}

// Duplicate names the representative a variation duplicates.
type Duplicate struct {
	VariationID string `json:"variation_id"`
	DuplicateOf string `json:"duplicate_of"`
}

// Abstention is one variation the planner cannot place. Missing names absent anchors.
type Abstention struct {
	VariationID string   `json:"variation_id"`
	Reason      string   `json:"reason"`
	Missing     []string `json:"missing"`
}

// Rejection is one declared witness that does not qualify.
type Rejection struct {
	VariationID string `json:"variation_id"`
	TestID      string `json:"test_id"`
	Project     string `json:"-"`
	Reason      string `json:"reason"`
}

// Build computes the plan for one decoded input and its evidence (TCN-V0-003..010).
func Build(in *Input, evidence Evidence, maxSteps int) (*Plan, error) {
	if maxSteps < MinMaxSteps || maxSteps > MaxMaxSteps {
		return nil, invalidArguments("--max-steps must be %d..%d", MinMaxSteps, MaxMaxSteps)
	}
	p := &Plan{Schema: PlanSchema, InputDigest: in.Digest(), TestsDigest: evidence.TestsDigest, MapsDigest: none,
		Maps: []PlanMap{}, EvaluatedRevision: evidence.Revision, AnchorValidation: anchorsNotRun, Authority: "candidate",
		MaxSteps: maxSteps, Tests: []Test{}, Reused: []Reuse{}, Duplicates: []Duplicate{}, Abstained: []Abstention{},
		WitnessRejections: []Rejection{}, specs: map[string]string{}}
	if p.TestsDigest == "" {
		p.TestsDigest = none
	}
	if p.EvaluatedRevision == "" {
		p.EvaluatedRevision = none
	}
	maps := map[string]*MapView{}
	if evidence.Maps != nil {
		p.AnchorValidation = anchorsValidated
		digests := []string{}
		for i := range evidence.Maps {
			m := &evidence.Maps[i]
			maps[m.App] = m
			p.Maps = append(p.Maps, PlanMap{App: m.App, MapDigest: m.Digest})
			digests = append(digests, m.Digest)
		}
		sort.Slice(p.Maps, func(i, j int) bool { return p.Maps[i].App < p.Maps[j].App })
		p.MapsDigest = digestList(digests)
	}

	// TCN-V0-003: abstain before anything else.
	placed := []*Variation{}
	for i := range in.Variations {
		v := &in.Variations[i]
		if v.Spec != nil {
			p.specs[v.ID] = *v.Spec
		} else {
			p.specs[v.ID] = "-"
		}
		if reason := abstainReason(v, evidence.Maps != nil, maps); reason != "" {
			missing := []string{}
			if reason == ReasonMissingAnchor {
				missing = append(missing, v.Missing...)
			}
			p.Abstained = append(p.Abstained, Abstention{VariationID: v.ID, Reason: reason, Missing: missing})
			continue
		}
		placed = append(placed, v)
	}

	// TCN-V0-004: reuse from witnesses.
	reused := map[string][]ReuseWitness{}
	for _, v := range placed {
		for _, w := range v.Witnesses {
			projection, reason := judgeWitness(w, evidence.Documents)
			if reason != "" {
				p.WitnessRejections = append(p.WitnessRejections, Rejection{VariationID: v.ID, TestID: w.TestID, Project: w.Project, Reason: reason})
				continue
			}
			reused[v.ID] = append(reused[v.ID], ReuseWitness{TestID: w.TestID, Project: w.Project, Projection: projection})
		}
	}

	// TCN-V0-005: duplicate classes over every placed variation.
	classes := map[string][]*Variation{}
	keys := []string{}
	for _, v := range placed {
		key := duplicateKey(v)
		if _, ok := classes[key]; !ok {
			keys = append(keys, key)
		}
		classes[key] = append(classes[key], v)
	}
	remaining := []*Variation{}
	for _, key := range keys {
		members := classes[key]
		representative := members[0]
		for _, m := range members {
			if _, ok := reused[m.ID]; ok {
				representative = m
				break
			}
		}
		for _, m := range members {
			if m != representative {
				p.Duplicates = append(p.Duplicates, Duplicate{VariationID: m.ID, DuplicateOf: representative.ID})
			}
		}
		if witnesses, ok := reused[representative.ID]; ok {
			p.Reused = append(p.Reused, Reuse{VariationID: representative.ID, Witnesses: witnesses})
			continue
		}
		remaining = append(remaining, representative)
	}
	sort.Slice(remaining, func(i, j int) bool { return remaining[i].ID < remaining[j].ID })

	// TCN-V0-006..008: destructive isolation, then per-context compatible sets and order.
	type draft struct {
		context     Context
		steps       []*Variation
		conflicting bool
	}
	drafts := []draft{}
	contexts := map[Context][]*Variation{}
	contextOrder := []Context{}
	for _, v := range remaining {
		c := contextOf(v)
		if *v.Destructive {
			drafts = append(drafts, draft{context: c, steps: []*Variation{v}})
			continue
		}
		if _, ok := contexts[c]; !ok {
			contextOrder = append(contextOrder, c)
		}
		contexts[c] = append(contexts[c], v)
	}
	o := &orderer{}
	for _, c := range contextOrder {
		members := contexts[c]
		sets, err := o.plan(members)
		if err != nil {
			return nil, err
		}
		for _, set := range sets {
			alone := len(set.order) == 1 && len(members) > 1
			for start := 0; start < len(set.order); start += maxSteps {
				drafts = append(drafts, draft{context: c, steps: set.order[start:min(start+maxSteps, len(set.order))], conflicting: alone})
			}
		}
	}
	placedContexts := make([]Context, len(drafts))
	for i, d := range drafts {
		placedContexts[i] = d.context
	}
	for _, d := range drafts {
		t := Test{Spec: d.context.Spec, Context: d.context, Steps: []Step{}}
		for i, v := range d.steps {
			t.Steps = append(t.Steps, Step{Order: i + 1, VariationID: v.ID})
		}
		switch {
		case len(d.steps) == 1 && *d.steps[0].Destructive:
			t.Reason = ReasonDestructive
		case d.conflicting:
			t.Reason = ReasonConflictingState
		case len(d.steps) == 1 && differentUser(d.context, placedContexts):
			t.Reason = ReasonDifferentUser
		case len(d.steps) == 1:
			t.Reason = ReasonUniqueContext
		default:
			t.Reason = ReasonGrouped
		}
		p.Tests = append(p.Tests, t)
	}
	sort.Slice(p.Tests, func(i, j int) bool {
		if p.Tests[i].Spec != p.Tests[j].Spec {
			return p.Tests[i].Spec < p.Tests[j].Spec
		}
		return smallestID(p.Tests[i]) < smallestID(p.Tests[j])
	})
	for i := range p.Tests {
		p.Tests[i].Test = fmt.Sprintf("T%03d", i+1)
		if len(p.Tests[i].Steps) > 1 {
			p.Counts.Grouped += len(p.Tests[i].Steps)
		} else {
			p.Counts.Isolated++
		}
	}

	sort.Slice(p.Reused, func(i, j int) bool { return p.Reused[i].VariationID < p.Reused[j].VariationID })
	sort.Slice(p.Duplicates, func(i, j int) bool { return p.Duplicates[i].VariationID < p.Duplicates[j].VariationID })
	sort.Slice(p.WitnessRejections, func(i, j int) bool {
		a, b := p.WitnessRejections[i], p.WitnessRejections[j]
		if a.VariationID != b.VariationID {
			return a.VariationID < b.VariationID
		}
		if a.TestID != b.TestID {
			return a.TestID < b.TestID
		}
		return a.Project < b.Project
	})
	p.Counts.Variations = len(in.Variations)
	p.Counts.BaselineOnePerRow = len(in.Variations)
	p.Counts.NewTests = len(p.Tests)
	p.Counts.Reused = len(p.Reused)
	p.Counts.Duplicates = len(p.Duplicates)
	p.Counts.Abstained = len(p.Abstained)
	p.Status = statusComplete
	if len(p.Abstained) > 0 {
		p.Status = statusIncomplete
	}
	if p.Counts.Grouped+p.Counts.Isolated+p.Counts.Reused+p.Counts.Duplicates+p.Counts.Abstained != p.Counts.Variations {
		return nil, fmt.Errorf("test plan accounting failed")
	}
	return p, p.render()
}

// abstainReason applies TCN-V0-003: a missing anchor first, then the map checks when maps are given.
func abstainReason(v *Variation, validate bool, maps map[string]*MapView) string {
	if len(v.Missing) > 0 {
		return ReasonMissingAnchor
	}
	if !validate {
		return ""
	}
	m, ok := maps[*v.App]
	if !ok {
		return ReasonUnresolvedScreen
	}
	s, ok := m.Screens[*v.Screen]
	if !ok || !s.Resolved {
		return ReasonUnresolvedScreen
	}
	if v.Route != "" && v.Route != s.Template {
		return ReasonRouteMismatch
	}
	switch s.Lineage {
	case "FRESH":
		return ""
	case "STALE":
		return ReasonStaleAnchor
	default:
		return ReasonAnchorUnknown
	}
}

// judgeWitness applies TCN-V0-004 to one declared witness across every document. It returns the
// first match's projection when the witness qualifies, else the rejection reason.
func judgeWitness(w Witness, documents []testvaliditydoc.Document) (testvalidity.Projection, string) {
	var first *testvalidity.Projection
	preview := false
	conflicting := false
	for d := range documents {
		doc := &documents[d]
		for t := range doc.Tests {
			test := &doc.Tests[t]
			if test.ID != w.TestID || (w.Project != "" && (test.Project == nil || test.Project.Name != w.Project)) {
				continue
			}
			if doc.Source != jsProviderSource || doc.Kind != jsEndToEndKind {
				preview = true
			}
			if first == nil {
				first = &test.Projection
			} else if !sameStates(*first, test.Projection) {
				conflicting = true
			}
		}
	}
	switch {
	case first == nil:
		return testvalidity.Projection{}, WitnessNotFound
	case preview:
		return testvalidity.Projection{}, WitnessPreview
	case conflicting:
		return testvalidity.Projection{}, WitnessConflicting
	case first.Association.State != testvalidity.AssociationAssociated:
		return testvalidity.Projection{}, WitnessNotAssociated
	case first.Hygiene.State != testvalidity.HygieneEligible:
		return testvalidity.Projection{}, WitnessIneligible
	case first.Execution.State != testvalidity.ExecutionPassed:
		return testvalidity.Projection{}, WitnessNotPassed
	case first.Freshness.State == testvalidity.FreshnessStale:
		return testvalidity.Projection{}, WitnessStale
	case first.Freshness.State != testvalidity.FreshnessCurrent:
		return testvalidity.Projection{}, WitnessUnbound
	}
	return *first, ""
}

func sameStates(a, b testvalidity.Projection) bool {
	return a.Association.State == b.Association.State && a.Hygiene.State == b.Hygiene.State &&
		a.Freshness.State == b.Freshness.State && a.Execution.State == b.Execution.State && a.Strength.State == b.Strength.State
}

func contextOf(v *Variation) Context {
	return Context{Spec: *v.Spec, App: *v.App, Setup: *v.Setup, Screen: *v.Screen, User: *v.User, Org: *v.Org}
}

// duplicateKey joins every TCN-V0-005 class member with separators no identifier, path or fact
// can hold unescaped in this position (NUL and newline are refused by TCN-V0-002).
func duplicateKey(v *Variation) string {
	c := contextOf(v)
	parts := []string{c.Spec, c.App, c.Setup, c.Screen, c.User, c.Org, strings.Join(v.Requires, "\n"),
		strings.Join(v.Changes, "\n"), fmt.Sprint(*v.Destructive), strings.Join(v.Action, "\n"), strings.Join(v.Assertion, "\n")}
	return strings.Join(parts, "\x00")
}

// differentUser is TCN-V0-006: another new test's variation shares spec, app, setup and screen
// but differs in user or org.
func differentUser(c Context, placed []Context) bool {
	for _, other := range placed {
		if other.Spec == c.Spec && other.App == c.App && other.Setup == c.Setup && other.Screen == c.Screen &&
			(other.User != c.User || other.Org != c.Org) {
			return true
		}
	}
	return false
}

func smallestID(t Test) string {
	id := t.Steps[0].VariationID
	for _, s := range t.Steps[1:] {
		if s.VariationID < id {
			id = s.VariationID
		}
	}
	return id
}

// digestList is "sha256:" over the sorted digests, each followed by LF.
func digestList(digests []string) string {
	sorted := sortedCopy(digests)
	var b strings.Builder
	for _, d := range sorted {
		b.WriteString(d)
		b.WriteByte('\n')
	}
	return sha([]byte(b.String()))
}
