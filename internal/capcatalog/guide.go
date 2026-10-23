package capcatalog

import (
	"regexp"
	"slices"
	"strings"
)

var (
	quotedPattern   = regexp.MustCompile(`"[^"]*"`)
	backtickPattern = regexp.MustCompile("`([^`]+)`")
	issuePattern    = regexp.MustCompile(`#[0-9]+\b`)
)

// GuideProblems checks a guide against a catalog (CAP-V0-018): every corvint
// command line in it, after any --root preamble, names a catalogued command or
// help topic, a catalogued operation, every option its usage form requires
// (CAP-V0-017), and only options that command or the global block names;
// every schema identifier it
// names is in the catalog; and every non-capability names a tracking issue or
// states that it is excluded by design. It returns one line per problem.
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
	for _, id := range schemaIDs(guide) {
		if !known[id] {
			problems = append(problems, "schema "+id+" is not named by any command's help")
		}
	}
	items := nonCapabilities(guide)
	if len(items) == 0 {
		problems = append(problems, "the guide has no Non-capabilities items")
	}
	for _, item := range items {
		if !issuePattern.MatchString(item) && !strings.Contains(item, "By design") {
			problems = append(problems, "non-capability names no tracking issue and no By design reason: "+item)
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
		for _, redirect := range []string{" > ", " < ", " | "} {
			line, _, _ = strings.Cut(line, redirect)
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
// its continuation lines joined.
func nonCapabilities(guide string) []string {
	_, section, found := strings.Cut(guide, "\nNon-capabilities\n")
	if !found {
		return nil
	}
	var items []string
	for _, line := range strings.Split(section, "\n") {
		switch {
		case strings.HasPrefix(line, "- "):
			items = append(items, strings.TrimPrefix(line, "- "))
		case strings.HasPrefix(line, "  ") && len(items) > 0:
			items[len(items)-1] += " " + strings.TrimSpace(line)
		}
	}
	return items
}
