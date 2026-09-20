package jstestprovider

// QualifiedReceiptBindingReady verifies the qualified external lifecycle,
// identity and attempt structure independently of an observed infrastructure
// outcome. Classification remains the consumer's separate responsibility.
func QualifiedReceiptBindingReady(r Receipt, t TestOutcome) bool {
	if !isExternalProfile(r.Profile) || r.Cancelled || r.Infrastructure != nil || r.StaleAppBuild {
		return false
	}
	normalized := t
	normalized.Attempts = append([]Attempt(nil), t.Attempts...)
	for i := range normalized.Attempts {
		if normalized.Attempts[i].State == StateInfrastructure {
			normalized.Attempts[i].State = StateFailed
		}
	}
	return !qualifiedUnknown(r, normalized)
}

// QualifiedApplicationRevision returns the application identity only after the
// complete external receipt and the selected outcome have been qualified.
func QualifiedApplicationRevision(r Receipt, t TestOutcome) (string, bool) {
	if !QualifiedReceiptBindingReady(r, t) {
		return "", false
	}
	if r.Profile == AttestedExternalProfile {
		return r.ApplicationAttestation.Before.Attestation.Repository.Revision, true
	}
	return r.External.DeclaredAppIdentity, true
}
