package appflows

import (
	"bytes"
	"context"
	"errors"
	"slices"
	"strings"

	"github.com/Beamfall/corvint/internal/doccorpus"
)

// Query document schemas (AFU-V1-015..017).
const (
	MapSchema    = "application-flow-map/1"
	LookupSchema = "application-flow-lookup/1"
	GapsSchema   = "application-flow-gaps/1"
)

// The closed gap codes (AFU-V1-016).
const (
	GapUnmappedFlow           = "unmapped-flow"
	GapNoTest                 = "no-test"
	GapTestWithoutAssertion   = "test-without-assertion"
	GapAssertionUnlinked      = "assertion-unlinked"
	GapStaleLink              = "stale-link"
	GapInferredOnly           = "inferred-only"
	GapEvidenceMissing        = "evidence-missing"
	GapEvidenceStale          = "evidence-stale"
	GapEvidenceFlaky          = "evidence-flaky"
	GapNegativeControlMissing = "negative-control-missing"
	GapCleanupUnverified      = "cleanup-unverified"
	GapUnreviewed             = "unreviewed"
)

// evidenceGaps maps every evidence state except verified onto its gap code; a failed run has no
// code of its own, so it is evidence-missing: no passed run holds at the evaluated revision.
var evidenceGaps = map[string]string{
	"missing": GapEvidenceMissing, "failed": GapEvidenceMissing, "stale": GapEvidenceStale, "flaky": GapEvidenceFlaky,
	"negative-control-missing": GapNegativeControlMissing, "cleanup-unverified": GapCleanupUnverified,
}

// authorityRank orders the authority ladder; an absent authority ranks below STATIC.
var authorityRank = map[string]int{AuthorityStatic: 1, AuthorityIngested: 2, AuthorityLocallyObserved: 3}

// MapReport is `flows map`: every flow member with its evaluated links and variation evidence.
type MapReport struct {
	Schema   string        `json:"schema"`
	Revision string        `json:"revision"`
	Review   ReviewSummary `json:"review"`
	Flows    []MapFlow     `json:"flows"`
}

// MapFlow is one flow; Status is complete only when `flows gaps` reports no gap for it.
type MapFlow struct {
	FlowID     string         `json:"flow_id"`
	Proposed   bool           `json:"proposed"`
	Status     string         `json:"status"`
	Steps      []MapMember    `json:"steps"`
	Outcomes   []MapMember    `json:"outcomes"`
	Variations []MapVariation `json:"variations"`
}

// MapMember is one step or outcome and the links whose from names it.
type MapMember struct {
	ID    string          `json:"id"`
	Links []EvaluatedLink `json:"links"`
}

// MapVariation is verified only when it has a non-inferred test and every required evidence pair is.
type MapVariation struct {
	ID       string          `json:"id"`
	Steps    []string        `json:"steps"`
	Outcomes []string        `json:"outcomes"`
	Projects []string        `json:"projects"`
	Links    []EvaluatedLink `json:"links"`
	Evidence []TestEvidence  `json:"evidence"`
	Verified bool            `json:"verified"`
}

// TestEvidence is the evidence state of one required test key and project pair.
type TestEvidence struct {
	TestKey   string `json:"test_key"`
	Project   string `json:"project,omitempty"`
	State     string `json:"state"`
	Authority string `json:"authority"`
}

// LookupReport is a reverse lookup derived at query time from the forward links (AFU-V1-010).
type LookupReport struct {
	Schema   string       `json:"schema"`
	Revision string       `json:"revision"`
	Path     string       `json:"path,omitempty"`
	TestKey  string       `json:"test_key,omitempty"`
	Flows    []ReverseHit `json:"flows"`
}

// GapsReport is `flows gaps`.
type GapsReport struct {
	Schema   string    `json:"schema"`
	Revision string    `json:"revision"`
	Flows    []GapFlow `json:"flows"`
}

// GapFlow is one flow; any gap makes it incomplete.
type GapFlow struct {
	FlowID string `json:"flow_id"`
	Status string `json:"status"`
	Gaps   []Gap  `json:"gaps"`
}

// Gap is one closed-code gap. Member names the variation, outcome or link source it is about.
type Gap struct {
	Code    string `json:"code"`
	Member  string `json:"member,omitempty"`
	TestKey string `json:"test_key,omitempty"`
	Project string `json:"project,omitempty"`
	Detail  string `json:"detail"`
}

// head is the evaluated commit and its tree; run evidence holds only for exactly this source.
type head struct{ commit, tree string }

// ReadRunEvidence reads --evidence files of canonical test-run-evidence/0 records, one per line,
// each with the flow-input discipline: regular file before open, byte bound, strict decode and the
// secret screen. The record bound applies to all files together (AFU-V1-036, AFU-V1-037).
func ReadRunEvidence(filenames []string) ([]TestRunEvidence, error) {
	records := []TestRunEvidence{}
	for _, name := range filenames {
		raw, err := ReadFile(name)
		if err != nil {
			return nil, err
		}
		for line := range bytes.Lines(raw) {
			if len(records) >= maxRunRecords {
				return nil, boundError(BoundRecords)
			}
			r, err := DecodeRunEvidence(line)
			if err != nil {
				return nil, err
			}
			records = append(records, r)
		}
	}
	return records, nil
}

// FlowMap reports every flow, member, link and variation evidence state at the set's revision
// (AFU-V1-015), with the reviewed denominator and its self-attestation limitation (AFU-V1-008, 009).
func FlowMap(ctx context.Context, root string, set IntentSet, evidence []TestRunEvidence) ([]byte, error) {
	links, at, err := evaluateSet(ctx, root, set)
	if err != nil {
		return nil, err
	}
	report := MapReport{Schema: MapSchema, Revision: at.commit, Review: Summarize(at.commit, links), Flows: []MapFlow{}}
	for _, intent := range set.Flows {
		own := flowLinks(links, intent.FlowID)
		flow := MapFlow{FlowID: intent.FlowID, Proposed: intent.Proposed, Status: status(flowGaps(intent, own, evidence, at)),
			Steps: []MapMember{}, Outcomes: []MapMember{}, Variations: []MapVariation{}}
		for _, s := range intent.Steps {
			flow.Steps = append(flow.Steps, MapMember{ID: s.StepID, Links: linksFrom(own, s.StepID)})
		}
		for _, o := range intent.Outcomes {
			flow.Outcomes = append(flow.Outcomes, MapMember{ID: o.OutcomeID, Links: linksFrom(own, o.OutcomeID)})
		}
		for _, v := range intent.Variations {
			pairs := variationEvidence(intent, v, own, evidence, at)
			verified := len(pairs) != 0 && !slices.ContainsFunc(pairs, func(e TestEvidence) bool { return e.State != "verified" })
			flow.Variations = append(flow.Variations, MapVariation{ID: v.VariationID, Steps: v.Steps, Outcomes: v.Outcomes, Projects: v.Projects,
				Links: linksFrom(own, v.VariationID), Evidence: pairs, Verified: verified})
		}
		report.Flows = append(report.Flows, flow)
	}
	return doccorpus.Encode(report)
}

// FlowLookup answers one reverse lookup, source path or test key to flows (AFU-V1-010).
func FlowLookup(ctx context.Context, root string, set IntentSet, path, testKey string) ([]byte, error) {
	links, at, err := evaluateSet(ctx, root, set)
	if err != nil {
		return nil, err
	}
	report := LookupReport{Schema: LookupSchema, Revision: at.commit, Path: path, TestKey: testKey, Flows: FlowsForTestKey(links, testKey)}
	if path != "" {
		report.Flows = FlowsForPath(links, path)
	}
	return doccorpus.Encode(report)
}

// FlowGaps reports each flow's closed-code gaps at the set's revision (AFU-V1-016).
func FlowGaps(ctx context.Context, root string, set IntentSet, evidence []TestRunEvidence) ([]byte, error) {
	links, at, err := evaluateSet(ctx, root, set)
	if err != nil {
		return nil, err
	}
	report := GapsReport{Schema: GapsSchema, Revision: at.commit, Flows: []GapFlow{}}
	for _, intent := range set.Flows {
		gaps := flowGaps(intent, flowLinks(links, intent.FlowID), evidence, at)
		report.Flows = append(report.Flows, GapFlow{FlowID: intent.FlowID, Status: status(gaps), Gaps: gaps})
	}
	return doccorpus.Encode(report)
}

func evaluateSet(ctx context.Context, root string, set IntentSet) ([]EvaluatedLink, head, error) {
	if set.Revision == "" {
		return nil, head{}, errors.New("flow queries need intents read from a commit")
	}
	links, err := EvaluateLinks(ctx, root, set, set.Revision)
	if err != nil {
		return nil, head{}, err
	}
	tree, err := git(ctx, root, "rev-parse", "--verify", "--end-of-options", set.Revision+"^{tree}")
	if err != nil {
		return nil, head{}, errors.New("evaluated revision has no tree")
	}
	return links, head{commit: set.Revision, tree: strings.TrimSpace(string(tree))}, nil
}

func flowLinks(links []EvaluatedLink, flow string) []EvaluatedLink {
	return slices.DeleteFunc(slices.Clone(links), func(l EvaluatedLink) bool { return l.Flow != flow })
}

func linksFrom(links []EvaluatedLink, from string) []EvaluatedLink {
	return slices.DeleteFunc(slices.Clone(links), func(l EvaluatedLink) bool { return l.From != from })
}

func status(gaps []Gap) string {
	if len(gaps) == 0 {
		return "complete"
	}
	return "incomplete"
}

// flowGaps derives one flow's gaps. A flow with no link is only unmapped; inferred links never
// satisfy a test, assertion or evidence requirement (AFU-V1-009).
func flowGaps(intent FlowIntent, links []EvaluatedLink, evidence []TestRunEvidence, at head) []Gap {
	if len(links) == 0 {
		return []Gap{{Code: GapUnmappedFlow, Detail: "the flow has no link"}}
	}
	gaps := []Gap{}
	if !slices.ContainsFunc(links, func(l EvaluatedLink) bool { return l.ReviewState != ReviewInferred }) {
		gaps = append(gaps, Gap{Code: GapInferredOnly, Detail: "every link is inferred"})
	}
	gaps = append(gaps, linkGaps(links)...)
	for _, v := range intent.Variations {
		gaps = append(gaps, variationGaps(intent, v, links, evidence, at)...)
	}
	return gaps
}

func linkGaps(links []EvaluatedLink) []Gap {
	gaps := []Gap{}
	for _, l := range links {
		switch l.ReviewState {
		case ReviewReviewed, ReviewInferred:
		case ReviewStale:
			gaps = append(gaps, Gap{Code: GapStaleLink, Member: l.From, Detail: l.Target.Type + " target changed since its review anchor"})
		default:
			gaps = append(gaps, Gap{Code: GapUnreviewed, Member: l.From, Detail: l.Target.Type + " link is " + l.ReviewState})
		}
	}
	return gaps
}

func variationGaps(intent FlowIntent, v FlowVariation, links []EvaluatedLink, evidence []TestRunEvidence, at head) []Gap {
	gaps := []Gap{}
	tests := declaredTargets(links, "test", v.VariationID)
	asserted := len(declaredTargets(links, "assertion", append([]string{v.VariationID}, v.Outcomes...)...)) != 0
	if len(tests) == 0 {
		gaps = append(gaps, Gap{Code: GapNoTest, Member: v.VariationID, Detail: "no declared test link"})
	}
	if len(tests) != 0 && !asserted {
		gaps = append(gaps, Gap{Code: GapTestWithoutAssertion, Member: v.VariationID, Detail: "declared tests but no declared assertion link"})
	}
	for _, o := range v.Outcomes {
		if len(declaredTargets(links, "assertion", o)) == 0 {
			gaps = append(gaps, Gap{Code: GapAssertionUnlinked, Member: o, Detail: "outcome of " + v.VariationID + " has no declared assertion link"})
		}
	}
	for _, e := range variationEvidence(intent, v, links, evidence, at) {
		if code, ok := evidenceGaps[e.State]; ok {
			gaps = append(gaps, Gap{Code: code, Member: v.VariationID, TestKey: e.TestKey, Project: e.Project, Detail: "run evidence is " + e.State})
		}
	}
	return gaps
}

// declaredTargets returns the sorted test keys or assertion names of non-inferred links of one type
// from any of the named members.
func declaredTargets(links []EvaluatedLink, kind string, from ...string) []string {
	values := []string{}
	for _, l := range links {
		if l.ReviewState != ReviewInferred && l.Target.Type == kind && slices.Contains(from, l.From) {
			values = append(values, l.Target.TestKey+l.Target.Assertion)
		}
	}
	return sortedSet(values)
}

// variationEvidence evaluates every required pair: each declared test key of the variation, crossed
// with its declared projects, or with any project when it declares none.
func variationEvidence(intent FlowIntent, v FlowVariation, links []EvaluatedLink, evidence []TestRunEvidence, at head) []TestEvidence {
	projects := v.Projects
	if len(projects) == 0 {
		projects = []string{""}
	}
	controls := []string{}
	if intent.Adapter != nil {
		controls = intent.Adapter.NegativeControls
	}
	pairs := []TestEvidence{}
	for _, key := range declaredTargets(links, "test", v.VariationID) {
		for _, project := range projects {
			matching := slices.DeleteFunc(slices.Clone(evidence), func(r TestRunEvidence) bool {
				return r.TestKey != key || (project != "" && r.Project != project)
			})
			pairs = append(pairs, TestEvidence{TestKey: key, Project: project, State: evidenceState(matching, controls, at), Authority: topAuthority(matching)})
		}
	}
	return pairs
}

// evidenceState names the first unmet condition of the Verified rule for the records of one pair.
// A record holds only when it ran exactly the evaluated commit and tree from a clean worktree; STATIC
// records never count (AFU-V1-014).
func evidenceState(records []TestRunEvidence, controls []string, at head) string {
	ran := slices.DeleteFunc(slices.Clone(records), func(r TestRunEvidence) bool { return r.Authority == AuthorityStatic })
	current := slices.DeleteFunc(slices.Clone(ran), func(r TestRunEvidence) bool {
		return r.Source != RunSource{Commit: at.commit, Tree: at.tree, Clean: true}
	})
	passed := slices.DeleteFunc(slices.Clone(current), func(r TestRunEvidence) bool { return Classify(r.Attempts) != "passed" })
	controlled := slices.DeleteFunc(slices.Clone(passed), func(r TestRunEvidence) bool { return !carriesControls(r, controls) })
	switch {
	case len(ran) == 0:
		return "missing"
	case len(current) == 0:
		return "stale"
	case slices.ContainsFunc(current, func(r TestRunEvidence) bool { return Classify(r.Attempts) == "flaky" }):
		return "flaky"
	case len(passed) == 0:
		return "failed"
	case len(controlled) == 0:
		return "negative-control-missing"
	case !slices.ContainsFunc(controlled, Verified):
		return "cleanup-unverified"
	}
	return "verified"
}

// carriesControls reports whether every control the flow adapter declares was run and observed as
// expected, and no control the record carries was observed otherwise.
func carriesControls(r TestRunEvidence, declared []string) bool {
	for _, c := range r.NegativeControls {
		if c.Observed != c.Expected {
			return false
		}
	}
	for _, key := range declared {
		if !slices.ContainsFunc(r.NegativeControls, func(c NegativeControl) bool { return c.TestKey == key }) {
			return false
		}
	}
	return true
}

func topAuthority(records []TestRunEvidence) string {
	top := "none"
	for _, r := range records {
		if authorityRank[r.Authority] > authorityRank[top] {
			top = r.Authority
		}
	}
	return top
}
