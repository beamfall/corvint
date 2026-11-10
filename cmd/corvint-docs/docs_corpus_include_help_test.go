package main

import (
	"strings"
	"testing"
)

// DCP-V1-579: `docs corpus search --help` says that --include claims adds claim hits to the subject
// hits, ranked together and labelled in result_kinds, and names the claims operation for claims only.
func TestDCPV1579SearchIncludeHelpSaysClaimsAreAdded(t *testing.T) {
	help := corpusHelpRun(t, "docs", "corpus", "search", "--help")
	want := "flow record kinds: claims, relations, test_knowledge, paragraphs; search admits only claims, " +
		"which adds claim hits to the subject hits, ranked together and labelled in result_kinds " +
		"(the claims operation returns claims only)"
	if !strings.Contains(help, want) {
		t.Fatalf("search help does not carry the include text %q:\n%s", want, help)
	}
}
