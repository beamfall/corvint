package dogfoodflow

import (
	"fmt"
	"os"
	"path/filepath"
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

// runQuotedWord is runAlternation plus the single forms only a quoted word
// admits: one plain name (-run 'TestA', -run '.'), one anchored name
// (-run '^TestA$') and the match-everything -run '.*' (DCW-V0-904). Only an
// unquoted alternation gets a quoted-form suggestion; the generic hint names
// these forms.
const runQuotedWord = `(?:` + runAlternation + `|` + runName + `|\^` + runName + `\$|\.\*)`

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
// around it hold only the recorder's v1 command bytes. DCW-V0-904 also admits
// one plain or anchored name and .*; no other quote, class or nested group is
// admitted.
var quotedRunAlternation = regexp.MustCompile(`^` + runPrefix + ` -run(=| )(?:'(` + runQuotedWord + `)'|"(` + runQuotedWord + `)")` + runSuffix + `$`)

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

// envAssignment is one leading NAME=value word of a verify line whose value
// may be single-quoted (printable ASCII but ') or double-quoted (printable
// ASCII but ", $, `, \ and !), so that every quoted byte is literal to a POSIX
// shell; an unquoted value holds only the recorder's v1 bytes (DCW-V0-804).
var envAssignment = regexp.MustCompile(`^([A-Za-z_][A-Za-z0-9_]*)=(?:'([\x20-\x26\x28-\x7e]*)'|"([\x20\x23\x25-\x5b\x5d-\x5f\x61-\x7e]*)"|([A-Za-z0-9_./:@=+,-]*)) +`)

// envAssignments splits the leading NAME=value words off a verify line. It
// returns each as one argv word with its quotes removed and the rest of the
// line, and quoted reports whether any value was quoted; a line with no
// quoted value keeps its v1 command form (DCW-V0-804).
func envAssignments(line string) (words []string, rest string, quoted bool) {
	rest = line
	for {
		match := envAssignment.FindStringSubmatch(rest)
		if match == nil {
			return words, rest, quoted
		}
		if opening := rest[len(match[1])+1]; opening == '\'' || opening == '"' {
			quoted = true
		}
		words = append(words, match[1]+"="+match[2]+match[3]+match[4])
		rest = rest[len(match[0]):]
	}
}

// verifyLineArguments is one verify line's recorder argument pair: a quoted
// -run alternation becomes a typed argv entry whose quoted word is exactly
// one argument, so no shell ever parses it (LTPM-V0-013); a line led by
// quoted NAME=value assignments, such as RUN_REASON='whole run'
// script/run.sh go test ./a, becomes a typed argv entry whose first words
// are those assignments verbatim, without their quotes (DCW-V0-804); every
// other line stays a v1 command under the unchanged grammar.
func verifyLineArguments(line string) []string {
	assignments, rest, quoted := envAssignments(strings.TrimSpace(line))
	match := quotedRunAlternation.FindStringSubmatch(rest)
	if match == nil {
		if !quoted || trace.ValidateVerificationCommands([]string{rest}) != nil {
			return []string{"--verify", line}
		}
		return typedVerifyArguments(line, append(assignments, strings.Fields(rest)...))
	}
	word := match[3] + match[4]
	argv := append(assignments, strings.Fields(match[1])...)
	if match[2] == "=" {
		argv = append(argv, "-run="+word)
	} else {
		argv = append(argv, "-run", word)
	}
	argv = append(argv, strings.Fields(match[5])...)
	return typedVerifyArguments(line, argv)
}

// typedVerifyArguments is the typed recorder argument pair for argv, or the
// v1 pair for line when argv has no canonical encoding.
func typedVerifyArguments(line string, argv []string) []string {
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

// tildeFix names the first refused line of the verify file at path whose word
// starts with ~ and gives it with each leading ~ replaced by the home
// directory, or returns "" when there is none or no home directory is known
// (DCW-V0-905). The recorded command is never expanded silently.
func tildeFix(path string) string {
	data, err := readPrefix(path, maxVerifyFileBytes)
	home, homeErr := os.UserHomeDir()
	if path == "" || err != nil || homeErr != nil || !filepath.IsAbs(home) {
		return ""
	}
	line, text := refusedVerifyLine(data)
	if line == 0 {
		return ""
	}
	words := strings.Fields(text)
	changed := false
	for index, word := range words {
		if word == "~" || strings.HasPrefix(word, "~/") {
			words[index] = home + word[1:]
			changed = true
		}
	}
	if !changed {
		return ""
	}
	return fmt.Sprintf("%s line %d: write the path in full, not with ~: %s (DCW-V0-905)", path, line, strings.Join(words, " "))
}
