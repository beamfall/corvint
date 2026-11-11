package main

import (
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/contextindex"
)

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
