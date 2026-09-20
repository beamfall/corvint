package jstestprovider

// QualifiedReceiptBindingReady verifies the qualified external lifecycle,
// identity and attempt structure independently of an observed infrastructure
// outcome. Classification remains the consumer's separate responsibility.
func QualifiedReceiptBindingReady(r Receipt, t TestOutcome) bool {
	if r.Profile != ExternalProfile || r.Cancelled || r.Infrastructure != nil || r.StaleAppBuild {
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
