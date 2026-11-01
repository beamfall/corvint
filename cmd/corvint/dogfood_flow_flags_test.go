package main

import (
	"strings"
	"testing"
)

// TestDCWV0808DogfoodOptionRefusalNamesTheFault pins the V1-3382 lane friction
// (F2): `dogfood finish-lane --verify-file FILE` printed only
// invalid-local-completion-option. The refusal now names the option at fault,
// the options the subverb takes and, for --verify-file, the --verify flag; a
// positional BASE to finish-lane names --base; an unsafe option name is not
// echoed.
func TestDCWV0808DogfoodOptionRefusalNamesTheFault(t *testing.T) {
	t.Parallel()
	for name, c := range map[string]struct {
		args []string
		code string
		want []string
	}{
		"verify-file": {[]string{"dogfood", "finish-lane", "--citations", "c", "--verify-file", "v", "--base", "B"}, "invalid-local-completion-option",
			[]string{"dogfood finish-lane does not take --verify-file; it takes --base, --base-verifier, --citations,", "the verify file is --verify FILE (DOGFOOD_VERIFY_FILE is its environment form)"}},
		"positional base": {[]string{"dogfood", "finish-lane", "--citations", "c", "B"}, "dogfood-base-required",
			[]string{"dogfood finish-lane takes the base as --base BASE, never as a positional argument"}},
		"check option": {[]string{"dogfood", "seal", "--citations", "c", "B"}, "invalid-local-completion-option",
			[]string{"dogfood seal does not take --citations; it takes "}},
		"unsafe name": {[]string{"dogfood", "check", "--X\x1b[31m", "B"}, "invalid-local-completion-option",
			[]string{"dogfood check does not take an option; it takes --base, --base-verifier, --override-verifier, --tree-verifier"}},
	} {
		code, stdout, stderr := runCLI(t, c.args...)
		if code != 2 || stdout != "" || !strings.HasPrefix(stderr, `{"code":"`+c.code+`"`) {
			t.Errorf("%s: exit=%d stdout=%q stderr=%q", name, code, stdout, stderr)
			continue
		}
		for _, want := range c.want {
			if !strings.Contains(stderr, want) {
				t.Errorf("%s: stderr lacks %q: %s", name, want, stderr)
			}
		}
		if strings.Contains(stderr, "\x1b") {
			t.Errorf("%s: stderr echoes an unsafe byte: %q", name, stderr)
		}
	}
}

// TestDCWV0880PrechangeArguments: dogfood prechange takes --task, BASE first
// and every later positional as an impact path; a missing BASE still refuses.
func TestDCWV0880PrechangeArguments(t *testing.T) {
	t.Parallel()
	flags, base, err := dogfoodFlowArguments("prechange", []string{"--task", "Fix a", "B", "a.go", "b.go"})
	if err != nil || base != "B" || flags["--task"] != "Fix a" || flags[prechangePathsFlag] != "a.go\x00b.go" {
		t.Errorf("flags=%q base=%q err=%v", flags, base, err)
	}
	if _, _, err = dogfoodFlowArguments("prechange", []string{"--task", "x"}); err == nil || err.Error() != "dogfood-base-required" {
		t.Errorf("no BASE: err=%v; want dogfood-base-required", err)
	}
	if _, _, err = dogfoodFlowArguments("prechange", []string{"--refresh", "p", "B"}); err == nil {
		t.Error("prechange accepted --refresh")
	}
}
