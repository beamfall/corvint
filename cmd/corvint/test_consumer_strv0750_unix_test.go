//go:build darwin || linux

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/sessiontest"
)

// STR-V0-750, V1-4008: on the local profile a change that reaches a Go
// command with a declared binary consumer (AFP-V0-762) stays NARROWED and
// names the consumer once as a check still required and not run, where it
// went FULL before.
func TestSTRV0750CLIBinaryConsumerIsANamedCheck(t *testing.T) {
	root, _, _ := sessionFixture(t, "pass")
	for name, body := range map[string]string{
		"cmd/tool/main.go":           "package main\n\nimport \"example.test/m/a\"\n\nfunc main() { _ = a.A() }\n",
		"web/test/e2e.mjs":           "// runs the built tool binary\n",
		affected.BinaryConsumersPath: `{"profile":"corvint-test-binary-consumers/0","consumers":{"web/test/e2e.mjs":["cmd/tool"]}}` + "\n",
	} {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, name)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	affectedGit(t, root, "add", ".")
	affectedGit(t, root, "commit", "-qm", "add a tool and its consumer")
	base := strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
	if err := os.WriteFile(filepath.Join(root, "a/a.go"), []byte("package a\n\nfunc A() int { return 4 }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	affectedGit(t, root, "commit", "-qam", "change a again")

	receipt, raw, stderr, code := runAffectedCLI(t, root, "--base", base)
	plan, _ := receipt["plan"].(map[string]any)
	if code != 0 || !strings.Contains(string(raw), `"UNSELECTED_BINARY_CONSUMER"`) || !strings.Contains(string(raw), `"web/test/e2e.mjs"`) || plan == nil {
		t.Fatalf("the plan does not name the consumer: exit=%d stderr=%s receipt=%s", code, stderr, raw)
	}

	code, stdout, stderr := runCLI(t, "--root", root, "test", "--base", base, "--dry-run")
	want := "corvint-test: still required, not run: " + sessiontest.BinaryConsumerCheck("web/test/e2e.mjs") + "\n"
	if code != 0 || !strings.HasPrefix(stderr, "corvint-test: NARROWED ") || strings.Count(stderr, want) != 1 || strings.Contains(stderr, "FALLBACK") {
		t.Fatalf("dry run code %d stderr %q", code, stderr)
	}
	var printed struct {
		Checks []string `json:"checks"`
		Scope  string   `json:"scope"`
	}
	if err := json.Unmarshal([]byte(stdout), &printed); err != nil {
		t.Fatal(err)
	}
	if printed.Scope != "NARROWED" || strings.Join(printed.Checks, "\n") != sessiontest.BinaryConsumerCheck("web/test/e2e.mjs") {
		t.Fatalf("dry run printed %s", stdout)
	}
}
