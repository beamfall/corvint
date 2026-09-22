package witnesscollapse

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"

	"github.com/Beamfall/corvint/internal/cem/wire"
)

func validCauseKind(kind CauseKind) bool {
	switch kind {
	case CauseSource, CauseGenerator, CauseParser, CauseOracle, CausePremise:
		return true
	default:
		return false
	}
}

// CauseIdentity domain-separates the cause kind from its opaque subject digest.
func CauseIdentity(kind CauseKind, subjectSHA256 string) (string, error) {
	if !validCauseKind(kind) {
		return "", failure("invalid-cause-kind", "common cause kind is unsupported")
	}
	if !wire.IsSha256(subjectSHA256) {
		return "", failure("invalid-cause-subject", "common cause subject must be a lowercase SHA-256")
	}
	// Both values have already been restricted to an ASCII enumeration or
	// lowercase hex, so direct quoting is the exact frozen canonical encoding.
	canonical := `{"kind":"` + string(kind) + `","subjectSha256":"` + subjectSHA256 + `"}`
	digest := sha256.Sum256([]byte(canonical))
	return CausePrefix + hex.EncodeToString(digest[:]), nil
}

func groupIdentity(witnesses, causes []string) string {
	witnesses = append([]string(nil), witnesses...)
	causes = append([]string(nil), causes...)
	sort.Strings(witnesses)
	sort.Strings(causes)
	capacity := len(`{"causeIds":[],"witnessEvidenceIds":[]}`)
	for _, value := range causes {
		capacity += len(value) + 3
	}
	for _, value := range witnesses {
		capacity += len(value) + 3
	}
	canonical := make([]byte, 0, capacity)
	canonical = append(canonical, `{"causeIds":`...)
	canonical = appendCanonicalStrings(canonical, causes)
	canonical = append(canonical, `,"witnessEvidenceIds":`...)
	canonical = appendCanonicalStrings(canonical, witnesses)
	canonical = append(canonical, '}')
	digest := sha256.Sum256([]byte(canonical))
	return GroupPrefix + hex.EncodeToString(digest[:])
}

func appendCanonicalStrings(output []byte, values []string) []byte {
	output = append(output, '[')
	for index, value := range values {
		if index != 0 {
			output = append(output, ',')
		}
		// Cause and evidence IDs use closed ASCII prefixes plus lowercase hex,
		// so no JSON escape can be required here.
		output = append(output, '"')
		output = append(output, value...)
		output = append(output, '"')
	}
	return append(output, ']')
}
