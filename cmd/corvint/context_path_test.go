// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/testevidencedoc"
)

// TBC-V1-009: the argument shapes of `context --path`.
func TestPathContextParse(t *testing.T) {
	t.Parallel()
	root := taskContextRepository(t)
	parse := func(args ...string) (taskContextOptions, error) {
		options, isContext, err := parseTaskContextInvocation(append([]string{"--root", root, "context"}, args...))
		if !isContext {
			t.Fatalf("%v: not a context invocation", args)
		}
		return options, err
	}
	options, err := parse("--path", "a", "--path=b")
	if err != nil || !slices.Equal(options.paths, []string{"a", "b"}) {
		t.Fatalf("paths %v, %v", options.paths, err)
	}
	// A path with a space is accepted in both forms.
	options, err = parse("--path=docs/my file.md", "--path", "other dir/x.go")
	if err != nil || !slices.Equal(options.paths, []string{"docs/my file.md", "other dir/x.go"}) {
		t.Fatalf("paths with spaces %v, %v", options.paths, err)
	}
	// Where a flag can stand, `--path=` is the flag whatever its value holds: a declared option is
	// read before the rule that a token with a space is positional text.
	options, err = parse("--path= fix the parser")
	if err != nil || !slices.Equal(options.paths, []string{" fix the parser"}) || options.task != "" {
		t.Fatalf("inline value with spaces %+v, %v", options, err)
	}
	// Task text that begins with the flag's spelling is the task of the task form when it is the
	// value of `--task`.
	for _, args := range [][]string{{"--task", "--path= fix the parser"}, {"--limit", "3", "--task", "--path= fix the parser"}} {
		options, err = parse(args...)
		if err != nil || options.task != "--path= fix the parser" || len(options.paths) != 0 {
			t.Errorf("%v: %+v, %v", args, options, err)
		}
	}
	for _, c := range []struct {
		args     []string
		mentions string
	}{
		{[]string{"--path"}, "expected one argument"},
		{[]string{"--path", "a", "some task text"}, "corvint impact"},
		{[]string{"--path", "a", "--task=fix the parser"}, "corvint impact"},
		{[]string{"--worktree-overlay", "--path", "a"}, "--worktree-overlay"},
		{[]string{"--path", "--path", "a"}, "expected one argument"},
		{[]string{"--path", "a", "--base", "X"}, "corvint impact"},
		{[]string{"--path", "a", "--task", "t"}, "corvint impact"},
		{[]string{"--path", "a", "sometext"}, "corvint impact"},
		{[]string{"--path=a", "--base=X"}, "corvint impact"},
		{[]string{"--path", "a", "--ticket", "V1-1"}, "--ticket"},
		{[]string{"--path", "a", "--limit", "3"}, "--limit"},
	} {
		if _, err := parse(c.args...); err == nil || !strings.Contains(err.Error(), c.mentions) {
			t.Errorf("%v: err = %v, want one naming %q", c.args, err, c.mentions)
		}
	}
	// The task form is unchanged and carries no paths.
	options, err = parse("--task", "t")
	if err != nil || options.task != "t" || len(options.paths) != 0 {
		t.Errorf("task form: %+v, %v", options, err)
	}
}

// TBC-V1-009: a path outside the repository is a typed refusal with exit 2.
func TestRunPathContextRefusesOutsidePath(t *testing.T) {
	t.Parallel()
	root := taskContextRepository(t)
	options, _, err := parseTaskContextInvocation([]string{"--root", root, "context", "--path", "../outside.go"})
	if err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := runPathContext(context.Background(), options, &stdout, &stderr); code != 2 || stdout.Len() != 0 {
		t.Fatalf("exit %d, stdout %q", code, stdout.String())
	}
	var refusal struct {
		Code string `json:"code"`
		OK   bool   `json:"ok"`
	}
	if err := json.Unmarshal(stderr.Bytes(), &refusal); err != nil || refusal.Code != "path-outside-repository" || refusal.OK {
		t.Errorf("stderr %q: %+v %v", stderr.String(), refusal, err)
	}
}

// TBC-V1-010: a declared consumer is a runnable test and is listed with its reason, although its
// selection carries a kind. Only a make target, a CI job and a frontier site are left out.
func TestPathContextSelection(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name      string
		selection affected.Selection
		want      int
	}{
		{"go unit", affected.Selection{UnitID: "go:a", Witness: affected.Witness{Kind: "IMPORT"}}, pathContextListed},
		{"declared consumer", affected.Selection{UnitID: "declared-consumer:web/test/e2e.mjs", Kind: "DECLARED_CONSUMER", Witness: affected.Witness{Kind: "DECLARED_CONSUMER"}}, pathContextListed},
		{"make target", affected.Selection{UnitID: "make:gate", Kind: affected.SelectionKindDriverTarget}, pathContextOmitted},
		{"CI job", affected.Selection{UnitID: "ci:go", Kind: affected.SelectionKindCIJob}, pathContextOmitted},
		{"frontier site", affected.Selection{UnitID: "make:lint", Witness: affected.Witness{Kind: affected.WitnessFrontierSite}}, pathContextOmitted},
		{"unbounded reader", affected.Selection{UnitID: "go:c", Witness: affected.Witness{Kind: affected.WitnessUnboundedReader}}, pathContextShared},
	} {
		if got := pathContextSelection(c.selection); got != c.want {
			t.Errorf("%s: got %d, want %d", c.name, got, c.want)
		}
	}
}

// TBC-V1-011: only a read that saw no record at all, and no per-record reason, stops the loop.
func TestPathContextNothingRetained(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		document testevidencedoc.Document
		want     bool
	}{
		{"no record", testevidencedoc.Document{Reasons: []string{"no-fresh-record"}}, true},
		{"no reason", testevidencedoc.Document{}, false},
		{"other reason", testevidencedoc.Document{Reasons: []string{"unknown"}}, false},
		{"record listed", testevidencedoc.Document{Reasons: []string{"no-fresh-record"}, Records: make([]testevidencedoc.RecordView, 1)}, false},
	}
	for _, reason := range []string{"identity-missing", "malformed-record", "run-context-unresolved", "stale-record", "component-unavailable"} {
		cases = append(cases, struct {
			name     string
			document testevidencedoc.Document
			want     bool
		}{reason, testevidencedoc.Document{Reasons: []string{"no-fresh-record", reason}}, false})
	}
	for _, c := range cases {
		if got := pathContextNothingRetained(c.document); got != c.want {
			t.Errorf("%s: got %v", c.name, got)
		}
	}
}
