package capcatalog

import (
	_ "embed"
	"regexp"
	"slices"
	"strings"
	"unicode"
)

var (
	quotedPattern   = regexp.MustCompile(`"[^"]*"`)
	backtickPattern = regexp.MustCompile("`([^`]+)`")
	issuePattern    = regexp.MustCompile(`#[0-9]+\b`)
	// requirementPattern matches a requirement ID such as CAP-V0-016, MCPV0-100
	// or PNC-009: a three-digit number after an uppercase prefix.
	requirementPattern = regexp.MustCompile(`\b[A-Z][A-Z0-9]*(?:-[A-Z][A-Z0-9]*)*-[0-9]{3}\b`)
	// listMarkerPattern matches the start of a list item in any bullet or
	// numbering style.
	listMarkerPattern = regexp.MustCompile(`^(?:[-*+]|[0-9]+[.)])`)
)

//go:embed NONCAPABILITY-STATUS.tsv
var nonCapabilityStatus string

// NonCapabilityStatus returns the checked-in status of every issue and
// requirement the guide's Non-capabilities section may name (CAP-V0-025).
func NonCapabilityStatus() string { return nonCapabilityStatus }

// GuideProblems checks a guide against a catalog (CAP-V0-018): every corvint
// command line in it, after any --root preamble, names a catalogued command or
// help topic, a catalogued operation, every option its usage form requires
// (CAP-V0-017), and only options that command or the global block names;
// every schema identifier it
// names is in the catalog (a command's or a workflow step's help); and every non-capability names a tracking issue or
// a requirement ID (CAP-V0-024). It returns one line per problem.
func GuideProblems(catalog Catalog, guide string) []string {
	var problems []string
	commands := map[string]Command{}
	for _, command := range catalog.Commands {
		commands[command.Name] = command
	}
	for _, line := range guideCommandLines(guide) {
		problems = append(problems, commandLineProblems(catalog, commands, line)...)
	}
	known := map[string]bool{catalog.Schema: true}
	for _, id := range catalog.GlobalSchemaIDs {
		known[id] = true
	}
	for _, command := range catalog.Commands {
		for _, id := range command.SchemaIDs {
			known[id] = true
		}
	}
	for _, workflow := range catalog.Workflows {
		for _, step := range workflow.Steps {
			for _, id := range step.SchemaIDs {
				known[id] = true
			}
		}
	}
	for _, id := range schemaIDs(guide) {
		if !known[id] {
			problems = append(problems, "schema "+id+" is not named by any command's help")
		}
	}
	items, lineProblems := nonCapabilities(guide)
	problems = append(problems, lineProblems...)
	if len(items) == 0 {
		problems = append(problems, "the guide has no Non-capabilities items")
	}
	for _, item := range items {
		if len(nonCapabilityIDs(item)) == 0 {
			problems = append(problems, "non-capability names no tracking issue and no requirement ID: "+item)
		}
	}
	return problems
}

// guideCommandLines returns each line whose text starts with "corvint " and
// each backticked "corvint ..." span, cut at a shell redirection.
func guideCommandLines(guide string) []string {
	var lines []string
	for _, line := range strings.Split(guide, "\n") {
		if trimmed := strings.TrimSpace(line); strings.HasPrefix(trimmed, "corvint ") {
			lines = append(lines, trimmed)
		}
		for _, match := range backtickPattern.FindAllStringSubmatch(line, -1) {
			if strings.HasPrefix(match[1], "corvint ") {
				lines = append(lines, match[1])
			}
		}
	}
	for index, line := range lines {
		// A redirection inside a quoted argument, such as a go test key
		// "PACKAGE > TEST", does not end the command.
		masked := quotedPattern.ReplaceAllStringFunc(line, func(quoted string) string { return strings.Repeat("x", len(quoted)) })
		for _, redirect := range []string{" > ", " < ", " | "} {
			if at := strings.Index(masked, redirect); at >= 0 {
				line, masked = line[:at], masked[:at]
			}
		}
		lines[index] = line
	}
	return lines
}

func commandLineProblems(catalog Catalog, commands map[string]Command, line string) []string {
	fields := strings.Fields(quotedPattern.ReplaceAllString(line, "TEXT"))[1:]
	// The global-option preamble: --root PATH or --root=PATH, before the command.
	for len(fields) > 0 && (fields[0] == "--root" || strings.HasPrefix(fields[0], "--root=")) {
		if fields[0] == "--root" {
			if len(fields) < 2 || strings.HasPrefix(fields[1], "--") {
				return []string{"--root without a PATH in: " + line}
			}
			fields = fields[1:]
		}
		fields = fields[1:]
	}
	if len(fields) == 0 {
		return []string{"empty command line: " + line}
	}
	verb := fields[0]
	allowed := slices.Clone(catalog.GlobalFlags)
	var problems []string
	switch {
	case strings.HasPrefix(verb, "--"):
		// A global invocation such as --version: every option is a global one.
	case verb == "help":
		problems = append(problems, helpTopicProblems(catalog, commands, fields[1:], line)...)
		fields = fields[1:]
	default:
		command, ok := commands[verb]
		if !ok {
			return []string{"unknown command " + verb + " in: " + line}
		}
		allowed = append(allowed, command.Flags...)
		if len(fields) > 1 && len(command.Operations) > 0 && operationWord(fields[1]) && !slices.Contains(command.Operations, fields[1]) {
			problems = append(problems, "unknown "+verb+" operation "+fields[1]+" in: "+line)
		}
		if len(fields) > 2 {
			if nested, ok := command.NestedOperations[fields[1]]; ok && operationWord(fields[2]) && !slices.Contains(nested, fields[2]) {
				problems = append(problems, "unknown "+verb+" "+fields[1]+" operation "+fields[2]+" in: "+line)
			}
		}
		form := FormKey(command, fields[1:])
		for _, flag := range command.RequiredFlags[form] {
			if !slices.ContainsFunc(fields[1:], func(field string) bool { return field == flag || strings.HasPrefix(field, flag+"=") }) {
				problems = append(problems, "required option "+flag+" of "+form+" is missing in: "+line)
			}
		}
		fields = fields[1:]
	}
	for _, field := range fields {
		flag, _, _ := strings.Cut(field, "=")
		if strings.HasPrefix(flag, "--") && !slices.Contains(allowed, flag) {
			problems = append(problems, "option "+flag+" is not in the help of "+verb+" in: "+line)
		}
	}
	return problems
}

// helpTopicProblems checks the words after `corvint help`: --json, guide, the
// COMMAND placeholder, a catalogued command, or one of the catalogued
// two-word topics.
func helpTopicProblems(catalog Catalog, commands map[string]Command, words []string, line string) []string {
	switch {
	case len(words) == 0, len(words) == 1 && (words[0] == "--json" || words[0] == "guide" || words[0] == "COMMAND"):
		return nil
	case len(words) == 1:
		if _, ok := commands[words[0]]; ok {
			return nil
		}
	case len(words) == 2 && slices.Contains(catalog.NestedHelpTopics, words[0]+" "+words[1]):
		return nil
	}
	return []string{"unknown help topic in: " + line}
}

// nonCapabilities returns each "- " item of the Non-capabilities section with
// its continuation lines joined. It fails closed (CAP-V0-026): besides one
// intro paragraph before the first item, every non-blank line must be a "- "
// item or a two-space continuation directly under one, and any other line,
// such as another bullet style or an indented or numbered item, is returned
// as a problem rather than skipped.
func nonCapabilities(guide string) ([]string, []string) {
	_, section, found := strings.Cut(guide, "\nNon-capabilities\n")
	if !found {
		return nil, nil
	}
	var items, problems []string
	// intro is open until the intro paragraph ends or an item starts.
	introOpen, introSeen, inItem := true, false, false
	for _, line := range strings.Split(section, "\n") {
		text, indented := strings.CutPrefix(line, "  ")
		switch {
		case strings.TrimSpace(line) == "":
			introOpen = introOpen && !introSeen
			inItem = false
		case strings.HasPrefix(line, "- ") && plainText(line[2:]):
			items = append(items, line[2:])
			introOpen, inItem = false, true
		case indented && inItem && plainText(text):
			items[len(items)-1] += " " + text
		case introOpen && plainText(line):
			introSeen = true
		default:
			problems = append(problems, "Non-capabilities line is not a recognised item or continuation: "+line)
		}
	}
	return items, problems
}

// plainText reports whether text starts with a non-space character and is not
// itself a list item such as "* x", "- x", "+ x", "1. x" or "2) x".
func plainText(text string) bool {
	return text != "" && !unicode.IsSpace(rune(text[0])) && !listMarkerPattern.MatchString(text)
}

// nonCapabilityIDs returns the issue numbers (#N) and requirement IDs an item
// names, in order of appearance.
func nonCapabilityIDs(item string) []string {
	return append(issuePattern.FindAllString(item, -1), requirementPattern.FindAllString(item, -1)...)
}

// NonCapabilityProblems checks every non-capability of a guide against a
// status table (CAP-V0-025, CAP-V0-026). Every ID an item names must have a
// status row, and none may be a closed issue or a delivered requirement. An
// item that says "By design" must name a requirement recorded as excludes;
// any other item must name an open issue or an undelivered requirement. It
// reads nothing but its arguments and returns one line per problem.
func NonCapabilityProblems(guide, status string) []string {
	states, problems := parseNonCapabilityStatus(status)
	items, lineProblems := nonCapabilities(guide)
	problems = append(problems, lineProblems...)
	for _, item := range items {
		byDesign, standing := strings.Contains(item, "By design"), false
		for _, id := range nonCapabilityIDs(item) {
			state, ok := states[id]
			switch {
			case !ok:
				problems = append(problems, "non-capability names "+id+", which has no row in NONCAPABILITY-STATUS.tsv: "+item)
			case state == "closed" || state == "delivered":
				problems = append(problems, "non-capability names "+id+", recorded as "+state+"; remove the item or name the open residual: "+item)
			case byDesign == (state == "excludes"):
				standing = true
			}
		}
		switch {
		case standing:
		case byDesign:
			problems = append(problems, "By design non-capability names no requirement recorded as excludes: "+item)
		default:
			problems = append(problems, "non-capability names no open issue and no undelivered requirement: "+item)
		}
	}
	return problems
}

// parseNonCapabilityStatus reads the id, state and evidence table. An issue
// is open or closed; a requirement is undelivered, delivered or excludes.
func parseNonCapabilityStatus(status string) (map[string]string, []string) {
	states := map[string]string{}
	lines := strings.Split(strings.TrimSuffix(status, "\n"), "\n")
	if lines[0] != "id\tstate\tevidence" {
		return states, []string{"NONCAPABILITY-STATUS.tsv must start with the header id, state, evidence"}
	}
	var problems []string
	for _, line := range lines[1:] {
		fields := strings.Split(line, "\t")
		if len(fields) != 3 || strings.TrimSpace(fields[2]) == "" {
			problems = append(problems, "status row needs an id, a state and evidence: "+line)
			continue
		}
		id, state := fields[0], fields[1]
		issue := id != "" && issuePattern.FindString(id) == id
		requirement := id != "" && requirementPattern.FindString(id) == id
		switch {
		case issue && (state == "open" || state == "closed"), requirement && (state == "undelivered" || state == "delivered" || state == "excludes"):
		default:
			problems = append(problems, "status row has an unknown id or state: "+line)
			continue
		}
		if _, ok := states[id]; ok {
			problems = append(problems, "status row repeats "+id)
			continue
		}
		states[id] = state
	}
	return states, problems
}
