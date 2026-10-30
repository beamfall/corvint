package main

// proveUnsupportedOption refuses impact options that prove would otherwise
// accept through the shared parser and silently ignore: it embeds the
// unbudgeted impact packet (FPK-V0-010) and has no strict abstention form
// (V1-1141).
func proveUnsupportedOption(parsed options) error {
	if parsed.impactBudget != nil {
		return argumentError("prove does not support --budget-bytes")
	}
	if parsed.requireClean {
		return argumentError("prove does not support --require-clean")
	}
	return nil
}
