package doccorpus

import (
	"context"
	"maps"
	"slices"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/contextindex"
)

// OperationInput is shared by CLI, MCP and indexed companions. The native
// Query function retains compatibility with older permissive pure callers.
func OperationInput(op string) string {
	switch op {
	case "search", "concept", "vocabulary":
		return "query"
	case "locate", "claims", "recommend-tests":
		return "path"
	case "get", "trace", "related", "journey", "stability", "flow", "dependencies", "navigation", "intent":
		return "id"
	case "info", "validate", "inventory", "coverage", "gaps":
		return ""
	}
	return "unsupported"
}
func typedCapability(op string) string {
	switch op {
	case "concept", "flow", "navigation", "vocabulary", "intent":
		return "subjects"
	case "claims":
		return "claims"
	case "dependencies", "recommend-tests":
		return "relations"
	}
	return ""
}

type DependencyEdge struct {
	Direction string   `json:"direction"`
	Relation  Relation `json:"relation"`
}
type NavigationSelector struct {
	ID         string `json:"id"`
	Flow       string `json:"flow"`
	Test       string `json:"test"`
	Locator    string `json:"locator"`
	Annotation Anchor `json:"annotation"`
	Trust      string `json:"trust"`
}
type TestRecommendation struct {
	State            string   `json:"state"`
	NarrowingAllowed bool     `json:"narrowing_allowed"`
	Required         string   `json:"required"`
	MissingEvidence  []string `json:"missing_evidence"`
	Paths            []string `json:"paths"`
	BlockedBy        []string `json:"blocked_by,omitempty"`
	// BlockedByCount and BlockedByTruncated keep a bounded blocked_by list explicit (DCP-V1-091).
	BlockedByCount     int  `json:"blocked_by_count,omitempty"`
	BlockedByTruncated bool `json:"blocked_by_truncated,omitempty"`
}

// MaxBlockedBy bounds the defect IDs a test recommendation lists by name.
const MaxBlockedBy = 32

func addRecord(r *Receipt, v any) {
	r.Results = append(r.Results, v)
	r.Citations = append(r.Citations, recordAnchors(v)...)
}
func indexedSubject(a *Artifact, id string) (Subject, bool) {
	ref, ok := a.RuntimeIndex.IDs[id]
	if !ok || ref.Kind != "subject" {
		return Subject{}, false
	}
	return a.Subjects[ref.Offset], true
}
func typedQuery(ctx context.Context, a *Artifact, q Request, r *Receipt) error {
	x := a.RuntimeIndex
	switch q.Operation {
	case "concept", "vocabulary":
		if err := search(ctx, a, q, r); err != nil {
			return err
		}
		selected := map[string]bool{}
		for _, v := range r.Results {
			s := v.(Subject)
			if q.Operation == "concept" || s.Kind == "business_term" {
				selected[s.ID] = true
			}
		}
		r.Results = nil
		for id := range selected {
			s, _ := indexedSubject(a, id)
			addRecord(r, s)
			for _, i := range x.Edges[id] {
				if err := ctx.Err(); err != nil {
					return err
				}
				rel := a.Relations[i]
				addRecord(r, rel)
				for _, target := range []string{rel.From, rel.To} {
					if s, ok := indexedSubject(a, target); ok && s.Kind == "flow" {
						addRecord(r, s)
					}
				}
			}
		}
		for _, s := range a.Subjects {
			if err := ctx.Err(); err != nil {
				return err
			}
			if flow := a.Details[s.ID].Flow; flow != nil {
				for _, id := range flow.Entrypoints {
					if selected[id] {
						addRecord(r, s)
						break
					}
				}
			}
		}
	case "claims":
		if q.Query != "" {
			// DCP-V1-313: a claim text query is ranked, so it returns
			// before the identity sort below.
			return claimSearch(ctx, a, q, r)
		}
		if q.State != "" {
			if err := suppliedStatusClaims(ctx, a, q, r); err != nil {
				return err
			}
			break
		}
		for _, ref := range x.Paths[q.Path] {
			if err := ctx.Err(); err != nil {
				return err
			}
			if ref.Kind == "claim" && claimKindAdmitted(a, q, a.Claims[ref.Offset].ID) {
				addRecord(r, a.indexedRecord(ref))
			}
		}
		for _, j := range testKnowledgeOnPath(a, q.Path) {
			addRecord(r, j)
		}
	case "flow":
		if err := flowQuery(ctx, a, q, r); err != nil {
			return err
		}
	case "dependencies":
		for _, direction := range []string{"upstream", "downstream"} {
			seen := map[string]bool{q.ID: true}
			edges := map[string]bool{}
			queue := []string{q.ID}
			work := 0
			for len(queue) > 0 {
				if err := ctx.Err(); err != nil {
					return err
				}
				id := queue[0]
				queue = queue[1:]
				for _, i := range x.Edges[id] {
					work++
					if work > MaxIndexPostings {
						return fail("dependency traversal bound exceeded")
					}
					if err := ctx.Err(); err != nil {
						return err
					}
					rel := a.Relations[i]
					if rel.Type != "depends_on" {
						continue
					}
					next := ""
					if direction == "downstream" && rel.From == id {
						next = rel.To
					}
					if direction == "upstream" && rel.To == id {
						next = rel.From
					}
					if next == "" {
						continue
					}
					if !edges[rel.ID] {
						edges[rel.ID] = true
						addRecord(r, DependencyEdge{direction, rel})
					}
					if !seen[next] {
						seen[next] = true
						queue = append(queue, next)
					}
				}
			}
		}
		r.Limitations = append(r.Limitations, "only explicit directed depends_on edges; dynamic or undocumented dependencies remain unknown")
	case "recommend-tests":
		selected := map[string]bool{}
		anchored := []string{}
		for _, ref := range x.Paths[q.Path] {
			if err := ctx.Err(); err != nil {
				return err
			}
			// DCP-V1-310: a repository qualifier selects only records anchored at the path there.
			switch ref.Kind {
			case "subject":
				s := a.Subjects[ref.Offset]
				if pathInRepository(s.Evidence.Anchors, q.Path, q.Repository) {
					selected[s.ID] = true
					anchored = append(anchored, s.ID)
					addRecord(r, s)
				}
			case "claim":
				if c := a.Claims[ref.Offset]; pathInRepository(c.Evidence.Anchors, q.Path, q.Repository) {
					selected[c.Subject] = true
				}
			}
		}
		// DCS-V1-595: the subjects documenting the anchored ones, one hop,
		// with their documents relation; their test links are followed below.
		owners, err := addDocumentingOwners(ctx, a, anchored, map[string]bool{}, r)
		if err != nil {
			return err
		}
		for _, id := range owners {
			selected[id] = true
		}
		for id := range selected {
			for _, i := range x.Edges[id] {
				if err := ctx.Err(); err != nil {
					return err
				}
				rel := a.Relations[i]
				if a.Details[rel.ID].TestLink != nil {
					addRecord(r, rel)
				}
			}
		}
		missing := []string{"source-to-test completeness and semantic assertion adequacy NOT_OBSERVED", "manual case completeness NOT_OBSERVED"}
		if len(selected) == 0 {
			missing = append(missing, "changed path not documented")
		}
		if r.Freshness != "fresh" {
			missing = append(missing, "fresh corpus evidence NOT_OBSERVED")
		}
		knowledge, blocked := []TestKnowledgeJoin{}, []string{}
		for _, j := range testKnowledgeOnPath(a, q.Path) {
			if pathInRepository(recordAnchors(j), q.Path, q.Repository) {
				knowledge = append(knowledge, j)
			}
		}
		for _, j := range knowledge {
			if j.Selection == "blocks" {
				blocked = append(blocked, j.ID)
			}
		}
		for _, j := range knowledge {
			if len(blocked) == 0 && j.Selection == "candidate" {
				addRecord(r, j)
			}
		}
		if len(blocked) > 0 {
			missing = append(missing, "test-knowledge defect blocks selection")
		}
		r.Selection = &TestRecommendation{State: "full-relevant-suite-required", Required: "full relevant suite and mandatory repository checks", MissingEvidence: missing, Paths: []string{q.Path}, BlockedBy: blocked, BlockedByCount: len(blocked)}
		if len(blocked) > MaxBlockedBy {
			r.Selection.BlockedBy, r.Selection.BlockedByTruncated = blocked[:MaxBlockedBy], true
		}
	case "navigation":
		if q.ID == "" && q.Query != "" {
			// DCS-V1-596: the flows a query selects, each with its records.
			if err := navigationQuery(ctx, a, q.Query, r); err != nil {
				return err
			}
		} else if err := navigationRecords(ctx, a, []string{q.ID}, r); err != nil {
			return err
		}
		r.Limitations = append(r.Limitations, "routes, screens and selectors are documented anchors/declarations; live UI and navigation completeness NOT_OBSERVED")
	case "intent":
		if q.ID == "" {
			// DCP-V1-319: without an id, list every intent comparison.
			for _, s := range a.Subjects {
				if err := ctx.Err(); err != nil {
					return err
				}
				if a.Details[s.ID].IntentComparison != nil {
					addRecord(r, s)
				}
			}
			r.Limitations = append(r.Limitations, "recorded intent and observed claims are distinct attributed records; no accepted intent inferred")
			break
		}
		targets := map[string]bool{q.ID: true}
		for _, i := range x.Edges[q.ID] {
			if err := ctx.Err(); err != nil {
				return err
			}
			rel := a.Relations[i]
			targets[rel.From] = true
			targets[rel.To] = true
		}
		for id := range targets {
			if s, ok := indexedSubject(a, id); ok && (a.Details[s.ID].Ticket != nil || a.Details[s.ID].IntentComparison != nil) {
				addRecord(r, s)
			}
		}
		r.Limitations = append(r.Limitations, "recorded intent and observed claims are distinct attributed records; no accepted intent inferred")
	}
	sort.SliceStable(r.Results, func(i, j int) bool { return typedKey(r.Results[i]) < typedKey(r.Results[j]) })
	unique := r.Results[:0]
	last := ""
	for _, v := range r.Results {
		k := typedKey(v)
		if k != last {
			unique = append(unique, v)
			last = k
		}
	}
	r.Results = unique
	return ctx.Err()
}

// claimSearch ranks claims by the distinct query terms their ID and text
// hold, a term also matching its regular inflections (DCX-V1-476), with an
// exact case-insensitive text match first (DCP-V1-313). Claims holding as
// many terms order by the summed inverse document frequency of the terms
// they hold, then the shorter text, then natural claim ID order
// (DCX-V1-477). Terms are summed in sorted order so the weight is
// deterministic. Each claim appears once.
func claimSearch(ctx context.Context, a *Artifact, q Request, r *Receipt) error {
	x := a.RuntimeIndex
	terms := contextindex.EvidenceTerms(q.Query)
	scores := map[int]int{}
	weights := map[int]float64{}
	// total is the summed idf of every query term, the most a claim can hold.
	total := 0.0
	for _, term := range slices.Sorted(maps.Keys(terms)) {
		if err := ctx.Err(); err != nil {
			return err
		}
		held := claimPostings(x, term)
		idf := searchIDF(len(a.Claims), len(held))
		total += idf
		for _, i := range held {
			if !claimKindAdmitted(a, q, a.Claims[i].ID) {
				continue
			}
			scores[i]++
			weights[i] += idf
		}
	}
	held := make(map[int]int, len(scores))
	exact := map[int]bool{}
	for i := range scores {
		held[i] = scores[i]
		if strings.EqualFold(strings.TrimSpace(q.Query), strings.TrimSpace(a.Claims[i].Text)) {
			scores[i] += len(terms) + 1
			exact[i] = true
		}
	}
	ids := make([]int, 0, len(scores))
	for i := range scores {
		ids = append(ids, i)
	}
	// DCS-V1-593: a claim whose typed details make it a supplied test run
	// ranks after every other claim; its served terms and tier are kept.
	run := make(map[int]bool, len(ids))
	for _, i := range ids {
		run[i] = claimParityKind(a.Details[a.Claims[i].ID]) == "test_run"
	}
	sort.Slice(ids, func(i, j int) bool {
		ci, cj := ids[i], ids[j]
		if run[ci] != run[cj] {
			return run[cj]
		}
		if scores[ci] != scores[cj] {
			return scores[ci] > scores[cj]
		}
		if weights[ci] != weights[cj] {
			return weights[ci] > weights[cj]
		}
		if li, lj := specificity(a.Claims[ci].Text), specificity(a.Claims[cj].Text); li != lj {
			return li < lj
		}
		return naturalLess(a.Claims[ci].ID, a.Claims[cj].ID)
	})
	// Claims tie when they hold as many query terms of the same summed idf;
	// specificity and the natural ID only order a tie group (DCS-V1-480).
	r.ranks = make(map[string]ResultRank, len(ids))
	group := 0
	for n, i := range ids {
		if n == 0 || run[i] != run[ids[n-1]] || scores[i] != scores[ids[n-1]] || weights[i] != weights[ids[n-1]] {
			group++
		}
		tier := "terms"
		if exact[i] {
			tier = "exact"
		}
		r.ranks[a.Claims[i].ID] = ResultRank{Rank: n + 1, Group: group, Tier: tier, Score: servedScore(normalized(weights[i], total)), Terms: held[i]}
		addRecord(r, a.Claims[i])
	}
	return ctx.Err()
}

// suppliedStatusClaims selects the supplied status claims in q.State (DCP-V1-318):
// only unqualified ones unless q.Qualifier names a qualifier, matching the
// docs_coverage supplied_status rows, and with q.Run only claims whose subject
// is an end of a migration_mapping relation of that run.
func suppliedStatusClaims(ctx context.Context, a *Artifact, q Request, r *Receipt) error {
	x := a.RuntimeIndex
	for _, c := range a.Claims {
		if err := ctx.Err(); err != nil {
			return err
		}
		status := a.Details[c.ID].Status
		if status == nil || status.State != q.State || status.Qualifier != q.Qualifier || !claimKindAdmitted(a, q, c.ID) {
			continue
		}
		inRun := q.Run == ""
		for _, i := range x.Edges[c.Subject] {
			if inRun {
				break
			}
			rel := a.Relations[i]
			mapping := a.Details[rel.ID].MigrationMapping
			inRun = rel.Type == "migration_mapping" && mapping != nil && mapping.Run == q.Run
		}
		if inRun {
			addRecord(r, c)
		}
	}
	r.Limitations = append(r.Limitations, "supplied status claims are producer records; no readiness or proof is derived")
	return nil
}

// claimKindAdmitted reports whether a claims query's kind filter and
// exclusion admit the claim, by the kind its typed details give it
// (DCS-V1-592).
func claimKindAdmitted(a *Artifact, q Request, id string) bool {
	if q.Kinds == nil && q.ExcludeKinds == nil {
		return true
	}
	kind := claimParityKind(a.Details[id])
	return (q.Kinds == nil || slices.Contains(q.Kinds, kind)) && !slices.Contains(q.ExcludeKinds, kind)
}

func typedKey(v any) string {
	if e, ok := v.(DependencyEdge); ok {
		return e.Direction + ":" + e.Relation.ID
	}
	// A selector is one per root flow, so two selected flows sharing a test keep
	// both (DCS-V1-596). The NUL keeps a selector's place among other IDs.
	if n, ok := v.(NavigationSelector); ok {
		return n.ID + "\x00" + n.Flow
	}
	return recordID(v)
}
func typedGaps(ctx context.Context, a *Artifact, freshness string) ([]Gap, error) {
	out := append(append(append(testKnowledgeGaps(a), flowRelationGaps(a)...), claimAuditGaps(a)...), claimMentionGaps(a)...)
	counted := map[string]bool{}
	for _, j := range a.TestKnowledge {
		for _, f := range j.Flows {
			counted[f] = counted[f] || j.CountsTowardCoverage
		}
	}
	journeys := map[string][]Journey{}
	for _, j := range a.Journeys {
		journeys[j.Subject] = append(journeys[j.Subject], j)
	}
	for _, s := range a.Subjects {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if s.Kind != "flow" {
			continue
		}
		tests, manual := counted[s.ID], false
		asserting := assertingE2E{}
		for _, i := range a.RuntimeIndex.Edges[s.ID] {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			rel := a.Relations[i]
			if link := a.Details[rel.ID].TestLink; link != nil {
				tests = true
				asserting.observe(link, rel.Evidence)
			}
			if rel.Type == "tested_by" {
				tests = true
			}
			if rel.Type == "journey_for" || rel.Type == "manual_case_for" && rel.To == s.ID {
				manual = true
			}
		}
		for _, j := range journeys[s.ID] {
			manual = true
			if j.Evidence.Freshness == "stale" || freshness == "stale" {
				out = append(out, Gap{s.ID, "stale-journey", "journey evidence is stale"})
			}
		}
		if !tests {
			out = append(out, Gap{s.ID, "no-tests", "no documented test links"})
		}
		if g, ok := asserting.gap(s.ID); ok {
			out = append(out, g)
		}
		if !manual {
			out = append(out, Gap{s.ID, "missing-manual-cases", "no documented manual disposition; non-UI disposition NOT_OBSERVED"})
		}
		if freshness == "stale" || s.Evidence.Freshness == "stale" {
			out = append(out, Gap{s.ID, "stale-anchors", "source freshness is stale"})
		}
	}
	return out, ctx.Err()
}

// assertingE2E tracks, for one flow, the test link closest to an exact,
// supported, authenticated-review asserting link (DCP-V1-150..152). An
// imported "reviewed" label is attributed, not authenticated: import keeps
// it only as ReportedTrust, so it yields the weaker distinct gap instead.
type assertingE2E struct {
	links      int
	satisfied  bool
	unverified bool
	closest    []string
}

func (t *assertingE2E) observe(link *TestLinkDetails, e Evidence) {
	t.links++
	failed := []string{}
	if link.Role != "asserting" {
		failed = append(failed, "not asserting")
	}
	if link.JoinConfidence != "exact" {
		failed = append(failed, "not exact")
	}
	if e.State != "supported" {
		failed = append(failed, "not supported")
	}
	if len(failed) == 0 && e.Trust == "reviewed" {
		t.satisfied = true
		return
	}
	if len(failed) == 0 && e.ReportedTrust == "reviewed" {
		t.unverified = true
		return
	}
	if e.ReportedTrust == "reviewed" {
		failed = append(failed, "review not authenticated")
	} else if e.Trust != "reviewed" {
		failed = append(failed, "not reviewed")
	}
	if t.closest == nil || len(failed) < len(t.closest) {
		t.closest = failed
	}
}

func (t *assertingE2E) gap(subject string) (Gap, bool) {
	switch {
	case t.satisfied:
		return Gap{}, false
	case t.unverified:
		return Gap{subject, "asserting-e2e-tests-unauthenticated-review", "exact supported asserting test link with provider-reported review; the review is attributed, not authenticated; adequacy remains unknown"}, true
	case t.links == 0:
		return Gap{subject, "missing-asserting-e2e-tests", "no test link; adequacy remains unknown"}, true
	}
	return Gap{subject, "missing-asserting-e2e-tests", "closest test link is " + strings.Join(t.closest, ", ") + "; adequacy remains unknown"}, true
}

// navigationRecords adds the navigation records of each root: its navigates
// relations, the subjects its relations and flow entrypoints reach (the root
// itself only when it is an endpoint, ui_surface or interaction), and the
// declared selectors of the behaviour-contract tests linked to it.
func navigationRecords(ctx context.Context, a *Artifact, roots []string, r *Receipt) error {
	x := a.RuntimeIndex
	root := map[string]bool{}
	selected := map[string]bool{}
	for _, id := range roots {
		root[id], selected[id] = true, true
		if flow := a.Details[id].Flow; flow != nil {
			for _, entry := range flow.Entrypoints {
				selected[entry] = true
			}
		}
		for _, i := range x.Edges[id] {
			rel := a.Relations[i]
			selected[rel.From] = true
			selected[rel.To] = true
			if rel.Type == "navigates" {
				addRecord(r, rel)
			}
		}
	}
	for id := range selected {
		if err := ctx.Err(); err != nil {
			return err
		}
		if s, ok := indexedSubject(a, id); ok && (s.Kind == "endpoint" || s.Kind == "ui_surface" || s.Kind == "interaction" || !root[id]) {
			addRecord(r, s)
		}
	}
	for _, report := range a.BehaviorContracts {
		for _, test := range report.Registry.Tests {
			if err := ctx.Err(); err != nil {
				return err
			}
			for _, flow := range test.Flows {
				if !root[flow] {
					continue
				}
				// Each root flow the test names gets its own selectors (DCS-V1-596); with one
				// root, as in the id form, a test lists that flow at most once.
				for _, assertion := range test.Assertions {
					addRecord(r, NavigationSelector{ID: test.ID + ":" + assertion.ID, Flow: flow, Test: test.ID, Locator: assertion.Locator, Annotation: assertion.Annotation, Trust: "attributed-declaration"})
				}
			}
		}
	}
	return nil
}

// navigationQuery selects each flow subject whose ID, name, or the file or
// title of one of its asserts or navigates test links holds every query term
// (a term also matching its regular inflections), and adds the flow, its
// navigation records and those test links with their TestLink details
// (DCS-V1-596). A query without a searchable term selects nothing; a subject
// that is not a flow is never selected by itself.
func navigationQuery(ctx context.Context, a *Artifact, query string, r *Receipt) error {
	x := a.RuntimeIndex
	want := []string{}
	for term := range contextindex.EvidenceTerms(query) {
		// A compound word's parts are required on their own.
		if !strings.Contains(term, "-") {
			want = append(want, term)
		}
	}
	if len(want) == 0 {
		return nil
	}
	roots := []string{}
	links := []int{}
	for _, s := range a.Subjects {
		if err := ctx.Err(); err != nil {
			return err
		}
		if s.Kind != "flow" {
			continue
		}
		held := map[string]struct{}{}
		contextindex.CollectEvidenceTerms(s.ID, held)
		contextindex.CollectEvidenceTerms(s.Name, held)
		tests := []int{}
		for _, i := range x.Edges[s.ID] {
			rel := a.Relations[i]
			if link := a.Details[rel.ID].TestLink; link != nil && rel.From == s.ID && (rel.Type == "asserts" || rel.Type == "navigates") {
				contextindex.CollectEvidenceTerms(link.File, held)
				contextindex.CollectEvidenceTerms(link.Title, held)
				tests = append(tests, i)
			}
		}
		if holdsEveryTerm(held, want) {
			roots = append(roots, s.ID)
			links = append(links, tests...)
			addRecord(r, s)
		}
	}
	for _, i := range links {
		addRecord(r, a.Relations[i])
	}
	return navigationRecords(ctx, a, roots, r)
}

// holdsEveryTerm reports whether held has each wanted term or one of its
// regular inflections, or a held term is an inflection of it.
func holdsEveryTerm(held map[string]struct{}, want []string) bool {
	for _, term := range want {
		_, ok := held[term]
		for _, form := range inflections(term) {
			_, inflected := held[form]
			ok = ok || inflected
		}
		for h := range held {
			if ok {
				break
			}
			ok = slices.Contains(inflections(h), term)
		}
		if !ok {
			return false
		}
	}
	return true
}
