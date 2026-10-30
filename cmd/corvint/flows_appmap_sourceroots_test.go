package main

import (
	"strings"
	"testing"
)

// SKH-V0-189: every source verb takes --source ALIAS:DIR and a repeatable --repo ALIAS=ROOT; a
// malformed or duplicate --repo is an argument error, and an alias the map did not read refuses
// as an unavailable root naming it, with the repository unchanged.
func TestSKHV0189SourceVerbsTakeRepo(t *testing.T) {
	root, filename := railsCLIRepo(t, false)
	other := t.TempDir()
	for _, verb := range []string{"refs", "pageobjects", "steps", "setup", "links", "infer", "how"} {
		tail := []string{}
		if verb == "how" {
			tail = []string{"ship an order"}
		}
		for _, tc := range []struct {
			code string
			args []string
		}{
			{"invalid-arguments", []string{"--map", filename, "--source", "e2e", "--repo", "e2e"}},
			{"invalid-arguments", []string{"--map", filename, "--source", "e2e", "--repo", "e2e=" + other, "--repo", "e2e=" + other}},
			{"appmap-root-unavailable", []string{"--map", filename, "--source", "tests:e2e", "--repo", "tests=" + other}},
			{"appmap-root-unavailable", []string{"--map", filename, "--source", "tests:e2e"}},
		} {
			code, out, diag := runFlowsCLI(root, append(append([]string{"appmap", verb}, tc.args...), tail...)...)
			if code != 2 || out != "" || !strings.Contains(diag, tc.code) {
				t.Fatalf("%s %v: %d %s %s", verb, tc.args, code, out, diag)
			}
			if tc.code == "appmap-root-unavailable" && !strings.Contains(diag, "tests") {
				t.Fatalf("%s refusal does not name the alias: %s", verb, diag)
			}
		}
	}
	assertCleanRepo(t, root)
}
