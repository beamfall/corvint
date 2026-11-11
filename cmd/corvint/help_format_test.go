package main

import (
	"strings"
	"testing"
)

// TestEveryVerbHelpFollowsTheHouseFormat pins CCF-V1-650 (V1-3730): every dispatched verb's help
// topic, listed or hidden, opens with a one-line summary sentence, a blank line, a Usage: heading
// and an indented corvint invocation, so a reader and the catalog find the same shape everywhere.
func TestEveryVerbHelpFollowsTheHouseFormat(t *testing.T) {
	t.Parallel()
	for _, command := range topLevelCommands {
		topic, ok := publicHelpTopic(command)
		if !ok {
			t.Fatalf("verb %q has no help topic", command)
		}
		lines := strings.SplitN(helpText(topic), "\n", 5)
		if len(lines) < 5 {
			t.Errorf("%s help has %d lines, want a summary, a blank line, Usage: and an invocation", command, len(lines))
			continue
		}
		summary := lines[0]
		if summary == "" || strings.HasPrefix(summary, " ") || !strings.HasSuffix(summary, ".") {
			t.Errorf("%s help summary %q is not one unindented sentence ending in a period", command, summary)
		}
		if lines[1] != "" || lines[2] != "Usage:" || !strings.HasPrefix(lines[3], "  corvint ") {
			t.Errorf("%s help does not continue with a blank line, Usage: and a corvint invocation: %q", command, lines[1:4])
		}
	}
}

// TestInitIsDocumentedAsAnAliasOfAdopt pins V1-3730: root help and the shared activation help
// name init as an alias of adopt, and both verbs share one help topic.
func TestInitIsDocumentedAsAnAliasOfAdopt(t *testing.T) {
	t.Parallel()
	if !strings.Contains(rootHelp, "\n    init  alias of adopt\n") {
		t.Error("root help does not name init as an alias of adopt")
	}
	if topicText("init") != topicText("adopt") || !strings.Contains(helpText("init"), "init is an alias of adopt") {
		t.Error("init and adopt help differ or do not state the alias")
	}
}
