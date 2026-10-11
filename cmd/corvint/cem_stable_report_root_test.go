package main

import (
	"path/filepath"
	"reflect"
	"testing"
)

// CEM-V1-034: a cem/1.0 `cem report --format json` is admitted against the
// root byte for byte as the caller named it. For every spelling of the root
// (absolute or relative, clean or not, through the alias or the real
// directory) it reproduces `cem verify-stable --repository <same string>`
// (complete result and exit).
func TestCEMStableReportKeepsCallerRoot(t *testing.T) {
	f := materializeCEMStableRoot(t)
	t.Chdir(filepath.Dir(f.repo))
	for _, c := range []struct{ root, stage string }{
		{f.repo, ""},
		{f.alias, "repository"},
		{f.alias + "/../repo", "repository"},
		{f.repo + "/", "repository"},
		{"./repo", "arguments"},
		{"repo", "arguments"},
		{"./alias", "arguments"},
	} {
		wantExit, want := runCEMStableEntry(t, "cem", "verify-stable", "--repository", c.root,
			"--map", filepath.Join(f.repo, "change.cem.json"), "--expected-base", f.expectedBase,
			"--target", f.target, "--artifacts", f.artifacts)
		stage, _ := want["stage"].(string)
		if c.stage != stage && (c.stage != "" || stage == "arguments" || stage == "repository" || stage == "runtime") {
			t.Fatalf("fixture does not discriminate %q: verify-stable stage %q, want %q", c.root, stage, c.stage)
		}
		exit, envelope := runCEMStableEntry(t, "--root", c.root, "cem", "report", "--format", "json", "--map", "change.cem.json",
			"--expected-base", f.expectedBase, "--target", f.target, "--artifacts", f.artifacts)
		if exit != wantExit || !reflect.DeepEqual(envelope["stableVerification"], want) {
			t.Errorf("--root %s cem report = exit %d %v\nverify-stable --repository %s = exit %d %v",
				c.root, exit, envelope["stableVerification"], c.root, wantExit, want)
		}
	}
}
