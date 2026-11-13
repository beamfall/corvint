// SPDX-License-Identifier: Apache-2.0

// Package killswitch reads the CORVINT_SELECTION kill switch. Its grammar is
// the one NMS-V1-008 (change `narrowing-measurement`) owns: exactly `full`,
// which makes every runner run its full command, or
// `full:<runner>[,<runner>]`, which makes each named runner run its full
// command on every instance. Every surface reads the parsed value from this
// package and defines no grammar of its own (RNC-V1-008). The package reads
// no file and starts no process.
package killswitch

import (
	"fmt"
	"slices"
	"strings"

	"github.com/Beamfall/corvint/internal/testid"
)

// Env is the environment variable the switch is read from.
const Env = "CORVINT_SELECTION"

// Reason is the fallback reason a switched runner prints and records.
const Reason = "kill-switch"

// CodeUnknown is the refusal code of a value outside the grammar: a CLI
// surface exits 2 with it and runs nothing (owner decision R13).
const CodeUnknown = "kill-switch-value-unknown"

// CodeActive is the refusal of a surface that cannot run the full command
// the switch asks for (corvint watch, which answers only narrowed Go
// selections): it exits 2 rather than answer narrowed.
const CodeActive = "kill-switch-full-unsupported"

// Switch is one parsed value. The zero Switch is off.
type Switch struct {
	// Raw is the value as received.
	Raw string
	// All is set by `full`: every runner runs its full command.
	All bool
	// Runners are the runners `full:<runner>[,<runner>]` names, sorted and
	// unique.
	Runners []string
}

// Active reports whether the switch forces any runner to run in full.
func (s Switch) Active() bool { return s.All || len(s.Runners) != 0 }

// Full reports whether runner runs its full command under the switch.
func (s Switch) Full(runner string) bool { return s.All || slices.Contains(s.Runners, runner) }

// Describe is the switch as one line a surface prints, naming the raw value.
func (s Switch) Describe() string {
	if s.All {
		return Reason + ": " + Env + "=" + s.Raw + " makes every runner run its full command"
	}
	return Reason + ": " + Env + "=" + s.Raw + " makes " + strings.Join(s.Runners, ", ") + " run the full command on every instance"
}

// Accepted names the accepted forms, for a refusal message.
func Accepted() string {
	return "`full` or `full:<runner>[,<runner>]` with runners " + strings.Join(testid.Runners, ", ")
}

// Parse reads value. An empty value is off, as an unset variable is. Any
// other value must be exactly `full` or `full:` followed by a comma list of
// known runner tokens; anything else, a different case, whitespace or an
// empty list item included, is refused with an error naming the value and
// the accepted forms, never guessed.
func Parse(value string) (Switch, error) {
	switch {
	case value == "":
		return Switch{}, nil
	case value == "full":
		return Switch{Raw: value, All: true}, nil
	}
	list, ok := strings.CutPrefix(value, "full:")
	if !ok || list == "" {
		return Switch{}, unknown(value)
	}
	var runners []string
	for _, runner := range strings.Split(list, ",") {
		if !testid.KnownRunner(runner) {
			return Switch{}, unknown(value)
		}
		if !slices.Contains(runners, runner) {
			runners = append(runners, runner)
		}
	}
	slices.Sort(runners)
	return Switch{Raw: value, Runners: runners}, nil
}

// FromEnv parses the switch from getenv(Env).
func FromEnv(getenv func(string) string) (Switch, error) {
	return Parse(getenv(Env))
}

func unknown(value string) error {
	return fmt.Errorf("%s=%q is not a kill-switch value; the accepted forms are %s", Env, value, Accepted())
}
