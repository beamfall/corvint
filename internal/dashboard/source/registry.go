package source

const maxIdentifierBytes = 128

func registered(kind Kind, verifier VerifierID) bool {
	return len(kind) <= maxIdentifierBytes && len(verifier) <= maxIdentifierBytes &&
		kind == KindLocalTrace && verifier == VerifierLocalTraceV1
}
