package dogfoodflow

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/Beamfall/corvint/internal/trace"
)

// runName is one plain test name of a -run alternation: the recorder's v1
// command bytes without space or comma.
const runName = `[A-Za-z0-9_./:@=+-]+`

// runAlternation is the regex of one quoted -run word: names joined by |,
// bare (TestA|TestB), as one anchored group (^(TestA|TestB)$), or with every
// name anchored (^TestA$|^TestB$) (DCW-V0-384, DCW-V0-801).
const runAlternation = `(?:` + runName + `(?:\|` + runName + `)+` +
	`|\^\(` + runName + `(?:\|` + runName + `)+\)\$` +
	`|\^` + runName + `\$(?:\|\^` + runName + `\$)+)`

// runPrefix and runSuffix are the words around the -run word: the
// recorder's v1 command bytes only.
const (
	runPrefix = `([A-Za-z0-9_./:@=+, -]+)`
	runSuffix = `((?: [A-Za-z0-9_./:@=+, -]*)?)`
)

// quotedRunAlternation is a verify line whose one quoted word is a -run
// alternation of plain test names, such as go test ./pkg -run 'TestA|TestB'
// (DCW-V0-384). DCW-V0-801 also admits the word in double quotes, attached as
// -run='TestA|TestB', and anchored as ^(TestA|TestB)$ or ^TestA$|^TestB$:
// inside either quote every one of those bytes is literal to a POSIX shell,
// and the word is recorded as one argv argument no shell parses. The words
// around it hold only the recorder's v1 command bytes. No other quote,
// class, nested group or single anchored name is admitted.
var quotedRunAlternation = regexp.MustCompile(`^` + runPrefix + ` -run(=| )(?:'(` + runAlternation + `)'|"(` + runAlternation + `)")` + runSuffix + `$`)

// unquotedRunAlternation is the same line with the -run word unquoted, which
// a shell would split at |; the refusal shows the line's quoted form
// (DCW-V0-801).
var unquotedRunAlternation = regexp.MustCompile(`^` + runPrefix + ` -run(=| )(` + runAlternation + `)` + runSuffix + `$`)

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
	// DCW-V0-403 and DCW-V0-801 share one unquoted-alternation grammar.
	quoted := quotedRunSuggestion(text)
	if quoted == "" {
		return ""
	}
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
	word := match[3] + match[4]
	argv := strings.Fields(match[1])
	if match[2] == "=" {
		argv = append(argv, "-run="+word)
	} else {
		argv = append(argv, "-run", word)
	}
	argv = append(argv, strings.Fields(match[5])...)
	encoded, err := trace.CanonicalArgv(argv)
	if err != nil {
		return []string{"--verify", line}
	}
	return []string{"--verify-argv-json", string(encoded)}
}

// quotedRunSuggestion is the admitted form of a verify line whose -run
// alternation is unquoted, or "" for any other line (DCW-V0-801).
func quotedRunSuggestion(line string) string {
	match := unquotedRunAlternation.FindStringSubmatch(strings.TrimSpace(line))
	if match == nil {
		return ""
	}
	return match[1] + " -run" + match[2] + "'" + match[3] + "'" + match[4]
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

// quotedRunSuggestions is one "line N: FORM" entry per verify input line
// whose -run alternation is unquoted, FORM being that line's admitted quoted
// form (DCW-V0-801). The verify input is the file when one is named, else the
// inline lines.
func quotedRunSuggestions(options ChangeOptions) []string {
	text := options.Verify
	if options.VerifyFile != "" {
		data, err := readPrefix(options.VerifyFile, maxVerifyFileBytes)
		if err != nil {
			return nil
		}
		text = string(data)
	}
	var suggestions []string
	for index, line := range strings.Split(text, "\n") {
		if form := quotedRunSuggestion(line); form != "" {
			suggestions = append(suggestions, fmt.Sprintf("line %d: %s", index+1, form))
		}
	}
	return suggestions
}
