package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// AFP-V0-011: the selector refuses an oversized external plan before JSON
// decoding instead of allowing the plan file to consume unbounded memory.
func TestSelectorRejectsOversizedPlan(t *testing.T) {
	if os.Getenv("CORVINT_TEST_OVERSIZED_PLAN") == "1" {
		os.Args = []string{"gate-affected-select", os.Getenv("CORVINT_TEST_PLAN"), "example.com/fixture", os.Getenv("CORVINT_TEST_ROOT")}
		main()
		return
	}
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "root.go"), []byte("package fixture\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	plan := filepath.Join(t.TempDir(), "plan.json")
	if err := os.WriteFile(plan, []byte(strings.Repeat(" ", (8<<20)+1)), 0o644); err != nil {
		t.Fatal(err)
	}
	command := exec.Command(os.Args[0], "-test.run=^TestSelectorRejectsOversizedPlan$")
	command.Env = append(os.Environ(), "CORVINT_TEST_OVERSIZED_PLAN=1", "CORVINT_TEST_PLAN="+plan, "CORVINT_TEST_ROOT="+root)
	output, err := command.CombinedOutput()
	if err == nil {
		t.Fatalf("oversized plan succeeded: %s", output)
	}
	if !strings.Contains(string(output), "plan exceeds 8388608 bytes") {
		t.Fatalf("oversized plan did not report its bound: %s", output)
	}
}
