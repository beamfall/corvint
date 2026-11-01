package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// PUB-V0-671 (V1-0643): publication-status reads the Git state of its -root
// argument, not the process working directory, so its test needs no enclosing
// checkout. A fixture repository without a receipt is ABSENT; a root that is not
// a repository is INVALID.
func TestPUBV0671PublicationStatusReadsTheRootArgument(t *testing.T) {
	root := initArchiveTestRepository(t)
	output, err := exec.Command("git", "-C", root, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	revision := strings.TrimSpace(string(output))
	witness := filepath.Join(t.TempDir(), "witness.json")
	run := func(root string) (int, string) {
		t.Helper()
		stdout, err := os.CreateTemp(t.TempDir(), "stdout")
		if err != nil {
			t.Fatal(err)
		}
		defer stdout.Close()
		stderr, err := os.CreateTemp(t.TempDir(), "stderr")
		if err != nil {
			t.Fatal(err)
		}
		defer stderr.Close()
		code := runPublicationStatusCLI([]string{"-root", root, "-witness", witness, "-revision", revision, "-tag", publicationTestTag}, stdout, stderr)
		raw, err := os.ReadFile(stdout.Name())
		if err != nil {
			t.Fatal(err)
		}
		return code, strings.TrimSpace(string(raw))
	}
	if code, status := run(root); code != 0 || status != publicationAbsent {
		t.Fatalf("fixture root: code=%d status=%q", code, status)
	}
	if code, status := run(t.TempDir()); code != 1 || status != publicationInvalid {
		t.Fatalf("non-repository root: code=%d status=%q", code, status)
	}
}
