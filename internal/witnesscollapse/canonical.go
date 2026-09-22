package witnesscollapse

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"reflect"
)

// Canonical renders the closed struct-only report with one terminal LF.
func Canonical(report Report) ([]byte, string, error) {
	document, err := json.Marshal(report)
	if err != nil {
		return nil, "", failure("report-encode-failed", "report cannot be encoded")
	}
	document = append(document, '\n')
	digest := sha256.Sum256(document)
	return document, hex.EncodeToString(digest[:]), nil
}

// Verify reconstructs the declarations carried by report, recompiles against
// exact CEM bytes, and requires byte-identical canonical output.
func Verify(rawCEM []byte, report Report) error {
	attributions := make([]Attribution, 0)
	for _, cause := range report.Causes {
		for _, member := range cause.Members {
			attributions = append(attributions, Attribution{
				WitnessEvidenceID: member.WitnessEvidenceID, Kind: cause.Kind,
				CauseSubjectSHA256:    cause.SubjectSHA256,
				AttestationEvidenceID: member.AttestationEvidenceID,
			})
		}
	}
	recompiled, err := Compile(rawCEM, attributions)
	if err != nil {
		return err
	}
	// Report is a closed struct-only shape and Compile sorts every set-valued
	// field. Exact structural equality is therefore equivalent to equality of
	// Canonical bytes, while avoiding two maximum-size JSON materializations.
	// DeepEqual deliberately distinguishes nil from empty slices, matching the
	// canonical JSON distinction between null and [].
	if !reflect.DeepEqual(report, recompiled) {
		return failure("report-mismatch", "report differs from deterministic recompilation")
	}
	return nil
}
