package testvalidity

// Static test-source validity rules (docs/specs/test-source-analysis-v0.md, TSA-V0-045..052 and
// TSA-V0-053..056).
// A rule reads committed test source, not a run, and reports a SourceFinding. The readers live in
// internal/testsource, which already imports this package; this file holds only the shared
// vocabulary and the one place a finding reaches a projection.

// Source rule IDs.
const (
	// RuleIsolatedInSerial: an isolation-annotated Playwright test sits under a serial-mode
	// describe, so an earlier serial failure skips it (TSA-V0-045).
	RuleIsolatedInSerial = "ISOLATED-IN-SERIAL"
	// RuleMemoizedBlockExpectation: an RSpec block expectation names a subject or let that an
	// enclosing before or let! has already memoized, so the block runs nothing (TSA-V0-048).
	RuleMemoizedBlockExpectation = "MEMOIZED-BLOCK-EXPECTATION"
	// RuleBlockNotCalled: an RSpec helper takes a block but hands an assertion a block that only
	// references it, or never calls it, so every example passing it a block is vacuous
	// (TSA-V0-054).
	RuleBlockNotCalled = "BLOCK-NOT-CALLED"
)

// Source finding severities. An invalid finding makes the test unusable as proof; an advisory
// one is reported and changes no axis.
const (
	SeverityInvalid  = "invalid"
	SeverityAdvisory = "advisory"
	// SeverityAbstained: the reader could not decide a rule for the test, so an ELIGIBLE hygiene
	// axis becomes ABSTAINED with the rule as its reason instead of staying ELIGIBLE (TSA-V0-088).
	SeverityAbstained = "abstained"
)

// HygieneReasonMemoizedBlockExpectation is the hygiene reason an invalid
// MEMOIZED-BLOCK-EXPECTATION finding sets (TSA-V0-051).
const HygieneReasonMemoizedBlockExpectation = "memoized-block-expectation"

// HygieneReasonBlockNotCalled is the hygiene reason an invalid BLOCK-NOT-CALLED finding sets
// (TSA-V0-056).
const HygieneReasonBlockNotCalled = "block-not-called"

// SourceFinding is one static test-source validity finding. Test and Location are `path:line`
// of the test declaration and of the flagged statement; Cause is `path:line` of the serial
// ancestor or the memoizing hook the finding depends on.
type SourceFinding struct {
	Rule     string `json:"rule"`
	Severity string `json:"severity"`
	Test     string `json:"test"`
	Title    string `json:"title,omitempty"`
	Location string `json:"location"`
	Cause    string `json:"cause"`
	Message  string `json:"message"`
}

// ApplySourceFindings returns p with the hygiene axis made INELIGIBLE when any of the given
// findings, which the caller has selected for the test p projects, is invalid (TSA-V0-051). The
// reason is the first invalid finding's rule reason unless hygiene was already INELIGIBLE, and
// each invalid finding's location and cause are appended to the existing anchors, keeping the first
// occurrence of each anchor in emission order (V1-1865). Advisory findings and
// the other four axes are left unchanged, so no axis rewrites another. With no invalid finding, the
// first abstained finding turns an ELIGIBLE hygiene axis ABSTAINED (TSA-V0-088).
func ApplySourceFindings(p Projection, findings []SourceFinding) Projection {
	anchors := []string{}
	reason := ""
	for _, f := range findings {
		if f.Severity != SeverityInvalid {
			continue
		}
		if reason == "" {
			reason = sourceRuleReason(f.Rule)
		}
		anchors = append(anchors, f.Location, f.Cause)
	}
	if reason == "" {
		for _, f := range findings {
			if f.Severity == SeverityAbstained && p.Hygiene.State == HygieneEligible {
				p.Hygiene = Axis{State: HygieneAbstained, Reason: f.Rule, Anchors: appendAnchors(p.Hygiene.Anchors, f.Location, f.Cause)}
				break
			}
		}
		return p
	}
	if p.Hygiene.State != HygieneIneligible {
		p.Hygiene.State, p.Hygiene.Reason = HygieneIneligible, reason
	}
	p.Hygiene.Anchors = appendAnchors(p.Hygiene.Anchors, anchors...)
	return p
}

// appendAnchors returns a new slice holding existing then added, keeping only the first occurrence
// of each anchor, so a location that is also a cause, or a finding repeated across rules, is
// listed once (V1-1865).
func appendAnchors(existing []string, added ...string) []string {
	out := make([]string, 0, len(existing)+len(added))
	seen := make(map[string]bool, len(existing)+len(added))
	for _, a := range append(append([]string{}, existing...), added...) {
		if seen[a] {
			continue
		}
		seen[a] = true
		out = append(out, a)
	}
	return out
}

// sourceRuleReason maps an invalid rule to its hygiene reason. Any other invalid rule uses its
// rule ID as the reason, so a finding is never dropped.
func sourceRuleReason(rule string) string {
	switch rule {
	case RuleMemoizedBlockExpectation:
		return HygieneReasonMemoizedBlockExpectation
	case RuleBlockNotCalled:
		return HygieneReasonBlockNotCalled
	}
	return rule
}
