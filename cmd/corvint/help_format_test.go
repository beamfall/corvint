package main

import (
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/contextindex"
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

// TestHiddenVerbsStayCallableButUnlisted pins CCF-V1-008 as amended by V1-3730: each hidden
// verb is dispatched, absent from the root-help verb index, and its own help says it is unlisted.
func TestHiddenVerbsStayCallableButUnlisted(t *testing.T) {
	t.Parallel()
	want := []string{"changes", "dogfood-ocm", "lrf", "migrate-traces", "migration-ratchet", "prove-observe"}
	if len(hiddenCommands) != len(want) {
		t.Fatalf("hiddenCommands has %d verbs, want %q", len(hiddenCommands), want)
	}
	commands, _, _ := strings.Cut(strings.SplitN(rootHelp, "\nCommands:\n", 2)[1], "\nGlobal options:")
	for _, verb := range want {
		if _, ok := hiddenCommands[verb]; !ok {
			t.Errorf("verb %q is not hidden", verb)
		}
		if _, dispatched := publicHelpTopic(verb); !dispatched {
			t.Errorf("hidden verb %q is no longer dispatched", verb)
		}
		if strings.Contains(commands, "    "+verb+" ") || strings.Contains(commands, "| "+verb+" ") {
			t.Errorf("hidden verb %q is listed in root help", verb)
		}
		if !strings.HasSuffix(helpText(verb), hiddenHelpNote) {
			t.Errorf("hidden verb %q help does not say it is unlisted", verb)
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

// TestFeatureHelpGivesAValidFeatureIDExample pins V1-3730: feature help's example ID, after the
// options so the catalog does not read it as an operation, passes the feature ID check, and the help names the requirement-prefix form the check refuses.
func TestFeatureHelpGivesAValidFeatureIDExample(t *testing.T) {
	t.Parallel()
	text := helpText("feature")
	_, example, found := strings.Cut(text, "\nExample:\n  corvint feature ")
	if !found {
		t.Fatalf("feature help has no authored example:\n%s", text)
	}
	line, _, _ := strings.Cut(example, "\n")
	fields := strings.Fields(line)
	featureID := fields[len(fields)-1]
	if err := contextindex.ValidateFeature(featureID, 1); err != nil {
		t.Errorf("feature help example ID %q is refused: %v", featureID, err)
	}
	if !strings.Contains(text, "GPK-V0 is\nrefused") || contextindex.ValidateFeature("GPK-V0", 1) == nil {
		t.Error("feature help does not name the refused requirement-prefix form")
	}
}
