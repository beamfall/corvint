package contextindex

// Cancel deterministically during the index scan, not merely before invocation.
type localPromptCancelContext struct{ calls, cancelAt int }
