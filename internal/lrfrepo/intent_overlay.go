package lrfrepo

import "math"

// IntentRequirements enumerates the requirement IDs of one Markdown intent
// document by the OCM-V0-001 rule and returns them with the byte span of its
// Requirements section and that span's SHA-256. It reads no repository and
// binds no blob: the reviewer overlay of `corvint obligations` (CEM-V1-642)
// uses it to read an intent the target tree does not carry.
func IntentRequirements(data []byte) ([]string, int64, int64, string, error) {
	intent, requirements, _, err := requirementsFromBlob("", "", data)
	if err != nil {
		return nil, 0, 0, "", err
	}
	return requirements, intent.start, intent.end, intent.spanSHA256, nil
}

// requirementsFromBlob applies the OCM-V0-001 Requirements rule within the
// per-map obligation bound, refusing too-many-obligations above it.
func requirementsFromBlob(path, oid string, data []byte) (ocmIntent, []string, []byte, error) {
	return scanRequirements(path, oid, data, maxObligations)
}

// IntentObligationCount counts one intent document's requirement IDs by the
// OCM-V0-001 rule without the per-map obligation bound, so `cem defer` can
// record an intent too large to declare (CEM-V1-644). Every other refusal of
// the rule still applies.
func IntentObligationCount(data []byte) (int, error) {
	_, requirements, _, err := scanRequirements("", "", data, math.MaxInt)
	return len(requirements), err
}
