package main

import (
	"regexp"
	"slices"
	"testing"
)

// ACX-V0-858: the range-end, base and output flags each verb's help names.
// The vocabulary is still mixed (V1-3731); this table makes any change to it
// a reviewed edit.
func TestRangeAndOutputFlagVocabulary_ACXV0858(t *testing.T) {
	t.Parallel()
	vocabulary := regexp.MustCompile(`--(head|target|revision|rev|base|format|json|summary)\b`)
	for verb, want := range map[string][]string{
		"affected":  {"--base", "--format", "--summary"},
		"context":   {"--base", "--json"},                           // --base only in the pointer to corvint affected (decision 0597)
		"flows":     {"--base", "--format", "--json", "--revision"}, // the shim help; the forwarded verbs' flags are corvint-docs's (decision 0595)
		"impact":    {"--base", "--revision"},
		"runs":      {"--base", "--rev", "--summary"},
		"test-plan": {"--format", "--revision"},
		"tests":     {"--base", "--format", "--json", "--rev"},
	} {
		_, stdout, stderr := runCLI(t, "help", verb)
		got := []string{}
		for _, match := range vocabulary.FindAllString(stdout+stderr, -1) {
			if !slices.Contains(got, match) {
				got = append(got, match)
			}
		}
		slices.Sort(got)
		if !slices.Equal(got, want) {
			t.Errorf("help %s names %q, want %q", verb, got, want)
		}
	}
}
