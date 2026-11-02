//go:build darwin || linux

package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// AFP-V0-822: `test --dry-run` plans from the default affected graph cache and
// the gateselect scan memo, and the cached plan is byte-identical to the
// uncached one, before and after a source change.
func TestAFPV0822DryRunCachedPlanMatchesUncached(t *testing.T) {
	root, base := sliceFixture(t)
	outcomes := observeAffectedCache(t)
	cache := t.TempDir()
	plan := func() (string, []byte) {
		t.Helper()
		manifest := filepath.Join(t.TempDir(), "manifest.json")
		code, stdout, stderr := runCLI(t, "--root", root, "test", "--base", base, "--dry-run", "--manifest", manifest)
		if code != 0 {
			t.Fatalf("code %d stderr %q", code, stderr)
		}
		written, err := os.ReadFile(manifest)
		if err != nil {
			t.Fatal(err)
		}
		return stdout, written
	}
	compare := func(edit string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(root, "b", "b.go"), []byte(edit), 0o644); err != nil {
			t.Fatal(err)
		}
		useAffectedUserCacheDir(t, "", errors.New("no cache in this run"))
		*outcomes = nil
		uncachedOut, uncachedManifest := plan()
		if len(*outcomes) != 0 {
			t.Fatalf("an uncached plan used the cache: %+v", *outcomes)
		}
		useAffectedUserCacheDir(t, cache, nil)
		for run := range 2 {
			*outcomes = nil
			out, manifest := plan()
			if out != uncachedOut || string(manifest) != string(uncachedManifest) {
				t.Fatalf("cached run %d differs from the uncached plan:\n%s\n%s", run, out, uncachedOut)
			}
			if run == 1 && (len(*outcomes) != 1 || !(*outcomes)[0].Hit) {
				t.Fatalf("the second cached run did not hit the graph cache: %+v", *outcomes)
			}
		}
		// The memo is keyed by the resolved root, so it is found by name.
		if memos, _ := filepath.Glob(filepath.Join(cache, "corvint", "affected", "affected-graph-scan-*.json")); len(memos) != 1 {
			t.Fatalf("want one stored scan memo, got %q", memos)
		}
	}
	compare("package b\n\nimport \"example.test/m/a\"\n\nfunc B() int { return a.A() + 2 }\n")
	compare("package b\n\nimport \"example.test/m/a\"\n\nvar spec = \"docs/b.md\"\n\nfunc B() int { return a.A() + 3 }\n")
}
