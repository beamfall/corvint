package appmap

import (
	"context"
	"encoding/json"
	"math"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/gokernel"
	"github.com/Beamfall/corvint/internal/secretscreen"
)

// Stage 6 task query (SKH-V0-060..064, SKH-V0-143..152). `flows appmap how GOAL` resolves a goal
// to one stage-5 candidate flow with the AMSP-V0-002 lexical resolver, then answers it from the
// stage 3..5 records only: the goal screen, the setup that arranges the primary source test and
// any --condition, the step path with the admitted edges of each screen hop, and the outcome
// oracle. Every element cites its anchor; a missing element is a named gap, never a guess. The
// answer is a proposal with learned authority, not accepted intent (invariant 8).

// HowSchema is the profile of `flows appmap how`.
const HowSchema = "site-knowledge-how/0"

// How answer states (SKH-V0-063, SKH-V0-144).
const (
	HowComplete   = "COMPLETE"
	HowIncomplete = "INCOMPLETE"
	HowStale      = "STALE"
	HowAbstained  = "ABSTAINED"
)

// How gap codes, a closed set (SKH-V0-144).
const (
	GapNoRoute          = "no-route"
	GapNoSetup          = "no-setup-for-precondition"
	GapNoOracle         = "no-oracle"
	GapNoPageObject     = "no-page-object"
	GapAmbiguousGoal    = "ambiguous-goal"
	GapStaleEvidence    = "stale-evidence"
	maxGoalBytes        = 256
	ProvenanceLedger    = "ledger"
	howBasisEdge        = "edge"
	howBasisVisit       = "visit"
	howBasisPageObject  = "page-object"
	howProposalLabel    = "proposal; learned authority, not accepted intent"
	howUnresolvedScreen = ""
)

// HowGap is one missing element of an answer.
type HowGap struct {
	Code   string  `json:"code"`
	Detail string  `json:"detail,omitempty"`
	Anchor *Anchor `json:"anchor,omitempty"`
}

// HowScreen is the goal screen.
type HowScreen struct {
	ID     string `json:"id"`
	State  string `json:"state"`
	URL    string `json:"url,omitempty"`
	Anchor Anchor `json:"anchor"`
}

// HowSetup is one setup record that arranges the primary source test (SKH-V0-031).
type HowSetup struct {
	Setup  string `json:"setup"`
	Kind   string `json:"kind"`
	Name   string `json:"name,omitempty"`
	Target string `json:"target,omitempty"`
	Status string `json:"status"`
	Reason string `json:"reason,omitempty"`
	Mocked bool   `json:"mocked,omitempty"`
	Anchor Anchor `json:"anchor"`
}

// HowStep is one action of the primary source test, on the screen it acts on.
type HowStep struct {
	Index  int    `json:"index"`
	Kind   string `json:"kind"`
	Action string `json:"action"`
	Via    string `json:"via,omitempty"`
	Screen string `json:"screen,omitempty"`
	Reason string `json:"reason,omitempty"`
	Anchor Anchor `json:"anchor"`
}

// HowEdge is one admitted edge of a hop, with its provenance kept (SKH-V0-146).
type HowEdge struct {
	ID         string  `json:"id"`
	Provenance string  `json:"provenance"`
	Kind       string  `json:"kind,omitempty"`
	Anchor     *Anchor `json:"anchor,omitempty"`
	Ledger     string  `json:"ledger,omitempty"`
	Test       string  `json:"test,omitempty"`
}

// HowHop is one screen change of the step path, at the visit step that reaches To.
type HowHop struct {
	From  string    `json:"from"`
	To    string    `json:"to"`
	At    int       `json:"at"`
	Basis string    `json:"basis"`
	Edges []HowEdge `json:"edges"`
}

// HowOracle is one assertion of the primary test that every source of its variation makes.
type HowOracle struct {
	Matcher  string `json:"matcher"`
	Locator  string `json:"locator,omitempty"`
	Expected string `json:"expected,omitempty"`
	Target   string `json:"target,omitempty"`
	Anchor   Anchor `json:"anchor"`
}

// howLedger admits a ledger loaded by LoadEdgeLedger only when it was printed untrimmed for this
// map and its self-computed digest matches its content (SKH-V0-146).
func howLedger(m *Map, l *EdgeLedger) error {
	bad := &gokernel.Error{Code: "appmap-invalid-query", Message: "--ledger must be an untrimmed application-edge-ledger/0 built from --map"}
	if l.App != m.App || l.MapDigest != m.Digest || l.Edges == nil {
		return bad
	}
	for _, n := range l.Omitted {
		if n != 0 {
			return bad
		}
	}
	check := *l
	check.Digest = ""
	complete, _ := json.Marshal(check)
	if digest(complete) != l.Digest {
		return bad
	}
	return nil
}

func checkGoal(goal string) error {
	if strings.TrimSpace(goal) == "" || len(goal) > maxGoalBytes || strings.ContainsAny(goal, "\x00\n") {
		return &gokernel.Error{Code: "appmap-invalid-query", Message: "the goal must be 1 to 256 bytes of one line"}
	}
	if secretscreen.MatchString(goal) {
		return &gokernel.Error{Code: "appmap-invalid-query", Message: "the goal holds secret-shaped text"}
	}
	return nil
}

// howFlows are the candidate flows as resolver inputs: each is described by its source test
// titles, its actions, its start and end screens and its outcomes (SKH-V0-061, SKH-V0-143).
func howFlows(m *Map, flows []SKCandidateFlow) ([]*planFlow, func(string) float64) {
	df := map[string]int{}
	out := []*planFlow{}
	for i := range flows {
		c := &flows[i]
		texts := []string{}
		for _, v := range c.Variations {
			for _, s := range v.Sources {
				texts = append(texts, s.Title)
			}
			for _, st := range v.Steps {
				texts = append(texts, st.Action)
			}
		}
		for _, id := range []string{c.Start, c.End} {
			if s := m.screen(id); s != nil {
				texts = append(texts, s.State, m.url(s))
			}
		}
		for _, o := range c.Outcomes {
			texts = append(texts, o.Expected)
		}
		pf := &planFlow{m: m, f: &Flow{ID: c.ID, FlowID: c.FlowID}, terms: map[string]bool{}}
		for _, t := range terms(texts...) {
			pf.terms[t] = true
			df[t]++
		}
		out = append(out, pf)
	}
	n := float64(len(out))
	return out, func(t string) float64 { return math.Log(1 + n/float64(max(df[t], 1))) }
}

// unmatchedGoalTerm is the gap detail prefix of a goal term the chosen candidate lacks (SKH-V0-188).
const unmatchedGoalTerm = "unmatched-goal-term: "

// outweighed checks the goal terms a term-matched candidate lacks (SKH-V0-188). Coverage alone
// lets common terms outvote the goal's specific one: "open the manage area" chose a viewer flow on
// "open" and "area" while other candidates carried "manage". A lacked term that another candidate
// carries and that weighs at least as much as the lightest term the choice carries refuses the
// choice, keeping the candidates listed. A lacked term no candidate uses names nothing the map
// knows, so it is returned for a no-route gap: the answer may stand, but never as COMPLETE.
func outweighed(res resolution, goal string, flows []*planFlow, idf func(string) float64) (resolution, []string) {
	if res.flow == nil || res.match.Basis != "terms" {
		return res, nil
	}
	lightest := math.Inf(1)
	for _, t := range res.match.Terms {
		lightest = min(lightest, idf(t))
	}
	elsewhere, unknown := []string{}, []string{}
	for _, t := range terms(goal) {
		if res.flow.terms[t] {
			continue
		}
		carried := false
		for _, pf := range flows {
			carried = carried || pf.terms[t]
		}
		switch {
		case !carried:
			unknown = append(unknown, t)
		case idf(t) >= lightest:
			elsewhere = append(elsewhere, t)
		}
	}
	if len(elsewhere) > 0 {
		return resolution{reason: unmatchedGoalTerm + strings.Join(elsewhere, " "), candidates: res.candidates}, nil
	}
	return res, unknown
}

// goalVariation is the variation whose own source titles and step actions best match the goal,
// by the summed weight of the goal terms they carry. The first (representative) variation wins a
// tie, so a goal naming only terms every variation shares keeps it; a goal naming a term only one
// variation carries never answers with another variation's test (SKH-V0-177).
func goalVariation(f *SKCandidateFlow, goal string, idf func(string) float64) SKVariation {
	q := terms(goal)
	best, bestScore := 0, -1.0
	for i, v := range f.Variations {
		texts := []string{}
		for _, s := range v.Sources {
			texts = append(texts, s.Title)
		}
		for _, st := range v.Steps {
			texts = append(texts, st.Action)
		}
		has := map[string]bool{}
		for _, t := range terms(texts...) {
			has[t] = true
		}
		score := 0.0
		for _, t := range q {
			if has[t] {
				score += idf(t)
			}
		}
		if score > bestScore {
			best, bestScore = i, score
		}
	}
	return f.Variations[best]
}

// admittedEdges indexes the edges a hop may cite by from and to screen: resolved static links,
// the map's test-observed and flow-declared edges, and the ledger's PROVEN rows (SKH-V0-146).
func admittedEdges(static, mapped []SKEdge, ledger *EdgeLedger) map[string][]HowEdge {
	out := map[string][]HowEdge{}
	for _, e := range append(append([]SKEdge{}, static...), mapped...) {
		if e.FromStatus != StatusResolved || e.Status != StatusResolved {
			continue
		}
		a := e.Anchor
		out[e.From+"\x00"+e.To] = append(out[e.From+"\x00"+e.To], HowEdge{ID: e.ID, Provenance: e.Provenance, Kind: e.Kind, Anchor: &a})
	}
	if ledger != nil {
		for _, r := range ledger.Edges {
			if key, ok := provenBy(r); ok {
				out[r.From+"\x00"+r.To] = append(out[r.From+"\x00"+r.To], HowEdge{ID: r.ID, Provenance: ProvenanceLedger, Kind: r.Trigger, Ledger: ledger.Digest, Test: key})
			}
		}
	}
	return out
}

func outcomeOf(s SKStep) SKFlowOutcome {
	o := SKFlowOutcome{Matcher: s.Matcher, Expected: s.Expected, Target: s.Target, Slotted: len(s.Slots) > 0}
	if s.Locator != nil {
		o.Locator = s.Locator.Kind + "=" + s.Locator.Value
		if s.Locator.Name != "" {
			o.Locator += " name=" + s.Locator.Name
		}
	}
	return o
}

// How answers "how do I GOAL" from the site knowledge of the declared sources (SKH-V0-060..064,
// SKH-V0-143..152). conditions are --condition texts the answer must also arrange; ledger, when
// given, adds its PROVEN edges. It reads only committed sources and writes nothing.
func How(ctx context.Context, m *Map, sources []string, goal string, conditions []string, trust []TestTrust, ledger *EdgeLedger, o Options) ([]byte, error) {
	a, err := howCompute(ctx, m, sources, goal, conditions, trust, ledger, o)
	if err != nil {
		return nil, err
	}
	return render(a.head, a.sections, a.budget)
}

// howAnswer is one computed answer: its rendered head and sections, and the typed parts the
// stage 7 draft copies from.
type howAnswer struct {
	head       []field
	sections   []section
	budget     int
	status     string
	gaps       []HowGap
	k          *SKModel
	test       *SKTest
	setup      []HowSetup
	conditions []SetupCondition
	evaluated  string
	revision   string
}

func howCompute(ctx context.Context, m *Map, sources []string, goal string, conditions []string, trust []TestTrust, ledger *EdgeLedger, o Options) (*howAnswer, error) {
	budget, err := o.budget(DefaultFlowBudget)
	if err != nil {
		return nil, err
	}
	if err := checkGoal(goal); err != nil {
		return nil, err
	}
	if err := checkConditions(conditions); err != nil {
		return nil, err
	}
	if ledger != nil {
		if err := howLedger(m, ledger); err != nil {
			return nil, err
		}
	}
	k, flows, _, read, skipped, err := inferRead(ctx, m, sources, trust, o)
	if err != nil {
		return nil, err
	}
	pfs, idf := howFlows(m, flows)
	res, unknown := outweighed(resolve(strings.TrimSpace(goal), pfs, idf), goal, pfs, idf)
	head := append(skHead(HowSchema, m, sources, budget, o, read, skipped),
		field{"goal", goal}, field{"label", howProposalLabel}, field{"candidates_total", len(flows)})
	gaps := []HowGap{}
	cands := []any{}
	if res.flow == nil {
		for _, c := range res.candidates {
			cands = append(cands, c)
		}
		code := GapNoRoute
		if res.reason == "ambiguous-flow" {
			code = GapAmbiguousGoal
		}
		gaps = append(gaps, HowGap{Code: code, Detail: res.reason})
		head = append(head, field{"status", HowAbstained}, field{"gaps", gaps}, field{"candidate", nil})
		return &howAnswer{head: head, sections: []section{{"candidates", cands}}, budget: budget, status: HowAbstained, gaps: gaps, k: k, revision: m.Revision}, nil
	}
	var f *SKCandidateFlow
	for i := range flows {
		if flows[i].ID == res.flow.f.ID {
			f = &flows[i]
		}
	}
	for _, term := range unknown {
		gaps = append(gaps, HowGap{Code: GapNoRoute, Detail: unmatchedGoalTerm + term})
	}
	primary := goalVariation(f, goal, idf)
	var t *SKTest
	for i := range k.Tests {
		if k.Tests[i].ID == primary.Sources[0].Test {
			t = &k.Tests[i]
		}
	}
	anchors := []Anchor{t.Anchor}

	// The goal screen is the candidate's end screen (SKH-V0-143).
	var goalScreen *HowScreen
	if s := m.screen(f.End); s != nil {
		goalScreen = &HowScreen{ID: s.ID, State: s.State, URL: m.url(s), Anchor: s.Anchor}
		anchors = append(anchors, s.Anchor)
	}

	// Steps and hops (SKH-V0-145, SKH-V0-146).
	static, mapped, _, _, _, err := buildLinks(ctx, m, sources, o)
	if err != nil {
		return nil, err
	}
	admitted := admittedEdges(static, mapped, ledger)
	idx := newScreenIndex(m.HashPrefix, m.BasePath, m.Screens)
	lastAction := -1
	for i, s := range t.Steps {
		if s.Kind != StepAssert && s.Kind != StepWaitFor {
			lastAction = i
		}
	}
	steps, hops, oracle := []any{}, []any{}, []HowOracle{}
	screen := howUnresolvedScreen
	shared := map[SKFlowOutcome]bool{}
	for _, oc := range primary.Outcomes {
		shared[oc] = true
	}
	for i, s := range t.Steps {
		if i > lastAction {
			if oc := outcomeOf(s); s.Kind == StepAssert && shared[oc] && !oc.Slotted {
				oracle = append(oracle, HowOracle{Matcher: oc.Matcher, Locator: oc.Locator, Expected: oc.Expected, Target: oc.Target, Anchor: s.Anchor})
				anchors = append(anchors, s.Anchor)
			}
			continue
		}
		if s.Kind == StepAssert || s.Kind == StepWaitFor {
			continue
		}
		anchors = append(anchors, s.Anchor)
		st := HowStep{Index: i + 1, Kind: s.Kind, Action: skActionLabel(s), Screen: screen, Anchor: s.Anchor}
		if len(s.Via) > 0 {
			st.Via = s.Via[0]
		}
		switch {
		case s.Kind == StepVisit:
			to, why := skVisitScreen(idx, s)
			if to == "" {
				a := s.Anchor
				gaps = append(gaps, HowGap{Code: GapNoRoute, Detail: "visit-unresolved: " + why, Anchor: &a})
			} else if screen != howUnresolvedScreen && to != screen {
				h := HowHop{From: screen, To: to, At: i + 1, Basis: howBasisVisit, Edges: admitted[screen+"\x00"+to]}
				if len(h.Edges) > 0 {
					h.Basis = howBasisEdge
					for _, e := range h.Edges {
						if e.Anchor != nil {
							anchors = append(anchors, *e.Anchor)
						}
					}
				} else {
					h.Edges = []HowEdge{}
				}
				hops = append(hops, h)
			}
			screen, st.Screen = to, to
		case k.stepScreen(s) != "":
			// A page-object step runs on its page object's joined screen (SKH-V0-185).
			to := k.stepScreen(s)
			if screen != howUnresolvedScreen && to != screen {
				h := HowHop{From: screen, To: to, At: i + 1, Basis: howBasisPageObject, Edges: admitted[screen+"\x00"+to]}
				if len(h.Edges) > 0 {
					h.Basis = howBasisEdge
					for _, e := range h.Edges {
						if e.Anchor != nil {
							anchors = append(anchors, *e.Anchor)
						}
					}
				} else {
					h.Edges = []HowEdge{}
				}
				hops = append(hops, h)
			}
			screen, st.Screen = to, to
			if s.Kind == StepOpaque {
				st.Reason = s.Reason
				code := GapNoRoute
				if s.Reason == OpaqueUnresolvedCall {
					code = GapNoPageObject
				}
				a := s.Anchor
				gaps = append(gaps, HowGap{Code: code, Detail: s.Reason + ": " + s.Call, Anchor: &a})
			}
		case s.Kind == StepOpaque:
			st.Reason = s.Reason
			code := GapNoRoute
			if s.Reason == OpaqueUnresolvedCall {
				code = GapNoPageObject
			}
			a := s.Anchor
			gaps = append(gaps, HowGap{Code: code, Detail: s.Reason + ": " + s.Call, Anchor: &a})
		}
		steps = append(steps, st)
	}
	if f.Status == CandidateBlocked {
		gaps = append(gaps, HowGap{Code: GapNoOracle, Detail: "blocked-by: " + strings.Join(f.BlockedBy, ",")})
	} else if len(oracle) == 0 {
		gaps = append(gaps, HowGap{Code: GapNoOracle, Detail: "no-shared-literal-assertion"})
	}

	// Setup: the records arranging the primary test, then each asked condition (SKH-V0-147).
	records, uses, _, err := k.allSetup(repo{ctx: ctx, root: o.Root}, m, o.Repos)
	if err != nil {
		return nil, err
	}
	byID := map[string]SKSetup{}
	for _, r := range records {
		byID[r.ID] = r
	}
	setup, setupTyped := []any{}, []HowSetup{}
	for _, u := range uses {
		if u.Test != t.ID {
			continue
		}
		for _, id := range u.Uses {
			r := byID[id]
			hs := HowSetup{Setup: r.ID, Kind: r.Kind, Name: r.Name, Target: r.Target, Status: r.Status, Reason: r.Reason, Mocked: r.Mocked, Anchor: r.Anchor}
			setup, setupTyped = append(setup, hs), append(setupTyped, hs)
			anchors = append(anchors, r.Anchor)
			if r.Status != StatusResolved {
				a := r.Anchor
				gaps = append(gaps, HowGap{Code: GapNoSetup, Detail: r.ID + ": " + r.Reason, Anchor: &a})
			}
		}
		for _, name := range u.Unresolved {
			gaps = append(gaps, HowGap{Code: GapNoSetup, Detail: "unresolved-fixture: " + name})
		}
	}
	asked, typed := make([]any, len(conditions)), make([]SetupCondition, len(conditions))
	for i, c := range conditions {
		q := conditionQuery(c, records)
		asked[i], typed[i] = q, q
		if q.Status != ConditionCandidates {
			gaps = append(gaps, HowGap{Code: GapNoSetup, Detail: "condition: " + c})
		}
		for _, cand := range q.Candidates {
			anchors = append(anchors, byID[cand.Setup].Anchor)
		}
	}

	// Freshness of every cited anchor (SKH-V0-063): STALE is never COMPLETE.
	fresh := newFreshness(ctx, o, anchors)
	state := Fresh
	for _, a := range anchors {
		switch fresh.of(a) {
		case Stale:
			state = Stale
		case FreshUnknown:
			if state != Stale {
				state = FreshUnknown
			}
		}
	}
	switch state {
	case Stale:
		gaps = append(gaps, HowGap{Code: GapStaleEvidence, Detail: "a cited span changed at " + fresh.evaluated})
	case FreshUnknown:
		gaps = append(gaps, HowGap{Code: GapStaleEvidence, Detail: "freshness-unknown"})
	}
	status := HowComplete
	switch {
	case state == Stale:
		status = HowStale
	case len(gaps) > 0:
		status = HowIncomplete
	}
	sort.SliceStable(gaps, func(i, j int) bool { return gaps[i].Code < gaps[j].Code })
	type candidateRef struct {
		ID           string   `json:"id"`
		FlowID       string   `json:"flow_id"`
		Status       string   `json:"status"`
		Support      int      `json:"support"`
		SingleSource bool     `json:"single_source,omitempty"`
		Test         string   `json:"test"`
		Rank         string   `json:"rank"`
		Gaps         []string `json:"knowledge_gaps,omitempty"`
		Anchor       Anchor   `json:"anchor"`
	}
	head = append(head, field{"status", status}, field{"freshness", state}, field{"evaluated_revision", fresh.evaluated},
		field{"match", res.match}, field{"gaps", gaps},
		field{"candidate", candidateRef{ID: f.ID, FlowID: f.FlowID, Status: f.Status, Support: f.Support, SingleSource: f.SingleSource,
			Test: t.ID, Rank: primary.Sources[0].Rank, Gaps: f.Gaps, Anchor: t.Anchor}},
		field{"goal_screen", goalScreen}, field{"start_screen", f.Start}, field{"oracle", oracle},
		field{"ledger", ledgerDigest(ledger)})
	return &howAnswer{head: head, sections: []section{{"setup", setup}, {"conditions", asked}, {"steps", steps}, {"path", hops}, {"candidates", cands}},
		budget: budget, status: status, gaps: gaps, k: k, test: t, setup: setupTyped, conditions: typed, evaluated: fresh.evaluated, revision: m.Revision}, nil
}

func ledgerDigest(l *EdgeLedger) any {
	if l == nil {
		return nil
	}
	return l.Digest
}
