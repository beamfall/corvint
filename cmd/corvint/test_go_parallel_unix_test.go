//go:build darwin || linux

package main

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/sessiontest"
)

// TestSTRV0173GoParallelBindsTheManifest drives `corvint test` end to end
// (V1-3149): --go-parallel 4 emits `-p 4` in place of the declared `-p 1`
// in the dry-run argv and in the launched go test, changes the manifest's
// declaredDigest, and a shard run against that manifest with another
// parallelism (or none) is refused as a manifest mismatch.
func TestSTRV0173GoParallelBindsTheManifest(t *testing.T) {
	root, base, log := sessionFixture(t, "pass")
	dir := t.TempDir()
	declared, parallel := filepath.Join(dir, "declared.json"), filepath.Join(dir, "parallel.json")
	if code, _, stderr := runCLI(t, "--root", root, "test", "--base", base, "--dry-run", "--manifest", declared); code != 0 {
		t.Fatalf("declared dry run %d %s", code, stderr)
	}
	code, stdout, stderr := runCLI(t, "--root", root, "test", "--base", base, "--go-parallel", "4", "--dry-run", "--manifest", parallel)
	if code != 0 || !strings.Contains(stdout, `"-p","4"`) || strings.Contains(stdout, `"-p","1"`) {
		t.Fatalf("--go-parallel dry run %d %s\n%s", code, stderr, stdout)
	}
	before, _, err := sessiontest.ReadManifest(declared)
	if err != nil {
		t.Fatal(err)
	}
	after, _, err := sessiontest.ReadManifest(parallel)
	if err != nil {
		t.Fatal(err)
	}
	if before.DeclaredDigest == "" || before.DeclaredDigest == after.DeclaredDigest {
		t.Fatalf("declaredDigest did not bind the parallelism: %q and %q", before.DeclaredDigest, after.DeclaredDigest)
	}
	for _, other := range [][]string{{"--go-parallel", "2"}, {}} {
		arguments := append([]string{"--root", root, "test", "--base", base, "--manifest", parallel, "--receipt", filepath.Join(dir, "refused.json")}, other...)
		if code, _, stderr := runCLI(t, arguments...); code == 0 || !strings.Contains(stderr, "manifest") {
			t.Fatalf("%v against the -p 4 manifest: exit %d %s", other, code, stderr)
		}
	}
	if strings.Contains(sessionRunnerLog(t, log), "-json") {
		t.Fatal("a refused shard launched go test")
	}
	if code, _, stderr := runCLI(t, "--root", root, "test", "--base", base, "--go-parallel", "4", "--manifest", parallel, "--receipt", filepath.Join(dir, "run.json")); code != 0 {
		t.Fatalf("matching shard %d %s", code, stderr)
	}
	if launched := sessionRunnerLog(t, log); !strings.Contains(launched, "-p 4") || strings.Contains(launched, "-p 1 ") {
		t.Fatalf("launched go test did not run -p 4:\n%s", launched)
	}
}
