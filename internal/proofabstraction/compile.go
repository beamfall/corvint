package proofabstraction

import (
	"sort"
)

// Compile builds a destination projection and certificate, then verifies the
// non-strengthening relation before returning either artifact.
func Compile(source Envelope, request Request) (Envelope, Certificate, error) {
	if err := validateEnvelope(source); err != nil {
		return Envelope{}, Certificate{}, err
	}
	if err := validateRequestAgainstSource(request, source); err != nil {
		return Envelope{}, Certificate{}, err
	}
	destination, relations, omitted, added, err := project(source, request)
	if err != nil {
		return Envelope{}, Certificate{}, err
	}
	certificate, err := buildCertificate(source, destination, request, relations, omitted, added)
	if err != nil {
		return Envelope{}, Certificate{}, err
	}
	if err := Verify(source, request, destination, certificate); err != nil {
		return Envelope{}, Certificate{}, err
	}
	return destination, certificate, nil
}

func validateRequestAgainstSource(request Request, source Envelope) error {
	if err := validateRequest(request); err != nil {
		return err
	}
	if request.SourceEnvelopeID != source.EnvelopeID {
		return fail(SourceIDMismatch)
	}
	if len(request.Actions) != len(source.Claims) {
		return fail(OmissionUnaccounted)
	}
	for index, action := range request.Actions {
		if action.ClaimID != source.Claims[index].ID {
			return fail(ClaimAdded)
		}
	}
	return nil
}

func project(source Envelope, request Request) (Envelope, []ClaimRelation, []OmittedClaim, []Frontier, error) {
	frontierClaims := sourceFrontierClaims(source.Frontier)
	destinationClaims := make([]Claim, 0, len(source.Claims))
	relations := make([]ClaimRelation, 0, len(source.Claims))
	omitted := make([]OmittedClaim, 0)
	added := make([]Frontier, 0)
	for index, sourceClaim := range source.Claims {
		action := request.Actions[index]
		switch action.Action {
		case ActionRetain:
			destinationClaims = append(destinationClaims, cloneClaim(sourceClaim))
			relations = append(relations, ClaimRelation{SourceClaimID: sourceClaim.ID, DestinationClaimID: sourceClaim.ID, Relation: RelationIdentical})
		case ActionDemoteToUnknown:
			if err := mayWeaken(sourceClaim, frontierClaims); err != nil {
				return Envelope{}, nil, nil, nil, err
			}
			destinationClaims = append(destinationClaims, demoteClaim(sourceClaim))
			relations = append(relations, ClaimRelation{SourceClaimID: sourceClaim.ID, DestinationClaimID: sourceClaim.ID, Relation: RelationDemotedToUnknown, Reason: cloneReason(action.Reason)})
		case ActionOmit:
			if err := mayWeaken(sourceClaim, frontierClaims); err != nil {
				return Envelope{}, nil, nil, nil, err
			}
			frontier, err := omissionFrontier(source.EnvelopeID, sourceClaim.ID, *action.Reason)
			if err != nil {
				return Envelope{}, nil, nil, nil, err
			}
			added = append(added, frontier)
			omitted = append(omitted, OmittedClaim{SourceClaimID: sourceClaim.ID, SourceVerdict: sourceClaim.Verdict, Reason: *action.Reason, FrontierID: frontier.ID})
		}
	}
	destinationFrontier := cloneFrontiers(source.Frontier)
	destinationFrontier = append(destinationFrontier, added...)
	sort.Slice(destinationFrontier, func(left, right int) bool { return destinationFrontier[left].ID < destinationFrontier[right].ID })
	preservedEvidence := referencedEvidence(destinationClaims)
	destinationEvidence := filterEvidence(source.Evidence, preservedEvidence)
	destination := Envelope{
		Profile:       EnvelopeProfile,
		Repository:    source.Repository,
		PolicySHA256:  source.PolicySHA256,
		PurposeSHA256: request.PurposeSHA256,
		Completeness:  projectedCompleteness(source.Completeness, request.Actions),
		Claims:        destinationClaims,
		Evidence:      destinationEvidence,
		Frontier:      destinationFrontier,
	}
	bound, err := BindEnvelope(destination)
	return bound, relations, omitted, added, err
}

func buildCertificate(source, destination Envelope, request Request, relations []ClaimRelation, omitted []OmittedClaim, added []Frontier) (Certificate, error) {
	requestDigest, err := requestSHA256(request)
	if err != nil {
		return Certificate{}, err
	}
	certificate := Certificate{
		Profile:               CertificateProfile,
		Algorithm:             AlgorithmProfile,
		Claim:                 CertificateClaim,
		SourceEnvelopeID:      source.EnvelopeID,
		DestinationEnvelopeID: destination.EnvelopeID,
		RequestSHA256:         requestDigest,
		Repository:            source.Repository,
		PolicySHA256:          source.PolicySHA256,
		PurposeSHA256:         request.PurposeSHA256,
		CompletenessRelation:  completenessRelation(source.Completeness, destination.Completeness),
		ClaimRelations:        relations,
		OmittedClaims:         omitted,
		EvidenceAccounting:    evidenceAccounting(source, destination),
		FrontierAccounting: FrontierAccounting{
			Preserved: frontierIDs(source.Frontier),
			Added:     frontierIDs(added),
		},
	}
	id, err := certificateID(certificate)
	if err != nil {
		return Certificate{}, err
	}
	certificate.CertificateID = id
	return certificate, nil
}

func mayWeaken(claim Claim, frontierClaims map[string]struct{}) error {
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

func demoteClaim(source Claim) Claim {
	return Claim{
		ID:              source.ID,
		StatementSHA256: source.StatementSHA256,
		ScopeSHA256:     source.ScopeSHA256,
		Critical:        source.Critical,
		Verdict:         VerdictUnknown,
		AuthorityClass:  AuthorityNone,
		EvidenceClasses: []EvidenceClass{},
		Evidence:        []string{},
		CounterEvidence: []string{},
		UnknownReasons:  []string{"ABSTRACTION_WEAKENED"},
	}
}

func omissionFrontier(sourceEnvelopeID, claimID string, reason Reason) (Frontier, error) {
	source := sourceEnvelopeID
	return BindFrontier(Frontier{
		ClaimID:          claimID,
		Kind:             FrontierAbstractedClaim,
		Reason:           string(reason),
		SourceEnvelopeID: &source,
		Recovery:         RecoveryLoadSourceEnvelope,
	})
}

func sourceFrontierClaims(values []Frontier) map[string]struct{} {
	result := make(map[string]struct{})
	for _, value := range values {
		if value.Kind != FrontierAbstractedClaim {
			result[value.ClaimID] = struct{}{}
		}
	}
	return result
}

func referencedEvidence(claims []Claim) map[string]struct{} {
	result := make(map[string]struct{})
	for _, claim := range claims {
		for _, id := range claim.Evidence {
			result[id] = struct{}{}
		}
		for _, id := range claim.CounterEvidence {
			result[id] = struct{}{}
		}
	}
	return result
}

func filterEvidence(values []Evidence, included map[string]struct{}) []Evidence {
	result := make([]Evidence, 0, len(included))
	for _, value := range values {
		if _, ok := included[value.ID]; ok {
			result = append(result, value)
		}
	}
	return result
}

func evidenceAccounting(source, destination Envelope) []EvidenceAccounting {
	preserved := referencedEvidence(destination.Claims)
	claimsByEvidence := make(map[string][]string)
	for _, claim := range source.Claims {
		for _, evidenceID := range appendEvidenceIDs(claim) {
			claimsByEvidence[evidenceID] = append(claimsByEvidence[evidenceID], claim.ID)
		}
	}
	result := make([]EvidenceAccounting, 0, len(source.Evidence))
	for _, evidence := range source.Evidence {
		state := EvidenceOmitted
		if _, ok := preserved[evidence.ID]; ok {
			state = EvidencePreserved
		}
		result = append(result, EvidenceAccounting{EvidenceID: evidence.ID, State: state, SourceClaimIDs: claimsByEvidence[evidence.ID]})
	}
	return result
}

func appendEvidenceIDs(claim Claim) []string {
	result := make([]string, 0, len(claim.Evidence)+len(claim.CounterEvidence))
	result = append(result, claim.Evidence...)
	result = append(result, claim.CounterEvidence...)
	sort.Strings(result)
	return result
}

func projectedCompleteness(source Completeness, actions []Action) Completeness {
	if source != CompletenessComplete {
		return source
	}
	for _, action := range actions {
		if action.Action != ActionRetain {
			return CompletenessPartial
		}
	}
	return CompletenessComplete
}

func completenessRelation(source, destination Completeness) CompletenessRelation {
	if source == CompletenessComplete && destination == CompletenessPartial {
		return CompletenessWeakenedToPartial
	}
	return CompletenessEqual
}

func frontierIDs(values []Frontier) []string {
	result := make([]string, len(values))
	for index, value := range values {
		result[index] = value.ID
	}
	sort.Strings(result)
	return result
}

func cloneReason(value *Reason) *Reason {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func cloneClaim(value Claim) Claim {
	value.EvidenceClasses = append([]EvidenceClass{}, value.EvidenceClasses...)
	value.Evidence = append([]string{}, value.Evidence...)
	value.CounterEvidence = append([]string{}, value.CounterEvidence...)
	value.UnknownReasons = append([]string{}, value.UnknownReasons...)
	return value
}

func cloneFrontiers(values []Frontier) []Frontier {
	result := make([]Frontier, len(values))
	for index, value := range values {
		result[index] = value
		if value.SourceEnvelopeID != nil {
			source := *value.SourceEnvelopeID
			result[index].SourceEnvelopeID = &source
		}
	}
	return result
}
