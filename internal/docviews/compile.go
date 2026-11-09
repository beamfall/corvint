package docviews

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"

	"github.com/Beamfall/corvint/internal/doccompiler"
)

var truthDigestDomain = []byte(TruthProfile + "\x00")

func Compile(source SourcePlan, options CompileOptions) (Bundle, error) {
	plan, err := verifiedPlan(source)
	if err != nil {
		return Bundle{}, err
	}
	if !digestValid(options.DisclosurePolicySHA256) || !tokenValid(options.DisclosureScope, 128) {
		return Bundle{}, failure("INVALID_INPUT", "disclosure binding must be closed and immutable")
	}

	claims, err := claimsFromPlan(plan, options.Overlays)
	if err != nil {
		return Bundle{}, err
	}
	truth := TruthCorpus{
		Claims:                 claims,
		DisclosurePolicySHA256: options.DisclosurePolicySHA256,
		DisclosureScope:        options.DisclosureScope,
		PlanSHA256:             sha256Hex(source.Plan),
		Profile:                TruthProfile,
		Revision:               plan.SourceIdentity.Revision,
	}
	truthSHA256, err := truthDigest(truth)
	if err != nil {
		return Bundle{}, err
	}
	views, err := compileViews(truth, truthSHA256, options.Recipes)
	if err != nil {
		return Bundle{}, err
	}
	bundle := Bundle{Profile: BundleProfile, Truth: truth, TruthSHA256: truthSHA256, Views: views}
	if _, _, err := CanonicalBundle(source, bundle); err != nil {
		return Bundle{}, err
	}
	return bundle, nil
}

func CanonicalBundle(source SourcePlan, bundle Bundle) ([]byte, string, error) {
	report := Verify(source, bundle)
	if report.Status != "PASS" {
		divergenceCode := "NON_EQUIVALENT"
		if len(report.Divergences) > 0 {
			divergenceCode = report.Divergences[0].Code
		}
		return nil, "", failure(canonicalFailureCode(report.Divergences), "bundle failed verification: %s", divergenceCode)
	}
	encoded, err := doccompiler.CanonicalJSONUnbounded(bundle)
	if err != nil {
		return nil, "", failure("INTERNAL_ERROR", "cannot encode canonical bundle")
	}
	if len(encoded) > maxBundleBytes {
		return nil, "", failure("LIMIT_EXCEEDED", "canonical bundle exceeds %d bytes", maxBundleBytes)
	}
	return encoded, sha256Hex(encoded), nil
}

func canonicalFailureCode(divergences []Divergence) string {
	if len(divergences) == 0 {
		return "NON_EQUIVALENT"
	}
	for _, divergence := range divergences {
		if divergence.Code != "LIMIT_EXCEEDED" {
			return "NON_EQUIVALENT"
		}
	}
	return "LIMIT_EXCEEDED"
}

// verifiedPlan admits only plan bytes that doccompiler.VerifyAdmittedPlan
// reproduces byte-for-byte, with their patch, at the supplied index.
func verifiedPlan(source SourcePlan) (doccompiler.AdmittedPlan, error) {
	if source.Index == nil {
		return doccompiler.AdmittedPlan{}, failure("INVALID_PLAN", "no index is supplied to verify the plan against")
	}
	plan, err := doccompiler.VerifyAdmittedPlan(source.Index, source.Plan, source.Patch)
	if err != nil {
		return doccompiler.AdmittedPlan{}, failure("INVALID_PLAN", "input is not a verified Human Documentation Compiler plan: %v", err)
	}
	return plan, nil
}

func sha256Hex(data []byte) string {
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}

func claimsFromPlan(plan doccompiler.AdmittedPlan, overlays []ClaimOverlay) ([]TruthClaim, error) {
	byID := make(map[string]doccompiler.Clause, len(plan.Clauses))
	for _, claim := range plan.Clauses {
		if !claimIDValid(claim.ID) {
			return nil, failure("INVALID_PLAN", "claim ID is invalid")
		}
		byID[claim.ID] = claim
	}
	if len(byID) > maxClaims {
		return nil, failure("LIMIT_EXCEEDED", "plan contains more than %d claims", maxClaims)
	}
	if len(overlays) != len(byID) {
		return nil, failure("INVALID_INPUT", "every truth claim requires exactly one lifecycle overlay")
	}
	overlayByID := make(map[string]ClaimOverlay, len(overlays))
	for _, overlay := range overlays {
		if _, exists := overlayByID[overlay.ClaimID]; exists {
			return nil, failure("INVALID_INPUT", "claim overlay is duplicated: %s", overlay.ClaimID)
		}
		if _, exists := byID[overlay.ClaimID]; !exists {
			return nil, failure("INVALID_INPUT", "claim overlay references an unknown claim: %s", overlay.ClaimID)
		}
		overlayByID[overlay.ClaimID] = overlay
	}

	ids := make([]string, 0, len(byID))
	for id := range byID {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	result := make([]TruthClaim, 0, len(ids))
	for _, id := range ids {
		claim := byID[id]
		overlay := overlayByID[id]
		truth, err := compileTruthClaim(claim, overlay)
		if err != nil {
			return nil, err
		}
		result = append(result, truth)
	}
	return result, nil
}

func compileTruthClaim(claim doccompiler.Clause, overlay ClaimOverlay) (TruthClaim, error) {
	if !claimTextValid(claim.Text) {
		return TruthClaim{}, failure("INVALID_PLAN", "claim %s contains invalid or oversized text", claim.ID)
	}
	if len(claim.Anchors) > maxEvidencePerClaim {
		return TruthClaim{}, failure("LIMIT_EXCEEDED", "claim %s has too many evidence anchors", claim.ID)
	}
	if !currencyValid(overlay.Currency) || !reviewStateValid(overlay.ReviewState) {
		return TruthClaim{}, failure("INVALID_INPUT", "claim %s has invalid lifecycle metadata", claim.ID)
	}
	if overlay.Currency == CurrencyCurrent && overlay.CurrencyReason != "" {
		return TruthClaim{}, failure("INVALID_INPUT", "CURRENT claim %s cannot carry a stale reason", claim.ID)
	}
	if overlay.Currency != CurrencyCurrent && !claimTextValid(overlay.CurrencyReason) {
		return TruthClaim{}, failure("INVALID_INPUT", "non-current claim %s requires a reason", claim.ID)
	}
	limitations, err := canonicalLimitations(overlay.Limitations)
	if err != nil {
		return TruthClaim{}, err
	}
	evidence := sortedEvidence(claim.Anchors)
	for _, anchor := range evidence {
		if !evidenceValid(anchor) {
			return TruthClaim{}, failure("INVALID_PLAN", "claim %s contains oversized or invalid UTF-8 evidence", claim.ID)
		}
	}
	truth := TruthClaim{
		Currency: overlay.Currency, CurrencyReason: overlay.CurrencyReason, Evidence: evidence,
		Frontier: claim.Frontier, ID: claim.ID, Kind: claim.Kind, Limitations: limitations,
		ReviewState: overlay.ReviewState, Scope: claim.Scope, State: claim.State, Text: claim.Text,
	}
	if !claimShapeValid(truth) {
		return TruthClaim{}, failure("INVALID_PLAN", "claim %s has an invalid state, kind, scope, frontier, or conflict evidence", claim.ID)
	}
	return truth, nil
}

// sortedEvidence copies the plan anchors in complete canonical key order.
func sortedEvidence(anchors []doccompiler.Anchor) []TruthEvidence {
	evidence := append(make([]TruthEvidence, 0, len(anchors)), anchors...)
	sort.Slice(evidence, func(left, right int) bool { return evidenceKey(evidence[left]) < evidenceKey(evidence[right]) })
	return evidence
}

func canonicalLimitations(values []string) ([]string, error) {
	if len(values) > maxLimitationsPerClaim {
		return nil, failure("LIMIT_EXCEEDED", "claim has more than %d limitations", maxLimitationsPerClaim)
	}
	result := append([]string(nil), values...)
	for _, value := range result {
		if !boundedTextValid(value, maxLimitationBytes) {
			return nil, failure("INVALID_INPUT", "limitation is empty, invalid, or oversized")
		}
	}
	sort.Strings(result)
	for index := 1; index < len(result); index++ {
		if result[index-1] == result[index] {
			return nil, failure("INVALID_INPUT", "limitation is duplicated")
		}
	}
	return result, nil
}

func compileViews(truth TruthCorpus, truthSHA256 string, recipes []ProjectionRecipe) ([]AudienceView, error) {
	if len(recipes) != len(audienceOrder) {
		return nil, failure("INVALID_INPUT", "exactly five audience recipes are required")
	}
	byAudience := make(map[Audience]ProjectionRecipe, len(recipes))
	for _, recipe := range recipes {
		if !audienceValid(recipe.Audience) || !detailValid(recipe.Detail) {
			return nil, failure("INVALID_INPUT", "audience recipe has an invalid audience or detail")
		}
		if _, exists := byAudience[recipe.Audience]; exists {
			return nil, failure("INVALID_INPUT", "audience recipe is duplicated: %s", recipe.Audience)
		}
		if err := exactClaimPermutation(truth.Claims, recipe.ClaimIDs); err != nil {
			return nil, err
		}
		copyRecipe := recipe
		copyRecipe.ClaimIDs = append([]string(nil), recipe.ClaimIDs...)
		byAudience[recipe.Audience] = copyRecipe
	}
	views := make([]AudienceView, 0, len(audienceOrder))
	for _, audience := range audienceOrder {
		recipe, exists := byAudience[audience]
		if !exists {
			return nil, failure("INVALID_INPUT", "audience recipe is missing: %s", audience)
		}
		views = append(views, AudienceView{
			Audience: audience, ClaimIDs: recipe.ClaimIDs, Detail: recipe.Detail,
			DisclosurePolicySHA256: truth.DisclosurePolicySHA256, DisclosureScope: truth.DisclosureScope,
			Profile: ViewProfile, TruthSHA256: truthSHA256,
		})
	}
	return views, nil
}

func exactClaimPermutation(claims []TruthClaim, ids []string) error {
	if len(ids) != len(claims) {
		return failure("INVALID_INPUT", "audience projection must reference every truth claim exactly once")
	}
	expected := make(map[string]struct{}, len(claims))
	for _, claim := range claims {
		expected[claim.ID] = struct{}{}
	}
	seen := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		if _, exists := expected[id]; !exists {
			return failure("INVALID_INPUT", "audience projection references an unknown claim: %s", id)
		}
		if _, exists := seen[id]; exists {
			return failure("INVALID_INPUT", "audience projection duplicates claim: %s", id)
		}
		seen[id] = struct{}{}
	}
	return nil
}

func truthDigest(truth TruthCorpus) (string, error) {
	encoded, err := doccompiler.CanonicalJSONUnbounded(truth)
	if err != nil {
		return "", failure("INTERNAL_ERROR", "cannot encode truth corpus")
	}
	encoded = encoded[:len(encoded)-1]
	hasher := sha256.New()
	_, _ = hasher.Write(truthDigestDomain)
	_, _ = hasher.Write(encoded)
	return hex.EncodeToString(hasher.Sum(nil)), nil
}
