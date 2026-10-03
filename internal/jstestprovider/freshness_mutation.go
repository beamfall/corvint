package jstestprovider

import (
	"bytes"
	"errors"
	"regexp"
	"strings"

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

var freshAssertionANSI = regexp.MustCompile("\x1b\\[[0-9;]*m")

// TargetAssertionFailure recognizes the registered marker and actual locator
// assertion shape from the native row. Playwright's terminal color controls
// are removed narrowly; unrelated errors cannot establish a target kill.
func TargetAssertionFailure(t TestOutcome, id string) bool {
	if !tokenPattern.MatchString(id) || t.State != StateFailed || t.Retries != 0 || len(t.Attempts) != 1 || t.Attempts[0].State != StateFailed || t.Attempts[0].Retry != 0 || t.Attempts[0].FailureKind != "assertion-or-test" {
		return false
	}
	message := freshAssertionANSI.ReplaceAllString(t.FailureMessage, "")
	line, _, ok := strings.Cut(message, "\n")
	if !ok {
		return false
	}
	marker := "PTF-ASSERTION:" + id
	return (line == marker || line == "Error: "+marker) && strings.Contains(message, "expect(locator).toHaveText(expected) failed")
}

// ResponseMutation derives the mutant from actual retained source bytes. Pins
// are checked against that derivation, not used as a replacement for bytes.
func ResponseMutation(source []byte, path string, d ResponseMutationDefinition) ([]byte, error) {
	if len(source) == 0 || len(source) > 64<<10 || d.ArtifactPath != path || path == "" || !rawDigestPattern.MatchString(d.SourceSHA256) || !rawDigestPattern.MatchString(d.MutantSHA256) || sha256Hex(source) != d.SourceSHA256 || d.From == "" || len(d.From) > 1024 || len(d.To) > 1024 || d.From == d.To || bytes.Count(source, []byte(d.From)) != 1 || secretscreen.MatchString(string(source)) || secretscreen.MatchString(d.From) || secretscreen.MatchString(d.To) {
		return nil, errors.New("freshness-response-delta-invalid")
	}
	mutant := bytes.Replace(source, []byte(d.From), []byte(d.To), 1)
	if len(mutant) == 0 || len(mutant) > 64<<10 || sha256Hex(mutant) != d.MutantSHA256 || secretscreen.MatchString(string(mutant)) {
		return nil, errors.New("freshness-response-delta-invalid")
	}
	return mutant, nil
}
