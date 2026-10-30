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

// unquotedRunAlternation is the same line with the alternation left unquoted,
// go test ./pkg -run TestA|TestB, which the recorder refuses as a shell pipe;
// its refusal names the quoted form instead of the grammar (DCW-V0-403).
var unquotedRunAlternation = regexp.MustCompile(`^([A-Za-z0-9_./:@=+, -]+) -run ([A-Za-z0-9_./:@=+-]+(?:\|[A-Za-z0-9_./:@=+-]+)+)((?: [A-Za-z0-9_./:@=+, -]*)?)$`)

// quotedRunFix names the first refused line of the verify file at path and
// its admitted quoted form when that line is an unquoted -run alternation,
// and returns "" otherwise (DCW-V0-403). It reads the file once, so the line
// it names is the line it rewrites even when the file changes meanwhile.
func quotedRunFix(path string) string {
	data, err := readPrefix(path, maxVerifyFileBytes)
	if path == "" || err != nil {
		return ""
	}
	line, text := refusedVerifyLine(data)
	if line == 0 {
		return ""
	}
	match := unquotedRunAlternation.FindStringSubmatch(strings.TrimSpace(text))
	if match == nil {
		return ""
	}
	quoted := match[1] + " -run '" + match[2] + "'" + match[3]
	if validateVerify(verifyLineArguments(quoted)) != nil {
		return ""
	}
	return fmt.Sprintf("%s line %d: quote the -run alternation as one word: %s (DCW-V0-403)", path, line, quoted)
}

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
