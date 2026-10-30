package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const provePyCalc = "\"\"\"Tiny calculator.\"\"\"\n\n\ndef add(a, b):\n    return a + b\n\n\ndef is_positive(n):\n    if n > 0:\n        return True\n    return False\n"

const provePyKillingTest = "from calc import add, is_positive\n\n\ndef test_add():\n    assert add(2, 3) == 5\n\n\ndef test_is_positive():\n    assert is_positive(1)\n    assert not is_positive(-1)\n"

// requirePytest skips when python3 on PATH cannot import pytest, the way
// requireNestableSandbox skips without a sandbox.
func requirePytest(t *testing.T) {
	t.Helper()
	if os.Getenv("CORVINT_TEST_EXTERNAL_PYTEST") != "1" {
		t.Skip("external Python project qualification requires CORVINT_TEST_EXTERNAL_PYTEST=1")
	}
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 is not available")
	}
	if output, err := exec.Command(python, "-c", "import pytest").CombinedOutput(); err != nil {
		t.Skipf("pytest is not available to %s: %s", python, strings.TrimSpace(string(output)))
	}
}

// provePyRepository commits a root-level Python module with its pytest file,
// then a second commit that changes calc.py, so `prove --base FIRST` judges
// a committed range whose affected test is Python.
func provePyRepository(t *testing.T) (root, base string) {
	t.Helper()
	requireNestableSandbox(t)
	requirePytest(t)
	root = cliRepository(t)
	// go.mod: change mode's embedded range impact still demands a Go module
	// even when no changed path is Go (internal/contextindex/range_impact.go).
	files := map[string]string{"calc.py": provePyCalc, "test_calc.py": provePyKillingTest, ".gitignore": ".corvint/\n", "go.mod": "module example.test/prove\n\ngo 1.27.0\n"}
	for relative, content := range files {
		if err := os.WriteFile(filepath.Join(root, relative), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	affectedGit(t, root, "add", ".")
	affectedGit(t, root, "commit", "-qm", "python fixture")
	base = strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
	if err := os.WriteFile(filepath.Join(root, "calc.py"), []byte(strings.Replace(provePyCalc, "return a + b", "return b + a", 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	affectedGit(t, root, "add", ".")
	affectedGit(t, root, "commit", "-qm", "commute add")
	return root, base
}

func pythonTestRow(t *testing.T, receipt map[string]any) map[string]any {
	t.Helper()
	for _, row := range proofRows(t, receipt) {
		if stringAt(row, "kind") == kindAffectedTest && stringAt(row, "result") == "test_calc.py" {
			return row
		}
	}
	t.Fatalf("no affected-test row for test_calc.py: %v", receipt["proof"])
	return nil
}

// TestProveChangeModeJudgesPythonTestRowsUnderMutate pins FPK-V0-019 on the
// CLI: a Python affected-test row gets test-kills-mutant, stays NOT_RUN
// without --mutate, and passes under it when the cited pytest file kills a
// mutant confined to the change; the repository is never written. Impact
// mode emits same-package test rows for Go only, so change mode is the path
// a Python claim reaches the runner by.
func TestProveChangeModeJudgesPythonTestRowsUnderMutate(t *testing.T) {
	t.Parallel()
	root, base := provePyRepository(t)
	before := treeDigest(t, root)
	receipt, _, stderr, code := runProveCLI(t, root, "--base", base)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	row := pythonTestRow(t, receipt)
	if row["falsifier"] != falsifierMutant || row["falsified"] != falsifiedNotRun || len(stringAt(row, "blob_hash")) != 40 {
		t.Fatalf("without --mutate: %v", row)
	}
	if _, present := row["detail"]; present {
		t.Fatalf("without --mutate the row must carry no detail: %v", row)
	}
	receipt, _, stderr, code = runProveCLI(t, root, "--base", base, "--mutate")
	if code != 0 {
		t.Fatalf("--mutate exit %d: %s", code, stderr)
	}
	row = pythonTestRow(t, receipt)
	if row["falsified"] != falsifiedPass || !strings.Contains(stringAt(row, "detail"), "killed") || !strings.Contains(stringAt(row, "detail"), "calc.py:5") {
		t.Fatalf("with --mutate: %v", row)
	}
	// The embedded range packet holds no Go result for a .py-only change, so
	// its state is the packet's own business; the proof counts are this test's.
	if proofField(t, receipt, "proven_results") != 1 || proofField(t, receipt, "failed_results") != 0 {
		t.Fatalf("proof %v", receipt["proof"])
	}
	if after := treeDigest(t, root); after != before {
		t.Fatal("prove --base --mutate changed the repository")
	}
}

func TestMutationClaimSeparatesLanguagesAndTests(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		test, changed string
		want          bool
	}{
		{"test_calc.py", "calc.py", true},
		{"calc_test.py", "pkg/calc.py", true},
		{"pkg/calc_test.go", "pkg/calc.go", true},
		{"test_calc.py", "test_calc.py", false},
		{"test_calc.py", "tests/test_other.py", false},
		{"test_calc.py", "calc.go", false},
		{"pkg/calc_test.go", "calc.py", false},
		{"tests/helpers.py", "calc.py", true},
		{"test_calc.py", "calc.rs", false},
	} {
		if got := mutationClaim(test.test, test.changed); got != test.want {
			t.Errorf("mutationClaim(%q, %q) = %v, want %v", test.test, test.changed, got, test.want)
		}
	}
	for path, want := range map[string]string{"test_calc.py": falsifierMutant, "calc_test.py": falsifierMutant, "tests/helpers.py": falsifierNone, "calc.py": falsifierNone} {
		if got := falsifierFor(proveRow{Kind: kindAffectedTest, Authority: authorityAffected, Path: path}); got != want {
			t.Errorf("%s: got %s want %s", path, got, want)
		}
	}
	if row := affectedRow("tests/helpers.py", "calc.py"); row.Falsifier != falsifierNone {
		t.Errorf("a helper module is not a test: %s", row.Falsifier)
	}
}
