package witnesscollapse

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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
