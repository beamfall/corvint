package analyzerstructured

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

type descriptorBody struct {
	Protocol       string           `json:"protocol"`
	Profile        string           `json:"profile"`
	Family         string           `json:"family"`
	FormatProfiles []FormatProfile  `json:"format_profiles"`
	FactSchema     string           `json:"fact_schema"`
	Limits         descriptorLimits `json:"limits"`
}
type descriptorLimits struct {
	FrameBytes  int `json:"frame_bytes"`
	Base64Bytes int `json:"base64_bytes"`
	InputBytes  int `json:"input_bytes"`
	Inputs      int `json:"inputs"`
	Facts       int `json:"facts"`
	OutputBytes int `json:"output_bytes"`
	Depth       int `json:"depth"`
	Tokens      int `json:"tokens"`
	StringBytes int `json:"string_bytes"`
}
type Descriptor struct {
	descriptorBody
	IdentitySHA256 string `json:"identity_sha256"`
}

// CandidateDescriptor returns the stable byte-bound selection descriptor. It is
// informative only: it does not register, select, or authorize this candidate.
func CandidateDescriptor() ([]byte, error) {
	body := descriptorBody{"corvint-structured-data-protocol/1", Profile, Family, formatProfiles[:], "structured-coordinate/v1", descriptorLimits{maxFrameBytes, maxBase64Bytes, maxInputBytes, maxInputs, maxFacts, maxOutputBytes, maxDepth, maxTokenCount, maxStringBytes}}
	canonical, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(canonical)
	return append(jsonAppend(Descriptor{body, "sha256:" + hex.EncodeToString(sum[:])}), '\n'), nil
}
func jsonAppend(value any) []byte { encoded, _ := json.Marshal(value); return encoded }
