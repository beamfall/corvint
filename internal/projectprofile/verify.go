package projectprofile

// Repository-declared verification commands (AFP-V0-009, AFP-V0-196,
// NGI-V0-060..062). A project's AGENTS.md is project-owned authority: what it
// declares as its verify commands outranks any command inferred from the
// repository's shape (AGENTS.md invariant 3). `corvint affected` and
// `corvint impact` read the declaration through these functions so the two
// cannot name different mandatory checks for the same instructions.

import "strings"

// VerifyHeading is the one heading whose fenced shell blocks declare checks.
const VerifyHeading = "Verify"

// InstructionsPath is the repository-root instruction file the declaration is
// read from.
const InstructionsPath = "AGENTS.md"

// MaxInstructionBytes bounds how much of the instruction file is parsed.
const MaxInstructionBytes = 256 << 10

// DeclaredCommand is one command the instructions declare. Inline is true for
// a command named on an inline **Verify:** line rather than in a fenced block
// under a Verify heading.
type DeclaredCommand struct {
	Command string
	Inline  bool
}

var verifyShellFences = map[string]bool{"sh": true, "bash": true, "console": true}

// inlineVerifyLabels open an inline declaration (AFP-V0-196).
var inlineVerifyLabels = []string{"**verify:**", "**verify**:"}

// VerifyCommands returns, in order of appearance, the command lines of every
// fenced sh/bash/console block under a heading whose text is exactly "Verify"
// in any case, with shell comments removed, and every command an inline
// **Verify:** line names outside a fence. It also returns, in order, each
// other heading containing "verify" that has such a block, whose commands are
// not taken as checks.
func VerifyCommands(body string) (commands []DeclaredCommand, unrecognized []string) {
	commands, unrecognized = []DeclaredCommand{}, []string{}
	heading, underVerify, inBlock, fence := "", false, false, ""
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		shell := verifyShellFences[strings.ToLower(strings.TrimPrefix(trimmed, "```"))]
		marker := markdownFence(trimmed)
		switch {
		case fence != "":
			// A non-shell fence hides its content from every rule.
			if marker != "" && marker[0] == fence[0] && len(marker) >= len(fence) && strings.TrimSpace(trimmed[len(marker):]) == "" {
				fence = ""
			}
		case inBlock && strings.HasPrefix(trimmed, "```"):
			inBlock = false
		case inBlock:
			if command := stripShellComment(strings.TrimPrefix(trimmed, "$ ")); command != "" {
				commands = append(commands, DeclaredCommand{Command: command})
			}
		case strings.HasPrefix(trimmed, "#"):
			heading = strings.TrimSpace(strings.Trim(trimmed, "#"))
			underVerify = strings.EqualFold(heading, VerifyHeading)
		case strings.HasPrefix(trimmed, "```") && shell && !underVerify && strings.Contains(strings.ToLower(heading), "verify"):
			unrecognized = appendOnce(unrecognized, heading)
			fence = marker
		case strings.HasPrefix(trimmed, "```") && shell && underVerify:
			inBlock = true
		case marker != "":
			fence = marker
		default:
			for _, command := range inlineVerifyCommands(trimmed) {
				commands = append(commands, DeclaredCommand{Command: command, Inline: true})
			}
		}
	}
	return commands, unrecognized
}

// inlineVerifyCommands reads one prose line: after an inline **Verify:**
// label, the single-backtick code spans that follow it separated only by
// spaces, commas, semicolons, "+", "and" or "then" are commands. The first
// other text ends the declaration, so a later span on the same line, such as a
// gate id in parentheses, is not a command.
func inlineVerifyCommands(line string) []string {
	lower := strings.ToLower(line)
	start := -1
	for _, label := range inlineVerifyLabels {
		if index := strings.Index(lower, label); index >= 0 && (start < 0 || index < start) {
			start = index + len(label)
		}
	}
	if start < 0 {
		return nil
	}
	commands := []string{}
	rest := line[start:]
	for {
		rest = strings.TrimLeft(rest, " \t,;+")
		for _, word := range []string{"and ", "then "} {
			rest = strings.TrimPrefix(rest, word)
		}
		rest = strings.TrimLeft(rest, " \t")
		if !strings.HasPrefix(rest, "`") || strings.HasPrefix(rest, "``") {
			return commands
		}
		end := strings.IndexByte(rest[1:], '`')
		if end < 0 {
			return commands
		}
		if command := strings.TrimSpace(rest[1 : end+1]); command != "" {
			commands = append(commands, command)
		}
		rest = rest[end+2:]
	}
}

// conditionPhrases mark a sentence that scopes a command to a condition. They
// are matched as whole words: every non-letter of the sentence is a space.
var conditionPhrases = []string{" unless ", " only when ", " only if "}

// VerifyScopes returns, in order, each prose sentence under a heading whose
// text is exactly "Verify", outside ``` and ~~~ fences, that states a
// condition with "unless", "only when" or "only if". A paragraph's lines are
// joined before it is split into sentences at ".", "!" or "?" followed by a
// space or its end; a blank line, a heading, a fence or a list item ends a
// paragraph.
func VerifyScopes(body string) []string {
	scopes := []string{}
	paragraph := []string{}
	flush := func() {
		text := strings.Join(paragraph, " ")
		paragraph = paragraph[:0]
		for len(text) > 0 {
			end := len(text)
			for index := 0; index < len(text); index++ {
				if strings.IndexByte(".!?", text[index]) >= 0 && (index+1 == len(text) || text[index+1] == ' ') {
					end = index + 1
					break
				}
			}
			sentence := strings.TrimSpace(text[:end])
			text = text[end:]
			words := strings.Join(strings.Fields(strings.Map(func(r rune) rune {
				if r >= 'a' && r <= 'z' {
					return r
				}
				return ' '
			}, strings.ToLower(sentence))), " ")
			for _, phrase := range conditionPhrases {
				if strings.Contains(" "+words+" ", phrase) {
					scopes = append(scopes, sentence)
					break
				}
			}
		}
	}
	underVerify, fence := false, ""
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		marker := markdownFence(trimmed)
		switch {
		case fence != "":
			if marker != "" && marker[0] == fence[0] && len(marker) >= len(fence) && strings.TrimSpace(trimmed[len(marker):]) == "" {
				fence = ""
			}
		case marker != "":
			flush()
			fence = marker
		case strings.HasPrefix(trimmed, "#"):
			flush()
			underVerify = strings.EqualFold(strings.TrimSpace(strings.Trim(trimmed, "#")), VerifyHeading)
		case trimmed == "":
			flush()
		case underVerify:
			if markdownListItem(trimmed) {
				flush()
			}
			paragraph = append(paragraph, trimmed)
		}
	}
	flush()
	return scopes
}

// ScopedBy returns the first scope sentence that names command in backticks,
// or "" when none does: a named command is conditional, not mandatory.
func ScopedBy(command string, scopes []string) string {
	for _, sentence := range scopes {
		if strings.Contains(sentence, "`"+command+"`") {
			return sentence
		}
	}
	return ""
}

// launchers are the commands whose only job is to open a program the caller
// then leaves running.
var launchers = map[string]bool{"open": true, "xdg-open": true}

// NonTerminatingCommand reports a command that launches a program or runs in
// the background. Any other command stays a check: requiring too much is safe.
func NonTerminatingCommand(command string) bool {
	fields := strings.Fields(command)
	background := strings.HasSuffix(command, "&") && !strings.HasSuffix(command, "&&")
	return background || len(fields) > 0 && launchers[fields[0]]
}

// MandatoryVerifyCommands returns, deduplicated in order of appearance, the
// declared commands that end on their own and that no Verify scope sentence
// makes conditional: the checks the instructions require on every change.
func MandatoryVerifyCommands(body string) []string {
	if len(body) > MaxInstructionBytes {
		body = body[:MaxInstructionBytes]
	}
	declared, _ := VerifyCommands(body)
	scopes := VerifyScopes(body)
	mandatory := []string{}
	for _, item := range declared {
		if NonTerminatingCommand(item.Command) || ScopedBy(item.Command, scopes) != "" {
			continue
		}
		mandatory = appendOnce(mandatory, item.Command)
	}
	return mandatory
}

// markdownListItem reports a line that opens a Markdown list item ("-", "*",
// "+", or digits then "." or ")", followed by a space), which starts a new
// paragraph.
func markdownListItem(trimmed string) bool {
	digits := 0
	for digits < len(trimmed) && trimmed[digits] >= '0' && trimmed[digits] <= '9' {
		digits++
	}
	marker := digits == 0 && len(trimmed) > 0 && strings.IndexByte("-*+", trimmed[0]) >= 0
	marker = marker || digits > 0 && digits < len(trimmed) && strings.IndexByte(".)", trimmed[digits]) >= 0
	width := digits + 1
	return marker && width < len(trimmed) && (trimmed[width] == ' ' || trimmed[width] == '\t')
}

// markdownFence returns the run of three or more backticks or tildes that
// opens or closes a fenced block on a trimmed line, or "".
func markdownFence(trimmed string) string {
	for _, char := range []byte{'`', '~'} {
		length := 0
		for length < len(trimmed) && trimmed[length] == char {
			length++
		}
		if length >= 3 {
			return trimmed[:length]
		}
	}
	return ""
}

// stripShellComment removes a shell comment: a "#" that begins a word outside
// quotes runs to the end of the line.
func stripShellComment(line string) string {
	quote := byte(0)
	for index := 0; index < len(line); index++ {
		char := line[index]
		switch {
		case quote != 0 && char == quote:
			quote = 0
		case quote == '\'':
		case char == '\\':
			index++
		case quote == 0 && (char == '\'' || char == '"'):
			quote = char
		case quote == 0 && char == '#' && (index == 0 || line[index-1] == ' ' || line[index-1] == '\t'):
			return strings.TrimSpace(line[:index])
		}
	}
	return line
}

func appendOnce(values []string, value string) []string {
	for _, present := range values {
		if present == value {
			return values
		}
	}
	return append(values, value)
}
