package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// CEM-V1-034: a cem/1.0 `cem report --format json` is admitted against the
// root byte for byte as the caller named it. For every root spelling the S1
// read covers (absolute or relative, clean or not, through the alias or the
// real directory, empty, omitted or option-shaped) it reproduces
// `cem verify-stable --repository <same string>` (complete result and exit).
func TestCEMStableReportKeepsCallerRoot(t *testing.T) {
	f := materializeCEMStableRoot(t)
	base := filepath.Dir(f.repo)
	for _, name := range []string{"--repo", "-x", "--map"} {
		if err := os.Symlink(f.repo, filepath.Join(base, name)); err != nil {
			t.Fatal(err)
		}
	}
	for _, c := range []struct {
		dir        string
		core       []string
		repository string
		stage      string
	}{
		{base, []string{"--root", f.repo}, f.repo, ""},
		{base, []string{"--root", f.alias}, f.alias, "repository"},
		{base, []string{"--root", f.alias + "/../repo"}, f.alias + "/../repo", "repository"},
		{base, []string{"--root", f.repo + "/"}, f.repo + "/", "repository"},
		{base, []string{"--root", "./repo"}, "./repo", "arguments"},
		{base, []string{"--root", "repo"}, "repo", "arguments"},
		{base, []string{"--root", "./alias"}, "./alias", "arguments"},
		{f.repo, []string{"--root", ""}, "", "arguments"},
		{f.repo, []string{"--root="}, "", "arguments"},
		{f.repo, nil, f.repo, ""},
		{base, []string{"--root=--repo"}, "--repo", "arguments"},
		{base, []string{"--root=-x"}, "-x", "arguments"},
		{base, []string{"--root=--map"}, "--map", "arguments"},
	} {
		t.Chdir(c.dir)
		wantExit, want := runCEMStableEntry(t, "cem", "verify-stable", "--repository", c.repository,
			"--map", filepath.Join(f.repo, "change.cem.json"), "--expected-base", f.expectedBase,
			"--target", f.target, "--artifacts", f.artifacts)
		stage, _ := want["stage"].(string)
		if c.stage != stage && (c.stage != "" || stage == "arguments" || stage == "repository" || stage == "runtime") {
			t.Fatalf("fixture does not discriminate %q: verify-stable stage %q, want %q", c.core, stage, c.stage)
		}
		args := append(append([]string{}, c.core...), "cem", "report", "--format", "json", "--map", "change.cem.json",
			"--expected-base", f.expectedBase, "--target", f.target, "--artifacts", f.artifacts)
		exit, envelope := runCEMStableEntry(t, args...)
		if exit != wantExit || !reflect.DeepEqual(envelope["stableVerification"], want) {
			t.Errorf("%q cem report in %s = exit %d %v\nverify-stable --repository %q = exit %d %v",
				c.core, c.dir, exit, envelope["stableVerification"], c.repository, wantExit, want)
		}
	}
}
