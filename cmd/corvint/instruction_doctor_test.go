package main

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestInstructionDoctorFlagsAndDefaultWire(t *testing.T) {
	root := taskContextRepository(t)
	base := []string{"--root", root, "context", "--task", "inspect instructions", "--subject", "cache/demux.go"}
	flags := []string{"--instruction-host", "codex", "--instruction-host-version", "0.153.2", "--instruction-cwd", "cache", "--instruction-profile", "default"}
	runPacket := func(args []string) []byte {
		t.Helper()
		var out, stderr bytes.Buffer
		if code := runContext(context.Background(), args, strings.NewReader(""), &out, &stderr); code != 0 {
			t.Fatalf("exit %d: %s", code, &stderr)
		}
		return out.Bytes()
	}
	before := repositoryListing(t, root)
	original := runPacket(base)
	attached := runPacket(append(append([]string{}, base...), flags...))
	var plain, with map[string]json.RawMessage
	if err := json.Unmarshal(original, &plain); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(attached, &with); err != nil {
		t.Fatal(err)
	}
	if _, ok := with["instruction_load_set"]; !ok {
		t.Fatal("missing prediction")
	}
	delete(with, "instruction_load_set")
	a, _ := json.Marshal(plain)
	b, _ := json.Marshal(with)
	if !bytes.Equal(a, b) {
		t.Fatalf("prediction changed default packet/authority\n%s\n%s", a, b)
	}
	if !bytes.Equal(original, runPacket(base)) {
		t.Fatal("default output changed after optional invocation")
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
	if err := json.Unmarshal(runPacket(append(append([]string{}, base...), unknown...)), &receipt); err != nil {
		t.Fatal(err)
	}
	if receipt["instruction_load_set"].(map[string]any)["state"] != "UNKNOWN" {
		t.Fatal(receipt)
	}
}
