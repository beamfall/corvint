package main

import (
	"strings"
	"testing"
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
