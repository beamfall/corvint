package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// SKH-V0-013 SKH-V0-005 SKH-V0-007: pageobjects and steps are served by the flows appmap family as
// byte-capped, byte-identical reports, refuse invalid input with exit 2 and change no repository file.
func TestSKHV0_013_PageObjectsAndStepsCLI(t *testing.T) {
	root, filename := railsCLIRepo(t, false)
	for verb, schema := range map[string]string{"pageobjects": "site-knowledge-page-objects/0", "steps": "site-knowledge-steps/0"} {
		code, out, diag := runFlowsCLI(root, "appmap", verb, "--map", filename, "--source", "e2e")
		var doc map[string]any
		if code != 0 || json.Unmarshal([]byte(out), &doc) != nil || doc["schema"] != schema || doc["authority"] != "learned" ||
			doc["files_read"] != float64(2) || len(out) > 6144 {
			t.Fatalf("%s: %d %s %s", verb, code, diag, out)
		}
		if _, again, _ := runFlowsCLI(root, "appmap", verb, "--map", filename, "--source", "e2e"); again != out {
			t.Fatalf("%s rerun differs", verb)
		}
		if verb == "steps" && !strings.Contains(out, `"kind":"visit"`) {
			t.Fatalf("steps: %s", out)
		}
		for _, tc := range []struct {
			code string
			args []string
		}{
			{"invalid-arguments", []string{"--map", filename, "--source", "e2e", "extra"}},
			{"appmap-invalid-query", []string{"--map", filename}},
			{"appmap-invalid-query", []string{"--map", filename, "--source", "../e2e"}},
			{"appmap-invalid-query", []string{"--map", filename, "--source", "missing"}},
			{"appmap-invalid-query", []string{"--map", filename, "--source", "e2e", "--budget", "0"}},
		} {
			code, out, diag = runFlowsCLI(root, append([]string{"appmap", verb}, tc.args...)...)
			if code != 2 || out != "" || !strings.Contains(diag, tc.code) {
				t.Fatalf("%s admitted %v: %d %s %s", verb, tc.args, code, out, diag)
			}
		}
	}
	assertCleanRepo(t, root)
}
