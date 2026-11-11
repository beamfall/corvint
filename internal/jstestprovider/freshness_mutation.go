package jstestprovider

import (
	"regexp"
	"strings"

	"github.com/Beamfall/corvint/internal/jstestreceipt"
)

// ResponseMutationDefinition is jstestreceipt.ResponseMutationDefinition, kept under its old name.
type ResponseMutationDefinition = jstestreceipt.ResponseMutationDefinition

var freshAssertionANSI = regexp.MustCompile("\x1b\\[[0-9;]*m")

// TargetAssertionFailure recognizes the registered marker and actual locator
// assertion shape from the native row. Playwright's terminal color controls
// are removed narrowly; unrelated errors cannot establish a target kill.
func TargetAssertionFailure(t TestOutcome, id string) bool {
	if !jstestreceipt.TokenPattern.MatchString(id) || t.State != StateFailed || t.Retries != 0 || len(t.Attempts) != 1 || t.Attempts[0].State != StateFailed || t.Attempts[0].Retry != 0 || t.Attempts[0].FailureKind != "assertion-or-test" {
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

// ResponseMutation is jstestreceipt.ResponseMutation, kept under its old name.
func ResponseMutation(source []byte, path string, d ResponseMutationDefinition) ([]byte, error) {
	return jstestreceipt.ResponseMutation(source, path, d)
}
