package appmap

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"path"
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/gokernel"
)

// Scenario planner (docs/specs/application-map-scenario-planner-v0.md, AMSP-V0): resolves a
// plain-language multi-step request against one or more application maps and composes a
// fail-closed end-to-end plan. It reads the maps and Git only; it never writes, runs a browser or
// invents a path.

// PlanSchema is the wire identity of a plan document (AMSP-V0-001).
const PlanSchema = "application-map-plan/0"

// Plan bounds (AMSP-V0-001, AMSP-V0-009).
const (
	DefaultPlanBudget = 16384
	maxPlanSteps      = 16
	maxPlanStepBytes  = 512
	maxPlanMaps       = 8
	// minCoverage is the share of a request's term weight a flow must exceed to be chosen.
	minCoverage = 0.5
	// matchCandidates is how many near misses an unmapped step lists.
	matchCandidates = 3
)

// Plan step statuses (AMSP-V0-005). Only MAPPED steps are composed into navigation and code.
const (
	StepMapped       = "MAPPED"
	StepUnmapped     = "UNMAPPED"
	StepStale        = "STALE"
	StepUnknown      = "UNKNOWN"
	StepContradicted = "CONTRADICTED"
)

// VerificationFactKind is the overlay Fact kind the planner reads as run verification
// (AMSP-V0-007). Such a fact's Text is one of VERIFIED, UNVERIFIED_AT_HEAD, CONTRADICTED or
// unverified, and a VERIFIED fact names the full commit it was observed at in Revision. Facts of
// any other kind are not verification. The receipt-bound producer is issue 658 (V1-0957); until
// an overlay supplies such facts every element reads unverified.
const VerificationFactKind = "run-verification"

// Plan verification statuses (AMSP-V0-007). They are unexported so the issue-658 package-level
// status names stay free.
const (
	planVerified         = "VERIFIED"
	planVerifiedPrefix   = "VERIFIED@"
	planUnverifiedAtHead = "UNVERIFIED_AT_HEAD"
	planContradicted     = "CONTRADICTED"
	planUnverified       = "unverified"
)

// PlanOptions are the planner inputs beyond the shared projection Options.
type PlanOptions struct {
	Options
	// Draft adds the Playwright spec skeleton to the plan (AMSP-V0-008). Verification is read
	// from Options.Overlays (VerificationFactKind facts); with none every element reads unverified.
	Draft bool
}

var enumerator = regexp.MustCompile(`^\(?[0-9]{1,2}[.)]\s*`)

// SplitRequest splits one plain-language request into steps at `;` and line breaks, dropping a
// leading `1)`, `(2)` or `3.` enumerator and empty parts (AMSP-V0-001).
func SplitRequest(request string) []string {
	out := []string{}
	for _, part := range strings.FieldsFunc(request, func(r rune) bool { return r == ';' || r == '\n' || r == '\r' }) {
		part = strings.TrimSpace(enumerator.ReplaceAllString(strings.TrimSpace(part), ""))
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func planQueryError(format string, args ...any) error {
	return &gokernel.Error{Code: "appmap-invalid-query", Message: fmt.Sprintf(format, args...)}
}

// planFlow is one flow of one map, with the terms it can be matched by.
type planFlow struct {
	m     *Map
	f     *Flow
	terms map[string]bool
}

var stopwords = map[string]bool{"a": true, "an": true, "the": true, "to": true, "of": true, "in": true, "on": true, "for": true,
	"and": true, "then": true, "with": true, "as": true, "at": true, "by": true, "from": true, "it": true, "its": true,
	"is": true, "are": true, "be": true, "that": true, "this": true, "into": true, "or": true, "my": true, "their": true}

// stem folds a plural `s` and then an `ing` or `ed` suffix, so book, books, booked and booking
// share one term.
func stem(w string) string {
	if len(w) > 3 && strings.HasSuffix(w, "s") && !strings.HasSuffix(w, "ss") {
		w = w[:len(w)-1]
	}
	switch {
	case len(w) > 5 && strings.HasSuffix(w, "ing"):
		w = w[:len(w)-3]
	case len(w) > 4 && strings.HasSuffix(w, "ed"):
		w = w[:len(w)-2]
	}
	return w
}

func terms(texts ...string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, t := range texts {
		for _, w := range strings.FieldsFunc(strings.ToLower(t), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) }) {
			if len(w) < 2 || stopwords[w] {
				continue
			}
			if w = stem(w); !seen[w] {
				seen[w] = true
				out = append(out, w)
			}
		}
	}
	sort.Strings(out)
	return out
}

type matchView struct {
	Basis    string   `json:"basis"`
	Coverage float64  `json:"coverage"`
	Terms    []string `json:"terms"`
}

type candidateView struct {
	App      string  `json:"app"`
	Flow     string  `json:"flow"`
	Coverage float64 `json:"coverage"`
}

// resolution is how one request step resolved to a flow, or why it did not (AMSP-V0-002).
type resolution struct {
	flow       *planFlow
	match      matchView
	reason     string
	candidates []candidateView
}

func round3(x float64) float64 { return math.Round(x*1000) / 1000 }

// resolve matches a request step against every flow: an explicit `flow:<flow_id>` names one flow;
// otherwise inverse-document-frequency weighted term coverage, where a term no flow uses still
// counts against the match. A coverage of at most minCoverage, or a tie for the best, is not chosen.
func resolve(text string, flows []*planFlow, idf func(string) float64) resolution {
	if id, ok := strings.CutPrefix(text, "flow:"); ok {
		var hits []*planFlow
		cands := []candidateView{}
		for _, pf := range flows {
			if pf.f.FlowID == id {
				hits = append(hits, pf)
				cands = append(cands, candidateView{App: pf.m.App, Flow: pf.f.ID, Coverage: 1})
			}
		}
		switch len(hits) {
		case 1:
			return resolution{flow: hits[0], match: matchView{Basis: "explicit", Coverage: 1, Terms: []string{}}}
		case 0:
			return resolution{reason: "no-matching-flow", candidates: cands}
		}
		return resolution{reason: "ambiguous-flow", candidates: cands}
	}
	q := terms(text)
	total := 0.0
	for _, t := range q {
		total += idf(t)
	}
	type scored struct {
		pf      *planFlow
		cov     float64
		matched []string
	}
	all := []scored{}
	for _, pf := range flows {
		got, matched := 0.0, []string{}
		for _, t := range q {
			if pf.terms[t] {
				got += idf(t)
				matched = append(matched, t)
			}
		}
		if got > 0 {
			all = append(all, scored{pf, round3(got / total), matched})
		}
	}
	sort.SliceStable(all, func(i, j int) bool { return all[i].cov > all[j].cov })
	cands := []candidateView{}
	for i := 0; i < len(all) && i < matchCandidates; i++ {
		cands = append(cands, candidateView{App: all[i].pf.m.App, Flow: all[i].pf.f.ID, Coverage: all[i].cov})
	}
	switch {
	case len(all) == 0 || all[0].cov <= minCoverage:
		return resolution{reason: "no-matching-flow", candidates: cands}
	case len(all) > 1 && all[1].cov == all[0].cov:
		return resolution{reason: "ambiguous-flow", candidates: cands}
	}
	return resolution{flow: all[0].pf, match: matchView{Basis: "terms", Coverage: all[0].cov, Terms: all[0].matched}}
}

// Plan document parts (AMSP-V0-003..008).

type methodRef struct {
	ID           string `json:"id"`
	Ref          string `json:"ref"`
	Freshness    string `json:"freshness"`
	Verification string `json:"verification"`
}

type selectorRef struct {
	ID string `json:"id"`
	selectorView
	Verification string `json:"verification"`
}

type actionView struct {
	Step         string       `json:"step"`
	Action       string       `json:"action"`
	Screen       string       `json:"screen,omitempty"`
	URL          string       `json:"url,omitempty"`
	Status       string       `json:"status"`
	Reason       string       `json:"reason,omitempty"`
	Navigation   string       `json:"navigation,omitempty"`
	Lineage      string       `json:"lineage_freshness,omitempty"`
	Selector     *selectorRef `json:"selector,omitempty"`
	Verification string       `json:"verification"`
	Methods      []methodRef  `json:"methods"`
	MethodsTotal int          `json:"methods_total"`
}

type exploration struct {
	Need   string   `json:"need"`
	Detail string   `json:"detail"`
	Refs   []string `json:"refs"`
}

type specRef struct {
	File      string `json:"file"`
	Ref       string `json:"ref"`
	Freshness string `json:"freshness"`
}

type planStep struct {
	Index       int             `json:"index"`
	Request     string          `json:"request"`
	Status      string          `json:"status"`
	Reason      string          `json:"reason,omitempty"`
	Confidence  string          `json:"confidence"`
	App         string          `json:"app,omitempty"`
	Flow        string          `json:"flow,omitempty"`
	Match       *matchView      `json:"match,omitempty"`
	Candidates  []candidateView `json:"candidates,omitempty"`
	FlowAnchor  *anchorView     `json:"flow_anchor,omitempty"`
	Session     string          `json:"session,omitempty"`
	Actions     []actionView    `json:"actions"`
	Asserts     []Outcome       `json:"asserts"`
	Produces    []string        `json:"produces"`
	Consumes    []string        `json:"consumes"`
	Spec        *specRef        `json:"spec,omitempty"`
	Exploration []exploration   `json:"exploration"`

	res     resolution
	methods [][]*Method // per action: the shown reuse methods, nil entries for unresolvable IDs
	files   [][]*TestFile
}

type preconditionView struct {
	Flow        string `json:"flow"`
	Text        string `json:"text"`
	SatisfiedBy int    `json:"satisfied_by_step,omitempty"`
	Status      string `json:"status"`
}

type setupView struct {
	Scenario      string             `json:"scenario,omitempty"`
	Status        string             `json:"status"`
	Reason        string             `json:"reason,omitempty"`
	CoversSteps   []int              `json:"covers_steps"`
	Freshness     string             `json:"freshness,omitempty"`
	Preconditions []preconditionView `json:"preconditions"`
}

type sessionView struct {
	ident       string
	App         string    `json:"app"`
	Page        string    `json:"page"`
	Steps       []int     `json:"steps"`
	Permissions []string  `json:"permissions"`
	TestJoin    string    `json:"test_join"`
	Setup       setupView `json:"setup"`
}

type handoffView struct {
	App        string `json:"app"`
	Param      string `json:"param"`
	Source     string `json:"source"`
	Producer   int    `json:"producer_step,omitempty"`
	Capture    string `json:"capture,omitempty"`
	ConsumedBy []int  `json:"consumed_by"`
}

type gapView struct {
	Index  int    `json:"index"`
	Status string `json:"status"`
	Reason string `json:"reason"`
}

type mapRef struct {
	App      string `json:"app"`
	Revision string `json:"map_revision"`
	Digest   string `json:"map_digest"`
}

type draftView struct {
	ProposedPath string   `json:"proposed_path"`
	Imports      []string `json:"imports"`
	Lines        []string `json:"lines"`
}

// verificationOf folds one verification fact into the closed set: a VERIFIED fact reads
// VERIFIED@<rev> only when its Revision is the evaluated revision and the anchor is FRESH, else
// UNVERIFIED_AT_HEAD; an unknown value or a VERIFIED fact without a full commit ID reads
// unverified and is reported (AMSP-V0-007).
func verificationOf(f Fact, evaluated, freshness string) (string, bool) {
	switch f.Text {
	case planUnverified:
		return planUnverified, true
	case planUnverifiedAtHead, planContradicted:
		return f.Text, true
	case planVerified:
		if !isHexID(f.Revision) {
			return planUnverified, false
		}
		if f.Revision != evaluated || freshness != Fresh {
			return planUnverifiedAtHead, true
		}
		return planVerifiedPrefix + f.Revision, true
	}
	return planUnverified, false
}

// verificationRank orders folded statuses from the most to the least restrictive, so conflicting
// facts about one element resolve fail-closed: any CONTRADICTED wins, and VERIFIED stands only
// when every fact agrees.
func verificationRank(v string) int {
	switch {
	case v == planContradicted:
		return 0
	case v == planUnverified:
		return 1
	case v == planUnverifiedAtHead:
		return 2
	}
	return 3
}

func isHexID(s string) bool {
	if len(s) != 40 && len(s) != 64 {
		return false
	}
	for _, r := range s {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return false
		}
	}
	return true
}

// Plan is map_plan (AMSP-V0): one capped, deterministic, fail-closed plan for steps over maps.
func Plan(ctx context.Context, maps []*Map, steps []string, o PlanOptions) ([]byte, error) {
	budget, err := o.budget(DefaultPlanBudget)
	if err != nil {
		return nil, err
	}
	if len(maps) == 0 || len(maps) > maxPlanMaps {
		return nil, planQueryError("map_plan needs 1 to %d maps", maxPlanMaps)
	}
	maps = append([]*Map{}, maps...)
	sort.SliceStable(maps, func(i, j int) bool { return maps[i].App < maps[j].App })
	for i, m := range maps {
		if m == nil || (i > 0 && maps[i-1].App == m.App) {
			return nil, planQueryError("map_plan maps must name distinct apps")
		}
	}
	if len(steps) == 0 || len(steps) > maxPlanSteps {
		return nil, planQueryError("map_plan needs 1 to %d steps", maxPlanSteps)
	}
	for _, s := range steps {
		if t := strings.TrimSpace(s); t == "" || len(s) > maxPlanStepBytes || !utf8.ValidString(s) ||
			strings.ContainsFunc(s, func(r rune) bool { return unicode.IsControl(r) || r == ' ' || r == ' ' }) {
			return nil, planQueryError("each map_plan step must be 1 to %d bytes of single-line UTF-8 text", maxPlanStepBytes)
		}
	}

	// Resolve every step to a flow (AMSP-V0-002).
	flows := []*planFlow{}
	df := map[string]int{}
	for _, m := range maps {
		for i := range m.Flows {
			f := &m.Flows[i]
			texts := []string{strings.ReplaceAll(f.FlowID, "-", " ")}
			for _, st := range f.Steps {
				texts = append(texts, st.Action)
			}
			for _, oc := range f.Outcomes {
				texts = append(texts, oc.Behavior)
			}
			pf := &planFlow{m: m, f: f, terms: map[string]bool{}}
			for _, t := range terms(texts...) {
				pf.terms[t] = true
				df[t]++
			}
			flows = append(flows, pf)
		}
	}
	n := float64(len(flows))
	idf := func(t string) float64 { return math.Log(1 + n/float64(max(df[t], 1))) }
	plan := make([]*planStep, len(steps))
	anchors := []Anchor{}
	for i, text := range steps {
		ps := &planStep{Index: i + 1, Request: strings.TrimSpace(text), Actions: []actionView{}, Asserts: []Outcome{}, Produces: []string{},
			Consumes: []string{}, Exploration: []exploration{}}
		ps.res = resolve(ps.Request, flows, idf)
		plan[i] = ps
		pf := ps.res.flow
		if pf == nil {
			continue
		}
		anchors = append(anchors, pf.f.Anchor)
		for _, st := range pf.f.Steps {
			var shown []*Method
			var files []*TestFile
			if s := pf.m.screen(st.Screen); s != nil {
				anchors = append(anchors, pf.m.lineage(s)...)
			}
			if ms := reuseMethods(st); len(ms) > 0 {
				for _, r := range ms[:min(len(ms), reuseShown)] {
					f, me := pf.m.method(r)
					shown, files = append(shown, me), append(files, f)
					if me != nil {
						anchors = append(anchors, me.Anchor)
					}
					if f != nil {
						// The class file and every file importing it are the import evidence.
						anchors = append(anchors, f.Anchor)
						for _, tf := range importersOf(pf.m, f.Path) {
							anchors = append(anchors, tf.Anchor)
						}
					}
				}
			}
			ps.methods, ps.files = append(ps.methods, shown), append(ps.files, files)
		}
		if spec, _, _ := pf.m.closestSpec(pf.f); spec != nil {
			anchors = append(anchors, spec.Anchor)
		}
	}
	scenarios := map[string]map[string][]int{} // app -> scenario path -> plan steps covered
	for _, ps := range plan {
		if pf := ps.res.flow; pf != nil {
			for _, st := range pf.f.Steps {
				if s := pf.m.screen(st.Screen); s != nil {
					for _, sc := range s.Scenarios {
						if scenarios[pf.m.App] == nil {
							scenarios[pf.m.App] = map[string][]int{}
						}
						if c := scenarios[pf.m.App][sc]; len(c) == 0 || c[len(c)-1] != ps.Index {
							scenarios[pf.m.App][sc] = append(c, ps.Index)
						}
						if f := pf.m.file(sc); f != nil {
							anchors = append(anchors, f.Anchor)
						}
					}
				}
			}
		}
	}
	fresh := newFreshness(ctx, o.Options, anchors)
	p := &projection{ctx: ctx, o: o.Options, fresh: fresh, elements: map[string]bool{}}
	worst := func(as []Anchor) string { return p.worst(as) }

	// Ask the verification seam once (AMSP-V0-007).
	verifyIDs := map[string]bool{}
	idApps := map[string]map[string]bool{} // element ID -> apps whose plan steps print it
	addID := func(id, app string) {
		verifyIDs[id] = true
		if idApps[id] == nil {
			idApps[id] = map[string]bool{}
		}
		idApps[id][app] = true
	}
	for _, ps := range plan {
		if pf := ps.res.flow; pf != nil {
			for j, st := range pf.f.Steps {
				addID(st.ID, pf.m.App)
				if st.Selector != nil {
					addID(st.Selector.ID, pf.m.App)
				}
				for _, me := range ps.methods[j] {
					if me != nil {
						addID(me.ID, pf.m.App)
					}
				}
			}
		}
	}
	unknowns := []any{}
	// Element IDs carry no app, so a fact about an ID printed by two apps cannot be attributed to
	// either: such an ID reads unverified whatever its facts say.
	ambiguous := map[string]bool{}
	for _, id := range sortedKeys(idApps) {
		if len(idApps[id]) > 1 {
			ambiguous[id] = true
			unknowns = append(unknowns, Unknown{Kind: "verification", Ref: id, Reason: "verification-ambiguous"})
		}
	}
	facts, kept := map[string][]Fact{}, 0
	if len(verifyIDs) > 0 {
		ids := sortedKeys(verifyIDs)
	overlays:
		for _, ov := range o.Overlays {
			got, err := ov.Facts(ctx, ids)
			if err != nil {
				unknowns = append(unknowns, Unknown{Kind: "verification", Ref: "map_plan", Reason: "verification-unavailable"})
				continue
			}
			for _, f := range got {
				if f.Kind != VerificationFactKind || !verifyIDs[f.ElementID] {
					continue
				}
				if kept == maxFacts {
					// A dropped fact may be the contradiction, so no retained fact can stand.
					unknowns = append(unknowns, Unknown{Kind: "verification", Ref: "map_plan", Reason: "verification-bound-exceeded"})
					facts = map[string][]Fact{}
					break overlays
				}
				facts[f.ElementID] = append(facts[f.ElementID], f)
				kept++
			}
		}
	}
	invalid := map[string]bool{}
	verify := func(id, freshness string) string {
		v := planUnverified
		if ambiguous[id] {
			return v
		}
		for i, f := range facts[id] {
			got, ok := verificationOf(f, fresh.evaluated, freshness)
			if !ok && !invalid[id] {
				invalid[id] = true
				unknowns = append(unknowns, Unknown{Kind: "verification", Ref: id, Reason: "verification-invalid"})
			}
			if i == 0 || verificationRank(got) < verificationRank(v) {
				v = got
			}
		}
		return v
	}

	// Per-step mapping, freshness and verification (AMSP-V0-003..007).
	for _, ps := range plan {
		pf := ps.res.flow
		if pf == nil {
			ps.Status, ps.Reason, ps.Confidence, ps.Candidates = StepUnmapped, ps.res.reason, "none", ps.res.candidates
			detail := "no flow intent matches this request; explore the app for it, record an application-flow-intent/1 with navigation steps, then rebuild the map"
			if ps.res.reason == "ambiguous-flow" {
				detail = "several flows match equally; name one as flow:<flow_id> or record a more specific flow intent"
			}
			refs := []string{}
			for _, c := range ps.res.candidates {
				refs = append(refs, c.Flow)
			}
			ps.Exploration = append(ps.Exploration, exploration{Need: "flow-intent", Detail: detail, Refs: refs})
			continue
		}
		m, fl := pf.m, pf.f
		match := ps.res.match
		ps.App, ps.Flow, ps.Match, ps.Session = m.App, fl.ID, &match, m.App
		fa := anchorView{Ref: anchorRef(fl.Anchor), Blob: fl.Anchor.Blob, Freshness: fresh.of(fl.Anchor)}
		ps.FlowAnchor = &fa
		ps.Asserts = append(ps.Asserts, fl.Outcomes...)
		stale, unknownFresh, unplaced, contradicted := []string{}, false, []string{}, []string{}
		note := func(state, ref string) {
			switch state {
			case Stale:
				stale = append(stale, ref)
			case FreshUnknown:
				unknownFresh = true
			}
		}
		note(fa.Freshness, fa.Ref)
		allVerified := true
		for j, st := range fl.Steps {
			av := actionView{Step: st.ID, Action: st.Action, Status: st.Status, Reason: st.Reason, Methods: []methodRef{}}
			s := m.screen(st.Screen)
			switch {
			case st.Status != StatusResolved || s == nil:
				unplaced = append(unplaced, st.ID)
				if av.Reason == "" {
					av.Reason = "no-matching-screen"
				}
			case s.Abstract:
				unplaced = append(unplaced, st.ID)
				av.Reason = "abstract-screen"
			default:
				av.Screen, av.URL, av.Lineage = s.ID, m.url(s), worst(m.lineage(s))
				note(av.Lineage, s.ID)
			}
			av.Verification = verify(st.ID, fa.Freshness)
			if st.Selector != nil {
				sr := selectorRef{ID: st.Selector.ID, selectorView: *viewSelector(st.Selector), Verification: verify(st.Selector.ID, fa.Freshness)}
				av.Selector = &sr
				if sr.Verification == planContradicted {
					contradicted = append(contradicted, st.Selector.ID)
				}
				if !strings.HasPrefix(sr.Verification, planVerifiedPrefix) {
					allVerified = false
				}
			}
			if av.Verification == planContradicted {
				contradicted = append(contradicted, st.ID)
			}
			if !strings.HasPrefix(av.Verification, planVerifiedPrefix) {
				allVerified = false
			}
			av.MethodsTotal = len(reuseMethods(st))
			for k, me := range ps.methods[j] {
				if me == nil {
					continue
				}
				mf := fresh.of(me.Anchor)
				note(mf, anchorRef(me.Anchor))
				mr := methodRef{ID: me.ID, Ref: refAt(ps.files[j][k].Path, me.Anchor.Start), Freshness: mf, Verification: verify(me.ID, mf)}
				if mr.Verification == planContradicted {
					contradicted = append(contradicted, me.ID)
				}
				if !strings.HasPrefix(mr.Verification, planVerifiedPrefix) {
					allVerified = false
				}
				av.Methods = append(av.Methods, mr)
			}
			ps.Actions = append(ps.Actions, av)
		}
		if spec, _, _ := m.closestSpec(fl); spec != nil {
			ps.Spec = &specRef{File: spec.Path, Ref: anchorRef(spec.Anchor), Freshness: fresh.of(spec.Anchor)}
		}
		switch {
		case len(fl.Steps) == 0:
			ps.Status, ps.Reason = StepUnmapped, "no-flow-steps"
			ps.Exploration = append(ps.Exploration, exploration{Need: "flow-steps", Detail: "the flow declares no steps; record its steps with navigation entries", Refs: []string{fl.ID}})
		case len(unplaced) > 0:
			ps.Status, ps.Reason = StepUnmapped, "unplaced-step"
			reasons := []string{}
			for _, av := range ps.Actions {
				if av.Screen == "" {
					reasons = append(reasons, av.Step+" "+av.Reason)
				}
			}
			ps.Exploration = append(ps.Exploration, exploration{Need: "navigation-step",
				Detail: "explore where each listed step happens and give it one navigation state that matches exactly one screen: " + strings.Join(reasons, "; "), Refs: unplaced})
		case len(fl.Outcomes) == 0:
			ps.Status, ps.Reason = StepUnmapped, "no-declared-outcome"
			ps.Exploration = append(ps.Exploration, exploration{Need: "outcome", Detail: "the flow declares no expected outcome to assert; record one in its intent", Refs: []string{fl.ID}})
		case len(stale) > 0:
			ps.Status, ps.Reason = StepStale, "stale-anchors"
			ps.Exploration = append(ps.Exploration, exploration{Need: "rebuild-map",
				Detail: fmt.Sprintf("anchored lines changed since map revision %s; re-explore the changed screens or methods, rebuild the map at the evaluated revision and plan again", m.Revision), Refs: uniqueSorted(stale)})
		case len(contradicted) > 0:
			ps.Status, ps.Reason = StepContradicted, "run-contradicted"
			ps.Exploration = append(ps.Exploration, exploration{Need: "re-explore", Detail: "a run receipt contradicts the listed elements; re-explore the step and correct the flow or page object", Refs: uniqueSorted(contradicted)})
		case unknownFresh:
			ps.Status, ps.Reason = StepUnknown, "freshness-unknown"
			ps.Exploration = append(ps.Exploration, exploration{Need: "freshness", Detail: "Git could not evaluate the anchors; plan against a resolvable --revision in the mapped repository", Refs: []string{fl.ID}})
		default:
			ps.Status = StepMapped
		}
		switch {
		case ps.Status == StepUnmapped:
			ps.Confidence = "none"
		case ps.Status == StepStale:
			ps.Confidence = "stale"
		case ps.Status == StepContradicted:
			ps.Confidence = "contradicted"
		case ps.Status == StepUnknown:
			ps.Confidence = "unknown"
		case allVerified:
			ps.Confidence = "run-verified"
		default:
			ps.Confidence = "candidate"
		}
	}

	// Sessions, navigation and handoff over MAPPED steps only (AMSP-V0-004, AMSP-V0-006).
	byApp := map[string]*Map{}
	for _, m := range maps {
		byApp[m.App] = m
	}
	sessions := map[string]*sessionView{}
	idents := map[string]bool{}
	current := map[string]string{}
	bound := map[string]map[string]*handoffView{}
	handoffs := []*handoffView{}
	flowStep := map[string]int{} // app/flow -> first MAPPED step resolving to it
	for _, ps := range plan {
		pf := ps.res.flow
		if pf == nil {
			continue
		}
		app := pf.m.App
		sv := sessions[app]
		if sv == nil {
			ident := jsIdent(app)
			for k := 2; idents[ident]; k++ {
				ident = fmt.Sprintf("%s%d", jsIdent(app), k)
			}
			idents[ident] = true
			sv = &sessionView{ident: ident, App: app, Page: ident + "Page", Steps: []int{}, Permissions: []string{}, TestJoin: pf.m.testJoin()}
			sessions[app] = sv
		}
		sv.Steps = append(sv.Steps, ps.Index)
		if ps.Status != StepMapped {
			current[app] = "" // the page state after a step that is not composed is unknown
			continue
		}
		if bound[app] == nil {
			bound[app] = map[string]*handoffView{}
		}
		prev := ""
		for j := range ps.Actions {
			av := &ps.Actions[j]
			s := pf.m.screen(av.Screen)
			sv.Permissions = append(sv.Permissions, s.Permissions...)
			switch {
			case j == 0 && current[app] == s.ID:
				av.Navigation = "stay"
			case j == 0:
				av.Navigation = "goto"
				for _, param := range s.Params {
					h := bound[app][param]
					if h == nil {
						h = &handoffView{App: app, Param: param, Source: "setup", ConsumedBy: []int{}}
						bound[app][param] = h
						handoffs = append(handoffs, h)
					}
					if h.Producer != ps.Index {
						h.ConsumedBy = append(h.ConsumedBy, ps.Index)
						ps.Consumes = append(ps.Consumes, param)
					}
				}
			case s.ID == prev:
				av.Navigation = "in-screen"
			default:
				av.Navigation = "follow"
				for _, param := range s.Params {
					if bound[app][param] == nil {
						h := &handoffView{App: app, Param: param, Source: "step", Producer: ps.Index, Capture: av.URL, ConsumedBy: []int{}}
						bound[app][param] = h
						handoffs = append(handoffs, h)
						ps.Produces = append(ps.Produces, param)
					}
				}
			}
			prev = s.ID
		}
		current[app] = prev
		if _, seen := flowStep[app+"/"+pf.f.ID]; !seen {
			flowStep[app+"/"+pf.f.ID] = ps.Index
		}
	}
	sessionList := []*sessionView{}
	for _, app := range sortedKeys(sessions) {
		sv := sessions[app]
		sv.Permissions = uniqueSorted(sv.Permissions)
		sv.Setup = setupView{Status: StatusUnknown, Reason: "no-scenario", CoversSteps: []int{}, Preconditions: []preconditionView{}}
		best := ""
		for _, sc := range sortedKeys(scenarios[app]) {
			if best == "" || len(scenarios[app][sc]) > len(scenarios[app][best]) {
				best = sc
			}
		}
		if best != "" {
			sv.Setup.Scenario, sv.Setup.Status, sv.Setup.Reason, sv.Setup.CoversSteps = best, StatusResolved, "", scenarios[app][best]
			if f := byApp[app].file(best); f != nil {
				sv.Setup.Freshness = fresh.of(f.Anchor)
			}
		}
		seenPre := map[string]bool{}
		for _, ps := range plan {
			pf := ps.res.flow
			if pf == nil || pf.m.App != app {
				continue
			}
			// Precondition flows reach the map only through screen requirements (AMAP-V0-006).
			texts := append([]string{}, pf.f.Preconditions...)
			for _, st := range pf.f.Steps {
				if s := pf.m.screen(st.Screen); s != nil {
					for _, r := range s.Preconditions {
						if r.Flow == pf.f.ID && strings.HasPrefix(r.Text, "flow:") {
							texts = append(texts, r.Text)
						}
					}
				}
			}
			for _, t := range texts {
				if seenPre[pf.f.ID+"\x00"+t] {
					continue
				}
				seenPre[pf.f.ID+"\x00"+t] = true
				pv := preconditionView{Flow: pf.f.ID, Text: t, Status: planUnverified}
				if by := flowStep[app+"/"+t]; strings.HasPrefix(t, "flow:") && by > 0 && by < ps.Index {
					pv.SatisfiedBy = by
				}
				sv.Setup.Preconditions = append(sv.Setup.Preconditions, pv)
			}
		}
		sessionList = append(sessionList, sv)
	}

	// Plan status and gaps (AMSP-V0-005).
	status := "COMPLETE"
	gaps := []gapView{}
	for _, ps := range plan {
		if ps.Status != StepMapped {
			status = "INCOMPLETE"
			gaps = append(gaps, gapView{Index: ps.Index, Status: ps.Status, Reason: ps.Reason})
		}
	}
	stepsOut := make([]*planStep, len(plan))
	copy(stepsOut, plan)
	handoffOut := make([]handoffView, 0, len(handoffs))
	for _, h := range handoffs {
		handoffOut = append(handoffOut, *h)
	}
	mapRefs := []mapRef{}
	for _, m := range maps {
		mapRefs = append(mapRefs, mapRef{App: m.App, Revision: m.Revision, Digest: m.Digest})
	}
	head := []field{{"schema", PlanSchema}, {"maps", mapRefs}, {"evaluated_revision", fresh.evaluated}, {"budget", budget},
		{"full", o.Full}, {"authority", "candidate"}, {"status", status}, {"steps", stepsOut}, {"sessions", sessionList},
		{"handoff", handoffOut}, {"gaps", gaps}}
	if o.Draft {
		head = append(head, field{"draft", draft(plan, sessionList, handoffs, byApp, steps, fresh)})
	}
	sort.SliceStable(unknowns, func(i, j int) bool { return unknownLess(unknowns[i].(Unknown), unknowns[j].(Unknown)) })
	return render(head, []section{{"unknowns", unknowns}}, budget)
}

// reuseMethods is the step's reuse points that are methods; selector lines outside a method are
// not callable. An unreadable selector has no reuse.
func reuseMethods(st Step) []string {
	out := []string{}
	if st.Selector == nil || st.Selector.Strength == strengthUnknown {
		return out
	}
	for _, r := range st.Reuse {
		if strings.HasPrefix(r, "method:") {
			out = append(out, r)
		}
	}
	return out
}

// jsIdent turns an app name into a lower-camel JavaScript identifier.
func jsIdent(app string) string {
	var b strings.Builder
	upper := false
	for _, r := range app {
		switch {
		case r >= 'a' && r <= 'z' || r >= '0' && r <= '9':
			if upper && b.Len() > 0 {
				r = unicode.ToUpper(r)
			}
			b.WriteRune(r)
			upper = false
		default:
			upper = true
		}
	}
	s := b.String()
	if s == "" || (s[0] >= '0' && s[0] <= '9') {
		s = "app" + strings.ToUpper(s[:min(len(s), 1)]) + s[min(len(s), 1):]
	}
	return s
}

// templateParts splits a printed URL template into literal text and `{param}` names.
func templateParts(url string) (lits []string, params []string) {
	rest := url
	for {
		i := strings.IndexByte(rest, '{')
		j := strings.IndexByte(rest[max(i, 0):], '}')
		if i < 0 || j < 0 {
			lits = append(lits, rest)
			return lits, params
		}
		lits = append(lits, rest[:i])
		params = append(params, rest[i+1:i+j])
		rest = rest[i+j+1:]
	}
}

// paramKey names one route parameter of one app in the draft's params map; handoff never crosses
// apps, so two apps with a parameter of the same name keep separate values.
func paramKey(app, param string) string { return app + ":" + param }

// urlExpr is a JavaScript expression for a URL template of app, every parameter read through
// param().
func urlExpr(app, url string) string {
	lits, params := templateParts(url)
	if len(params) == 0 {
		return quote(url)
	}
	esc := strings.NewReplacer("\\", "\\\\", "`", "\\`", "$", "\\$")
	var b strings.Builder
	b.WriteByte('`')
	for i, l := range lits {
		b.WriteString(esc.Replace(l))
		if i < len(params) {
			b.WriteString("${param(" + quote(paramKey(app, params[i])) + ")}")
		}
	}
	b.WriteByte('`')
	return b.String()
}

// urlRegex is a JavaScript regular-expression literal matching a URL template; capture names the
// parameter to capture, if any.
func urlRegex(url, capture string) string {
	lits, params := templateParts(url)
	var b strings.Builder
	b.WriteByte('/')
	for i, l := range lits {
		b.WriteString(strings.ReplaceAll(regexp.QuoteMeta(l), "/", `\/`))
		if i < len(params) {
			if params[i] == capture {
				b.WriteString(`([^/?#]+)`)
			} else {
				b.WriteString(`[^/?#]+`)
			}
		}
	}
	b.WriteString(`(?:[?#]|$)/`)
	return b.String()
}

var commentSafe = strings.NewReplacer("\r", " ", "\n", " ", " ", " ", " ", " ")

// draft writes the Playwright skeleton: one browser context per app, one test.step per request
// step, reused page-object calls, guarded navigation, captured handoff parameters and each named
// outcome assertion as a TODO. A step that is not MAPPED throws instead of running (AMSP-V0-008).
func draft(plan []*planStep, sessions []*sessionView, handoffs []*handoffView, byApp map[string]*Map, steps []string, fresh *freshness) draftView {
	sum := sha256.Sum256([]byte(strings.Join(steps, "\x00")))
	dir := ""
	for _, ps := range plan {
		if ps.Status == StepMapped && ps.Spec != nil {
			dir = path.Dir(ps.Spec.File)
			break
		}
	}
	if dir == "" {
		for _, sv := range sessions {
			for _, f := range byApp[sv.App].Files {
				if f.Role == roleSpec && dir == "" {
					dir = path.Dir(f.Path)
				}
			}
		}
	}
	dv := draftView{Imports: []string{`import { test, expect } from "@playwright/test";`}, Lines: []string{}}
	if dir != "" {
		dv.ProposedPath = path.Join(dir, "plan-"+hex.EncodeToString(sum[:6])+".spec.ts")
	}
	ident := map[string]string{}
	owner := map[string]string{"test": "-", "expect": "-", "browser": "-", "params": "-", "param": "-", "routeParam": "-"}
	for _, sv := range sessions {
		owner[sv.Page], owner[sv.ident+"Context"] = "-", "-"
		ident[sv.App] = sv.ident
	}
	// Page-object classes the MAPPED steps call: one import per class, one variable per app and file.
	vars := map[string]string{} // app + "\x00" + file -> variable
	blocked := map[string]bool{}
	type inst struct{ app, file, class, v string }
	insts := []inst{}
	for _, ps := range plan {
		if ps.Status != StepMapped {
			continue
		}
		for j := range ps.Actions {
			f := reuseTarget(ps, j)
			if f == nil {
				continue
			}
			key := ps.App + "\x00" + f.Path
			if _, seen := vars[key]; seen || blocked[key] {
				continue
			}
			v := ident[ps.App] + f.Class
			if !namedExport(byApp[ps.App], f, fresh) {
				blocked[key] = true
				dv.Imports = append(dv.Imports, "// UNRESOLVED import { "+commentSafe.Replace(f.Class)+" }: no FRESH named import of it from "+commentSafe.Replace(f.Path)+" in the suite")
				continue
			}
			if o, taken := owner[f.Class]; (taken && o != f.Path) || owner[v] != "" || !jsName.MatchString(f.Class) {
				blocked[key] = true
				dv.Imports = append(dv.Imports, "// UNRESOLVED import { "+f.Class+" } collides with an existing binding")
				continue
			}
			if owner[f.Class] == "" {
				owner[f.Class] = f.Path
				if spec, ok := relSpecifier(dir, f.Path); ok && dir != "" {
					dv.Imports = append(dv.Imports, "import { "+f.Class+" } from "+quote(spec)+";")
				} else {
					dv.Imports = append(dv.Imports, "// UNRESOLVED import { "+f.Class+" } from <no proposed path>;")
				}
			}
			owner[v], vars[key] = f.Path, v
			insts = append(insts, inst{ps.App, f.Path, f.Class, v})
		}
	}
	title := []string{}
	for _, ps := range plan {
		title = append(title, ps.Request)
	}
	t := strings.Join(title, "; ")
	if len(t) > 120 {
		t = t[:120]
		for !utf8.ValidString(t) {
			t = t[:len(t)-1]
		}
	}
	// Every draft line stays one line: code text is quoted, so a line terminator can only come
	// from repository text interpolated into a comment, where it would end the comment.
	L := func(format string, args ...any) {
		dv.Lines = append(dv.Lines, commentSafe.Replace(fmt.Sprintf(format, args...)))
	}
	L("test(%s, async ({ browser }) => {", quote("plan: "+t))
	L(`  test.fixme(true, "draft skeleton: write every named outcome assertion and resolve each TODO, then remove this line");`)
	L(`  const params = new Map<string, string>();`)
	L("  const param = (name: string): string => { const v = params.get(name); if (v === undefined) throw new Error(`unbound route parameter ${name}`); return v; };")
	for _, sv := range sessions {
		id := sv.ident
		L("  // session %s: one browser context reused by steps %s; requires permissions %s", sv.App, ints(sv.Steps), orNone(sv.Permissions))
		if sv.Setup.Scenario != "" {
			L("  // setup %s: scenario %s (covers steps %s); bind route parameters and preconditions through it or an API client", sv.App, sv.Setup.Scenario, ints(sv.Setup.CoversSteps))
		}
		for _, pv := range sv.Setup.Preconditions {
			if pv.SatisfiedBy == 0 {
				L("  // TODO precondition (%s): %s", pv.Flow, commentSafe.Replace(pv.Text))
			}
		}
		L("  const %sContext = await browser.newContext(); // TODO storageState for permissions: %s", id, orNone(sv.Permissions))
		L("  const %s = await %sContext.newPage();", sv.Page, id)
		for _, in := range insts {
			if in.app == sv.App {
				L("  const %s = new %s(%s);", in.v, in.class, sv.Page)
			}
		}
	}
	for _, h := range handoffs {
		if h.Source == "setup" {
			L("  params.set(%s, \"TODO\"); // bind from setup for steps %s", quote(paramKey(h.App, h.Param)), ints(h.ConsumedBy))
		}
	}
	for _, ps := range plan {
		L("  await test.step(%s, async () => {", quote(fmt.Sprintf("%d. %s", ps.Index, ps.Request)))
		if ps.Status != StepMapped {
			for _, ex := range ps.Exploration {
				L("    // %s %s: %s", ps.Status, ex.Need, commentSafe.Replace(ex.Detail))
			}
			L("    throw new Error(%s);", quote(fmt.Sprintf("%s step %d: %s", ps.Status, ps.Index, ps.Reason)))
			L("  });")
			continue
		}
		page := sessions[0].Page
		for _, sv := range sessions {
			if sv.App == ps.App {
				page = sv.Page
			}
		}
		L("    // %s [%s]; closest spec %s", ps.Flow, ps.Confidence, specRefText(ps.Spec))
		for j, av := range ps.Actions {
			switch av.Navigation {
			case "goto":
				L("    await %s.goto(%s);", page, urlExpr(ps.App, av.URL))
			case "stay":
				L("    await expect(%s).toHaveURL(%s); // stay: no re-navigation", page, urlRegex(av.URL, ""))
			case "follow":
				L("    await %s.waitForURL(%s);", page, urlRegex(av.URL, ""))
				for _, h := range handoffs {
					if h.Producer == ps.Index && h.Capture == av.URL && len(h.ConsumedBy) > 0 {
						L("    params.set(%s, routeParam(%s.url(), %s)); // handoff to steps %s", quote(paramKey(h.App, h.Param)), page, urlRegex(av.URL, h.Param), ints(h.ConsumedBy))
					}
				}
			}
			L("    // %s: %s [%s]", av.Step, commentSafe.Replace(av.Action), av.Screen)
			if f := reuseTarget(ps, j); f != nil && !blocked[ps.App+"\x00"+f.Path] {
				me := ps.methods[j][0]
				L("    await %s.%s(); // reuse %s [%s; %s]", vars[ps.App+"\x00"+f.Path], me.Name, av.Methods[0].Ref, av.Selector.Strength, av.Methods[0].Verification)
				continue
			}
			if len(av.Methods) > 0 && ps.methods[j][0] != nil && !ps.methods[j][0].Callable {
				L("    // reuse %s is not a public method callable without arguments; not called", av.Methods[0].Ref)
			}
			switch {
			case av.Selector == nil:
				L("    // TODO: no locator declared for this step")
			case av.Selector.Kind == "test-id":
				L("    await %s.getByTestId(%s); // TODO: perform %s [%s; %s]", page, quote(av.Selector.Value), quote(av.Action), av.Selector.Strength, av.Selector.Verification)
			default:
				L("    await %s.getByRole(%s, { name: %s }); // TODO: perform %s [%s; %s]", page, quote(av.Selector.Value), quote(av.Selector.Name), quote(av.Action), av.Selector.Strength, av.Selector.Verification)
			}
		}
		for _, oc := range ps.Asserts {
			L("    // TODO assert outcome %s: %s %s %s (%s)", commentSafe.Replace(oc.ID), commentSafe.Replace(oc.Matcher), quote(oc.Locator), quote(oc.Value), commentSafe.Replace(oc.Behavior))
		}
		L("  });")
	}
	for _, sv := range sessions {
		L("  await %sContext.close();", sv.ident)
	}
	L("});")
	L("")
	L("function routeParam(url: string, pattern: RegExp): string {")
	L("  const m = url.match(pattern);")
	L("  if (!m) throw new Error(`route parameter not found in ${url}`);")
	L("  return m[1];")
	L("}")
	return dv
}

var jsName = regexp.MustCompile(`^[A-Za-z_$][A-Za-z0-9_$]*$`)

// reuseTarget is the page-object or workflow class file whose first shown method performs action j,
// when that method is FRESH and Callable; nil otherwise.
func reuseTarget(ps *planStep, j int) *TestFile {
	if j >= len(ps.methods) || len(ps.methods[j]) == 0 || ps.methods[j][0] == nil || len(ps.Actions[j].Methods) == 0 || ps.Actions[j].Selector == nil {
		return nil
	}
	f := ps.files[j][0]
	if f == nil || f.Class == "" || (f.Role != rolePageObject && f.Role != roleWorkflow) || ps.Actions[j].Methods[0].Freshness != Fresh ||
		!ps.methods[j][0].Callable {
		return nil
	}
	return f
}

// importersOf lists the map files with a resolved import of path.
func importersOf(m *Map, path string) []*TestFile {
	out := []*TestFile{}
	for i := range m.Files {
		for _, imp := range m.Files[i].Imports {
			if imp.Status == importResolved && imp.Resolved == path {
				out = append(out, &m.Files[i])
				break
			}
		}
	}
	return out
}

// namedExport reports whether the suite binds f's class by name: some file with a resolved, non-type
// import of f.Path lists exactly f.Class between braces, and both that file and f are FRESH at the
// evaluated revision, so the map's import statement still describes the code (AMSP-V0-008).
func namedExport(m *Map, f *TestFile, fresh *freshness) bool {
	if fresh.of(f.Anchor) != Fresh {
		return false
	}
	for _, tf := range importersOf(m, f.Path) {
		if fresh.of(tf.Anchor) != Fresh {
			continue
		}
		for _, imp := range tf.Imports {
			if imp.Status != importResolved || imp.Resolved != f.Path || strings.HasPrefix(strings.TrimSpace(imp.Statement), "import type") {
				continue
			}
			i, j := strings.IndexByte(imp.Statement, '{'), strings.IndexByte(imp.Statement, '}')
			if i < 0 || j < i {
				continue
			}
			for _, name := range strings.Split(imp.Statement[i+1:j], ",") {
				if strings.TrimSpace(name) == f.Class {
					return true
				}
			}
		}
	}
	return false
}

func specRefText(s *specRef) string {
	if s == nil {
		return "UNKNOWN no-asserting-spec"
	}
	return s.Ref
}

func orNone(in []string) string {
	if len(in) == 0 {
		return "none declared"
	}
	return strings.Join(in, ", ")
}

func ints(in []int) string {
	out := make([]string, len(in))
	for i, v := range in {
		out[i] = fmt.Sprint(v)
	}
	return strings.Join(out, ", ")
}
