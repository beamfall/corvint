package main

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// TestInstructionDoctorFlagsAndDefaultWire is IID-V0-001 after decision 0597: the instruction load set
// needs no task text, refuses incomplete or repeated flags, and writes nothing.
func TestInstructionDoctorFlagsAndDefaultWire(t *testing.T) {
	root := taskContextRepository(t)
	base := []string{"--root", root, "context", "--subject", "cache/demux.go"}
	flags := []string{"--instruction-host", "codex", "--instruction-host-version", "0.153.2", "--instruction-cwd", "cache", "--instruction-profile", "default"}
	run := func(args []string) []byte {
		t.Helper()
		var out, stderr bytes.Buffer
		if code := runContext(context.Background(), args, strings.NewReader(""), &out, &stderr); code != 0 {
			t.Fatalf("exit %d: %s", code, &stderr)
		}
		return out.Bytes()
	}
	before := repositoryListing(t, root)
	first := run(append(append([]string{}, base...), flags...))
	var document map[string]any
	if err := json.Unmarshal(first, &document); err != nil {
		t.Fatal(err)
	}
	if _, ok := document["instruction_load_set"].(map[string]any); !ok || document["mutates"] != false {
		t.Fatalf("missing prediction: %s", first)
	}
	if !bytes.Equal(first, run(append(append([]string{}, base...), flags...))) {
		t.Fatal("instruction load set is not deterministic")
	}
	if after := repositoryListing(t, root); before != after {
		t.Fatal("read command mutated fixture")
	}
	for _, suffix := range [][]string{
		{"--instruction-host", "codex"},
		append(append([]string{}, flags...), "--instruction-host", "codex"),
		append(append([]string{}, flags...), "--summary"),
		{"--instruction-host", "codex", "--instruction-host-version", "0.153.2", "--instruction-cwd", "cache", "--instruction-profile", "actual"},
	} {
		if _, requested, err := parseTaskContextInvocation(append(append([]string{}, base...), suffix...)); !requested || err == nil {
			t.Fatal("accepted invalid instruction flags", suffix)
		}
	}
	unknown := append([]string{}, flags...)
	unknown[3] = "unknown"
	var receipt map[string]any
	if err := json.Unmarshal(run(append(append([]string{}, base...), unknown...)), &receipt); err != nil {
		t.Fatal(err)
	}
	if receipt["instruction_load_set"].(map[string]any)["state"] != "UNKNOWN" {
		t.Fatal(receipt)
	}
}

// TestInstructionsVerbMatchesTheAlias is IID-V0-001: corvint instructions and the context alias
// print the same instruction_load_set, only the alias prints the deprecation line, and a missing
// --cwd refuses with exit 2.
func TestInstructionsVerbMatchesTheAlias(t *testing.T) {
	root := taskContextRepository(t)
	run := func(args ...string) (int, string, string) {
		t.Helper()
		var out, stderr bytes.Buffer
		code := runContext(context.Background(), append([]string{"--root", root}, args...), strings.NewReader(""), &out, &stderr)
		return code, out.String(), stderr.String()
	}
	code, verb, verbErr := run("instructions", "--subject", "cache/demux.go", "--host", "codex", "--host-version", "0.153.2", "--cwd", "cache", "--profile", "default")
	if code != 0 || verbErr != "" {
		t.Fatalf("instructions: exit %d stderr %q", code, verbErr)
	}
	code, alias, aliasErr := run("context", "--subject", "cache/demux.go", "--instruction-host", "codex", "--instruction-host-version", "0.153.2", "--instruction-cwd", "cache", "--instruction-profile", "default")
	if code != 0 || alias != verb {
		t.Fatalf("alias: exit %d, output differs:\n%s\n%s", code, alias, verb)
	}
	if strings.Count(aliasErr, "\n") != 1 || !strings.Contains(aliasErr, "corvint instructions") {
		t.Fatalf("alias stderr %q, want one deprecation line naming corvint instructions", aliasErr)
	}
	before := repositoryListing(t, root)
	for _, args := range [][]string{
		{"instructions", "--subject", "cache/demux.go", "--host", "codex", "--host-version", "0.153.2", "--profile", "default"},
		{"instructions", "--subject", "cache/demux.go", "--host", "codex", "--host-version", "0.153.2", "--cwd", "cache", "--profile", "actual"},
		{"instructions", "--subject", "cache/demux.go", "--instruction-host", "codex", "--host-version", "0.153.2", "--cwd", "cache", "--profile", "default"},
		{"instructions", "--subject", "cache/demux.go", "--host", "codex", "--host", "codex", "--host-version", "0.153.2", "--cwd", "cache", "--profile", "default"},
		{"instructions", "--task", "inspect", "--subject", "cache/demux.go", "--host", "codex", "--host-version", "0.153.2", "--cwd", "cache", "--profile", "default"},
		{"instructions"},
	} {
		if code, out, _ := run(args...); code != 2 || out != "" {
			t.Errorf("%v: exit %d stdout %q, want 2 and no output", args, code, out)
		}
	}
	if after := repositoryListing(t, root); before != after {
		t.Fatal("a refused instructions call mutated the fixture")
	}
}

// TestContextAliasIgnoresTheRemovedPacketFlags is IID-V0-001: callers that passed --task, --limit or
// --worktree-overlay with the instruction flags still get the load set, byte-identical to the
// instructions verb, plus the deprecation line and a warning naming the ignored flags.
func TestContextAliasIgnoresTheRemovedPacketFlags(t *testing.T) {
	root := taskContextRepository(t)
	run := func(args ...string) (int, string, string) {
		t.Helper()
		var out, stderr bytes.Buffer
		code := runContext(context.Background(), append([]string{"--root", root}, args...), strings.NewReader(""), &out, &stderr)
		return code, out.String(), stderr.String()
	}
	_, want, _ := run("instructions", "--subject", "cache/demux.go", "--host", "codex", "--host-version", "0.153.2", "--cwd", "cache", "--profile", "default")
	instruction := []string{"--subject", "cache/demux.go", "--instruction-host", "codex", "--instruction-host-version", "0.153.2", "--instruction-cwd", "cache", "--instruction-profile", "default"}
	for _, extra := range [][]string{
		{"--task", "find the demux"},
		{"--task=demux"},
		{"--limit", "5", "--worktree-overlay"},
		{"--limit=5"},
	} {
		code, out, stderr := run(append(append([]string{"context"}, instruction...), extra...)...)
		if code != 0 || out != want {
			t.Fatalf("%v: exit %d, output differs from the instructions verb: %s", extra, code, out)
		}
		if strings.Count(stderr, "\n") != 2 || !strings.Contains(stderr, "corvint instructions") || !strings.Contains(stderr, "are ignored") {
			t.Fatalf("%v: stderr %q, want the deprecation line and the ignored-flags warning", extra, stderr)
		}
	}
	if code, out, _ := run("instructions", "--task", "x", "--subject", "cache/demux.go", "--host", "codex", "--host-version", "0.153.2", "--cwd", "cache", "--profile", "default"); code != 2 || out != "" {
		t.Fatalf("instructions accepted --task: exit %d", code)
	}
}

// TestContextTaskPacketIsRemoved is decision 0597: task text, and --task, --limit or
// --worktree-overlay without instruction flags, refuse with exit 2 and point at affected and impact.
func TestContextTaskPacketIsRemoved(t *testing.T) {
	root := taskContextRepository(t)
	for _, rest := range [][]string{
		{"--task", "find the demux"},
		{"find the demux"},
		{"--limit", "5"},
		{"--worktree-overlay"},
		{},
	} {
		var out, stderr bytes.Buffer
		arguments := append([]string{"--root", root, "context"}, rest...)
		if code := runContext(context.Background(), arguments, strings.NewReader(""), &out, &stderr); code != 2 || out.Len() != 0 {
			t.Fatalf("%v: exit %d stdout %q", rest, code, out.String())
		}
		if len(rest) > 0 && !strings.Contains(stderr.String(), "corvint affected --base") {
			t.Fatalf("%v: refusal does not name the replacement: %s", rest, stderr.String())
		}
	}
}
