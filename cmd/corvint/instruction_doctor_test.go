package main

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// TestInstructionDoctorFlagsAndWire is IID-V0-001 after decision 0597: the instruction load set
// needs no task text, refuses incomplete or repeated flags, and writes nothing.
func TestInstructionDoctorFlagsAndWire(t *testing.T) {
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
		append(append([]string{}, flags...), "--task", "inspect instructions"),
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

// TestContextTaskPacketIsRemoved is decision 0597: task text, --task, --limit and
// --worktree-overlay refuse with exit 2 and point at affected and impact.
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
