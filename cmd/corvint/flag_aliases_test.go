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
		{[]string{"surprise", "--header", "H"}, []string{"surprise", "--header", "H"}},
		{[]string{"--root"}, []string{"--root"}},
	} {
		if got, err := withHeadAlias(slices.Clone(c.in)); err != nil || !slices.Equal(got, c.want) {
			t.Errorf("withHeadAlias(%q) = %q, %v; want %q", c.in, got, err, c.want)
		}
	}
	// --head with --target, or --head twice, is refused before any verb
	// parser could let the last value win; --target alone is the verb's own.
	for _, in := range [][]string{
		{"surprise", "--target", "A", "--head", "B"},
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
		{"surprise", "--task", "t", "--base", "B"}, {"lrf", "--cem", "c"}, {"obligations"}, {"dogfood-ocm", "status", "--expected-base", "B"},
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

// ACX-V0-858: the positional-BASE dogfood subverbs take --base instead, never
// both; prechange then reads every positional as an impact path.
func TestDogfoodFlowBaseOption_ACXV0858(t *testing.T) {
	t.Parallel()
	for _, action := range []string{"change", "check", "seal", "batch-check", "propose-citations"} {
		flags, base, err := dogfoodFlowArguments(action, []string{"--base", "B"})
		if err != nil || base != "B" || len(flags) != 0 {
			t.Errorf("%s --base B: flags=%q base=%q err=%v", action, flags, base, err)
		}
		if _, _, err := dogfoodFlowArguments(action, []string{"--base=B", "C"}); err == nil || err.Error() != "duplicate-local-completion-option" {
			t.Errorf("%s --base B C: err=%v; want duplicate-local-completion-option", action, err)
		}
	}
	flags, base, err := dogfoodFlowArguments("prechange", []string{"--base", "B", "a.go", "b.go"})
	if err != nil || base != "B" || flags[prechangePathsFlag] != "a.go\x00b.go" {
		t.Errorf("prechange --base: flags=%q base=%q err=%v", flags, base, err)
	}
	code, stdout, stderr := runCLI(t, "dogfood", "seal", "--base", "B", "C")
	if code != 2 || stdout != "" || !strings.HasPrefix(stderr, `{"code":"duplicate-local-completion-option"`) {
		t.Errorf("seal --base B C: exit=%d stdout=%q stderr=%q", code, stdout, stderr)
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
		"affected":    {"--base", "--format"},
		"calibrate":   {"--format"},
		"changes":     {"--base", "--format", "--head", "--json"},
		"context":     {"--json", "--summary"},
		"delta":       {"--base", "--head"},
		"flows":       {"--base", "--format", "--head", "--revision"},
		"graph":       {"--format", "--revision"},
		"impact":      {"--base", "--revision"},
		"lrf":         {"--target"},
		"obligations": {"--target"},
		"ocm":         {"--target"},
		"prove":       {"--base", "--target"},
		"repro":       {"--json"},
		"runs":        {"--base", "--rev", "--summary"},
		"surprise":    {"--base", "--target"},
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
