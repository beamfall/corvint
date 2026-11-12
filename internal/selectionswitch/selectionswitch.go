// SPDX-License-Identifier: Apache-2.0

// Package selectionswitch reads the CORVINT_SELECTION kill switch (owner
// decision R13, PROD-TEST-013). Its grammar is owned by the kill-switch
// requirement of change narrowing-measurement: exactly `full`, which forces
// every runner to a full run, or `full:<runner>[,<runner>]`, which forces
// each named runner. Any
// other non-empty value is unknown. Parse is the grammar's one parser: the
// MCP affected tool is its only caller today, selecting every test on any
// value (MTT-V1-008), and no command line reads the variable yet; the
// change's CLI reader, which exits 2 on an unknown value, must call Parse
// too, so the two can never disagree.
package selectionswitch

import (
	"regexp"
	"slices"
	"strings"
)

const (
	// Variable is the environment variable the switch is read from.
	Variable = "CORVINT_SELECTION"
	// ReasonKillSwitch names a selection forced full by a valid value.
	ReasonKillSwitch = "kill-switch"
	// ReasonUnknownValue names a selection forced full because the value is
	// not in the grammar; it is also the CLI's refusal code.
	ReasonUnknownValue = "kill-switch-value-unknown"
)

// runnerName is one runner token of `full:<runner>`. Which runners exist is
// the planner's knowledge; the grammar only fixes the token's shape.
var runnerName = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*$`)

// Switch is one parsed CORVINT_SELECTION value.
type Switch struct {
	// Value is the raw value; empty means the switch is off.
	Value string
	// All is true for `full`.
	All bool
	// Runners lists the runners `full:<runner>[,<runner>]` names, in order.
	Runners []string
	// Unknown is true for a non-empty value outside the grammar.
	Unknown bool
}

// Parse reads one value. The empty value is the switch off; nothing is
// trimmed, so a value with stray spaces is unknown.
func Parse(value string) Switch {
	switch {
	case value == "":
		return Switch{}
	case value == "full":
		return Switch{Value: value, All: true}
	}
	list, ok := strings.CutPrefix(value, "full:")
	if !ok {
		return Switch{Value: value, Unknown: true}
	}
	runners := strings.Split(list, ",")
	for index, runner := range runners {
		if !runnerName.MatchString(runner) || slices.Contains(runners[:index], runner) {
			return Switch{Value: value, Unknown: true}
		}
	}
	return Switch{Value: value, Runners: runners}
}

// On reports whether the value forces any selection full or is unknown.
func (s Switch) On() bool { return s.Value != "" }

// Reason is the typed reason a forced-full selection carries: kill-switch for
// a valid value, kill-switch-value-unknown for an unknown one, "" when off.
func (s Switch) Reason() string {
	switch {
	case s.Unknown:
		return ReasonUnknownValue
	case s.On():
		return ReasonKillSwitch
	}
	return ""
}
