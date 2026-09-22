package main

import (
	"errors"
	"fmt"
	"strings"

	"github.com/Beamfall/corvint/internal/diagnostic"
)

// diagnosticMembers renders the DRC-V0-006 additive members of a refusal envelope in the
// sorted-key order emitError writes: evidence precedes "ok", and subject, supported_fixes and
// terminal follow it. Only the screened and bounded form is rendered (DRC-V0-008). A refusal
// without a diagnostic yields two empty strings, so its envelope stays byte-identical.
func diagnosticMembers(err error) (beforeOK, afterOK string) {
	var carried *diagnostic.Error
	if !errors.As(err, &carried) {
		return "", ""
	}
	refusal := carried.Refusal.Bounded()
	evidence := make([]string, 0, len(refusal.Evidence))
	for _, pair := range refusal.Evidence {
		evidence = append(evidence, fmt.Sprintf("{\"name\": %s, \"value\": %s}", pythonJSONString(pair.Name), pythonJSONString(pair.Value)))
	}
	fixes := make([]string, 0, len(refusal.SupportedFixes))
	for _, fix := range refusal.SupportedFixes {
		fixes = append(fixes, pythonJSONString(fix))
	}
	beforeOK = "\"evidence\": [" + strings.Join(evidence, ", ") + "], "
	afterOK = fmt.Sprintf(
		", \"subject\": {\"kind\": %s, \"value\": %s}, \"supported_fixes\": [%s]",
		pythonJSONString(refusal.Subject.Kind), pythonJSONString(refusal.Subject.Value), strings.Join(fixes, ", "),
	)
	if refusal.Terminal != "" {
		afterOK += ", \"terminal\": " + pythonJSONString(refusal.Terminal)
	}
	return beforeOK, afterOK
}

// addDiagnosticRowMembers adds the same bounded DRC-V0-006 members to a batch operation's error
// object (SBQ-V0-004), which the canonical encoder orders; a refusal without a diagnostic adds none.
func addDiagnosticRowMembers(row map[string]any, err error) {
	var carried *diagnostic.Error
	if !errors.As(err, &carried) {
		return
	}
	refusal := carried.Refusal.Bounded()
	evidence := make([]any, 0, len(refusal.Evidence))
	for _, pair := range refusal.Evidence {
		evidence = append(evidence, map[string]any{"name": pair.Name, "value": pair.Value})
	}
	fixes := make([]any, 0, len(refusal.SupportedFixes))
	for _, fix := range refusal.SupportedFixes {
		fixes = append(fixes, fix)
	}
	row["subject"] = map[string]any{"kind": refusal.Subject.Kind, "value": refusal.Subject.Value}
	row["evidence"] = evidence
	row["supported_fixes"] = fixes
	if refusal.Terminal != "" {
		row["terminal"] = refusal.Terminal
	}
}
