package dogfoodflow

import (
	"encoding/json"
	"slices"
)

// scaledOperationsFlag opts a cem/1.0 status into the stable verifier's
// map-scaled Git operation bound (DCW-V0-373).
const scaledOperationsFlag = "--scaled-operations"

// resourceLimited reports whether a cem status stdout is the stable
// verifier's UNSUPPORTED unsupported-resource-limit refusal: the only result
// the flow retries with the scaled operation bound (DCW-V0-373). Every other
// result, including a REJECT, stands as the protocol budget produced it.
func resourceLimited(stdout []byte) bool {
	var envelope struct {
		StableVerification struct {
			Outcome string  `json:"outcome"`
			Code    *string `json:"code"`
		} `json:"stableVerification"`
	}
	if json.Unmarshal(firstLine(stdout), &envelope) != nil {
		return false
	}
	result := envelope.StableVerification
	return result.Outcome == "UNSUPPORTED" && result.Code != nil && *result.Code == "unsupported-resource-limit"
}

// withScaledOperations is a cem status argv with the opt-in scaled bound. The
// first run always uses the protocol argv, so a verifier that predates the
// flag still sees the argv it accepts on every change within 1,024 operations.
func withScaledOperations(args []string) []string {
	if slices.Contains(args, scaledOperationsFlag) {
		return args
	}
	return append(slices.Clone(args), scaledOperationsFlag)
}
