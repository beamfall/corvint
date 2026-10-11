package main

import (
	"path/filepath"
	"reflect"
	"testing"
)

// CEM-V1-034: a cem/1.0 `cem report --format json` is admitted against the
// root exactly as the caller named it. Through a symlinked alias it reproduces
// `cem verify-stable --repository <alias>` (complete result and exit); through
// the real root it passes repository admission.
func TestCEMStableReportKeepsCallerRoot(t *testing.T) {
	f := materializeCEMStableRoot(t)
	stages := map[string]any{}
	for _, root := range []string{f.repo, f.alias} {
		wantExit, want := runCEMStableEntry(t, "cem", "verify-stable", "--repository", root,
			"--map", filepath.Join(f.repo, "change.cem.json"), "--expected-base", f.expectedBase,
			"--target", f.target, "--artifacts", f.artifacts)
		stages[root] = want["stage"]
		exit, envelope := runCEMStableEntry(t, "--root", root, "cem", "report", "--format", "json", "--map", "change.cem.json",
			"--expected-base", f.expectedBase, "--target", f.target, "--artifacts", f.artifacts)
		if exit != wantExit || !reflect.DeepEqual(envelope["stableVerification"], want) {
			t.Errorf("--root %s cem report = exit %d %v\nverify-stable --repository %s = exit %d %v",
				root, exit, envelope["stableVerification"], root, wantExit, want)
		}
	}
	if stages[f.alias] != "repository" || stages[f.repo] == "repository" || stages[f.repo] == "runtime" {
		t.Fatalf("fixture does not discriminate the alias: real root stage %v, alias stage %v", stages[f.repo], stages[f.alias])
	}
}
