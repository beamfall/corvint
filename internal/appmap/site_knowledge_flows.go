package appmap

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/appflows"
	"github.com/Beamfall/corvint/internal/doccorpus"
	"github.com/Beamfall/corvint/internal/gokernel"
)

// Stage 5 flow inference (SKH-V0-050..054, SKH-V0-120..127). Step-IR sequences of the declared
// tests cluster into candidate flows by start and end screen, and into variations by their
// action sequence. Candidates are proposals with learned authority; they become intents only when
// the operator runs `flows import` on the emitted behavior-adapter request (AFU-V1-004).

// FlowsSchema is the profile of `flows appmap infer`.
const FlowsSchema = "site-knowledge-flows/0"

// Candidate states (SKH-V0-121, SKH-V0-123).
const (
	CandidateProposed   = "proposed"
	CandidateBlocked    = "blocked"
	CandidateIncomplete = "incomplete"
)

// Trust ranks of a source test (SKH-V0-052, SKH-V0-123).
const (
	rankCandidate = iota
	rankRunUnreviewed
	rankUnreviewed
)

var trustRankNames = []string{"candidate", "run-unreviewed", "unreviewed"}

// TestTrust is one joined DCP-V1 test-knowledge record (DCP-V1-089, DCP-V1-090) reduced to what
// flow inference honours: the test anchor it joins on and its labelled verdict and selection.
type TestTrust struct {
	ID         string `json:"id"`
	Path       string `json:"path"`
	Start      int    `json:"start"`
	End        int    `json:"end"`
	TrustLevel string `json:"trust_level"`
	Verdict    string `json:"verdict"`
	Selection  string `json:"selection"`
	Gap        string `json:"gap,omitempty"`
}

// SKFlowSource is one test a candidate cites, with its labelled trust (never promoted).
type SKFlowSource struct {
	Test       string `json:"test"`
	Title      string `json:"title"`
	Path       string `json:"path"`
	Record     string `json:"record"`
	Rank       string `json:"rank"`
	TrustLevel string `json:"trust_level,omitempty"`
	Verdict    string `json:"verdict,omitempty"`
	Selection  string `json:"selection,omitempty"`
	Knowledge  string `json:"knowledge,omitempty"`
	Complete   bool   `json:"complete"`
}

// SKFlowStep is one action of a variation: a page-object method run or a single step.
type SKFlowStep struct {
	Key    string   `json:"key"`
	Action string   `json:"action"`
	Kinds  []string `json:"kinds"`
	Record string   `json:"record,omitempty"`
}

// SKFlowOutcome is one assertion every source test of its scope makes after its last action.
type SKFlowOutcome struct {
	Matcher  string `json:"matcher"`
	Locator  string `json:"locator,omitempty"`
	Expected string `json:"expected,omitempty"`
	Target   string `json:"target,omitempty"`
	// Slotted marks an expected value that interpolates an unbound parameter slot: it is not a
	// literal and is never emitted (SKH-V0-125).
	Slotted bool `json:"slotted,omitempty"`
}

// SKFlowConflict names the tests of a variation that diverge from the primary at one step.
type SKFlowConflict struct {
	Variation string   `json:"variation"`
	At        int      `json:"at"`
	Step      string   `json:"step"`
	Tests     []string `json:"tests"`
}

// SKVariation is one distinct action sequence within a candidate flow.
type SKVariation struct {
	ID       string          `json:"id"`
	Steps    []SKFlowStep    `json:"steps"`
	Outcomes []SKFlowOutcome `json:"outcomes"`
	Support  int             `json:"support"`
	Sources  []SKFlowSource  `json:"sources"`
}

// SKCandidateFlow is one proposed flow (SKH-V0-050, SKH-V0-053).
type SKCandidateFlow struct {
	ID            string           `json:"id"`
	FlowID        string           `json:"flow_id"`
	Status        string           `json:"status"`
	Start         string           `json:"start"`
	End           string           `json:"end"`
	Steps         []SKFlowStep     `json:"steps"`
	Outcomes      []SKFlowOutcome  `json:"outcomes"`
	OutcomeStatus string           `json:"outcome_status"`
	Support       int              `json:"support"`
	SingleSource  bool             `json:"single_source,omitempty"`
	Variations    []SKVariation    `json:"variations"`
	Conflicts     []SKFlowConflict `json:"conflicts"`
	// Emit says whether --emit request carries the candidate, or why not (SKH-V0-125).
	Emit      string   `json:"emit"`
	BlockedBy []string `json:"blocked_by,omitempty"`
	Gaps      []string `json:"gaps,omitempty"`
	Authority string   `json:"authority"`
	Anchor    Anchor   `json:"anchor"`
}

// SKUnclustered is a test no candidate uses, with the closed reason.
type SKUnclustered struct {
	Test   string `json:"test"`
	Path   string `json:"path"`
	Reason string `json:"reason"`
	Detail string `json:"detail,omitempty"`
}

type skSegment struct {
	step SKFlowStep
	last int // index of its last step in the test
}

type skFlowTest struct {
	t        SKTest
	segments []skSegment
	outcomes []SKFlowOutcome
	start    string
	end      string
	rank     int
	trust    []*TestTrust
}

// skActionLabel is a short, value-free description of one step for an intent action.
func skActionLabel(s SKStep) string {
	parts := []string{s.Kind}
	switch {
	case s.Locator != nil && s.Locator.Name != "":
		parts = append(parts, s.Locator.Kind+"="+s.Locator.Value+" name="+s.Locator.Name)
	case s.Locator != nil:
		parts = append(parts, s.Locator.Kind+"="+s.Locator.Value)
	case s.LocatorSlot != "":
		parts = append(parts, "${"+s.LocatorSlot+"}")
	}
	if s.Kind == StepVisit && s.Target != "" && !s.Screened {
		parts = append(parts, s.Target)
	}
	if s.Kind == StepOpaque {
		parts = append(parts, s.Reason)
	}
	return strings.Join(parts, " ")
}

// skVisitScreen resolves a visit step's literal target through the screen index.
func skVisitScreen(idx *screenIndex, s SKStep) (string, string) {
	if s.Kind != StepVisit || s.Target == "" || s.Screened {
		return "", JoinDetailDynamic
	}
	url := s.Target
	for _, slot := range s.Slots {
		url = strings.ReplaceAll(url, "${"+slot+"}", substMark)
	}
	return idx.byURL(url)
}

// skRecordID is the stage 2 record identity of one step sequence (SKH-V0-022).
func skRecordID(source string, steps []SKStep) string {
	keys := make([]string, len(steps))
	for i, s := range steps {
		keys[i] = stepKey(s)
	}
	return "step-record:" + skDigest(append([]string{source}, keys...)...)
}

// flowTest reads one test into action segments, closing outcomes and its screens.
func (k *SKModel) flowTest(t SKTest, idx *screenIndex, records map[string]bool) (skFlowTest, string, string) {
	ft := skFlowTest{t: t, outcomes: []SKFlowOutcome{}}
	lastAction := -1
	for i, s := range t.Steps {
		if s.Kind != StepAssert && s.Kind != StepWaitFor {
			lastAction = i
		}
	}
	// Screens come from every visit, including those inside a page-object run (SKH-V0-121), and
	// from each action run through a page object joined to a screen (SKH-V0-185).
	firstVisit := true
	for i, s := range t.Steps {
		if s.Kind != StepVisit {
			if ps := k.stepScreen(s); ps != "" && !firstVisit && i <= lastAction && s.Kind != StepAssert && s.Kind != StepWaitFor {
				ft.end = ps
			}
			continue
		}
		id, why := skVisitScreen(idx, s)
		if firstVisit {
			if id == "" {
				return ft, "start-screen-unresolved", why
			}
			ft.start = id
			firstVisit = false
		}
		ft.end = id
	}
	for i := 0; i < len(t.Steps); i++ {
		s := t.Steps[i]
		if i > lastAction {
			if s.Kind == StepAssert {
				o := SKFlowOutcome{Matcher: s.Matcher, Expected: s.Expected, Target: s.Target, Slotted: len(s.Slots) > 0}
				if s.Locator != nil {
					// The full selector identity: a name distinguishes two locators of one role (SKH-V0-140).
					o.Locator = s.Locator.Kind + "=" + s.Locator.Value
					if s.Locator.Name != "" {
						o.Locator += " name=" + s.Locator.Name
					}
				}
				ft.outcomes = append(ft.outcomes, o)
			}
			continue
		}
		if s.Kind == StepAssert || s.Kind == StepWaitFor {
			continue
		}
		if len(s.Via) > 0 {
			j := i
			for j+1 <= lastAction && len(t.Steps[j+1].Via) > 0 && t.Steps[j+1].Via[0] == s.Via[0] {
				j++
			}
			run := t.Steps[i : j+1]
			kinds := []string{}
			for _, r := range run {
				kinds = appendUnique(kinds, r.Kind)
			}
			name := s.Via[0]
			if h := strings.LastIndex(name, "#"); h >= 0 {
				name = name[h+1:]
			}
			seg := SKFlowStep{Key: "run:" + skDigest(s.Via[0], skRecordID(s.Via[0], run)), Action: "call " + name, Kinds: kinds}
			if id := skRecordID(s.Via[0], run); records[id] {
				seg.Record = id
			}
			ft.segments = append(ft.segments, skSegment{step: seg, last: j})
			i = j
			continue
		}
		ft.segments = append(ft.segments, skSegment{step: SKFlowStep{Key: "step:" + skDigest(stepKey(s)), Action: skActionLabel(s), Kinds: []string{s.Kind}}, last: i})
	}
	if firstVisit {
		return ft, "no-start-screen", ""
	}
	if ft.end == "" {
		return ft, "end-screen-unresolved", ""
	}
	return ft, "", ""
}

// joinTrust returns every joined test-knowledge record whose test anchor shares the test's path
// and overlaps its span, ordered by ID. Each one's verdict and selection apply (SKH-V0-123).
func joinTrust(t SKTest, trust []TestTrust) []*TestTrust {
	out := []*TestTrust{}
	for i := range trust {
		tr := &trust[i]
		if tr.Path == t.Path && tr.Start <= t.Anchor.End && t.Anchor.Start <= tr.End {
			out = append(out, tr)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func intersectOutcomes(lists [][]SKFlowOutcome) []SKFlowOutcome {
	if len(lists) == 0 {
		return []SKFlowOutcome{}
	}
	out := []SKFlowOutcome{}
	for _, o := range lists[0] {
		all := true
		for _, l := range lists[1:] {
			all = all && slices.Contains(l, o)
		}
		if all && !slices.Contains(out, o) {
			out = append(out, o)
		}
	}
	return out
}

// inferFlows clusters the model's tests into candidate flows (SKH-V0-050, SKH-V0-120..124).
func (k *SKModel) inferFlows(m *Map, trust []TestTrust) ([]SKCandidateFlow, []SKUnclustered) {
	idx := newScreenIndex(m.HashPrefix, m.BasePath, m.Screens)
	records := map[string]bool{}
	for _, r := range k.Records {
		records[r.ID] = true
	}
	groups := map[string][]skFlowTest{}
	unclustered := []SKUnclustered{}
	for _, t := range k.Tests {
		ft, why, detail := k.flowTest(t, idx, records)
		ft.trust = joinTrust(t, trust)
		ft.rank = rankUnreviewed
		unrelated := ""
		for _, tr := range ft.trust {
			switch {
			case tr.Verdict == "unrelated":
				if unrelated == "" {
					unrelated = tr.ID
				}
			case tr.Selection == "candidate":
				ft.rank = rankCandidate
			case tr.TrustLevel == "run-unreviewed":
				ft.rank = min(ft.rank, rankRunUnreviewed)
			}
		}
		// Any unrelated verdict removes the test, whatever else is joined to it.
		if unrelated != "" {
			unclustered = append(unclustered, SKUnclustered{Test: t.ID, Path: t.Path, Reason: "unrelated-verdict", Detail: unrelated})
			continue
		}
		if why != "" {
			unclustered = append(unclustered, SKUnclustered{Test: t.ID, Path: t.Path, Reason: why, Detail: detail})
			continue
		}
		key := ft.start + "\x00" + ft.end
		groups[key] = append(groups[key], ft)
	}
	flows := []SKCandidateFlow{}
	for _, key := range sortedKeys(groups) {
		tests := groups[key]
		byVariation := map[string][]skFlowTest{}
		for _, ft := range tests {
			keys := []string{}
			for _, s := range ft.segments {
				keys = append(keys, s.step.Key)
			}
			byVariation[skDigest(keys...)] = append(byVariation[skDigest(keys...)], ft)
		}
		variations := []SKVariation{}
		bestRank := map[string]int{}
		blocked, gaps, allComplete := []string{}, []string{}, true
		for _, vk := range sortedKeys(byVariation) {
			vt := byVariation[vk]
			sort.SliceStable(vt, func(i, j int) bool {
				if vt[i].rank != vt[j].rank {
					return vt[i].rank < vt[j].rank
				}
				return vt[i].t.ID < vt[j].t.ID
			})
			v := SKVariation{ID: "variation:" + vk, Steps: []SKFlowStep{}, Support: len(vt), Sources: []SKFlowSource{}}
			for _, s := range vt[0].segments {
				v.Steps = append(v.Steps, s.step)
			}
			outs := [][]SKFlowOutcome{}
			bestRank[v.ID] = rankUnreviewed
			for _, ft := range vt {
				src := SKFlowSource{Test: ft.t.ID, Title: ft.t.Title, Path: ft.t.Path, Record: ft.t.Record, Rank: trustRankNames[ft.rank], Complete: ft.t.Complete}
				ids := []string{}
				for _, tr := range ft.trust {
					ids = append(ids, tr.ID)
					// A blocking record wins over any other record on the same test (fail closed).
					if tr.Selection == "blocks" {
						blocked, src.Rank = append(blocked, tr.ID), "blocked"
					}
					if tr.Verdict == "partial" && tr.Gap != "" {
						gaps = append(gaps, tr.ID+": "+tr.Gap)
					}
				}
				if len(ft.trust) > 0 {
					tr := ft.trust[0]
					for _, o := range ft.trust {
						if o.Selection == "blocks" || (tr.Selection != "blocks" && o.Selection == "candidate") {
							tr = o
						}
					}
					src.TrustLevel, src.Verdict, src.Selection, src.Knowledge = tr.TrustLevel, tr.Verdict, tr.Selection, strings.Join(ids, ",")
				}
				allComplete = allComplete && ft.t.Complete
				bestRank[v.ID] = min(bestRank[v.ID], ft.rank)
				v.Sources = append(v.Sources, src)
				outs = append(outs, ft.outcomes)
			}
			v.Outcomes = intersectOutcomes(outs)
			variations = append(variations, v)
		}
		// The primary variation is the best-trusted, then the best-supported (SKH-V0-123).
		sort.SliceStable(variations, func(i, j int) bool {
			a, b := variations[i], variations[j]
			if bestRank[a.ID] != bestRank[b.ID] {
				return bestRank[a.ID] < bestRank[b.ID]
			}
			if a.Support != b.Support {
				return a.Support > b.Support
			}
			return a.ID < b.ID
		})
		primary := variations[0]
		start, end, _ := strings.Cut(key, "\x00")
		f := SKCandidateFlow{ID: "candidate-flow:" + skDigest(start, end), Status: CandidateProposed, Start: start, End: end,
			Steps: primary.Steps, Variations: variations, Conflicts: []SKFlowConflict{}, Authority: skAuthority,
			Anchor: tests[0].t.Anchor}
		for _, ft := range tests {
			if ft.t.ID == primary.Sources[0].Test {
				f.Anchor = ft.t.Anchor
			}
		}
		f.FlowID = "sk-" + strings.TrimPrefix(f.ID, "candidate-flow:")[:12]
		vo := [][]SKFlowOutcome{}
		for _, v := range variations {
			f.Support += v.Support
			vo = append(vo, v.Outcomes)
		}
		f.SingleSource = f.Support == 1
		f.Outcomes = intersectOutcomes(vo)
		f.OutcomeStatus = StatusResolved
		if len(f.Outcomes) == 0 {
			f.OutcomeStatus = StatusUnknown
		}
		for _, v := range variations[1:] {
			at := 0
			for at < len(v.Steps) && at < len(primary.Steps) && v.Steps[at].Key == primary.Steps[at].Key {
				at++
			}
			step := "end-of-sequence"
			if at < len(v.Steps) {
				step = v.Steps[at].Action
			}
			tests := []string{}
			for _, s := range v.Sources {
				tests = append(tests, s.Test)
			}
			f.Conflicts = append(f.Conflicts, SKFlowConflict{Variation: v.ID, At: at, Step: step, Tests: tests})
		}
		if len(blocked) > 0 {
			f.Status, f.BlockedBy = CandidateBlocked, uniqueSorted(blocked)
		} else if !allComplete {
			f.Status = CandidateIncomplete
		}
		f.Gaps = nilIfEmpty(uniqueSorted(gaps))
		f.Emit = emitState(f)
		flows = append(flows, f)
	}
	sort.SliceStable(unclustered, func(i, j int) bool { return unclustered[i].Test < unclustered[j].Test })
	return flows, unclustered
}

// skValueless maps the valueless state matchers to their DCP-V1-084 canonical values.
var skValueless = map[string]string{"toBeVisible": "visible", "toBeHidden": "hidden", "toBeEnabled": "enabled", "toBeChecked": "checked"}

// outcomeValue is the value an emitted outcome states: the literal expected value, or the
// canonical state of a valueless matcher; anything else cannot be emitted.
func outcomeValue(o SKFlowOutcome) (string, bool) {
	if o.Slotted {
		return "", false
	}
	if o.Expected != "" {
		return o.Expected, true
	}
	v, ok := skValueless[o.Matcher]
	return v, ok
}

// emitState says whether a candidate enters the emitted request (SKH-V0-125): only a proposed
// candidate whose every variation has at least one emittable outcome.
func emitState(f SKCandidateFlow) string {
	if f.Status != CandidateProposed {
		return f.Status
	}
	for _, v := range f.Variations {
		ok := false
		for _, o := range v.Outcomes {
			_, good := outcomeValue(o)
			ok = ok || good
		}
		if !ok {
			return "no-emittable-outcome"
		}
	}
	return "emitted"
}

// MaxTestTrust bounds the joined test-knowledge records one inference reads (DCP-V1-092).
const MaxTestTrust = doccorpus.MaxTestKnowledge

func inferRead(ctx context.Context, m *Map, sources []string, trust []TestTrust, o Options) (*SKModel, []SKCandidateFlow, []SKUnclustered, int, int, error) {
	if len(trust) > MaxTestTrust {
		return nil, nil, nil, 0, 0, bound(fmt.Sprintf("more than %d test-knowledge joins", MaxTestTrust))
	}
	k, read, skipped, err := readSiteKnowledge(ctx, m, sources, o)
	if err != nil {
		return nil, nil, nil, 0, 0, err
	}
	flows, unclustered := k.inferFlows(m, trust)
	return k, flows, unclustered, read, skipped, nil
}

// InferFlows reports the candidate flows of the declared tests (SKH-V0-050, SKH-V0-052,
// SKH-V0-053, SKH-V0-120..124).
func InferFlows(ctx context.Context, m *Map, sources []string, trust []TestTrust, o Options) ([]byte, error) {
	budget, err := o.budget(DefaultFlowBudget)
	if err != nil {
		return nil, err
	}
	_, flows, unclustered, read, skipped, err := inferRead(ctx, m, sources, trust, o)
	if err != nil {
		return nil, err
	}
	status, single, conflicts := map[string]int{}, 0, 0
	items := make([]any, len(flows))
	for i, f := range flows {
		status[f.Status]++
		if f.SingleSource {
			single++
		}
		conflicts += len(f.Conflicts)
		items[i] = f
	}
	reasons := map[string]int{}
	rest := make([]any, len(unclustered))
	for i, u := range unclustered {
		reasons[u.Reason]++
		rest[i] = u
	}
	head := append(skHead(FlowsSchema, m, sources, budget, o, read, skipped),
		field{"test_knowledge_joins", len(trust)}, field{"candidates_total", len(flows)}, field{"candidate_status", status},
		field{"single_source", single}, field{"conflicts_total", conflicts}, field{"unclustered_total", len(unclustered)},
		field{"unclustered_reasons", reasons}, field{"import", "flows appmap infer --emit request | flows import --format behavior-adapter-request (proposed intents only)"})
	return render(head, []section{{"candidates", items}, {"unclustered", rest}}, budget)
}

// Request identities of the emitted adapter document (SKH-V0-051, SKH-V0-125).
const (
	inferProviderID = "corvint-site-knowledge"
	inferContractID = "site-knowledge-flows"
	inferReason     = "site-knowledge flow inference output; generated, not committed, nothing ran"
)

// skBlobID is the Git blob identity of data, so an emitted inline input names a well-formed blob.
func skBlobID(data []byte) string {
	h := sha1.New()
	fmt.Fprintf(h, "blob %d\x00", len(data))
	h.Write(data)
	return hex.EncodeToString(h.Sum(nil))
}

// InferFlowsRequest emits the proposed candidates as one corvint-behavior-adapter-request/1
// document on standard output (SKH-V0-051, SKH-V0-125). Blocked and incomplete candidates are
// left out. It writes nothing: `flows import` on this document writes proposed intents.
func InferFlowsRequest(ctx context.Context, m *Map, sources []string, trust []TestTrust, o Options) ([]byte, error) {
	k, flows, _, _, _, err := inferRead(ctx, m, sources, trust, o)
	if err != nil {
		return nil, err
	}
	// The request names one source repository; evidence read from an aliased root would be cited
	// under --root's identity, so it is refused rather than mislabelled (SKH-V0-189).
	for alias := range k.roots {
		if alias != "" {
			return nil, &gokernel.Error{Code: "appmap-invalid-query", Message: "--emit request reads --source from --root only; root " + alias + " is aliased"}
		}
	}
	r := repo{ctx: ctx, root: o.Root}
	rootOut, err := r.git(4096, nil, "rev-list", "--max-parents=0", "--end-of-options", m.Revision)
	if err != nil {
		return nil, err
	}
	roots := strings.Fields(string(rootOut))
	if len(roots) != 1 {
		return nil, &gokernel.Error{Code: "appmap-invalid-revision", Message: "the map revision has no single root commit to name the repository"}
	}
	source := doccorpus.Repository{ID: roots[0], Revision: m.Revision}
	revisions := doccorpus.BehaviorRevisions{App: source, E2E: source, Docs: source}
	stateOf := map[string]string{}
	for _, s := range m.Screens {
		stateOf[s.ID] = s.State
	}
	set := appflows.IntentSet{Flows: []appflows.FlowIntent{}}
	for _, f := range flows {
		if f.Emit != "emitted" {
			continue
		}
		intent, err := k.candidateIntent(f, source, stateOf)
		if err != nil {
			return nil, err
		}
		set.Flows = append(set.Flows, intent)
	}
	inventory, err := appflows.CompileInventory(set)
	if err != nil {
		return nil, err
	}
	encode := doccorpus.Encode
	migration, err := encode(doccorpus.BehaviorMigration{Revisions: revisions, Schema: 2, ContractID: inferContractID, SourceRevision: m.Revision, DocumentationRevision: m.Revision})
	if err != nil {
		return nil, err
	}
	discovery, err := encode(doccorpus.BehaviorDiscovery{Schema: "corvint-playwright-discovery/1", Mode: doccorpus.PlaywrightDiscoveryMode, Revisions: revisions, Executions: []doccorpus.BehaviorExecution{}})
	if err != nil {
		return nil, err
	}
	docs := []struct {
		id  string
		doc []byte
	}{
		{"application-flows", inventory}, {"candidates", []byte(`{"candidates":[]}`)}, {"discovery", discovery},
		{"migration", migration}, {"tests", []byte(`{"tests":[]}`)},
	}
	inputs := []doccorpus.BehaviorAdapterInput{}
	for _, d := range docs {
		digest := doccorpus.Digest(d.doc)
		lines := strings.Count(string(d.doc), "\n")
		if lines == 0 || !strings.HasSuffix(string(d.doc), "\n") {
			lines++
		}
		a := doccorpus.Anchor{Repository: source.ID, Revision: m.Revision, Path: "site-knowledge/" + d.id + ".json", Blob: skBlobID(d.doc),
			SHA256: digest, Start: 1, End: lines, SpanSHA256: digest, Authority: "external-provider", Kind: "imported", Reason: inferReason}
		inputs = append(inputs, doccorpus.BehaviorAdapterInput{ID: d.id, Anchor: a, Document: string(d.doc)})
	}
	fields := func(names ...string) map[string]string {
		out := map[string]string{}
		for _, n := range names {
			out[n] = "/" + n
		}
		return out
	}
	flowFields := fields("id", "derivation", "evidence", "required_pages", "negative_controls", "ordered_events", "missing_e2e_review")
	variationFields := fields("flow", "preconditions", "actions", "observable_facts", "expected_outcomes", "projects", "tests")
	variationFields["id"] = "/variation_id"
	request := doccorpus.BehaviorAdapterRequest{
		Schema: doccorpus.BehaviorAdapterRequestSchema, ProviderID: inferProviderID, ProviderVersion: "0", ContractID: inferContractID,
		Source: source, Revisions: revisions, SourceRevision: m.Revision, DocumentationRevision: m.Revision,
		MigrationInput: "migration", DiscoveryInput: "discovery", Inputs: inputs,
		Mappings: []doccorpus.BehaviorAdapterMapping{
			{Kind: "candidates", Input: "candidates", Records: "/candidates", Fields: fields("id", "evidence", "flows")},
			{Kind: "flows", Input: "application-flows", Records: "/flows", Fields: flowFields},
			{Kind: "tests", Input: "tests", Records: "/tests", Fields: fields("id", "project", "title", "evidence", "flows", "criteria", "assertions", "variation_claims")},
			{Kind: "variations", Input: "application-flows", Records: "/variations", Fields: variationFields},
		},
		Observations: []doccorpus.ObservationLink{},
	}
	out, err := encode(request)
	if err != nil {
		return nil, err
	}
	if len(out) > doccorpus.MaxBytes {
		return nil, bound(fmt.Sprintf("the emitted request exceeds %d bytes", doccorpus.MaxBytes))
	}
	if _, err := doccorpus.BuildBehaviorAdapter(out, nil); err != nil {
		return nil, &gokernel.Error{Code: "appmap-invalid-query", Message: "behavior adapter refused the inferred request: " + err.Error()}
	}
	return out, nil
}

// candidateIntent maps one proposed candidate to a proposed AFU-V1 intent: its variations' actions,
// the outcomes every source asserts, and inferred test links (SKH-V0-125).
func (k *SKModel) candidateIntent(f SKCandidateFlow, source doccorpus.Repository, stateOf map[string]string) (appflows.FlowIntent, error) {
	intent := appflows.FlowIntent{Schema: appflows.FlowIntentSchema, FlowID: f.FlowID, Revision: 1, Proposed: true, Kind: "ui", Actor: "unknown",
		Preconditions: []string{}, Steps: []appflows.FlowStep{}, Outcomes: []appflows.FlowOutcome{}, Variations: []appflows.FlowVariation{}, Links: []appflows.FlowLink{}}
	file := k.files[f.Anchor.Path]
	if file == nil {
		return intent, fmt.Errorf("candidate %s cites an unread file", f.ID)
	}
	intent.Adapter = &appflows.FlowAdapter{Derivation: "source-derived",
		Evidence: doccorpus.Anchor{Repository: source.ID, Revision: source.Revision, Path: f.Anchor.Path, Blob: f.Anchor.Blob,
			SHA256: doccorpus.Digest(file.data), Start: f.Anchor.Start, End: f.Anchor.End, SpanSHA256: f.Anchor.SpanSHA256,
			Authority: "external-provider", Kind: "declared", Reason: "mined from the cited test; learned, proposed"},
		RequiredPages: uniqueSorted([]string{stateOf[f.Start], stateOf[f.End]}), NegativeControls: []string{}, OrderedEvents: []doccorpus.BehaviorEvent{}}
	steps, outcomes := map[string]string{}, map[SKFlowOutcome]string{}
	for i, v := range f.Variations {
		iv := appflows.FlowVariation{VariationID: f.FlowID + ".v" + strconv.Itoa(i+1), Preconditions: []string{}, Steps: []string{},
			ObservableFacts: []string{}, Outcomes: []string{}, Projects: []string{"unknown"}}
		seen := map[string]int{}
		for _, s := range v.Steps {
			// A repeated action is numbered: the adapter requires unique actions per variation.
			action := s.Action
			if seen[s.Action]++; seen[s.Action] > 1 {
				action += " #" + strconv.Itoa(seen[s.Action])
			}
			if steps[action] == "" {
				steps[action] = "s" + strconv.Itoa(len(steps)+1)
				intent.Steps = append(intent.Steps, appflows.FlowStep{StepID: steps[action], Action: action})
			}
			iv.Steps = append(iv.Steps, steps[action])
		}
		for _, o := range v.Outcomes {
			value, ok := outcomeValue(o)
			if !ok {
				continue
			}
			locator := o.Locator
			if locator == "" {
				locator = "page"
			}
			behavior := strings.TrimSpace(strings.Join([]string{o.Matcher, locator, o.Target, value}, " "))
			if outcomes[o] == "" {
				outcomes[o] = "o" + strconv.Itoa(len(outcomes)+1)
				intent.Outcomes = append(intent.Outcomes, appflows.FlowOutcome{OutcomeID: outcomes[o], Behavior: behavior, Matcher: o.Matcher, Locator: locator, Value: value})
			}
			iv.Outcomes = append(iv.Outcomes, outcomes[o])
			iv.ObservableFacts = appendUnique(iv.ObservableFacts, behavior)
		}
		for _, s := range v.Sources {
			intent.Links = append(intent.Links, appflows.FlowLink{From: iv.VariationID, Basis: "inferred",
				Target: appflows.LinkTarget{Type: "test", TestKey: s.Path + " > " + s.Title, Path: s.Path}})
		}
		intent.Variations = append(intent.Variations, iv)
	}
	return intent, nil
}
