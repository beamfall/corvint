//go:build darwin || linux

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/sessiontest"
)

// STR-V0-064, AFP-V0-650: a change inside an independent nested module is a
// NARROWED run of that module's own go test ./... from its directory; the
// dry run prints the directory, and the run launches it and records it.
func TestSTRV0064CLINestedModuleRuns(t *testing.T) {
	root, _, log := sessionFixture(t, "pass")
	for name, body := range map[string]string{
		"nested/go.mod":    "module example.test/nested\n\ngo 1.27\n",
		"nested/n.go":      "package nested\n\nfunc N() int { return 1 }\n",
		"nested/n_test.go": "package nested\n\nimport \"testing\"\n\nfunc TestN(t *testing.T) { _ = N() }\n",
	} {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, name)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	affectedGit(t, root, "add", ".")
	affectedGit(t, root, "commit", "-qm", "add nested module")
	base := strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
	if err := os.WriteFile(filepath.Join(root, "nested/n.go"), []byte("package nested\n\nfunc N() int { return 2 }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	affectedGit(t, root, "commit", "-qam", "change nested")

	code, stdout, stderr := runCLI(t, "--root", root, "test", "--base", base, "--dry-run")
	// STR-V0-073: the nested module's ./... runs whole, so its HEAVY_WHOLE_REASON follows.
	wantDry := "corvint-test: NARROWED 1 execution units\n" +
		"corvint-test: whole-package go test planned; under script/heavy.sh run it with HEAVY_WHOLE_REASON='corvint test selected go-module:nested whole: " +
		"a nested Go module the root does not build: its packages are not indexed, so its own go test ./... runs whole from its directory (AFP-V0-650)' (STR-V0-073)\n"
	if code != 0 || stderr != wantDry {
		t.Fatalf("dry run code %d stderr %q", code, stderr)
	}
	var printed struct {
		Invocations []struct {
			Argv []string
			Dir  string
			Env  []string
		} `json:"invocations"`
		Scope string `json:"scope"`
	}
	if err := json.Unmarshal([]byte(stdout), &printed); err != nil {
		t.Fatal(err)
	}
	if printed.Scope != "NARROWED" || len(printed.Invocations) != 1 || printed.Invocations[0].Dir != "nested" ||
		strings.Join(printed.Invocations[0].Argv, " ") != "go test -json -p 1 -count=1 ./..." ||
		strings.Join(printed.Invocations[0].Env, " ") != "GOTOOLCHAIN=local GOWORK=off" {
		t.Fatalf("dry run printed %s", stdout)
	}
	if got := sessionRunnerLog(t, log); got != "" {
		t.Fatalf("a dry run launched runners:\n%s", got)
	}

	code, _, stderr = runCLI(t, "--root", root, "test", "--base", base)
	if code != 0 {
		t.Fatalf("run code %d stderr %q", code, stderr)
	}
	if got := sessionRunnerLog(t, log); got != "go env GOVERSION\ngo list ./...\ngo test -json -p 1 -count=1 ./...\n" {
		t.Fatalf("runner calls:\n%s", got)
	}
	receipt := readSessionReceipt(t, filepath.Join(root, ".git", "corvint", "test-run.json"))
	if receipt.Scope != sessiontest.ScopeNarrowed || len(receipt.Invocations) != 1 || receipt.Invocations[0].Dir != "nested" ||
		receipt.Invocations[0].Outcomes[0].Unit != sessiontest.ModuleUnitPrefix+"nested" {
		t.Fatalf("receipt %+v", receipt)
	}
}
