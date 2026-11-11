package jstestreceipt

import (
	"bytes"
	"errors"

	"github.com/Beamfall/corvint/internal/secretscreen"
)

// ResponseMutationDefinition is the closed response-only delta. It never
// authorizes changing tracked Git inputs, the test, or the serving command.
type ResponseMutationDefinition struct {
	ArtifactPath string `json:"artifactPath"`
	SourceSHA256 string `json:"sourceSha256"`
	From         string `json:"from"`
	To           string `json:"to"`
	MutantSHA256 string `json:"mutantSha256"`
}

// ResponseMutation derives the mutant from actual retained source bytes. Pins
// are checked against that derivation, not used as a replacement for bytes.
func ResponseMutation(source []byte, path string, d ResponseMutationDefinition) ([]byte, error) {
	if len(source) == 0 || len(source) > 64<<10 || d.ArtifactPath != path || path == "" || !RawDigestPattern.MatchString(d.SourceSHA256) || !RawDigestPattern.MatchString(d.MutantSHA256) || Sha256Hex(source) != d.SourceSHA256 || d.From == "" || len(d.From) > 1024 || len(d.To) > 1024 || d.From == d.To || bytes.Count(source, []byte(d.From)) != 1 || secretscreen.MatchString(string(source)) || secretscreen.MatchString(d.From) || secretscreen.MatchString(d.To) {
		return nil, errors.New("freshness-response-delta-invalid")
	}
	mutant := bytes.Replace(source, []byte(d.From), []byte(d.To), 1)
	if len(mutant) == 0 || len(mutant) > 64<<10 || Sha256Hex(mutant) != d.MutantSHA256 || secretscreen.MatchString(string(mutant)) {
		return nil, errors.New("freshness-response-delta-invalid")
	}
	return mutant, nil
}
