package doccorpus

import (
	"slices"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/Beamfall/corvint/internal/cem/wire"
)

// LegacyCrosswalk is the generated join of a static legacy inventory and a behavior registry
// (DCP-V1-065, DCP-V1-066). Candidates are scored suggestions for a reviewer; ReviewedChecks
// re-check the registry's existing reviewed legacy_criteria against the fresh inventory. Nothing
// here is a reviewed relation, and nothing here is runtime parity.
type LegacyCrosswalk struct {
	Status         string                `json:"status"`
	RegistrySHA256 string                `json:"registry_sha256"`
	Candidates     []LegacyCandidateSet  `json:"candidates"`
	ReviewedChecks []LegacyReviewedCheck `json:"reviewed_checks"`
}

// LegacyCandidateSet lists the best generated targets of one legacy case.
type LegacyCandidateSet struct {
	LegacyCase string            `json:"legacy_case"`
	Targets    []LegacyCandidate `json:"targets"`
}

// LegacyCandidate is one scored target flow or test with the reasons for its score.
type LegacyCandidate struct {
	Kind    string   `json:"kind"`
	ID      string   `json:"id"`
	Score   int      `json:"score"`
	Reasons []string `json:"reasons"`
}

// LegacyReviewedCheck is the verdict of one existing reviewed mapping against the inventory:
// refused, unresolved, not-parity or parity-candidate. parity-candidate still needs a qualified
// runtime baseline.
type LegacyReviewedCheck struct {
	Test            string   `json:"test"`
	Criterion       string   `json:"criterion"`
	LegacyCase      string   `json:"legacy_case"`
	LegacyCriterion string   `json:"legacy_criterion"`
	Relation        string   `json:"relation"`
	Verdict         string   `json:"verdict"`
	Reasons         []string `json:"reasons"`
}

const legacyCandidateLimit = 5

func (p *legacyProducer) crosswalk(raw []byte) (*LegacyCrosswalk, bool, error) {
	var registry BehaviorRegistry
	if err := decode(raw, &registry); err != nil {
		return nil, false, fail("crosswalk input is not a closed behavior registry")
	}
	if len(registry.Tests) > MaxRecords || len(registry.Flows) > MaxRecords {
		return nil, false, fail("crosswalk registry exceeds the record bound")
	}
	if err := legacyRegistryValid(registry); err != nil {
		return nil, false, err
	}
	out := &LegacyCrosswalk{Status: "generated", RegistrySHA256: Digest(raw), Candidates: []LegacyCandidateSet{}, ReviewedChecks: []LegacyReviewedCheck{}}
	for _, legacy := range p.inv.Legacy {
		set := LegacyCandidateSet{LegacyCase: legacy.ID, Targets: []LegacyCandidate{}}
		title := legacyTitleWords(legacy.Suite + " " + legacy.Case)
		for _, test := range registry.Tests {
			if candidate := legacyTestCandidate(legacy, title, test); candidate.Score > 0 {
				set.Targets = append(set.Targets, candidate)
			}
		}
		for _, flow := range registry.Flows {
			if candidate := legacyFlowCandidate(title, p.visits[legacy.ID], flow); candidate.Score > 0 {
				set.Targets = append(set.Targets, candidate)
			}
		}
		sort.SliceStable(set.Targets, func(i, j int) bool {
			left, right := set.Targets[i], set.Targets[j]
			if left.Score != right.Score {
				return left.Score > right.Score
			}
			if left.Kind != right.Kind {
				return left.Kind < right.Kind
			}
			return left.ID < right.ID
		})
		if len(set.Targets) > legacyCandidateLimit {
			set.Targets = set.Targets[:legacyCandidateLimit]
		}
		out.Candidates = append(out.Candidates, set)
	}
	cases := map[string]BehaviorLegacyCase{}
	for _, legacy := range p.inv.Legacy {
		cases[legacy.ID] = legacy
	}
	refused := false
	for _, test := range registry.Tests {
		for _, mapping := range test.Legacy {
			check := p.reviewedCheck(registry, cases, test, mapping)
			refused = refused || check.Verdict == "refused"
			out.ReviewedChecks = append(out.ReviewedChecks, check)
		}
	}
	if len(out.ReviewedChecks) > MaxRecords {
		return nil, false, fail("crosswalk reviewed mappings exceed the record bound")
	}
	return out, refused, nil
}

// legacyRegistryValid applies the behavior registry header and legacy mapping rules the compiler
// enforces (schema 2, pinned revisions, one identity namespace shared by flows, source behaviors
// and tests, closed relations), so a crosswalk is never computed against a registry the compiler
// would refuse.
func legacyRegistryValid(r BehaviorRegistry) error {
	if r.Schema != 2 || !textOK(r.ContractID) || !wire.IsSha256(r.ContractSHA256) || !wire.IsGitOid(r.SourceRevision) || !wire.IsGitOid(r.DocumentationRevision) {
		return fail("crosswalk registry header is not a schema 2 behavior registry with pinned revisions")
	}
	ids := []string{}
	for _, flow := range r.Flows {
		ids = append(ids, flow.ID)
		if !textOK(flow.ID) {
			return fail("crosswalk registry has an invalid flow identity")
		}
	}
	for _, behavior := range r.Behaviors {
		ids = append(ids, behavior.ID)
	}
	for _, test := range r.Tests {
		ids = append(ids, test.ID)
		if !textOK(test.ID) || !uniqueIdentities(test.Fixtures) || !uniqueIdentities(test.Roles) || len(test.Legacy) > MaxRecords {
			return fail("crosswalk registry has an invalid test identity or preconditions")
		}
		keys := map[[3]string]bool{}
		for _, mapping := range test.Legacy {
			if !textOK(mapping.Criterion) || !textOK(mapping.LegacyCase) || !textOK(mapping.LegacyCriterion) || !words("same stronger new obsolete blocked")[mapping.Relation] {
				return fail("crosswalk registry has an invalid legacy criterion mapping")
			}
			key := [3]string{mapping.Criterion, mapping.LegacyCase, mapping.LegacyCriterion}
			if keys[key] {
				return fail("crosswalk registry has a duplicate legacy criterion mapping")
			}
			keys[key] = true
		}
	}
	if !uniqueIdentities(ids) {
		return fail("crosswalk registry has duplicate flow, source behavior or test identities")
	}
	return nil
}

func legacyTitleWords(text string) map[string]bool {
	out := map[string]bool{}
	for _, word := range strings.FieldsFunc(strings.ToLower(text), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) }) {
		if len(word) > 1 {
			out[word] = true
		}
	}
	return out
}

// legacyOverlap scores title-word Jaccard similarity out of weight.
func legacyOverlap(left, right map[string]bool, weight int) (int, string) {
	shared := 0
	for word := range left {
		if right[word] {
			shared++
		}
	}
	union := len(left) + len(right) - shared
	if shared == 0 || union == 0 {
		return 0, ""
	}
	return weight * shared / union, "title-overlap:" + strconv.Itoa(shared) + "/" + strconv.Itoa(union)
}

func legacyTestCandidate(legacy BehaviorLegacyCase, title map[string]bool, test BehaviorTest) LegacyCandidate {
	candidate := LegacyCandidate{Kind: "test", ID: test.ID, Reasons: []string{}}
	exact, value := 0, 0
	for _, criterion := range legacy.Criteria {
		matched, valued := false, false
		for _, assertion := range test.Assertions {
			if assertion.Value == criterion.Value {
				valued = true
				matched = matched || assertion.Matcher == criterion.Matcher && assertion.Locator == criterion.Locator
			}
		}
		switch {
		case matched:
			exact++
			candidate.Reasons = append(candidate.Reasons, "tuple-equal:"+criterion.ID)
		case valued:
			value++
			candidate.Reasons = append(candidate.Reasons, "value-equal:"+criterion.ID)
		}
	}
	if n := len(legacy.Criteria); n > 0 {
		candidate.Score += 600*exact/n + 200*value/n
	}
	if score, reason := legacyOverlap(title, legacyTitleWords(test.Title), 300); score > 0 {
		candidate.Score += score
		candidate.Reasons = append(candidate.Reasons, reason)
	}
	if len(legacy.Fixtures) > 0 && slices.Equal(legacy.Fixtures, test.Fixtures) {
		candidate.Score += 100
		candidate.Reasons = append(candidate.Reasons, "preconditions-equal")
	}
	return candidate
}

func legacyFlowCandidate(title map[string]bool, visits []string, flow BehaviorFlow) LegacyCandidate {
	candidate := LegacyCandidate{Kind: "flow", ID: flow.ID, Reasons: []string{}}
	if score, reason := legacyOverlap(title, legacyTitleWords(flow.ID), 300); score > 0 {
		candidate.Score += score
		candidate.Reasons = append(candidate.Reasons, reason)
	}
	pages := 0
	for _, page := range flow.RequiredPages {
		if slices.Contains(visits, page) && pages < 2 {
			pages++
			candidate.Score += 100
			candidate.Reasons = append(candidate.Reasons, "page-visited:"+page)
		}
	}
	return candidate
}

// reviewedCheck compares one reviewed mapping with the fresh inventory. A same/stronger target
// that no longer asserts the extracted legacy tuple is refused; any precondition, relation or
// extraction difference is not parity.
func (p *legacyProducer) reviewedCheck(registry BehaviorRegistry, cases map[string]BehaviorLegacyCase, test BehaviorTest, mapping BehaviorLegacyMapping) LegacyReviewedCheck {
	check := LegacyReviewedCheck{Test: test.ID, Criterion: mapping.Criterion, LegacyCase: mapping.LegacyCase, LegacyCriterion: mapping.LegacyCriterion, Relation: mapping.Relation, Reasons: []string{}}
	legacy, ok := cases[mapping.LegacyCase]
	if !ok {
		check.Verdict, check.Reasons = "unresolved", []string{"legacy-case-not-discovered"}
		return check
	}
	var criterion *BehaviorLegacyCriterion
	for i := range legacy.Criteria {
		if legacy.Criteria[i].ID == mapping.LegacyCriterion {
			criterion = &legacy.Criteria[i]
		}
	}
	if criterion == nil {
		check.Verdict, check.Reasons = "unresolved", []string{"legacy-criterion-not-discovered"}
		return check
	}
	notParity := false
	if !slices.Contains(test.Criteria, mapping.Criterion) {
		notParity = true
		check.Reasons = append(check.Reasons, "target-criterion-not-declared")
	}
	if mapping.Review.Kind != "review" || mapping.Review.Revision != registry.SourceRevision {
		notParity = true
		check.Reasons = append(check.Reasons, "mapping-review-missing-or-stale")
	}
	if mapping.Relation == "same" || mapping.Relation == "stronger" {
		retained, reviewed, differs := false, false, false
		for _, assertion := range test.Assertions {
			if assertion.Criterion != mapping.Criterion {
				continue
			}
			if assertion.Matcher == criterion.Matcher && assertion.Locator == criterion.Locator && assertion.Value == criterion.Value {
				retained = true
				reviewed = reviewed || behaviorAssertionValid(registry, test, assertion)
			} else if assertion.Matcher == criterion.Matcher && assertion.Locator == criterion.Locator {
				differs = true
			}
		}
		if retained && !reviewed {
			notParity = true
			check.Reasons = append(check.Reasons, "target-assertion-not-reviewed")
		}
		if !retained {
			check.Verdict = "refused"
			if differs {
				check.Reasons = append(check.Reasons, "expected-value-differs")
			} else {
				check.Reasons = append(check.Reasons, "observable-dropped")
			}
		}
	} else {
		notParity = true
		check.Reasons = append(check.Reasons, "relation-"+mapping.Relation+"-is-not-parity")
	}
	if reasons := legacyPreconditionReasons(test.Fixtures, legacy.Fixtures); len(reasons) > 0 {
		notParity = true
		check.Reasons = append(check.Reasons, reasons...)
	}
	if len(test.Roles) > 0 {
		notParity = true
		check.Reasons = append(check.Reasons, "roles-not-discovered")
	}
	if legacy.State != "executable" {
		notParity = true
		check.Reasons = append(check.Reasons, "legacy-case-disabled")
	}
	if p.caseUnknown[legacy.ID] {
		notParity = true
		check.Reasons = append(check.Reasons, "legacy-case-has-unknowns")
	}
	switch {
	case check.Verdict == "refused":
	case notParity:
		check.Verdict = "not-parity"
	default:
		check.Verdict = "parity-candidate"
		check.Reasons = append(check.Reasons, "runtime-baseline-required")
	}
	return check
}

// legacyPreconditionReasons names how target fixtures differ from the extracted legacy ones. The
// behavior compiler compares the same lists with slices.Equal.
func legacyPreconditionReasons(target, legacy []string) []string {
	if slices.Equal(target, legacy) {
		return nil
	}
	prefix := func(fixture string) string {
		head, _, _ := strings.Cut(fixture, "@sha256:")
		return head
	}
	legacyPrefixes := map[string]bool{}
	for _, fixture := range legacy {
		legacyPrefixes[prefix(fixture)] = true
	}
	targetPrefixes := map[string]bool{}
	reasons := []string{}
	for _, fixture := range target {
		targetPrefixes[prefix(fixture)] = true
		switch {
		case slices.Contains(legacy, fixture):
		case legacyPrefixes[prefix(fixture)]:
			reasons = append(reasons, "precondition-digest-differs:"+prefix(fixture))
		default:
			reasons = append(reasons, "precondition-not-discovered:"+prefix(fixture))
		}
	}
	for _, fixture := range legacy {
		if !slices.Contains(target, fixture) && !targetPrefixes[prefix(fixture)] {
			reasons = append(reasons, "precondition-missing-from-target:"+prefix(fixture))
		}
	}
	if len(reasons) == 0 {
		reasons = append(reasons, "precondition-order-differs")
	}
	slices.Sort(reasons)
	return slices.Compact(reasons)
}
