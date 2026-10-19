package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// AMAP-V0-140 AMAP-V0-145: blocker writes the report, exits 1 when a cited 404 request is
// flagged and 0 when it is accepted, refuses invalid input with exit 2 and changes no file.
func TestAMAPV0145BlockerCLI(t *testing.T) {
	root, filename := railsCLIRepo(t, true)
	code, out, diag := runFlowsCLI(root, "appmap", "blocker", "--map", filename, "--note", "blocked: fixture gap, 404 POST /api/orders")
	var doc map[string]any
	if code != 1 || json.Unmarshal([]byte(out), &doc) != nil || doc["schema"] != "application-map-blocker/0" || doc["verdict"] != "flagged" ||
		!strings.Contains(out, `"rule":"BLOCKER-404-NO-ROUTE"`) || !strings.Contains(out, `"path":"/orders"`) || !strings.Contains(diag, "1 cited 404 requests flagged") {
		t.Fatalf("flagged: %d %s %s", code, diag, out)
	}
	if code, out, diag = runFlowsCLI(root, "appmap", "blocker", "--map", filename, "--note", "404 POST /orders", "--full"); code != 0 || !strings.Contains(out, `"verdict":"accepted"`) {
		t.Fatalf("accepted: %d %s %s", code, diag, out)
	}
	for _, tc := range []struct {
		code string
		args []string
	}{
		{"invalid-arguments", []string{"--note", "404 GET /x"}},
		{"appmap-invalid-query", []string{"--map", filename}},
		{"appmap-invalid-query", []string{"--map", filename, "--note", "404 GET /x", "--budget", "0"}},
		{"invalid-arguments", []string{"--map", filename, "--note", "404 GET /x", "extra"}},
	} {
		code, out, diag = runFlowsCLI(root, append([]string{"appmap", "blocker"}, tc.args...)...)
		if code != 2 || out != "" || !strings.Contains(diag, tc.code) {
			t.Fatalf("admitted %v: %d %s %s", tc.args, code, out, diag)
		}
	}
	if !strings.Contains(flowsAppmapHelp, "flows appmap blocker") {
		t.Fatal("help omits blocker")
	}
	assertCleanRepo(t, root)
}
