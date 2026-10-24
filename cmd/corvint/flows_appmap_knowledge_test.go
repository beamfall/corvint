package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// SKH-V0-013 SKH-V0-005 SKH-V0-007 SKH-V0-030 SKH-V0-115: pageobjects, steps, setup and links are served by the flows
// appmap family as byte-capped, byte-identical reports with no repeated top-level key, refuse
// invalid input with exit 2 and change no repository file.
func TestSKHV0_013_PageObjectsAndStepsCLI(t *testing.T) {
	root, filename := railsCLIRepo(t, false)
	for verb, schema := range map[string]string{"pageobjects": "site-knowledge-page-objects/0", "steps": "site-knowledge-steps/0",
		"setup": "site-knowledge-setup/1", "links": "site-knowledge-edges/0"} {
		code, out, diag := runFlowsCLI(root, "appmap", verb, "--map", filename, "--source", "e2e")
		var doc map[string]any
		if code != 0 || json.Unmarshal([]byte(out), &doc) != nil || doc["schema"] != schema || doc["authority"] != "learned" ||
			doc["files_read"] != float64(2) || len(out) > 6144 {
			t.Fatalf("%s: %d %s %s", verb, code, diag, out)
		}
		if _, again, _ := runFlowsCLI(root, "appmap", verb, "--map", filename, "--source", "e2e"); again != out {
			t.Fatalf("%s rerun differs", verb)
		}
		if key := duplicateTopLevelKey(out); key != "" {
			t.Fatalf("%s: duplicate top-level key %q: %s", verb, key, out)
		}
		if verb == "steps" && !strings.Contains(out, `"kind":"visit"`) {
			t.Fatalf("steps: %s", out)
		}
		if verb == "setup" && (doc["not_read"] == nil || doc["tests_total"] == nil) {
			t.Fatalf("setup: %s", out)
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

// duplicateTopLevelKey returns a key the JSON object repeats at its top level, which a decoder
// would silently collapse.
func duplicateTopLevelKey(doc string) string {
	dec := json.NewDecoder(strings.NewReader(doc))
	if _, err := dec.Token(); err != nil {
		return ""
	}
	seen := map[string]bool{}
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return ""
		}
		key, _ := tok.(string)
		if seen[key] {
			return key
		}
		seen[key] = true
		var skip json.RawMessage
		if dec.Decode(&skip) != nil {
			return ""
		}
	}
	return ""
}

// SKH-V0-103 SKH-V0-031: setup --condition answers each condition with candidates or unresolved,
// refuses an empty, multi-line, secret-shaped or 17th condition with exit 2, is only a setup flag,
// and changes no repository file.
func TestSKHV0_103_SetupConditionCLI(t *testing.T) {
	root, filename := railsCLIRepo(t, false)
	code, out, diag := runFlowsCLI(root, "appmap", "setup", "--map", filename, "--source", "e2e", "--condition", "Order.locked", "--condition", "Invoice")
	var doc struct {
		Conditions []struct {
			Condition, Status string
			Candidates        []struct{ Kind string }
		}
	}
	if code != 0 || json.Unmarshal([]byte(out), &doc) != nil || len(doc.Conditions) != 2 || doc.Conditions[0].Status != "candidates" ||
		doc.Conditions[0].Candidates[0].Kind != "factory" || doc.Conditions[1].Status != "unresolved" {
		t.Fatalf("conditions: %d %s %s", code, diag, out)
	}
	many := []string{"appmap", "setup", "--map", filename, "--source", "e2e"}
	for i := 0; i < 17; i++ {
		many = append(many, "--condition", "Order")
	}
	for _, args := range [][]string{
		{"appmap", "setup", "--map", filename, "--source", "e2e", "--condition", " "},
		{"appmap", "setup", "--map", filename, "--source", "e2e", "--condition", "a\nb"},
		{"appmap", "setup", "--map", filename, "--source", "e2e", "--condition", "token AKIAIOSFODNN7EXAMPLE"},
		many,
	} {
		if code, out, diag := runFlowsCLI(root, args...); code != 2 || out != "" || !strings.Contains(diag, "appmap-invalid-query") {
			t.Fatalf("admitted %q: %d %s %s", args[len(args)-1], code, out, diag)
		}
	}
	if code, _, _ := runFlowsCLI(root, "appmap", "steps", "--map", filename, "--source", "e2e", "--condition", "Order"); code != 2 {
		t.Fatal("--condition admitted outside setup")
	}
	assertCleanRepo(t, root)
}
