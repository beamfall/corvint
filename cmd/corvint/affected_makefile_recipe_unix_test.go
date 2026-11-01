//go:build darwin || linux

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TSP-V0-945 (V1-3798): a change to a Makefile recipe alone still selects the
// shell test that runs that target through make, so a recipe edit never plans
// no test. The Makefile is read by the shell test, which pins the recipe.
func TestAffectedMakefileRecipeOnlySelectsScriptTest_TSP_V0_945(t *testing.T) {
	root, _ := sourceSliceFixture(t)
	fixture := map[string]string{
		"Makefile":           ".PHONY: check-foo\ncheck-foo:\n\tscript/foo.sh\n",
		"script/foo.sh":      "#!/usr/bin/env bash\nset -euo pipefail\necho foo\n",
		"script/foo_test.sh": "#!/usr/bin/env bash\nset -euo pipefail\ncd \"$(dirname \"$0\")/..\"\nmake -s check-foo | grep -qx foo\n",
	}
	for name, body := range fixture {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, name)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	affectedGit(t, root, "add", ".")
	affectedGit(t, root, "commit", "-qm", "make")
	base := strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
	recipe := strings.Replace(fixture["Makefile"], "\tscript/foo.sh\n", "\t@script/foo.sh\n", 1)
	if err := os.WriteFile(filepath.Join(root, "Makefile"), []byte(recipe), 0o644); err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr := runCLI(t, "--root", root, "affected", "--base", base, "--format", "json")
	if code != 0 {
		t.Fatalf("code %d stderr %q", code, stderr)
	}
	var receipt struct {
		Plan struct {
			Selected []struct {
				UnitID string `json:"unitId"`
			} `json:"selected"`
		} `json:"plan"`
	}
	if err := json.Unmarshal([]byte(stdout), &receipt); err != nil {
		t.Fatalf("%v: %s", err, stdout)
	}
	for _, unit := range receipt.Plan.Selected {
		if unit.UnitID == "shell:script/foo_test.sh" {
			return
		}
	}
	t.Fatalf("a recipe-only Makefile change did not select shell:script/foo_test.sh:\n%s", stdout)
}

// TSP-V0-946 (V1-3616 a): a gitignored build product never enters the plan,
// in dirty mode or against a base, so a stray binary or generated Go file
// beside the sources neither selects nor widens anything.
func TestAffectedIgnoresGitignoredBuildProducts_TSP_V0_946(t *testing.T) {
	root, _ := sourceSliceFixture(t)
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("/corvint\n/out/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	affectedGit(t, root, "add", ".gitignore")
	affectedGit(t, root, "commit", "-qm", "ignore")
	base := strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
	for name, body := range map[string]string{
		"corvint":         "\x7fELF stray build product",
		"out/out.go":      "package out\n\nimport _ \"example.test/m/p\"\n",
		"out/out_test.go": "package out\n\nimport \"testing\"\n\nfunc TestOut(t *testing.T) {}\n",
	} {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, name)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, args := range [][]string{
		{"--root", root, "affected", "--format", "json"},
		{"--root", root, "affected", "--base", base, "--format", "json"},
	} {
		code, stdout, stderr := runCLI(t, args...)
		if code != 0 {
			t.Fatalf("%v: code %d stderr %q", args, code, stderr)
		}
		var receipt struct {
			Plan struct {
				Dirty    []string `json:"dirty"`
				Selected []struct {
					UnitID string `json:"unitId"`
				} `json:"selected"`
			} `json:"plan"`
		}
		if err := json.Unmarshal([]byte(stdout), &receipt); err != nil {
			t.Fatalf("%v: %s", err, stdout)
		}
		if len(receipt.Plan.Dirty) != 0 || len(receipt.Plan.Selected) != 0 {
			t.Fatalf("%v: ignored files entered the plan:\n%s", args, stdout)
		}
	}
}
