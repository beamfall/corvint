package docviews

import (
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/doccompiler"
)

var (
	digestPattern   = regexp.MustCompile(`^[0-9a-f]{64}$`)
	revisionPattern = regexp.MustCompile(`^(?:[0-9a-f]{40}|[0-9a-f]{64})$`)
)

func Verify(source SourcePlan, bundle Bundle) VerificationReport {
	collector := &divergenceCollector{}
	plan, err := verifiedPlan(source)
	if err != nil {
		collector.add("INVALID_PLAN", "", "")
	}
	planSHA256 := sha256Hex(source.Plan)
	if bundle.Profile != BundleProfile {
		collector.add("INVALID_PROFILE", "", "")
	}
	validateTruth(bundle.Truth, collector)
	computedTruthSHA256, err := truthDigest(bundle.Truth)
	if err != nil {
		collector.add("INVALID_TRUTH", "", "")
	}
	if bundle.TruthSHA256 != computedTruthSHA256 {
		collector.add("TRUTH_DIGEST_MISMATCH", "", "")
	}
	validatePlanPreservation(plan, planSHA256, bundle.Truth, collector)
	validateViews(bundle, computedTruthSHA256, collector)
	encoded, encodeErr := doccompiler.CanonicalJSONUnbounded(bundle)
	if encodeErr != nil {
		collector.add("INVALID_TRUTH", "", "")
	} else if len(encoded) > maxBundleBytes {
		collector.add("LIMIT_EXCEEDED", "", "")
	}
	collector.sortAndUnique()
	status := "PASS"
	if len(collector.values) > 0 {
		status = "FAIL"
	}
	return VerificationReport{
		Divergences: collector.values, PlanSHA256: planSHA256, Profile: VerificationProfile,
		Status: status, TruthSHA256: computedTruthSHA256,
	}
}

func validatePlanPreservation(plan doccompiler.AdmittedPlan, planSHA256 string, truth TruthCorpus, collector *divergenceCollector) {
	if truth.PlanSHA256 != planSHA256 || truth.Revision != plan.SourceIdentity.Revision {
		collector.add("PLAN_BINDING_MISMATCH", "", "")
	}
	planClaims := make(map[string]doccompiler.Clause, len(plan.Clauses))
	for _, claim := range plan.Clauses {
		planClaims[claim.ID] = claim
	}
	truthClaims := make(map[string]TruthClaim, len(truth.Claims))
	for _, claim := range truth.Claims {
		truthClaims[claim.ID] = claim
		expected, exists := planClaims[claim.ID]
		if !exists {
			collector.add("UNKNOWN_PLAN_CLAIM", "", claim.ID)
			continue
		}
		if !claimPreserved(expected, claim) {
			collector.add("PLAN_CLAIM_MISMATCH", "", claim.ID)
		}
	}
	for id := range planClaims {
		if _, exists := truthClaims[id]; !exists {
			collector.add("MISSING_PLAN_CLAIM", "", id)
		}
	}
}

func claimPreserved(expected doccompiler.Clause, actual TruthClaim) bool {
	if expected.ID != actual.ID || expected.State != actual.State || expected.Kind != actual.Kind || expected.Scope != actual.Scope {
		return false
	}
	if expected.Text != actual.Text || expected.Frontier != actual.Frontier || len(expected.Anchors) != len(actual.Evidence) {
		return false
	}
	evidence := sortedEvidence(expected.Anchors)
	for index := range evidence {
		if evidence[index] != actual.Evidence[index] {
			return false
		}
	}
	return true
}

func validateTruth(truth TruthCorpus, collector *divergenceCollector) {
	if truth.Profile != TruthProfile {
		collector.add("INVALID_PROFILE", "", "")
	}
	if !digestValid(truth.PlanSHA256) || !digestValid(truth.DisclosurePolicySHA256) || !revisionValid(truth.Revision) || !tokenValid(truth.DisclosureScope, 128) {
		collector.add("INVALID_TRUTH", "", "")
	}
	if len(truth.Claims) > maxClaims {
		collector.add("LIMIT_EXCEEDED", "", "")
	}
	seen := make(map[string]struct{}, len(truth.Claims))
	previous := ""
	for index, claim := range truth.Claims {
		if !claimIDValid(claim.ID) || !claimTextValid(claim.Text) || !claimShapeValid(claim) {
			collector.add("INVALID_TRUTH", "", claim.ID)
		}
		if _, exists := seen[claim.ID]; exists {
			collector.add("DUPLICATE_CLAIM", "", claim.ID)
		}
		seen[claim.ID] = struct{}{}
		if index > 0 && previous >= claim.ID {
			collector.add("NONCANONICAL_ORDER", "", claim.ID)
		}
		previous = claim.ID
		if !currencyValid(claim.Currency) {
			collector.add("INVALID_CURRENCY", "", claim.ID)
		} else if claim.Currency == CurrencyCurrent && claim.CurrencyReason != "" {
			collector.add("INVALID_CURRENCY", "", claim.ID)
		} else if claim.Currency != CurrencyCurrent && !claimTextValid(claim.CurrencyReason) {
			collector.add("MISSING_CURRENCY_REASON", "", claim.ID)
		}
		if !reviewStateValid(claim.ReviewState) {
			collector.add("INVALID_REVIEW_STATE", "", claim.ID)
		}
		validateCanonicalLimitations(claim, collector)
		validateCanonicalEvidence(claim, collector)
	}
}

func validateCanonicalLimitations(claim TruthClaim, collector *divergenceCollector) {
	if len(claim.Limitations) > maxLimitationsPerClaim {
		collector.add("LIMIT_EXCEEDED", "", claim.ID)
	}
	previous := ""
	for index, limitation := range claim.Limitations {
		if !boundedTextValid(limitation, maxLimitationBytes) {
			collector.add("INVALID_TRUTH", "", claim.ID)
		}
		if index > 0 && previous >= limitation {
			collector.add("NONCANONICAL_ORDER", "", claim.ID)
		}
		previous = limitation
	}
}

func validateCanonicalEvidence(claim TruthClaim, collector *divergenceCollector) {
	if len(claim.Evidence) > maxEvidencePerClaim {
		collector.add("LIMIT_EXCEEDED", "", claim.ID)
	}
	previous := ""
	for index, evidence := range claim.Evidence {
		if !evidenceValid(evidence) {
			collector.add("INVALID_TRUTH", "", claim.ID)
		}
		key := evidenceKey(evidence)
		if index > 0 && previous > key {
			collector.add("NONCANONICAL_ORDER", "", claim.ID)
		}
		previous = key
	}
}

func validateViews(bundle Bundle, computedTruthSHA256 string, collector *divergenceCollector) {
	if len(bundle.Views) != len(audienceOrder) {
		collector.add("MISSING_AUDIENCE", "", "")
	}
	knownClaims := make(map[string]struct{}, len(bundle.Truth.Claims))
	for _, claim := range bundle.Truth.Claims {
		knownClaims[claim.ID] = struct{}{}
	}
	seenAudiences := make(map[Audience]struct{}, len(bundle.Views))
	for index, view := range bundle.Views {
		if !audienceValid(view.Audience) {
			collector.add("INVALID_AUDIENCE", view.Audience, "")
		}
		if _, exists := seenAudiences[view.Audience]; exists {
			collector.add("DUPLICATE_AUDIENCE", view.Audience, "")
		}
		seenAudiences[view.Audience] = struct{}{}
		if index >= len(audienceOrder) || view.Audience != audienceOrder[index] {
			collector.add("NONCANONICAL_ORDER", view.Audience, "")
		}
		if view.Profile != ViewProfile {
			collector.add("INVALID_PROFILE", view.Audience, "")
		}
		if !detailValid(view.Detail) {
			collector.add("INVALID_DETAIL", view.Audience, "")
		}
		if view.TruthSHA256 != computedTruthSHA256 {
			collector.add("TRUTH_DIGEST_MISMATCH", view.Audience, "")
		}
		if view.DisclosurePolicySHA256 != bundle.Truth.DisclosurePolicySHA256 || view.DisclosureScope != bundle.Truth.DisclosureScope {
			collector.add("POLICY_SCOPE_MISMATCH", view.Audience, "")
		}
		seenClaims := make(map[string]struct{}, len(view.ClaimIDs))
		for _, id := range view.ClaimIDs {
			if _, exists := knownClaims[id]; !exists {
				collector.add("UNKNOWN_CLAIM", view.Audience, id)
			}
			if _, exists := seenClaims[id]; exists {
				collector.add("DUPLICATE_CLAIM", view.Audience, id)
			}
			seenClaims[id] = struct{}{}
		}
		for id := range knownClaims {
			if _, exists := seenClaims[id]; !exists {
				collector.add("MISSING_CLAIM", view.Audience, id)
			}
		}
	}
	for _, audience := range audienceOrder {
		if _, exists := seenAudiences[audience]; !exists {
			collector.add("MISSING_AUDIENCE", audience, "")
		}
	}
}

type divergenceCollector struct {
	values []Divergence
}

func (collector *divergenceCollector) add(code string, audience Audience, claimID string) {
	collector.values = append(collector.values, Divergence{Audience: audience, ClaimID: claimID, Code: code})
}

func (collector *divergenceCollector) sortAndUnique() {
	sort.Slice(collector.values, func(left, right int) bool {
		leftRank := audienceRank(collector.values[left].Audience)
		rightRank := audienceRank(collector.values[right].Audience)
		if leftRank != rightRank {
			return leftRank < rightRank
		}
		// Every invalid audience shares one rank; its spelling keeps equal
		// divergences adjacent for the dedup below.
		if collector.values[left].Audience != collector.values[right].Audience {
			return collector.values[left].Audience < collector.values[right].Audience
		}
		if collector.values[left].ClaimID != collector.values[right].ClaimID {
			return collector.values[left].ClaimID < collector.values[right].ClaimID
		}
		return collector.values[left].Code < collector.values[right].Code
	})
	result := collector.values[:0]
	for _, value := range collector.values {
		if len(result) == 0 || result[len(result)-1] != value {
			result = append(result, value)
		}
	}
	collector.values = result
}

func audienceRank(audience Audience) int {
	if audience == "" {
		return -1
	}
	for index, candidate := range audienceOrder {
		if audience == candidate {
			return index
		}
	}
	return len(audienceOrder)
}

func audienceValid(value Audience) bool {
	return audienceRank(value) >= 0 && audienceRank(value) < len(audienceOrder)
}

func detailValid(value Detail) bool {
	return value == DetailBrief || value == DetailStandard || value == DetailFull
}

func currencyValid(value Currency) bool {
	return value == CurrencyCurrent || value == CurrencyStale || value == CurrencyUnknown
}

func reviewStateValid(value ReviewState) bool {
	return value == ReviewGenerated || value == ReviewVerified || value == ReviewReviewed
}

func digestValid(value string) bool { return digestPattern.MatchString(value) }

func revisionValid(value string) bool { return revisionPattern.MatchString(value) }

func claimIDValid(value string) bool {
	return tokenValid(value, maxClaimIDBytes)
}

func tokenValid(value string, maximum int) bool {
	if value == "" || len(value) > maximum || !utf8.ValidString(value) || strings.TrimSpace(value) != value {
		return false
	}
	for _, character := range value {
		if character == 0 || character == '\n' || character == '\r' || character == '\t' {
			return false
		}
	}
	return true
}

func claimTextValid(value string) bool { return boundedTextValid(value, maxTextBytes) }

func boundedTextValid(value string, maximum int) bool {
	return value != "" && len(value) <= maximum && utf8.ValidString(value) && strings.IndexByte(value, 0) < 0
}

// evidenceValid bounds each anchor member the truth corpus copies. It does not
// re-qualify anchor identity, authority, range, or staleness (CATN-V0-014): a
// verified plan keeps HDCV0-023 disqualified anchors as review context, and
// only the admitted claim state says whether an anchor qualified (decision 0268).
func evidenceValid(value TruthEvidence) bool {
	for _, member := range []string{value.Authority, value.Reason, value.Path, value.Blob, value.SpanSHA256} {
		if len(member) > maxTextBytes || !utf8.ValidString(member) {
			return false
		}
	}
	return true
}

var (
	claimStates = map[string]bool{doccompiler.ClauseSupported: true, doccompiler.ClauseConflicted: true, doccompiler.ClauseUnknown: true}
	claimKinds  = map[string]bool{doccompiler.KindPrescriptive: true, doccompiler.KindDescriptive: true}
	claimScopes = map[string]bool{doccompiler.ScopeObserved: true, doccompiler.ScopeGeneral: true}
	frontiers   = map[string]bool{
		doccompiler.FrontierNoQualifyingSource: true, doccompiler.FrontierStaleAnchor: true, doccompiler.FrontierResolverUndecided: true,
	}
)

// claimShapeValid checks the admitted clause vocabulary a truth claim copies:
// closed state, kind, and scope; a frontier exactly on UNKNOWN; evidence on
// SUPPORTED; and two distinct locations on CONFLICTED (CATN-V0-014).
func claimShapeValid(claim TruthClaim) bool {
	if !claimStates[claim.State] || !claimKinds[claim.Kind] || !claimScopes[claim.Scope] {
		return false
	}
	if claim.State == doccompiler.ClauseUnknown {
		return frontiers[claim.Frontier]
	}
	if claim.Frontier != "" {
		return false
	}
	if claim.State == doccompiler.ClauseSupported {
		return len(claim.Evidence) > 0
	}
	return conflictAnchored(claim.Evidence)
}

// conflictAnchored reports whether evidence holds at least two distinct
// (path, blob, start_line, end_line) locations, which CONFLICTED requires.
func conflictAnchored(evidence []TruthEvidence) bool {
	type location struct {
		path, blob         string
		startLine, endLine int
	}
	locations := make(map[location]struct{}, len(evidence))
	for _, anchor := range evidence {
		locations[location{anchor.Path, anchor.Blob, anchor.StartLine, anchor.EndLine}] = struct{}{}
	}
	return len(locations) >= 2
}

func evidenceKey(value TruthEvidence) string {
	return strings.Join([]string{
		value.Path, value.Blob, value.SpanSHA256,
		strconv.Itoa(value.StartLine), strconv.Itoa(value.EndLine),
		value.Authority, value.Reason,
	}, "\x00")
}
