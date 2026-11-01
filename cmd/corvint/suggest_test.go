package main

import (
	"bytes"
	"slices"
	"strings"
	"testing"
)

// ACX-V0-859 (V1-3736): an unknown command keeps the argparse invalid-choice
// envelope and appends the near commands; an unrelated word gains nothing.
func TestUnknownCommandSuggestsNearChoices_ACXV0859(t *testing.T) {
	for _, test := range []struct {
		command, suffix string
	}{
		{"contxt", "; did you mean 'context'?"},
		{"ocn", "; did you mean 'ocm'?"},
		{"xyz", ""},
		{"zz-no-such-verb", ""},
	} {
		var stdout, stderr bytes.Buffer
		if code := run([]string{test.command}, strings.NewReader(""), &stdout, &stderr); code != 2 || stdout.Len() != 0 {
			t.Fatalf("%s: exit %d stdout %q", test.command, code, stdout.String())
		}
		got := stderr.String()
		if !strings.Contains(got, `"code": "invalid-arguments"`) || !strings.Contains(got, "argument command: invalid choice: '"+test.command+"' (choose from 'init'") {
			t.Fatalf("%s: stderr %s", test.command, got)
		}
		if test.suffix == "" && strings.Contains(got, "did you mean") || test.suffix != "" && !strings.Contains(got, ")"+test.suffix+`"`) {
			t.Fatalf("%s: want suffix %q, stderr %s", test.command, test.suffix, got)
		}
	}
	if got := suggestChoices("tset", []string{"test", "tests", "set", "zzzz", "tset"}); !slices.Equal(got, []string{"set", "test"}) {
		t.Fatalf("suggestChoices order: %v", got)
	}
	if got := suggestChoices("abcd", []string{"abce", "abcf", "abcg", "abch"}); !slices.Equal(got, []string{"abce", "abcf", "abcg"}) {
		t.Fatalf("suggestChoices cap: %v", got)
	}
}
