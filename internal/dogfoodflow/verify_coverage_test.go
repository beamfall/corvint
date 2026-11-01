package dogfoodflow

import (
	"strings"
	"testing"
)

// TestDCWV0727InputsCheckNotesARunSubsetOfAnEditedPackage (V1-4108): a
// passing inputs-check notes a package whose non-test source the change edits
// while the verify file runs only a -run subset of it; a whole-package line
// clears the NOTE.
func TestDCWV0727InputsCheckNotesARunSubsetOfAnEditedPackage(t *testing.T) {
	root, base := preflightRepository(t, nil)
	plan := writeInput(t, "1\tpre.go\t3:3\timplementation\n2\tpre.go\t20:20\timplementation\n")
	subset := writeInput(t, "go version\ngo test -count=1 -run TestFirst .\n")
	code, stdout, stderr, _ := preflightRun(t, root, ChangeOptions{Base: base, Citations: plan, VerifyFile: subset})
	if code != 0 || !strings.Contains(stdout, "inputs-check NOTE verify-run-subset package=.: ") || !strings.Contains(stdout, "(DCW-V0-727)") {
		t.Fatalf("code=%d stdout=%s stderr=%s", code, stdout, stderr)
	}
	whole := writeInput(t, "go version\ngo test -count=1 -run TestFirst .\ngo test -count=1 .\n")
	_, stdout, _, _ = preflightRun(t, root, ChangeOptions{Base: base, Citations: plan, VerifyFile: whole})
	if strings.Contains(stdout, "verify-run-subset") {
		t.Errorf("a whole-package line still notes the subset:\n%s", stdout)
	}
}

// TestDCWV0727VerifyPackageRuns pins the line classification.
func TestDCWV0727VerifyPackageRuns(t *testing.T) {
	whole, subset := verifyPackageRuns([]string{
		"script/heavy.sh go test -count=1 -run 'TestA|TestB' ./internal/a",
		"go test -run='.*' ./internal/b",
		"go test ./internal/c/",
		"go test -run TestD ./internal/d ./internal/e/...",
		"go vet ./internal/f",
	})
	if !subset["./internal/a"] || !subset["./internal/d"] || !whole["./internal/b"] || !whole["./internal/c"] {
		t.Fatalf("whole=%v subset=%v", whole, subset)
	}
	if subset["./internal/e/..."] || whole["./internal/f"] || subset["./internal/f"] {
		t.Fatalf("whole=%v subset=%v", whole, subset)
	}
}
