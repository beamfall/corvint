package tcq

// The complete TCQ-V0-039 reason vocabulary, in its frozen order. Reasons are
// unique per claim and always emitted in this order; the index below is the sole
// sort key, so no lexical accident can reorder a diagnostic.
const (
	reasonUnsupportedAnchorProfile     = "unsupported-anchor-profile"
	reasonClaimAssociationMissing      = "claim-association-missing"
	reasonClaimAssociationAmbiguous    = "claim-association-ambiguous"
	reasonUnsupportedPythonGrammar     = "unsupported-python-grammar"
	reasonPythonOffsetMismatch         = "python-offset-mismatch"
	reasonUnparseableTestUnit          = "unparseable-test-unit"
	reasonEmptyBody                    = "empty-body"
	reasonUnconditionalSkip            = "unconditional-skip"
	reasonExecutionIdentityAmbiguous   = "execution-identity-ambiguous"
	reasonRowIdentityUnavailable       = "row-identity-unavailable"
	reasonTestNotMatched               = "test-not-matched"
	reasonRepeatedTestRows             = "repeated-test-rows"
	reasonTestSkipped                  = "test-skipped"
	reasonTestFailed                   = "test-failed"
	reasonTestError                    = "test-error"
	reasonTargetCleanlinessNotAttested = "target-cleanliness-not-attested"
	reasonCommandFailed                = "command-failed"
)

var reasonOrder = []string{
	reasonUnsupportedAnchorProfile,
	reasonClaimAssociationMissing,
	reasonClaimAssociationAmbiguous,
	reasonUnsupportedPythonGrammar,
	reasonPythonOffsetMismatch,
	reasonUnparseableTestUnit,
	reasonEmptyBody,
	reasonUnconditionalSkip,
	reasonExecutionIdentityAmbiguous,
	reasonRowIdentityUnavailable,
	reasonTestNotMatched,
	reasonRepeatedTestRows,
	reasonTestSkipped,
	reasonTestFailed,
	reasonTestError,
	reasonTargetCleanlinessNotAttested,
	reasonCommandFailed,
}

var reasonIndex = buildReasonIndex()

func buildReasonIndex() map[string]int {
	index := make(map[string]int, len(reasonOrder))
	for position, reason := range reasonOrder {
		index[reason] = position
	}
	return index
}

// sortReasons deduplicates and orders one claim's reasons per TCQ-V0-039.
func sortReasons(reasons []string) []string {
	seen := make(map[string]bool, len(reasons))
	ordered := make([]string, 0, len(reasons))
	for _, reason := range reasonOrder {
		for _, candidate := range reasons {
			if candidate == reason && !seen[reason] {
				seen[reason] = true
				ordered = append(ordered, reason)
			}
		}
	}
	return ordered
}
