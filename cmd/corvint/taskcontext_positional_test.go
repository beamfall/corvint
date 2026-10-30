package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

// TestTaskContextPositionalTaskMatchesTheFlagBytes covers TCP-V0-062: one
// positional is the task, and its packet is byte-identical to --task's.
func TestTaskContextPositionalTaskMatchesTheFlagBytes(t *testing.T) {
	t.Parallel()
	root := taskContextRepository(t)
	run := func(arguments ...string) (string, string, int) {
		var stdout, stderr bytes.Buffer
		code := runContext(context.Background(), append([]string{"--root", root, "context"}, arguments...), strings.NewReader(""), &stdout, &stderr)
		return stdout.String(), stderr.String(), code
	}
	flagOut, flagErr, flagCode := run("--task", "does Split keep empty keys", "--limit", "5")
	if flagCode != 0 || flagOut == "" {
		t.Fatalf("--task exit %d: %s", flagCode, flagErr)
	}
	for _, arguments := range [][]string{
		{"does Split keep empty keys", "--limit", "5"},
		{"--limit", "5", "does Split keep empty keys"},
	} {
		out, errText, code := run(arguments...)
		if code != flagCode || out != flagOut || errText != flagErr {
			t.Fatalf("positional %q: exit %d stderr %q, bytes differ from --task: %v", arguments, code, errText, out != flagOut)
		}
	}
}

// TestTaskContextRefusesAmbiguousPositionalTasks covers TCP-V0-062's refusals:
// both forms, or two positionals, are refused naming --task with an example.
func TestTaskContextRefusesAmbiguousPositionalTasks(t *testing.T) {
	t.Parallel()
	root := taskContextRepository(t)
	for _, tc := range []struct {
		arguments []string
		reason    string
	}{
		{[]string{"--task", "find Split", "find Split"}, "argument --task: not allowed with a positional task"},
		{[]string{"find Split", "--task", "find Split"}, "argument --task: not allowed with a positional task"},
		{[]string{"find", "Split"}, "context accepts at most one positional task"},
	} {
		options, isContext, err := parseTaskContextInvocation(append([]string{"--root", root, "context"}, tc.arguments...))
		if !isContext || err == nil {
			t.Fatalf("%q: isContext=%v err=%v options=%+v", tc.arguments, isContext, err, options)
		}
		want := tc.reason + `; quote the task once or pass --task, for example: corvint context --task "fix the parser"`
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("%q: error %q lacks %q", tc.arguments, err.Error(), want)
		}
		var stdout, stderr bytes.Buffer
		code := runContext(context.Background(), append([]string{"--root", root, "context"}, tc.arguments...), strings.NewReader(""), &stdout, &stderr)
		if code != 2 || stdout.Len() != 0 || !strings.Contains(stderr.String(), "--task") {
			t.Fatalf("%q: exit %d stdout %q stderr %q", tc.arguments, code, stdout.String(), stderr.String())
		}
	}
	if _, _, err := parseTaskContextInvocation([]string{"--root", root, "context", "--subject", "x.go"}); err == nil || !strings.Contains(err.Error(), "the following arguments are required: --task") {
		t.Fatalf("neither form: err=%v", err)
	}
}
