package dogfoodflow

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/Beamfall/corvint/internal/trace"
)

// quotedRunAlternation is a verify line whose one quoted word is a -run
// alternation of plain test names, such as go test ./pkg -run 'TestA|TestB'
// (DCW-V0-384). The words around it hold only the recorder's v1 command
// bytes, and the quoted word holds the same bytes, without space or comma,
// joined by |. No other quote, anchor, group or class is admitted.
var quotedRunAlternation = regexp.MustCompile(`^([A-Za-z0-9_./:@=+, -]+) -run '([A-Za-z0-9_./:@=+-]+(?:\|[A-Za-z0-9_./:@=+-]+)+)'((?: [A-Za-z0-9_./:@=+, -]*)?)$`)

// verifyLineArguments is one verify line's recorder argument pair: a quoted
// -run alternation becomes a typed argv entry whose quoted word is exactly
// one argument, so no shell ever parses it (LTPM-V0-013); every other line
// stays a v1 command under the unchanged grammar.
func verifyLineArguments(line string) []string {
	match := quotedRunAlternation.FindStringSubmatch(strings.TrimSpace(line))
	if match == nil {
		return []string{"--verify", line}
	}
	argv := append(strings.Fields(match[1]), "-run", match[2])
	argv = append(argv, strings.Fields(match[3])...)
	encoded, err := trace.CanonicalArgv(argv)
	if err != nil {
		return []string{"--verify", line}
	}
	return []string{"--verify-argv-json", string(encoded)}
}

// validateVerify checks verifyInput's argument pairs against the recorder's
// admitted v1 command syntax, typed argv bounds and combined count.
func validateVerify(verify []string) error {
	if len(verify)/2 > trace.MaxVerificationCommands {
		return fmt.Errorf("verification exceeds %d commands", trace.MaxVerificationCommands)
	}
	commands := make([]string, 0, len(verify)/2)
	for i := 1; i < len(verify); i += 2 {
		if verify[i-1] == "--verify-argv-json" {
			if _, err := trace.ParseVerificationArgv([]byte(verify[i])); err != nil {
				return err
			}
			continue
		}
		commands = append(commands, verify[i])
	}
	return trace.ValidateVerificationCommands(commands)
}
