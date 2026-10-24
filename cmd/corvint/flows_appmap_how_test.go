package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// SKH-V0-143 SKH-V0-150: flows appmap how prints the site-knowledge-how/0 answer for a goal,
// byte-identically on rerun; a goal no candidate covers abstains with exit 0; a missing goal or
// map exits 2.
func TestSKHV0_143_HowCLI(t *testing.T) {
	root, filename := inferCLIRepo(t)
	code, out, diag := runFlowsCLI(root, "appmap", "how", "--map", filename, "--source", "e2e", "--condition", "an order exists", "--full", "ship an order")
	if code != 0 {
		t.Fatalf("how: %d %s", code, diag)
	}
	var doc struct {
		Schema     string            `json:"schema"`
		Status     string            `json:"status"`
		Steps      []json.RawMessage `json:"steps"`
		Conditions []json.RawMessage `json:"conditions"`
		Oracle     []json.RawMessage `json:"oracle"`
	}
	if err := json.Unmarshal([]byte(out), &doc); err != nil || doc.Schema != "site-knowledge-how/0" || len(doc.Steps) == 0 || len(doc.Conditions) != 1 || len(doc.Oracle) != 1 {
		t.Fatalf("answer = %v %s", err, out)
	}
	if _, again, _ := runFlowsCLI(root, "appmap", "how", "--map", filename, "--source", "e2e", "--condition", "an order exists", "--full", "ship an order"); again != out {
		t.Fatal("how CLI is not byte-stable")
	}
	if code, out, _ = runFlowsCLI(root, "appmap", "how", "--map", filename, "--source", "e2e", "launch the rocket"); code != 0 || !strings.Contains(out, `"status":"ABSTAINED"`) || !strings.Contains(out, `"no-route"`) {
		t.Fatalf("abstain: %d %s", code, out)
	}
	for _, args := range [][]string{
		{"appmap", "how", "--map", filename, "--source", "e2e"},
		{"appmap", "how", "--source", "e2e", "ship"},
		{"appmap", "how", "--map", filename, "--source", "e2e", "a", "b"},
	} {
		if code, _, _ := runFlowsCLI(root, args...); code != 2 {
			t.Fatalf("%v exit = %d, want 2", args, code)
		}
	}
}
