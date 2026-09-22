package frontier

import (
	"strings"
	"testing"
)

// closingEvidence and closingHunk share the term "widgetregistry", which is
// what makes their LRF basis edge emit `cem-lexical-v0`.
var closingEvidence = evidenceSpec{name: "spec", path: "docs/widget-spec.md", body: "widgetregistry contract"}

func closingHunk(name string) hunkSpec {
	return hunkSpec{
		name: name, disposition: CEMSupported, reason: "evidence-backed",
		body: "widgetregistry accepts entries", newPath: "src/" + name + ".go",
		bases: []string{"spec"},
	}
}

// CF-V0-008: a mechanically reverified whitespace-only or line-ending-only
// hunk closes and emits no item. With no linked obligation the frontier is
// then valid EMPTY (CF-V0-018), which is the only way V0 can be empty.
func TestMechanicalHunkClosesAndYieldsEmptyFrontier(t *testing.T) {
	for _, reason := range []string{"whitespace-only", "line-ending-only"} {
		t.Run(reason, func(t *testing.T) {
			document, encoded := scenario{
				hunks: []hunkSpec{{name: "ws", disposition: CEMMechanical, reason: reason, newPath: "src/ws.go"}},
			}.run(t)
			if document.FrontierState != StateEmpty || len(document.Items) != 0 {
				t.Fatalf("state %s with %d items, want EMPTY with none", document.FrontierState, len(document.Items))
			}
			if document.ExitCode() != 0 {
				t.Fatalf("exit %d, want 0 for a valid empty frontier", document.ExitCode())
			}
			if err := VerifyDocument(encoded); err != nil {
				t.Fatalf("empty document failed verification: %s", err.Code)
			}
		})
	}
}

// CF-V0-009: every CEM `unknown` reason maps to one exact frontier reason with
// NONE authority, empty relatedIds, ACTIONABLE, and SUPPLY_HUNK_BASIS.
func TestCEMUnknownReasonMapping(t *testing.T) {
	cases := []struct {
		upstream string
		reason   string
	}{
		{"no-evidence", ReasonHunkNoEvidence},
		{"insufficient-evidence", ReasonHunkInsufficientEvidence},
		{"conflicting-evidence", ReasonHunkConflictingEvidence},
	}
	for _, testCase := range cases {
		t.Run(testCase.upstream, func(t *testing.T) {
			document, _ := scenario{
				hunks: []hunkSpec{{name: "u", disposition: CEMUnknown, reason: testCase.upstream, newPath: "src/u.go"}},
			}.run(t)
			item := findItem(t, document, KindHunkBasis, hunkRef("u"))
			if !equalStrings(item.Reasons, []string{testCase.reason}) {
				t.Fatalf("reasons %v, want %v", item.Reasons, []string{testCase.reason})
			}
			if item.AuthorityClass != AuthorityNone || len(item.RelatedIDs) != 0 {
				t.Fatalf("authority %s with %d related ids, want NONE with none", item.AuthorityClass, len(item.RelatedIDs))
			}
			if item.ResolutionClass != ResolutionActionable || item.NextAction != ActionSupplyHunkBasis {
				t.Fatalf("resolution %s / action %s, want ACTIONABLE / SUPPLY_HUNK_BASIS", item.ResolutionClass, item.NextAction)
			}
		})
	}
}

// CF-V0-008: a qualifying basis is existential. One accepted basis closes the
// hunk even when another basis was rejected, and the rejected edge stays in the
// recomputed LRF diagnostics rather than reopening the hunk.
func TestOneQualifyingBasisClosesDespiteRejectedBasis(t *testing.T) {
	document, _ := scenario{
		evidence: []evidenceSpec{
			closingEvidence,
			{name: "unrelated", path: "docs/other.md", body: "gizmocatalog rules"},
		},
		hunks: []hunkSpec{{
			name: "h", disposition: CEMSupported, reason: "evidence-backed",
			body: "widgetregistry accepts entries", newPath: "src/h.go",
			bases: []string{"spec", "unrelated"},
		}},
	}.run(t)
	requireNoItem(t, document, KindHunkBasis, hunkRef("h"))
}

// CF-V0-010: with no qualifying basis the item carries the unique union of
// every rejected or term-bound-abstained reason, stored in the exact clause
// order, with relatedIds limited to the contributing evidence.
func TestUnresolvedSupportedHunkUnionAndOrder(t *testing.T) {
	document, _ := scenario{
		evidence: []evidenceSpec{
			{name: "broad", path: "docs/broad.md", body: strings.Repeat("widgetregistry ", 64)},
			{name: "self", path: "src/h.go", body: "widgetregistry contract"},
			{name: "unrelated", path: "docs/other.md", body: "gizmocatalog rules"},
		},
		hunks: []hunkSpec{{
			name: "h", disposition: CEMSupported, reason: "evidence-backed",
			body: "widgetregistry accepts entries", newPath: "src/h.go",
			bases: []string{"broad", "self", "unrelated"},
		}},
	}.run(t)
	item := findItem(t, document, KindHunkBasis, hunkRef("h"))
	want := []string{ReasonEvidenceSpanTooBroad, ReasonSelfReferentialBasis, ReasonInsufficientLexicalSupport}
	if !equalStrings(item.Reasons, want) {
		t.Fatalf("reasons %v, want %v in the frozen CF-V0-010 order", item.Reasons, want)
	}
	if item.AuthorityClass != AuthorityProducerDeclared {
		t.Fatalf("authority %s, want PRODUCER_DECLARED", item.AuthorityClass)
	}
	if item.ResolutionClass != ResolutionActionable || item.NextAction != ActionNarrowOrReplaceBasis {
		t.Fatalf("resolution %s / action %s, want ACTIONABLE / NARROW_OR_REPLACE_BASIS", item.ResolutionClass, item.NextAction)
	}
	if len(item.RelatedIDs) != 3 {
		t.Fatalf("%d related ids, want the three contributing evidence ids", len(item.RelatedIDs))
	}
}

// CF-V0-010: a supported deletion abstains on every basis and takes the
// profile-required action class, not the actionable one.
func TestSupportedDeletionRequiresProfile(t *testing.T) {
	document, _ := scenario{
		evidence: []evidenceSpec{closingEvidence},
		hunks: []hunkSpec{{
			name: "d", disposition: CEMSupported, reason: "evidence-backed",
			body: "widgetregistry accepts entries", oldPath: "src/d.go",
			bases: []string{"spec"},
		}},
	}.run(t)
	item := findItem(t, document, KindHunkBasis, hunkRef("d"))
	if !equalStrings(item.Reasons, []string{ReasonDeletionRelationRequired}) {
		t.Fatalf("reasons %v, want DELETION_RELATION_REQUIRED", item.Reasons)
	}
	if item.ResolutionClass != ResolutionProfileRequired || item.NextAction != ActionDefineSupportedProfile {
		t.Fatalf("resolution %s / action %s, want PROFILE_REQUIRED / DEFINE_SUPPORTED_PROFILE", item.ResolutionClass, item.NextAction)
	}
}

// CF-V0-010 and the adversarial matrix row for a local term-bound abstention:
// the affected subject stays OPEN under PROFILE_REQUIRED while an unrelated
// hunk in the same universe still closes normally.
func TestLocalTermBoundLeavesUnrelatedResultsAvailable(t *testing.T) {
	document, _ := scenario{
		evidence: []evidenceSpec{closingEvidence},
		hunks: []hunkSpec{
			{
				name: "bound", disposition: CEMSupported, reason: "evidence-backed",
				body: manyTerms(300), newPath: "src/bound.go", bases: []string{"spec"},
			},
			closingHunk("clean"),
		},
	}.run(t)
	item := findItem(t, document, KindHunkBasis, hunkRef("bound"))
	if !equalStrings(item.Reasons, []string{ReasonSubjectTermBoundExceeded}) {
		t.Fatalf("reasons %v, want SUBJECT_TERM_BOUND_EXCEEDED", item.Reasons)
	}
	if item.ResolutionClass != ResolutionProfileRequired || item.NextAction != ActionDefineSupportedProfile {
		t.Fatalf("resolution %s / action %s, want PROFILE_REQUIRED / DEFINE_SUPPORTED_PROFILE", item.ResolutionClass, item.NextAction)
	}
	requireNoItem(t, document, KindHunkBasis, hunkRef("clean"))
}

// CF-V0-011: an OCM `unknown` obligation emits exactly one INTENT_CHANGE item
// and no INTENT_TEST item, because no selected test edge exists.
func TestOCMUnknownEmitsNoFabricatedTestItem(t *testing.T) {
	cases := []struct {
		upstream string
		reason   string
	}{
		{"unassessed", ReasonObligationUnassessed},
		{"no-test-claim", ReasonObligationNoTestClaim},
		{"insufficient-evidence", ReasonObligationInsufficientEvidence},
		{"conflicting-evidence", ReasonObligationConflictingEvidence},
	}
	for _, testCase := range cases {
		t.Run(testCase.upstream, func(t *testing.T) {
			document, _ := scenario{
				evidence:    []evidenceSpec{closingEvidence},
				hunks:       []hunkSpec{closingHunk("h")},
				obligations: []obligationSpec{{id: "CFTEST-001", disposition: OCMUnknown, reason: testCase.upstream}},
			}.run(t)
			item := findItem(t, document, KindIntentChange, "CFTEST-001")
			if !equalStrings(item.Reasons, []string{testCase.reason}) {
				t.Fatalf("reasons %v, want %v", item.Reasons, []string{testCase.reason})
			}
			if item.AuthorityClass != AuthorityNone || len(item.RelatedIDs) != 0 {
				t.Fatalf("authority %s with %d related ids, want NONE with none", item.AuthorityClass, len(item.RelatedIDs))
			}
			if item.ResolutionClass != ResolutionActionable || item.NextAction != ActionLinkObligation {
				t.Fatalf("resolution %s / action %s, want ACTIONABLE / LINK_OBLIGATION", item.ResolutionClass, item.NextAction)
			}
			if len(itemsOfKind(document, KindIntentTest)) != 0 {
				t.Fatal("an OCM unknown obligation must not fabricate an intent-test item")
			}
		})
	}
}

// CF-V0-012: a linked obligation with no lexical candidate carries
// NO_MATERIAL_CHANGE_WITNESS over every structurally referenced hunk ID.
func TestLinkedObligationWithoutCandidate(t *testing.T) {
	document, _ := scenario{
		evidence: []evidenceSpec{closingEvidence},
		hunks:    []hunkSpec{closingHunk("h")},
		obligations: []obligationSpec{{
			id: "CFTEST-001", disposition: OCMLinked, statement: "the gizmocatalog rejects duplicates",
			hunks: []string{"h"}, claims: []string{"c1"},
		}},
		claims: map[string][]TCQClaimResult{
			"CFTEST-001": {callerReported("CFTEST-001", "c1", "test-not-matched")},
		},
	}.run(t)
	item := findItem(t, document, KindIntentChange, "CFTEST-001")
	if !equalStrings(item.Reasons, []string{ReasonNoMaterialChangeWitness}) {
		t.Fatalf("reasons %v, want NO_MATERIAL_CHANGE_WITNESS", item.Reasons)
	}
	if !equalStrings(item.RelatedIDs, []string{hunkRef("h")}) {
		t.Fatalf("related ids %v, want every structurally referenced hunk id", item.RelatedIDs)
	}
	if item.ResolutionClass != ResolutionActionable || item.NextAction != ActionLinkMaterialHunk {
		t.Fatalf("resolution %s / action %s, want ACTIONABLE / LINK_MATERIAL_HUNK", item.ResolutionClass, item.NextAction)
	}
}

// CF-V0-012: one or more lexical candidates change the action class but never
// close the item. The candidate is a retrieval aid, never a witness.
func TestLexicalCandidateIsNonclosing(t *testing.T) {
	document, _ := scenario{
		evidence: []evidenceSpec{closingEvidence},
		hunks:    []hunkSpec{closingHunk("h"), closingHunk("other")},
		obligations: []obligationSpec{{
			id: "CFTEST-001", disposition: OCMLinked, statement: "the widgetregistry accepts entries",
			hunks: []string{"h"}, claims: []string{"c1"},
		}},
		claims: map[string][]TCQClaimResult{
			"CFTEST-001": {callerReported("CFTEST-001", "c1", "test-not-matched")},
		},
	}.run(t)
	item := findItem(t, document, KindIntentChange, "CFTEST-001")
	if !equalStrings(item.Reasons, []string{ReasonLexicalCandidateNonclosing}) {
		t.Fatalf("reasons %v, want LEXICAL_CANDIDATE_NONCLOSING", item.Reasons)
	}
	if !equalStrings(item.RelatedIDs, []string{hunkRef("h")}) {
		t.Fatalf("related ids %v, want exactly the candidate hunk ids", item.RelatedIDs)
	}
	if item.AuthorityClass != AuthorityProducerDeclared {
		t.Fatalf("authority %s, want PRODUCER_DECLARED", item.AuthorityClass)
	}
	if item.ResolutionClass != ResolutionAuthorityRequired || item.NextAction != ActionEstablishChangeWitness {
		t.Fatalf("resolution %s / action %s, want AUTHORITY_REQUIRED / ESTABLISH_CHANGE_WITNESS", item.ResolutionClass, item.NextAction)
	}
	if document.FrontierState != StateOpen {
		t.Fatal("a lexical candidate must never close the frontier")
	}
}

// CF-V0-012: with no candidate, a referenced hunk that abstains for the
// term-bound issue selects SUBJECT_TERM_BOUND_EXCEEDED over the two ordinary
// reasons, and relatedIds narrows to the affected hunks.
func TestLinkedObligationTermBoundSelectsProfileRequired(t *testing.T) {
	document, _ := scenario{
		evidence: []evidenceSpec{closingEvidence},
		hunks: []hunkSpec{
			{
				name: "bound", disposition: CEMSupported, reason: "evidence-backed",
				body: manyTerms(300), newPath: "src/bound.go", bases: []string{"spec"},
			},
			{name: "plain", disposition: CEMUnknown, reason: "no-evidence", newPath: "src/plain.go"},
		},
		obligations: []obligationSpec{{
			id: "CFTEST-001", disposition: OCMLinked, statement: "the gizmocatalog rejects duplicates",
			hunks: []string{"bound", "plain"}, claims: []string{"c1"},
		}},
		claims: map[string][]TCQClaimResult{
			"CFTEST-001": {callerReported("CFTEST-001", "c1", "test-not-matched")},
		},
	}.run(t)
	item := findItem(t, document, KindIntentChange, "CFTEST-001")
	if !equalStrings(item.Reasons, []string{ReasonSubjectTermBoundExceeded}) {
		t.Fatalf("reasons %v, want SUBJECT_TERM_BOUND_EXCEEDED", item.Reasons)
	}
	if !equalStrings(item.RelatedIDs, []string{hunkRef("bound")}) {
		t.Fatalf("related ids %v, want exactly the affected hunk ids", item.RelatedIDs)
	}
	if item.ResolutionClass != ResolutionProfileRequired || item.NextAction != ActionDefineSupportedProfile {
		t.Fatalf("resolution %s / action %s, want PROFILE_REQUIRED / DEFINE_SUPPORTED_PROFILE", item.ResolutionClass, item.NextAction)
	}
}

// CF-V0-012: each obligation is evaluated independently even when hunks are
// shared, so one shared hunk can be a candidate for one obligation and not for
// another.
func TestSharedHunkYieldsIndependentObligationResults(t *testing.T) {
	document, _ := scenario{
		evidence: []evidenceSpec{closingEvidence},
		hunks:    []hunkSpec{closingHunk("h")},
		obligations: []obligationSpec{
			{
				id: "CFTEST-001", disposition: OCMLinked, statement: "the widgetregistry accepts entries",
				hunks: []string{"h"}, claims: []string{"c1"},
			},
			{
				id: "CFTEST-002", disposition: OCMLinked, statement: "the gizmocatalog rejects duplicates",
				hunks: []string{"h"}, claims: []string{"c2"},
			},
		},
		claims: map[string][]TCQClaimResult{
			"CFTEST-001": {callerReported("CFTEST-001", "c1", "test-not-matched")},
			"CFTEST-002": {callerReported("CFTEST-002", "c2", "test-not-matched")},
		},
	}.run(t)
	first := findItem(t, document, KindIntentChange, "CFTEST-001")
	second := findItem(t, document, KindIntentChange, "CFTEST-002")
	if !equalStrings(first.Reasons, []string{ReasonLexicalCandidateNonclosing}) {
		t.Fatalf("first reasons %v, want LEXICAL_CANDIDATE_NONCLOSING", first.Reasons)
	}
	if !equalStrings(second.Reasons, []string{ReasonNoMaterialChangeWitness}) {
		t.Fatalf("second reasons %v, want NO_MATERIAL_CHANGE_WITNESS", second.Reasons)
	}
}

// CF-V0-016: the INTENT_TEST reason union is stored in the frozen priority
// order and the FIRST retained reason alone selects the action.
func TestIntentTestReasonPriority(t *testing.T) {
	cases := []struct {
		name       string
		reasons    []string
		want       []string
		resolution string
		action     string
	}{
		{"unmatched", []string{"test-not-matched"}, []string{ReasonTestNotMatched}, ResolutionActionable, ActionSupplyTestObservation},
		{"cleanliness", []string{"test-failed", "target-cleanliness-not-attested"},
			[]string{ReasonTargetCleanlinessNotAttested, ReasonTestFailed}, ResolutionActionable, ActionRerunTestCommand},
		{"command", []string{"test-error", "command-failed"},
			[]string{ReasonCommandFailed, ReasonTestError}, ResolutionActionable, ActionRerunTestCommand},
		{"identity", []string{"repeated-test-rows", "execution-identity-ambiguous"},
			[]string{ReasonTestIdentityAmbiguous}, ResolutionActionable, ActionDisambiguateTestIdentity},
		{"rowidentity", []string{"row-identity-unavailable"}, []string{ReasonTestRowIdentityUnavailable}, ResolutionActionable, ActionSupplyTestObservation},
		{"empty", []string{"empty-body"}, []string{ReasonTestClaimEmpty}, ResolutionActionable, ActionRepairTestClaim},
		{"skip", []string{"unconditional-skip"}, []string{ReasonTestClaimUnconditionalSkip}, ResolutionActionable, ActionRepairTestClaim},
		{"unassociated", []string{"python-offset-mismatch", "claim-association-ambiguous", "unparseable-test-unit", "claim-association-missing"},
			[]string{ReasonTestClaimUnassociated}, ResolutionActionable, ActionRepairTestClaim},
		{"unsupported", []string{"unsupported-python-grammar"}, []string{ReasonTestClaimUnsupported}, ResolutionProfileRequired, ActionDefineSupportedProfile},
		{"skipped", []string{"test-skipped"}, []string{ReasonTestSkipped}, ResolutionActionable, ActionFixOrRerunTest},
		{"unsupported-anchor-after-lower", []string{"unsupported-anchor-profile", "test-failed"},
			[]string{ReasonTestFailed, ReasonTestClaimUnsupported}, ResolutionActionable, ActionFixOrRerunTest},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			document, _ := scenario{
				evidence: []evidenceSpec{closingEvidence},
				hunks:    []hunkSpec{closingHunk("h")},
				obligations: []obligationSpec{{
					id: "CFTEST-001", disposition: OCMLinked, statement: "the widgetregistry accepts entries",
					hunks: []string{"h"}, claims: []string{"c1"},
				}},
				claims: map[string][]TCQClaimResult{
					"CFTEST-001": {callerReported("CFTEST-001", "c1", testCase.reasons...)},
				},
			}.run(t)
			item := findItem(t, document, KindIntentTest, "CFTEST-001")
			if !equalStrings(item.Reasons, testCase.want) {
				t.Fatalf("reasons %v, want %v", item.Reasons, testCase.want)
			}
			if item.ResolutionClass != testCase.resolution || item.NextAction != testCase.action {
				t.Fatalf("resolution %s / action %s, want %s / %s",
					item.ResolutionClass, item.NextAction, testCase.resolution, testCase.action)
			}
		})
	}
}

// CF-V0-014: an exact passing TCQ caller report emits
// CALLER_REPORTED_NONCLOSING under AUTHORITY_REQUIRED and leaves the frontier
// OPEN. Nothing about the report can upgrade that authority.
func TestPassingTCQReportNeverCloses(t *testing.T) {
	document, _ := scenario{
		evidence: []evidenceSpec{closingEvidence},
		hunks:    []hunkSpec{closingHunk("h")},
		obligations: []obligationSpec{{
			id: "CFTEST-001", disposition: OCMLinked, statement: "the widgetregistry accepts entries",
			hunks: []string{"h"}, claims: []string{"c1"},
		}},
		claims: map[string][]TCQClaimResult{
			"CFTEST-001": {{
				ObligationID: "CFTEST-001", ClaimID: claimRef("c1"),
				Relation: tcqRelation, AuthorityClass: AuthorityCallerReported,
			}},
		},
	}.run(t)
	item := findItem(t, document, KindIntentTest, "CFTEST-001")
	if !equalStrings(item.Reasons, []string{ReasonCallerReportedNonclosing}) {
		t.Fatalf("reasons %v, want CALLER_REPORTED_NONCLOSING", item.Reasons)
	}
	if item.AuthorityClass != AuthorityCallerReported {
		t.Fatalf("authority %s, want CALLER_REPORTED", item.AuthorityClass)
	}
	if item.ResolutionClass != ResolutionAuthorityRequired || item.NextAction != ActionEstablishHarnessAuthority {
		t.Fatalf("resolution %s / action %s, want AUTHORITY_REQUIRED / ESTABLISH_HARNESS_AUTHORITY", item.ResolutionClass, item.NextAction)
	}
	if document.FrontierState != StateOpen || document.ExitCode() != 1 {
		t.Fatalf("state %s exit %d, want OPEN and exit 1", document.FrontierState, document.ExitCode())
	}
}

// CF-V0-015: relatedIds are exactly all OCM-selected claim IDs for the
// obligation, sorted and deduplicated, whatever each TCQ edge reported.
func TestIntentTestRelatedIDsCoverEverySelectedClaim(t *testing.T) {
	document, _ := scenario{
		evidence: []evidenceSpec{closingEvidence},
		hunks:    []hunkSpec{closingHunk("h")},
		obligations: []obligationSpec{{
			id: "CFTEST-001", disposition: OCMLinked, statement: "the widgetregistry accepts entries",
			hunks: []string{"h"}, claims: []string{"c2", "c1", "c2"},
		}},
		claims: map[string][]TCQClaimResult{
			"CFTEST-001": {
				callerReported("CFTEST-001", "c1", "test-not-matched"),
				callerReported("CFTEST-001", "c2", "empty-body"),
			},
		},
	}.run(t)
	item := findItem(t, document, KindIntentTest, "CFTEST-001")
	want := sortedUnique([]string{claimRef("c1"), claimRef("c2")})
	if !equalStrings(item.RelatedIDs, want) {
		t.Fatalf("related ids %v, want %v", item.RelatedIDs, want)
	}
	if len(item.Reasons) != 2 {
		t.Fatalf("reasons %v, want both selected edges retained", item.Reasons)
	}
}

// CF-V0-015: a producer that reports anything other than CALLER_REPORTED is
// refused rather than recorded, so authority cannot be silently upgraded.
func TestTCQAuthorityUpgradeRefused(t *testing.T) {
	_, _, err := scenario{
		evidence: []evidenceSpec{closingEvidence},
		hunks:    []hunkSpec{closingHunk("h")},
		obligations: []obligationSpec{{
			id: "CFTEST-001", disposition: OCMLinked, statement: "the widgetregistry accepts entries",
			hunks: []string{"h"}, claims: []string{"c1"},
		}},
		claims: map[string][]TCQClaimResult{
			"CFTEST-001": {{
				ObligationID: "CFTEST-001", ClaimID: claimRef("c1"),
				Reasons: []string{"test-not-matched"}, AuthorityClass: "HARNESS_ATTESTED",
			}},
		},
	}.compute(t)
	if CodeOf(err) != CodeInternalError {
		t.Fatalf("code %q, want %q", CodeOf(err), CodeInternalError)
	}
}

// CF-V0-014: a relation outside the one `tcq/0` defines is refused, never
// reinterpreted as a stronger authority.
func TestUnknownTCQRelationRefused(t *testing.T) {
	_, _, err := scenario{
		evidence: []evidenceSpec{closingEvidence},
		hunks:    []hunkSpec{closingHunk("h")},
		obligations: []obligationSpec{{
			id: "CFTEST-001", disposition: OCMLinked, statement: "the widgetregistry accepts entries",
			hunks: []string{"h"}, claims: []string{"c1"},
		}},
		claims: map[string][]TCQClaimResult{
			"CFTEST-001": {{
				ObligationID: "CFTEST-001", ClaimID: claimRef("c1"),
				Relation: "harness-executed-v1", AuthorityClass: AuthorityCallerReported,
			}},
		},
	}.compute(t)
	if CodeOf(err) != CodeInternalError {
		t.Fatalf("code %q, want %q", CodeOf(err), CodeInternalError)
	}
}

// CF-V0-018 and CF-V0-020: every linked obligation emits BOTH intent items, and
// items are grouped in the frozen kind order.
func TestLinkedObligationEmitsBothIntentItemsInKindOrder(t *testing.T) {
	document, _ := scenario{
		evidence: []evidenceSpec{closingEvidence},
		hunks: []hunkSpec{
			{name: "u", disposition: CEMUnknown, reason: "no-evidence", newPath: "src/u.go"},
			closingHunk("h"),
		},
		obligations: []obligationSpec{{
			id: "CFTEST-001", disposition: OCMLinked, statement: "the widgetregistry accepts entries",
			hunks: []string{"h"}, claims: []string{"c1"},
		}},
		claims: map[string][]TCQClaimResult{
			"CFTEST-001": {callerReported("CFTEST-001", "c1", "test-not-matched")},
		},
	}.run(t)
	kinds := []string{}
	for _, item := range document.Items {
		kinds = append(kinds, item.Kind)
	}
	want := []string{KindHunkBasis, KindIntentChange, KindIntentTest}
	if !equalStrings(kinds, want) {
		t.Fatalf("kinds %v, want %v", kinds, want)
	}
	if strings.Contains(string(mustBytes(t, document)), "stop") {
		t.Fatal("the wire must carry no stop-decision field")
	}
}

func mustBytes(t *testing.T, document Document) []byte {
	t.Helper()
	encoded, err := CanonicalBytes(document)
	if err != nil {
		t.Fatalf("canonical bytes failed: %v", err)
	}
	return encoded
}
