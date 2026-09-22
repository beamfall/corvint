// Package diagnostic is the refusal envelope of docs/specs/diagnostic-repair-contract-v0.md
// (DRC-V0): a refused subject, the facts measured while refusing, and the closed set of
// registry-owned repairs, carried beside an existing {code, message} error.
package diagnostic

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"unicode"
	"unicode/utf8"
)

// Kinds is DRC-V0-001's closed subject vocabulary; docs/specs/FIX-REGISTRY.tsv's kind rows
// and the clause text must equal it.
var Kinds = []string{"argument", "value", "artifact", "repository-state", "host-capability", "request"}

// TerminalReasons is DRC-V0-005's closed set of reasons no caller-side repair exists.
var TerminalReasons = []string{"missing-authority", "absent-evidence", "unsupported-platform", "owner-decision-required"}

// Subject names the refused thing (DRC-V0-001).
type Subject struct {
	Kind  string `json:"kind"`
	Value string `json:"value"`
}

// Evidence is one fact measured while producing the refusal (DRC-V0-002).
type Evidence struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// Refusal is the additive diagnostic a covered site attaches to its refusal.
type Refusal struct {
	Subject        Subject    `json:"subject"`
	Evidence       []Evidence `json:"evidence"`
	SupportedFixes []string   `json:"supported_fixes"`
	Terminal       string     `json:"terminal,omitempty"`
}

// MarshalJSON emits an absent evidence or fix list as an empty array, never null or omitted
// (DRC-V0-002, DRC-V0-005).
func (refusal Refusal) MarshalJSON() ([]byte, error) {
	type plain Refusal
	emitted := plain(refusal)
	emitted.Evidence = nonNil(emitted.Evidence)
	emitted.SupportedFixes = nonNil(emitted.SupportedFixes)
	return json.Marshal(emitted)
}

func nonNil[T any](values []T) []T {
	if values == nil {
		return []T{}
	}
	return values
}

// Error carries a Refusal beside the error it annotates; errors.As still reaches the
// wrapped error, so a consumer keyed on its code is unchanged (DRC-V0-006).
type Error struct {
	Err     error
	Refusal Refusal
}

func (err *Error) Error() string { return err.Err.Error() }

func (err *Error) Unwrap() error { return err.Err }

// Validate reports the first way refusal breaks DRC-V0-001, 002, 003 or 005.
func (refusal Refusal) Validate() error {
	checks := []func(Refusal) error{validateKind, validateValue, validateEvidence, validateFixes, validateTerminal}
	for _, check := range checks {
		if err := check(refusal); err != nil {
			return err
		}
	}
	return nil
}

func validateKind(refusal Refusal) error {
	if !slices.Contains(Kinds, refusal.Subject.Kind) {
		return fmt.Errorf("subject kind %q is outside the DRC-V0-001 vocabulary", refusal.Subject.Kind)
	}
	return nil
}

func validateValue(refusal Refusal) error {
	return singleLine("subject value", refusal.Subject.Value)
}

func validateEvidence(refusal Refusal) error {
	for _, pair := range refusal.Evidence {
		if err := singleLine("evidence name", pair.Name); err != nil {
			return err
		}
	}
	return nil
}

func validateFixes(refusal Refusal) error {
	for position, fix := range refusal.SupportedFixes {
		if err := singleLine("fix identifier", fix); err != nil {
			return err
		}
		if slices.Contains(refusal.SupportedFixes[:position], fix) {
			return fmt.Errorf("fix identifier %q is repeated", fix)
		}
	}
	return nil
}

func validateTerminal(refusal Refusal) error {
	if len(refusal.SupportedFixes) != 0 && refusal.Terminal != "" {
		return errors.New("terminal is set beside a non-empty supported_fixes")
	}
	if len(refusal.SupportedFixes) == 0 && !slices.Contains(TerminalReasons, refusal.Terminal) {
		return fmt.Errorf("empty supported_fixes needs a DRC-V0-005 terminal reason, got %q", refusal.Terminal)
	}
	return nil
}

func singleLine(field, value string) error {
	if value == "" || !utf8.ValidString(value) {
		return fmt.Errorf("%s must be non-empty valid UTF-8", field)
	}
	if slices.ContainsFunc([]rune(value), unicode.IsControl) {
		return fmt.Errorf("%s %q carries a control character", field, value)
	}
	return nil
}
