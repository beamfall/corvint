package frontier

import (
	"sort"

	"github.com/Beamfall/corvint/internal/cem/wire"
	"github.com/Beamfall/corvint/internal/lrf"
)

// disposition is the (resolutionClass, nextAction) pair one reason implies.
// Dispatch is table-driven rather than branched so that adding a reason
// without deciding its action is a compile-visible omission.
type disposition struct {
	resolution string
	action     string
}

// hunkReasonOrder is the stored order for HUNK_BASIS reasons (CF-V0-010,
// referenced by CF-V0-016). The three CEM-unknown reasons lead because a
// CEM `unknown` hunk has an empty basis and therefore never shares an item
// with a basis reason; the remaining four are the exact CF-V0-010 union order.
// DELETION_RELATION_REQUIRED precedes them for the same exclusivity reason: a
// deletion abstains on every basis before term extraction can run.
var hunkReasonOrder = []string{
	ReasonHunkNoEvidence,
	ReasonHunkInsufficientEvidence,
	ReasonHunkConflictingEvidence,
	ReasonDeletionRelationRequired,
	ReasonSubjectTermBoundExceeded,
	ReasonEvidenceSpanTooBroad,
	ReasonSelfReferentialBasis,
	ReasonInsufficientLexicalSupport,
}

// intentChangeReasonOrder records CF-V0-016's statement that INTENT_CHANGE
// selects SUBJECT_TERM_BOUND_EXCEEDED before its two ordinary reasons. The
// item itself has exactly one reason (CF-V0-012), so this order is a
// validation invariant rather than a runtime tie-break.
var intentChangeReasonOrder = []string{
	ReasonObligationUnassessed,
	ReasonObligationNoTestClaim,
	ReasonObligationInsufficientEvidence,
	ReasonObligationConflictingEvidence,
	ReasonSubjectTermBoundExceeded,
	ReasonLexicalCandidateNonclosing,
	ReasonNoMaterialChangeWitness,
}

// intentTestReasonOrder is the frozen CF-V0-016 priority table, in table
// order. Reasons are stored in this order, not lexical order, after taking the
// union across every selected claim edge.
var intentTestReasonOrder = []string{
	ReasonCallerReportedNonclosing,
	ReasonTargetCleanlinessNotAttested,
	ReasonCommandFailed,
	ReasonTestError,
	ReasonTestFailed,
	ReasonTestSkipped,
	ReasonTestIdentityAmbiguous,
	ReasonTestRowIdentityUnavailable,
	ReasonTestNotMatched,
	ReasonTestClaimEmpty,
	ReasonTestClaimUnconditionalSkip,
	ReasonTestClaimUnassociated,
	ReasonTestClaimUnsupported,
}

// reasonOrders and reasonDispositions are keyed by item kind because
// SUBJECT_TERM_BOUND_EXCEEDED is legitimately shared by two kinds.
var reasonOrders = map[string][]string{
	KindHunkBasis:    hunkReasonOrder,
	KindIntentChange: intentChangeReasonOrder,
	KindIntentTest:   intentTestReasonOrder,
}

var reasonDispositions = map[string]map[string]disposition{
	KindHunkBasis: {
		ReasonHunkNoEvidence:             {ResolutionActionable, ActionSupplyHunkBasis},
		ReasonHunkInsufficientEvidence:   {ResolutionActionable, ActionSupplyHunkBasis},
		ReasonHunkConflictingEvidence:    {ResolutionActionable, ActionSupplyHunkBasis},
		ReasonDeletionRelationRequired:   {ResolutionProfileRequired, ActionDefineSupportedProfile},
		ReasonSubjectTermBoundExceeded:   {ResolutionProfileRequired, ActionDefineSupportedProfile},
		ReasonEvidenceSpanTooBroad:       {ResolutionActionable, ActionNarrowOrReplaceBasis},
		ReasonSelfReferentialBasis:       {ResolutionActionable, ActionNarrowOrReplaceBasis},
		ReasonInsufficientLexicalSupport: {ResolutionActionable, ActionNarrowOrReplaceBasis},
	},
	KindIntentChange: {
		ReasonObligationUnassessed:           {ResolutionActionable, ActionLinkObligation},
		ReasonObligationNoTestClaim:          {ResolutionActionable, ActionLinkObligation},
		ReasonObligationInsufficientEvidence: {ResolutionActionable, ActionLinkObligation},
		ReasonObligationConflictingEvidence:  {ResolutionActionable, ActionLinkObligation},
		ReasonSubjectTermBoundExceeded:       {ResolutionProfileRequired, ActionDefineSupportedProfile},
		ReasonLexicalCandidateNonclosing:     {ResolutionAuthorityRequired, ActionEstablishChangeWitness},
		ReasonNoMaterialChangeWitness:        {ResolutionActionable, ActionLinkMaterialHunk},
	},
	KindIntentTest: {
		ReasonCallerReportedNonclosing:     {ResolutionAuthorityRequired, ActionEstablishHarnessAuthority},
		ReasonTargetCleanlinessNotAttested: {ResolutionActionable, ActionRerunTestCommand},
		ReasonCommandFailed:                {ResolutionActionable, ActionRerunTestCommand},
		ReasonTestError:                    {ResolutionActionable, ActionFixOrRerunTest},
		ReasonTestFailed:                   {ResolutionActionable, ActionFixOrRerunTest},
		ReasonTestSkipped:                  {ResolutionActionable, ActionFixOrRerunTest},
		ReasonTestIdentityAmbiguous:        {ResolutionActionable, ActionDisambiguateTestIdentity},
		ReasonTestRowIdentityUnavailable:   {ResolutionActionable, ActionSupplyTestObservation},
		ReasonTestNotMatched:               {ResolutionActionable, ActionSupplyTestObservation},
		ReasonTestClaimEmpty:               {ResolutionActionable, ActionRepairTestClaim},
		ReasonTestClaimUnconditionalSkip:   {ResolutionActionable, ActionRepairTestClaim},
		ReasonTestClaimUnassociated:        {ResolutionActionable, ActionRepairTestClaim},
		ReasonTestClaimUnsupported:         {ResolutionProfileRequired, ActionDefineSupportedProfile},
	},
}

// cemUnknownReasons is the exact CF-V0-009 map. A CEM unknown reason outside
// this closed set is an unadmitted upstream value, not a silently dropped one.
var cemUnknownReasons = map[string]string{
	"no-evidence":           ReasonHunkNoEvidence,
	"insufficient-evidence": ReasonHunkInsufficientEvidence,
	"conflicting-evidence":  ReasonHunkConflictingEvidence,
}

// ocmUnknownReasons is the exact CF-V0-011 map.
var ocmUnknownReasons = map[string]string{
	"unassessed":            ReasonObligationUnassessed,
	"no-test-claim":         ReasonObligationNoTestClaim,
	"insufficient-evidence": ReasonObligationInsufficientEvidence,
	"conflicting-evidence":  ReasonObligationConflictingEvidence,
}

// lrfBasisReasons is the exact CF-V0-010 map from LRF `cem-basis` issue codes.
var lrfBasisReasons = map[string]string{
	"deletion-relation-required":   ReasonDeletionRelationRequired,
	"subject-term-bound-exceeded":  ReasonSubjectTermBoundExceeded,
	"evidence-span-too-broad":      ReasonEvidenceSpanTooBroad,
	"self-referential-basis":       ReasonSelfReferentialBasis,
	"insufficient-lexical-support": ReasonInsufficientLexicalSupport,
}

// tcqReasons is the exact CF-V0-016 TCQ mapping. Two upstream codes share
// TEST_IDENTITY_AMBIGUOUS and four share TEST_CLAIM_UNASSOCIATED, exactly as
// the clause specifies; the union then dedupes them.
var tcqReasons = map[string]string{
	"target-cleanliness-not-attested": ReasonTargetCleanlinessNotAttested,
	"command-failed":                  ReasonCommandFailed,
	"test-error":                      ReasonTestError,
	"test-failed":                     ReasonTestFailed,
	"test-skipped":                    ReasonTestSkipped,
	"execution-identity-ambiguous":    ReasonTestIdentityAmbiguous,
	"repeated-test-rows":              ReasonTestIdentityAmbiguous,
	"row-identity-unavailable":        ReasonTestRowIdentityUnavailable,
	"test-not-matched":                ReasonTestNotMatched,
	"empty-body":                      ReasonTestClaimEmpty,
	"unconditional-skip":              ReasonTestClaimUnconditionalSkip,
	"claim-association-missing":       ReasonTestClaimUnassociated,
	"claim-association-ambiguous":     ReasonTestClaimUnassociated,
	"python-offset-mismatch":          ReasonTestClaimUnassociated,
	"unparseable-test-unit":           ReasonTestClaimUnassociated,
	"unsupported-anchor-profile":      ReasonTestClaimUnsupported,
	"unsupported-python-grammar":      ReasonTestClaimUnsupported,
}

// tcqRelation is the only relation `tcq/0` defines. CF-V0-014 freezes it as
// non-closing: nothing — not a signature, a clean-target statement, a passing
// row, or exit zero — upgrades it, so it maps to a reason rather than to a
// removed item.
const tcqRelation = "test-report-matched-v0"

// lrfIndex is the recomputed LRF result, indexed for the closure table. It is
// built once per invocation so the decision table never rescans the tuples.
type lrfIndex struct {
	witnessedHunks map[string]bool
	basisIssues    map[string][]basisIssue
	obligationEdge map[string]map[string]string
	obligationCode map[string]map[string]string
}

type basisIssue struct {
	evidenceID string
	code       string
}

func indexLRF(result lrf.Result) lrfIndex {
	index := lrfIndex{
		witnessedHunks: map[string]bool{},
		basisIssues:    map[string][]basisIssue{},
		obligationEdge: map[string]map[string]string{},
		obligationCode: map[string]map[string]string{},
	}
	for _, row := range result.Results() {
		switch row[0] {
		case "cem-basis":
			if row[5] == "cem-lexical-v0" {
				index.witnessedHunks[row[1]] = true
			}
		case "ocm-hunk":
			putNested(index.obligationEdge, row[1], row[2], row[5])
		}
	}
	for _, row := range result.Issues() {
		switch row[0] {
		case "cem-basis":
			index.basisIssues[row[1]] = append(index.basisIssues[row[1]], basisIssue{evidenceID: row[2], code: row[4]})
		case "ocm-hunk":
			putNested(index.obligationCode, row[1], row[2], row[4])
		}
	}
	return index
}

func putNested(target map[string]map[string]string, outer, inner, value string) {
	if target[outer] == nil {
		target[outer] = map[string]string{}
	}
	target[outer][inner] = value
}

// tcqIndex groups selected claim edges by obligation. CF-V0-015 requires every
// selected edge to be evaluated, so nothing here filters.
func indexTCQ(result TCQResult) map[string][]TCQClaimResult {
	edges := map[string][]TCQClaimResult{}
	for _, claim := range result.Claims {
		edges[claim.ObligationID] = append(edges[claim.ObligationID], claim)
	}
	return edges
}

// projectItems walks the declared universe once per item kind and emits the
// unresolved obligations in CF-V0-020 order: hunk items in canonical patch
// order, then both intent kinds in frozen intent requirement order.
func projectItems(universeID string, cem *wire.Map, obligations []Obligation, lrfResult lrf.Result, tcq TCQResult) ([]Item, *Error) {
	index := indexLRF(lrfResult)
	edges := indexTCQ(tcq)

	hunkItems, err := projectHunkItems(universeID, cem, index)
	if err != nil {
		return nil, err
	}
	changeItems, err := projectIntentChangeItems(universeID, obligations, index)
	if err != nil {
		return nil, err
	}
	testItems, err := projectIntentTestItems(universeID, obligations, edges)
	if err != nil {
		return nil, err
	}

	items := append(hunkItems, changeItems...)
	items = append(items, testItems...)
	if len(items) > MaxTotalItems {
		return nil, fail(CodeResourceExhausted, "total item ceiling exceeded")
	}
	return items, nil
}

// projectHunkItems implements CF-V0-008, CF-V0-009 and CF-V0-010. A hunk is
// closed only by a reverified mechanical disposition or an existential
// qualifying LRF basis; `supported` alone never closes it.
func projectHunkItems(universeID string, cem *wire.Map, index lrfIndex) ([]Item, *Error) {
	items := make([]Item, 0)
	for _, hunk := range cem.Hunks {
		item, emit, err := hunkItem(universeID, hunk, index)
		if err != nil {
			return nil, err
		}
		if !emit {
			continue
		}
		if len(items) >= MaxHunkItems {
			return nil, fail(CodeResourceExhausted, "hunk item ceiling exceeded")
		}
		items = append(items, item)
	}
	return items, nil
}

func hunkItem(universeID string, hunk wire.Hunk, index lrfIndex) (Item, bool, *Error) {
	switch hunk.Disposition {
	case CEMMechanical:
		// CF-V0-008: the verifier already reverified the mechanical claim
		// (`unproven-mechanical` is its refusal), so a surviving mechanical
		// hunk is closed and emits nothing.
		if !mechanicalReasons[hunk.Reason] {
			return Item{}, false, fail(CodeInternalError, "unadmitted mechanical reason")
		}
		return Item{}, false, nil
	case CEMUnknown:
		reason, admitted := cemUnknownReasons[hunk.Reason]
		if !admitted {
			return Item{}, false, fail(CodeInternalError, "unadmitted cem unknown reason")
		}
		item, err := buildItem(universeID, KindHunkBasis, hunk.ID, []string{reason}, nil, AuthorityNone)
		return item, true, err
	case CEMSupported:
		if index.witnessedHunks[hunk.ID] {
			// A qualifying basis is existential: rejected extra bases stay in
			// the LRF diagnostics and do not reopen the hunk (CF-V0-008).
			return Item{}, false, nil
		}
		item, err := unresolvedSupportedHunk(universeID, hunk, index)
		return item, true, err
	}
	return Item{}, false, fail(CodeInternalError, "unadmitted cem disposition")
}

// unresolvedSupportedHunk builds the CF-V0-010 item: the unique union of every
// rejected or term-bound-abstained basis reason, with related IDs limited to
// the evidence whose edge contributed a retained reason.
func unresolvedSupportedHunk(universeID string, hunk wire.Hunk, index lrfIndex) (Item, *Error) {
	reasons := map[string]bool{}
	related := map[string]bool{}
	for _, issue := range index.basisIssues[hunk.ID] {
		reason, admitted := lrfBasisReasons[issue.code]
		if !admitted {
			return Item{}, fail(CodeInternalError, "unadmitted lrf basis issue code")
		}
		reasons[reason] = true
		related[issue.evidenceID] = true
	}
	return buildItem(universeID, KindHunkBasis, hunk.ID, orderedReasons(KindHunkBasis, reasons), sortedKeys(related), AuthorityProducerDeclared)
}

// projectIntentChangeItems implements CF-V0-011 and CF-V0-012. Each obligation
// is evaluated independently even when hunks are shared.
func projectIntentChangeItems(universeID string, obligations []Obligation, index lrfIndex) ([]Item, *Error) {
	items := make([]Item, 0)
	for _, obligation := range obligations {
		item, err := intentChangeItem(universeID, obligation, index)
		if err != nil {
			return nil, err
		}
		if len(items) >= MaxIntentChangeItems {
			return nil, fail(CodeResourceExhausted, "intent change item ceiling exceeded")
		}
		items = append(items, item)
	}
	return items, nil
}

func intentChangeItem(universeID string, obligation Obligation, index lrfIndex) (Item, *Error) {
	if obligation.Disposition == OCMUnknown {
		reason, admitted := ocmUnknownReasons[obligation.Reason]
		if !admitted {
			return Item{}, fail(CodeInternalError, "unadmitted ocm unknown reason")
		}
		return buildItem(universeID, KindIntentChange, obligation.ID, []string{reason}, nil, AuthorityNone)
	}
	if obligation.Disposition != OCMLinked {
		return Item{}, fail(CodeInternalError, "unadmitted ocm disposition")
	}

	candidates := []string{}
	termBound := []string{}
	for _, hunkID := range obligation.HunkIDs {
		if index.obligationEdge[obligation.ID][hunkID] == "lexically-proximate-candidate" {
			candidates = append(candidates, hunkID)
		}
		if index.obligationCode[obligation.ID][hunkID] == "subject-term-bound-exceeded" {
			termBound = append(termBound, hunkID)
		}
	}

	// CF-V0-012 selects in this exact order. The candidate is a retrieval aid,
	// never a witness, so it changes the action class without closing anything.
	if len(candidates) > 0 {
		return buildItem(universeID, KindIntentChange, obligation.ID,
			[]string{ReasonLexicalCandidateNonclosing}, sortedUnique(candidates), AuthorityProducerDeclared)
	}
	if len(termBound) > 0 {
		return buildItem(universeID, KindIntentChange, obligation.ID,
			[]string{ReasonSubjectTermBoundExceeded}, sortedUnique(termBound), AuthorityProducerDeclared)
	}
	return buildItem(universeID, KindIntentChange, obligation.ID,
		[]string{ReasonNoMaterialChangeWitness}, sortedUnique(obligation.HunkIDs), AuthorityProducerDeclared)
}

// projectIntentTestItems implements CF-V0-015: every structurally linked
// obligation emits exactly one INTENT_TEST item, and an OCM `unknown`
// obligation emits none because no selected test edge exists (CF-V0-011).
func projectIntentTestItems(universeID string, obligations []Obligation, edges map[string][]TCQClaimResult) ([]Item, *Error) {
	items := make([]Item, 0)
	for _, obligation := range obligations {
		if obligation.Disposition != OCMLinked {
			continue
		}
		item, err := intentTestItem(universeID, obligation, edges[obligation.ID])
		if err != nil {
			return nil, err
		}
		if len(items) >= MaxIntentTestItems {
			return nil, fail(CodeResourceExhausted, "intent test item ceiling exceeded")
		}
		items = append(items, item)
	}
	return items, nil
}

func intentTestItem(universeID string, obligation Obligation, claims []TCQClaimResult) (Item, *Error) {
	reasons := map[string]bool{}
	for _, claim := range claims {
		if claim.AuthorityClass != AuthorityCallerReported {
			// CF-V0-015 fixes the class at CALLER_REPORTED for every TCQ V0
			// edge; a different value is a producer trying to upgrade its own
			// authority, which Frontier refuses rather than records.
			return Item{}, fail(CodeInternalError, "unadmitted tcq authority class")
		}
		if err := collectRelationReason(reasons, claim.Relation); err != nil {
			return Item{}, err
		}
		for _, diagnostic := range claim.Reasons {
			reason, admitted := tcqReasons[diagnostic]
			if !admitted {
				return Item{}, fail(CodeInternalError, "unadmitted tcq diagnostic")
			}
			reasons[reason] = true
		}
	}
	return buildItem(universeID, KindIntentTest, obligation.ID,
		orderedReasons(KindIntentTest, reasons), sortedUnique(obligation.ClaimIDs), AuthorityCallerReported)
}

// collectRelationReason applies CF-V0-014: the one TCQ relation maps to the
// highest-priority reason and never removes the item.
func collectRelationReason(reasons map[string]bool, relation string) *Error {
	switch relation {
	case "":
		return nil
	case tcqRelation:
		reasons[ReasonCallerReportedNonclosing] = true
		return nil
	}
	return fail(CodeInternalError, "unadmitted tcq relation")
}

// buildItem derives the item identity and resolves resolutionClass and
// nextAction from the first retained reason (CF-V0-016). Later reasons stay
// visible and cannot alter them.
func buildItem(universeID, kind, subjectID string, reasons, relatedIDs []string, authority string) (Item, *Error) {
	if len(reasons) == 0 {
		return Item{}, fail(CodeNoncanonical, "item has no retained reason")
	}
	if len(reasons) > MaxReasonsPerItem {
		return Item{}, fail(CodeResourceExhausted, "reason ceiling exceeded")
	}
	if len(relatedIDs) > MaxRelatedIDsPerItem {
		return Item{}, fail(CodeResourceExhausted, "related id ceiling exceeded")
	}
	chosen, admitted := reasonDispositions[kind][reasons[0]]
	if !admitted {
		return Item{}, fail(CodeInternalError, "reason has no disposition for its kind")
	}
	identity, err := ItemID(kind, subjectID, universeID)
	if err != nil {
		return Item{}, fail(CodeNoncanonical, "item identity is not derivable")
	}
	if relatedIDs == nil {
		relatedIDs = []string{}
	}
	return Item{
		AuthorityClass:  authority,
		ID:              identity,
		Kind:            kind,
		NextAction:      chosen.action,
		Reasons:         reasons,
		RelatedIDs:      relatedIDs,
		ResolutionClass: chosen.resolution,
		SubjectID:       subjectID,
	}, nil
}

// orderedReasons stores the union in the kind's frozen order, not lexical
// order (CF-V0-016).
func orderedReasons(kind string, present map[string]bool) []string {
	ordered := make([]string, 0, len(present))
	for _, reason := range reasonOrders[kind] {
		if present[reason] {
			ordered = append(ordered, reason)
		}
	}
	return ordered
}

// sortedKeys and sortedUnique produce the lexicographic related-ID order
// CF-V0-020 requires; caller array order can never override it.
func sortedKeys(set map[string]bool) []string {
	keys := make([]string, 0, len(set))
	for key := range set {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func sortedUnique(values []string) []string {
	set := map[string]bool{}
	for _, value := range values {
		set[value] = true
	}
	return sortedKeys(set)
}
