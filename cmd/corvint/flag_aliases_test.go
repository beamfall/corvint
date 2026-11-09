package main

import (
	"regexp"
	"slices"
	"strings"
	"testing"
)

// ACX-V0-858 (V1-3731): --head is --target on the --target verbs, after the
// global --root or not; other verbs, a "--" terminator and lookalikes stay.
func TestHeadAliasOnTargetVerbs_ACXV0858(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ in, want []string }{
		{[]string{"lrf", "--cem", "c", "--head", "H"}, []string{"lrf", "--cem", "c", "--target", "H"}},
		{[]string{"--root", "/r", "prove", "--head=H"}, []string{"--root", "/r", "prove", "--target=H"}},
		{[]string{"--root=/r", "dogfood-ocm", "status", "--head", "H"}, []string{"--root=/r", "dogfood-ocm", "status", "--target", "H"}},
		{[]string{"ocm", "verify", "--head", "H", "--", "--head"}, []string{"ocm", "verify", "--target", "H", "--", "--head"}},
		{[]string{"changes", "--head", "H"}, []string{"changes", "--head", "H"}},
		{[]string{"prove", "--header", "H"}, []string{"prove", "--header", "H"}},
		{[]string{"--root"}, []string{"--root"}},
	} {
		if got, err := withHeadAlias(slices.Clone(c.in)); err != nil || !slices.Equal(got, c.want) {
			t.Errorf("withHeadAlias(%q) = %q, %v; want %q", c.in, got, err, c.want)
		}
	}
	// --head with --target, or --head twice, is refused before any verb
	// parser could let the last value win; --target alone is the verb's own.
	for _, in := range [][]string{
		{"prove", "--target", "A", "--head", "B"},
		{"lrf", "--head=B", "--target=A"},
		{"ocm", "verify", "--head", "A", "--head", "B"},
	} {
		if got, err := withHeadAlias(slices.Clone(in)); err == nil || got != nil || !strings.Contains(err.Error(), "give the range end once") {
			t.Errorf("withHeadAlias(%q) = %q, %v; want the duplicate refusal", in, got, err)
		}
	}
	if got, err := withHeadAlias([]string{"ocm", "--target", "A", "--target", "B"}); err != nil || len(got) != 5 {
		t.Errorf("--target twice: %q, %v; want it left to the verb parser", got, err)
	}
	// Each verb answers --head exactly as it answers --target.
	root := t.TempDir()
	for _, args := range [][]string{
		{"lrf", "--cem", "c"}, {"obligations"}, {"dogfood-ocm", "status", "--expected-base", "B"},
	} {
		headCode, headOut, headErr := runCLI(t, append(append([]string{"--root", root}, args...), "--head", "zz")...)
		targetCode, targetOut, targetErr := runCLI(t, append(append([]string{"--root", root}, args...), "--target", "zz")...)
		if headCode != targetCode || headOut != targetOut || headErr != targetErr || strings.Contains(headErr, "--head") {
			t.Errorf("%q: --head gave %d %q %q; --target gave %d %q %q", args, headCode, headOut, headErr, targetCode, targetOut, targetErr)
		}
		code, stdout, stderr := runCLI(t, append(append([]string{"--root", root}, args...), "--target", "A", "--head", "B")...)
		if code != 2 || stdout != "" || !strings.Contains(stderr, `"code": "invalid-arguments"`) || !strings.Contains(stderr, "give the range end once") {
			t.Errorf("%q --target A --head B: exit=%d stdout=%q stderr=%q", args, code, stdout, stderr)
		}
	}
}

// ACX-V0-858: the range-end, base and output flags each verb's help names.
// The vocabulary is still mixed (V1-3731); this table makes any change to it
// a reviewed edit. Help names the canonical --target only: --head on these
// verbs is an undocumented compatibility alias (ACX-V0-858), so each --target
// verb here takes --head as well while its help leaves it out.
func TestRangeAndOutputFlagVocabulary_ACXV0858(t *testing.T) {
	t.Parallel()
	vocabulary := regexp.MustCompile(`--(head|target|revision|rev|base|format|json|summary)\b`)
	for verb, want := range map[string][]string{
		"affected":    {"--base", "--format", "--summary"},
		"changes":     {"--base", "--format", "--head", "--json"},
		"context":     {"--json"},
		"delta":       {"--base", "--head"},
		"flows":       {"--base", "--format", "--head", "--revision"},
		"impact":      {"--base", "--revision"},
		"lrf":         {"--target"},
		"obligations": {"--target"},
		"ocm":         {"--target"},
		"prove":       {"--base", "--target"},
		"runs":        {"--base", "--rev", "--summary"},
		"test-plan":   {"--format", "--revision"},
		"tests":       {"--base", "--format", "--json", "--rev"},
		"witness":     {"--base", "--head", "--json"},
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
		if slices.Contains(want, "--target") != headAliasVerbs[verb] {
			t.Errorf("%s: --target in help=%v but head alias=%v", verb, slices.Contains(want, "--target"), headAliasVerbs[verb])
		}
	}
}
