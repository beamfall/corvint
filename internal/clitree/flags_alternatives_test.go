package clitree

import (
	"slices"
	"testing"
)

// CAP-V0-975: Flags reads every option of a `|` alternative group, not only the first.
func TestCAPV0975FlagsSplitAlternatives(t *testing.T) {
	cases := map[string][]string{
		"corvint x [--summary|--pins] [--json]":     {"--json", "--pins", "--summary"},
		"corvint x (--a | --b) [-c|--cee]":          {"--a", "--b", "--cee", "-c"},
		"corvint x --format=json|text [--help|-h]":  {"--format"},
		"corvint x [--one|--two|--three] <path|->":  {"--one", "--three", "--two"},
		"corvint x [--mode a|b] --keep|--drop LIST": {"--drop", "--keep", "--mode"},
	}
	for form, want := range cases {
		if got := Flags([]string{form}); !slices.Equal(got, want) {
			t.Errorf("Flags(%q) = %v, want %v", form, got, want)
		}
	}
}
