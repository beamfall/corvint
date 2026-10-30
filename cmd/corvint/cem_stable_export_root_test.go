package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// CEM-V1-060: `cem export` admits the caller-named root byte for byte, as
// `cem verify-stable --repository` judges the same string. For every spelling
// that verify-stable does not accept, export returns verify-stable's complete
// result and exit and publishes nothing.
func TestCEMExportStableKeepsCallerRoot(t *testing.T) {
	f := materializeCEMStableRoot(t)
	base := filepath.Dir(f.repo)
	for _, name := range []string{"--repo"} {
		if err := os.Symlink(f.repo, filepath.Join(base, name)); err != nil {
			t.Fatal(err)
		}
	}
	t.Chdir(base)
	output := filepath.Join(base, "bundle")
	for _, root := range []string{f.alias, f.alias + "/../repo", f.repo + "/", "./repo", "repo", "./alias", "--repo"} {
		wantExit, want := runCEMStableEntry(t, "cem", "verify-stable", "--repository", root,
			"--map", filepath.Join(f.repo, "change.cem.json"), "--expected-base", f.expectedBase,
			"--target", f.target, "--artifacts", f.artifacts)
		if wantExit == 0 {
			t.Fatalf("fixture does not discriminate %q: verify-stable accepted", root)
		}
		exit, envelope := runCEMStableEntry(t, "--root="+root, "cem", "export", "--map", "change.cem.json",
			"--expected-base", f.expectedBase, "--target", f.target, "--output", output, "--artifacts", f.artifacts)
		if exit != wantExit || envelope["ok"] != false || !reflect.DeepEqual(envelope["stableVerification"], want) {
			t.Errorf("--root %q cem export = exit %d %v\nverify-stable = exit %d %v", root, exit, envelope["stableVerification"], wantExit, want)
		}
		if _, err := os.Lstat(output); !os.IsNotExist(err) {
			t.Fatalf("--root %q published a bundle", root)
		}
	}
}
