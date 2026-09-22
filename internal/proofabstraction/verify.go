package proofabstraction

import (
	"reflect"
	"sort"
)

// Verify independently checks that destination is no stronger than source and
// that certificate accounts for the exact requested transformation.
func Verify(source Envelope, request Request, destination Envelope, certificate Certificate) error {
	if err := validateEnvelope(source); err != nil {
		return err
	}
	if err := validateRequestAgainstSource(request, source); err != nil {
		return err
	}
	if err := validateEnvelope(destination); err != nil {
		return err
	}
	if err := validateCertificate(certificate); err != nil {
		return err
	}
	if source.Repository != destination.Repository || certificate.Repository != source.Repository {
		return fail(SnapshotChanged)
	}
	if source.PolicySHA256 != destination.PolicySHA256 || certificate.PolicySHA256 != source.PolicySHA256 {
		return fail(PolicyChanged)
	}
	if certificate.SourceEnvelopeID != source.EnvelopeID || certificate.DestinationEnvelopeID != destination.EnvelopeID {
		return fail(CertificateMismatch)
	}
	requestDigest, err := requestSHA256(request)
	if err != nil || certificate.RequestSHA256 != requestDigest {
		return fail(CertificateMismatch)
	}
	if request.PurposeSHA256 != destination.PurposeSHA256 || certificate.PurposeSHA256 != request.PurposeSHA256 {
		return fail(CertificateMismatch)
	}
	if err := verifyCompleteness(source, request, destination, certificate); err != nil {
		return err
	}
	if err := verifyEvidence(source, destination, certificate); err != nil {
		return err
	}
	if err := verifyClaims(source, request, destination, certificate); err != nil {
		return err
	}
	if err := verifyFrontier(source, request, destination, certificate); err != nil {
		return err
	}
	return nil
}

func verifyCompleteness(source Envelope, request Request, destination Envelope, certificate Certificate) error {
	expected := source.Completeness
	hasWeakening := false
	for _, action := range request.Actions {
		hasWeakening = hasWeakening || action.Action != ActionRetain
	}
	if source.Completeness == CompletenessComplete && hasWeakening {
		expected = CompletenessPartial
	}
	if destination.Completeness != expected {
		return fail(CompletenessStrengthened)
	}
	wantRelation := CompletenessEqual
	if source.Completeness == CompletenessComplete && destination.Completeness == CompletenessPartial {
		wantRelation = CompletenessWeakenedToPartial
	}
	if certificate.CompletenessRelation != wantRelation {
		return fail(CertificateMismatch)
	}
	return nil
}

func verifyClaims(source Envelope, request Request, destination Envelope, certificate Certificate) error {
	sourceClaims := claimsByID(source.Claims)
	destinationClaims := claimsByID(destination.Claims)
	relations := relationsByClaim(certificate.ClaimRelations)
	omissions := omissionsByClaim(certificate.OmittedClaims)
	expectedRelations := 0
	expectedOmissions := 0
	for _, action := range request.Actions {
		if action.Action == ActionOmit {
			expectedOmissions++
		} else {
			expectedRelations++
		}
	}
	if len(relations) != expectedRelations || len(omissions) != expectedOmissions {
		return fail(CertificateMismatch)
	}
	for _, destinationClaim := range destination.Claims {
		if _, ok := sourceClaims[destinationClaim.ID]; !ok {
			return fail(ClaimAdded)
		}
	}
	frontierClaims := sourceFrontierClaims(source.Frontier)
	for index, sourceClaim := range source.Claims {
		action := request.Actions[index]
		relation, hasRelation := relations[sourceClaim.ID]
		omission, hasOmission := omissions[sourceClaim.ID]
		switch action.Action {
		case ActionRetain:
			expected := ClaimRelation{SourceClaimID: sourceClaim.ID, DestinationClaimID: sourceClaim.ID, Relation: RelationIdentical}
			if !hasRelation || hasOmission || !reflect.DeepEqual(relation, expected) {
				return fail(CertificateMismatch)
			}
			if err := verifyRelation(destinationClaims, sourceClaim, relation, frontierClaims); err != nil {
				return err
			}
		case ActionDemoteToUnknown:
			expected := ClaimRelation{SourceClaimID: sourceClaim.ID, DestinationClaimID: sourceClaim.ID, Relation: RelationDemotedToUnknown, Reason: action.Reason}
			if !hasRelation || hasOmission || !reflect.DeepEqual(relation, expected) {
				return fail(CertificateMismatch)
			}
			if err := verifyRelation(destinationClaims, sourceClaim, relation, frontierClaims); err != nil {
				return err
			}
		case ActionOmit:
			if hasRelation || !hasOmission || omission.Reason != *action.Reason {
				return fail(CertificateMismatch)
			}
			if err := verifyOmission(source, destinationClaims, sourceClaim, omission, frontierClaims); err != nil {
				return err
			}
		}
	}
	return nil
}

func verifyRelation(destination map[string]Claim, source Claim, relation ClaimRelation, frontierClaims map[string]struct{}) error {
	value, ok := destination[source.ID]
	if !ok {
		return fail(OmissionUnaccounted)
	}
	switch relation.Relation {
	case RelationIdentical:
		if !reflect.DeepEqual(value, source) {
			return classifyClaimMutation(source, value)
		}
	case RelationDemotedToUnknown:
		if err := verifyMayWeaken(source, frontierClaims); err != nil {
			return err
		}
		if !isCanonicalDemotion(source, value) {
			return classifyClaimMutation(source, value)
		}
	default:
		return fail(CertificateMismatch)
	}
	return nil
}

func verifyOmission(source Envelope, destination map[string]Claim, claim Claim, omission OmittedClaim, frontierClaims map[string]struct{}) error {
	if _, exists := destination[claim.ID]; exists {
		return fail(CertificateMismatch)
	}
	if err := verifyMayWeaken(claim, frontierClaims); err != nil {
		return err
	}
	if omission.SourceVerdict != claim.Verdict {
		return fail(CertificateMismatch)
	}
	expected, err := verifyOmissionFrontier(source.EnvelopeID, claim.ID, omission.Reason)
	if err != nil || omission.FrontierID != expected.ID {
		return fail(OmissionUnaccounted)
	}
	return nil
}

func classifyClaimMutation(source, destination Claim) error {
	if destination.StatementSHA256 != source.StatementSHA256 || destination.ScopeSHA256 != source.ScopeSHA256 || destination.ID != source.ID {
		return fail(ClaimIdentityChanged)
	}
	if destination.Critical != source.Critical {
		return fail(CriticalClaimWeakened)
	}
	if destination.AuthorityClass != source.AuthorityClass && destination.AuthorityClass != AuthorityNone {
		return fail(AuthorityChanged)
	}
	if source.Verdict == VerdictConflicted {
		return fail(ConflictHidden)
	}
	if source.Verdict == VerdictUnknown {
		return fail(UnknownHidden)
	}
	return fail(VerdictStrengthened)
}

func verifyMayWeaken(claim Claim, frontierClaims map[string]struct{}) error {
	if claim.Critical {
		return fail(CriticalClaimWeakened)
	}
	if claim.Verdict == VerdictConflicted {
		return fail(ConflictHidden)
	}
	if claim.Verdict == VerdictUnknown {
		return fail(UnknownHidden)
	}
	if _, blocked := frontierClaims[claim.ID]; blocked {
		return fail(FrontierRemoved)
	}
	return nil
}

func isCanonicalDemotion(source, destination Claim) bool {
	return destination.ID == source.ID &&
		destination.StatementSHA256 == source.StatementSHA256 &&
		destination.ScopeSHA256 == source.ScopeSHA256 &&
		destination.Critical == source.Critical &&
		destination.Verdict == VerdictUnknown &&
		destination.AuthorityClass == AuthorityNone &&
		len(destination.EvidenceClasses) == 0 &&
		len(destination.Evidence) == 0 &&
		len(destination.CounterEvidence) == 0 &&
		reflect.DeepEqual(destination.UnknownReasons, []string{"ABSTRACTION_WEAKENED"})
}

func verifyOmissionFrontier(sourceEnvelopeID, claimID string, reason Reason) (Frontier, error) {
	source := sourceEnvelopeID
	return BindFrontier(Frontier{
		ClaimID:          claimID,
		Kind:             FrontierAbstractedClaim,
		Reason:           string(reason),
		SourceEnvelopeID: &source,
		Recovery:         RecoveryLoadSourceEnvelope,
	})
}

func verifyEvidence(source, destination Envelope, certificate Certificate) error {
	sourceEvidence := evidenceByID(source.Evidence)
	for _, value := range destination.Evidence {
		expected, ok := sourceEvidence[value.ID]
		if !ok {
			return fail(EvidenceAdded)
		}
		if !reflect.DeepEqual(value, expected) {
			return fail(EvidenceMutated)
		}
	}
	expected := verifyEvidenceAccounting(source, destination)
	if !reflect.DeepEqual(certificate.EvidenceAccounting, expected) {
		return fail(CertificateMismatch)
	}
	return nil
}

func verifyFrontier(source Envelope, request Request, destination Envelope, certificate Certificate) error {
	destinationFrontier := frontierByID(destination.Frontier)
	for _, value := range source.Frontier {
		observed, ok := destinationFrontier[value.ID]
		if !ok || !reflect.DeepEqual(value, observed) {
			return fail(FrontierRemoved)
		}
		delete(destinationFrontier, value.ID)
	}
	addedIDs := make([]string, 0, len(certificate.OmittedClaims))
	for _, action := range request.Actions {
		if action.Action != ActionOmit {
			continue
		}
		expected, err := verifyOmissionFrontier(source.EnvelopeID, action.ClaimID, *action.Reason)
		if err != nil {
			return err
		}
		observed, ok := destinationFrontier[expected.ID]
		if !ok || !reflect.DeepEqual(expected, observed) {
			return fail(OmissionUnaccounted)
		}
		delete(destinationFrontier, expected.ID)
		addedIDs = append(addedIDs, expected.ID)
	}
	if len(destinationFrontier) != 0 {
		return fail(OmissionUnaccounted)
	}
	if !reflect.DeepEqual(certificate.FrontierAccounting.Preserved, verifyFrontierIDs(source.Frontier)) {
		return fail(CertificateMismatch)
	}
	if !reflect.DeepEqual(certificate.FrontierAccounting.Added, sortedCopy(addedIDs)) {
		return fail(CertificateMismatch)
	}
	return nil
}

func verifyEvidenceAccounting(source, destination Envelope) []EvidenceAccounting {
	preserved := make(map[string]struct{})
	for _, claim := range destination.Claims {
		for _, id := range claim.Evidence {
			preserved[id] = struct{}{}
		}
		for _, id := range claim.CounterEvidence {
			preserved[id] = struct{}{}
		}
	}
	claimsByEvidence := make(map[string][]string)
	for _, claim := range source.Claims {
		for _, id := range claim.Evidence {
			claimsByEvidence[id] = append(claimsByEvidence[id], claim.ID)
		}
		for _, id := range claim.CounterEvidence {
			claimsByEvidence[id] = append(claimsByEvidence[id], claim.ID)
		}
	}
	result := make([]EvidenceAccounting, 0, len(source.Evidence))
	for _, evidence := range source.Evidence {
		state := EvidenceOmitted
		if _, ok := preserved[evidence.ID]; ok {
			state = EvidencePreserved
		}
		claims := append([]string{}, claimsByEvidence[evidence.ID]...)
		sort.Strings(claims)
		result = append(result, EvidenceAccounting{EvidenceID: evidence.ID, State: state, SourceClaimIDs: claims})
	}
	return result
}

func verifyFrontierIDs(values []Frontier) []string {
	result := make([]string, len(values))
	for index, value := range values {
		result[index] = value.ID
	}
	sort.Strings(result)
	return result
}

func claimsByID(values []Claim) map[string]Claim {
	result := make(map[string]Claim, len(values))
	for _, value := range values {
		result[value.ID] = value
	}
	return result
}

func evidenceByID(values []Evidence) map[string]Evidence {
	result := make(map[string]Evidence, len(values))
	for _, value := range values {
		result[value.ID] = value
	}
	return result
}

func frontierByID(values []Frontier) map[string]Frontier {
	result := make(map[string]Frontier, len(values))
	for _, value := range values {
		result[value.ID] = value
	}
	return result
}

func relationsByClaim(values []ClaimRelation) map[string]ClaimRelation {
	result := make(map[string]ClaimRelation, len(values))
	for _, value := range values {
		result[value.SourceClaimID] = value
	}
	return result
}

func omissionsByClaim(values []OmittedClaim) map[string]OmittedClaim {
	result := make(map[string]OmittedClaim, len(values))
	for _, value := range values {
		result[value.SourceClaimID] = value
	}
	return result
}

func sortedCopy(values []string) []string {
	result := append([]string{}, values...)
	sort.Strings(result)
	return result
}
