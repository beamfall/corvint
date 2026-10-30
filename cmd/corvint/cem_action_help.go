package main

import "strings"

// cemActionHelpPrefix marks the help topic of one cem action: `corvint cem
// ACTION --help` prints only that action's usage, description and options,
// projected from cemHelp so the two cannot drift (CEM-V1-340).
const cemActionHelpPrefix = "cem-"

// cemActionHelp projects cemHelp onto one action: its usage lines, its Actions
// paragraph, the Verification options when it is a verifying action, and the
// closing wire note. status, verify and report share one Actions paragraph.
// It returns "" for an action cemHelp does not document.
func cemActionHelp(action string) string {
	if action == "gate-verdict" {
		// Its usage lives in the closing Tasks gate section, not in Usage.
		return "Change Evidence Map gate-verdict action.\n" + cemGateVerdictHelp + "\nFull command help: corvint cem --help\n"
	}
	usage := cemHelpUsage(action)
	paragraphAction := action
	if action == "verify" || action == "report" {
		paragraphAction = "status"
	}
	paragraph := cemHelpSection("\nActions:\n", "  "+paragraphAction, "           ")
	if usage == "" || paragraph == "" {
		return ""
	}
	var text strings.Builder
	text.WriteString("Change Evidence Map " + action + " action.\n\nUsage:\n" + usage + "\nAction:\n" + paragraph)
	if action == "status" || action == "verify" || action == "report" {
		if options, ok := cemHelpBlock("\nVerification options:\n"); ok {
			text.WriteString("\nVerification options:\n" + options)
		}
	}
	switch action {
	case "status", "verify", "report", "anchor", "verify-stable", "bundle", "cite", "mark", "cover", "discriminate":
		// The out-of-tree map rules (CEM-V1-317, CEM-V1-640, CEM-V1-647).
		if paths, ok := cemHelpBlock("\nMap paths:\n"); ok {
			text.WriteString("\nMap paths:\n" + paths)
		}
	}
	if note, ok := cemHelpBlock("\ncem/1.0 is the only wire."); ok {
		text.WriteString("\ncem/1.0 is the only wire." + note)
	}
	text.WriteString("\nFull command help: corvint cem --help\n")
	return text.String()
}

// cemHelpUsage returns the Usage lines of one action with their continuation
// lines.
func cemHelpUsage(action string) string {
	block, ok := cemHelpBlock("\nUsage:\n")
	if !ok {
		return ""
	}
	var usage strings.Builder
	taking := false
	for _, line := range strings.SplitAfter(block, "\n") {
		switch {
		case strings.HasPrefix(line, "  corvint "):
			// bundle takes no --root (CSB-V0-001), so its usage omits it.
			taking = strings.HasPrefix(line, "  corvint [--root PATH] cem "+action+" ") ||
				strings.HasPrefix(line, "  corvint cem "+action+" ") ||
				strings.TrimRight(line, "\n") == "  corvint [--root PATH] cem "+action
		case !strings.HasPrefix(line, "    "):
			taking = false
		}
		if taking {
			usage.WriteString(line)
		}
	}
	return usage.String()
}

// cemHelpSection returns the paragraph inside the block after heading whose
// first line starts with lead followed by a space or a line end, through its
// continuation lines (indented by indent).
func cemHelpSection(heading, lead, indent string) string {
	block, ok := cemHelpBlock(heading)
	if !ok {
		return ""
	}
	var paragraph strings.Builder
	taking := false
	for _, line := range strings.SplitAfter(block, "\n") {
		trimmed := strings.TrimRight(line, "\n")
		switch {
		case trimmed == lead || strings.HasPrefix(trimmed, lead+" "):
			taking = true
		case !strings.HasPrefix(line, indent):
			taking = false
		}
		if taking {
			paragraph.WriteString(line)
		}
	}
	return paragraph.String()
}

// cemHelpBlock returns the text of cemHelp after marker through the next blank
// line.
func cemHelpBlock(marker string) (string, bool) {
	_, after, found := strings.Cut(cemHelp, marker)
	if !found {
		return "", false
	}
	block, _, _ := strings.Cut(after, "\n\n")
	return block + "\n", true
}

// cemOrNestedHelpTopic names the topic of `corvint help COMMAND SUBCOMMAND`:
// `help cem ACTION` is that action's help (CEM-V1-344), any other pair a
// nested topic. It lives here so help.go lines other documents cite stay put.
func cemOrNestedHelpTopic(command, subcommand string) string {
	if command == "cem" && cemHelpActions[subcommand] {
		return cemActionHelpPrefix + subcommand
	}
	return nestedHelpTopic(command, subcommand)
}
