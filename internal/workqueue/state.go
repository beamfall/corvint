package workqueue

// StateFacts preserves the predicates whose precedence is defined by WQO-V0-014.
// Qualification diagnostics alone do not imply Unable.
type StateFacts struct {
	IdentityContradiction    bool
	MutationOrAdapterFailure bool
	Stale                    bool
	PositivePartial          bool
	Unable                   bool
}

// ResolveState applies the closed observation-state precedence exactly once.
func ResolveState(facts StateFacts) string {
	switch {
	case facts.IdentityContradiction:
		return StateConflicted
	case facts.MutationOrAdapterFailure:
		return StateUnknown
	case facts.Stale:
		return StateStale
	case facts.PositivePartial:
		return StatePartial
	case facts.Unable:
		return StateUnknown
	default:
		return StateValidated
	}
}
