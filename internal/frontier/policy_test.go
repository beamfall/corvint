package frontier

import (
	"strings"
	"testing"
)

// CF-V0-005: `strict-v0` is the whole policy. It has no acknowledgement,
// exception, score, confidence, or permissive-success input, and `READY`,
// `NEEDS_WIDENING` and `OUT_OF_SCOPE` are retrieval-packet terms that must not
// appear in the frontier wire.
func TestPolicyIsLiteralStrictV0(t *testing.T) {
	document, encoded := openScenario().run(t)
	rendered := string(encoded) + RenderHuman(document)
	if !strings.Contains(string(encoded), `"policy":"`+Policy+`"`) {
		t.Fatal("the wire must carry the literal strict-v0 policy")
	}
	for _, banned := range []string{"READY", "NEEDS_WIDENING", "OUT_OF_SCOPE", "acknowledg", "confidence", "score"} {
		if strings.Contains(rendered, banned) {
			t.Fatalf("output contains the forbidden term %q", banned)
		}
	}
}

// CF-V0-013: V0 infers no test-claim quantifier. Every selected claim edge is
// evaluated independently and its mapped diagnostics all participate in the
// union, so neither an ANY nor an ALL reading can be read out of the item.
func TestNoTestClaimQuantifierIsInferred(t *testing.T) {
	document, _ := scenario{
		evidence: []evidenceSpec{closingEvidence},
		hunks:    []hunkSpec{closingHunk("h")},
		obligations: []obligationSpec{{
			id: "CFTEST-001", disposition: OCMLinked, statement: "the widgetregistry accepts entries",
			hunks: []string{"h"}, claims: []string{"c1", "c2", "c3"},
		}},
		claims: map[string][]TCQClaimResult{
			"CFTEST-001": {
				// One edge matched under the TCQ relation; two did not. Under an
				// ANY quantifier the matched edge would close the item, and under
				// ALL the unmatched edges would be the only reasons. V0 does both:
				// it retains every mapped reason and closes nothing.
				{
					ObligationID: "CFTEST-001", ClaimID: claimRef("c1"),
					Relation: tcqRelation, AuthorityClass: AuthorityCallerReported,
				},
				callerReported("CFTEST-001", "c2", "test-failed"),
				callerReported("CFTEST-001", "c3", "empty-body"),
			},
		},
	}.run(t)
	item := findItem(t, document, KindIntentTest, "CFTEST-001")
	want := []string{ReasonCallerReportedNonclosing, ReasonTestFailed, ReasonTestClaimEmpty}
	if !equalStrings(item.Reasons, want) {
		t.Fatalf("reasons %v, want %v", item.Reasons, want)
	}
	if item.ResolutionClass != ResolutionAuthorityRequired || item.NextAction != ActionEstablishHarnessAuthority {
		t.Fatalf("resolution %s / action %s, want the highest-priority reason to select both",
			item.ResolutionClass, item.NextAction)
	}
	if len(item.RelatedIDs) != 3 {
		t.Fatalf("%d related ids, want every selected claim", len(item.RelatedIDs))
	}
	if document.FrontierState != StateOpen {
		t.Fatal("no TCQ V0 relation closes a test item")
	}
}
