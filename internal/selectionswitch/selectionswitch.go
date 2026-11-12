// SPDX-License-Identifier: Apache-2.0

// Package selectionswitch reads the CORVINT_SELECTION kill switch. Its
// grammar is owned by the kill-switch requirement of change
// `narrowing-measurement` (owner decision R13 (2026-11-09)) and is
// exactly two forms: `full`, which forces every runner's full suite, and
// `full:<runner>[,<runner>]`, which forces each named runner's full suite on
// every instance. The reason is `kill-switch`; any other value or runner
// name is refused with `kill-switch-value-unknown`. This package parses
// only; each surface decides what a forced runner runs.
package selectionswitch

import (
	"fmt"
	"slices"
	"strings"

	"github.com/Beamfall/corvint/internal/testid"
)

// Env is the environment variable the switch is read from.
const Env = "CORVINT_SELECTION"

// Reason names a full run the switch forced; ReasonUnknown refuses a value
// outside the grammar (owner decision R13).
const (
	Reason        = "kill-switch"
	ReasonUnknown = "kill-switch-value-unknown"
)

// Switch is one parsed value. The zero Switch forces nothing.
type Switch struct {
	// All is set by `full`.
	All bool
	// Runners are the runners `full:<runner>[,<runner>]` names, sorted
	// and distinct.
	Runners []string
	// Value is the value as given, for the reason line.
	Value string
}

// Parse reads value. An empty value is the zero Switch. A runner must be
// one of the blocking runners (CTI-V1-001).
func Parse(value string) (Switch, error) {
	switch {
	case value == "":
		return Switch{}, nil
	case value == "full":
		return Switch{All: true, Value: value}, nil
	}
	list, ok := strings.CutPrefix(value, "full:")
	if !ok || list == "" {
		return Switch{}, fmt.Errorf("%s: %s=%q: want full or full:<runner>[,<runner>]", ReasonUnknown, Env, value)
	}
	parsed := Switch{Value: value}
	for _, runner := range strings.Split(list, ",") {
		if !slices.Contains(testid.Runners, runner) {
			return Switch{}, fmt.Errorf("%s: %s=%q names runner %q: want one of %s", ReasonUnknown, Env, value, runner, strings.Join(testid.Runners, ", "))
		}
		if !slices.Contains(parsed.Runners, runner) {
			parsed.Runners = append(parsed.Runners, runner)
		}
	}
	slices.Sort(parsed.Runners)
	return parsed, nil
}

// Forces reports whether the switch forces runner's full suite.
func (s Switch) Forces(runner string) bool {
	return s.All || slices.Contains(s.Runners, runner)
}

// Line is the switch's reason line for runner: `kill-switch:
// CORVINT_SELECTION=<value> forces ...`.
func (s Switch) Line(subject string) string {
	return fmt.Sprintf("%s: %s=%s forces %s", Reason, Env, s.Value, subject)
}
