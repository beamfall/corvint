//go:build darwin || linux

package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"strings"
	"testing"
)

// AFP-V0-825: corvint test judges its own binary against the HEAD it resolved
// for the plan. For a fresh, a stale and an unverified binary the verdict and
// its stderr line equal those of the check that resolves HEAD itself, and the
// check adds no Git process to the plan's own beyond the diff a binary built
// from another commit needs.
func TestAFPV0825StaleCheckReusesThePlanHEAD(t *testing.T) {
	root, built := selfBinaryRepository(t)
	useAffectedUserCacheDir(t, "", errors.New("no cache in this run"))
	gitExecutable, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	argv := filepath.Join(directory, "argv.log")
	shim := "#!/bin/sh\nprintf '%s\\n' \"$*\" >> '" + argv + "'\nexec '" + gitExecutable + "' \"$@\"\n"
	if err := os.WriteFile(filepath.Join(directory, "git"), []byte(shim), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", directory+string(os.PathListSeparator)+os.Getenv("PATH"))
	module := "example.test/self"
	// dryRun returns the exit code and stderr of one `test --dry-run` and the
	// Git processes it spawned, with this binary claiming info as its build.
	// The fixture declares no full scope, so its plan is incomplete (exit 3).
	dryRun := func(info *debug.BuildInfo) (int, string, []string) {
		stubSelfBuildInfo(t, info)
		if err := os.WriteFile(argv, nil, 0o644); err != nil {
			t.Fatal(err)
		}
		var stdout, stderr bytes.Buffer
		code := runContext(context.Background(), []string{"--root", root, "test", "--base", built, "--dry-run"}, strings.NewReader(""), &stdout, &stderr)
		raw, err := os.ReadFile(argv)
		if err != nil {
			t.Fatal(err)
		}
		return code, stderr.String(), strings.Split(strings.TrimSpace(string(raw)), "\n")
	}
	unstamped := &debug.BuildInfo{Main: debug.Module{Path: module}}
	missing := strings.Repeat("ab", 20)
	for _, state := range []struct {
		name, commit, code string
		extra              int
	}{
		{"fresh", built, "", 0},
		{"stale", built, selfBinaryStaleCode, 1},
		{"unverified", missing, selfBinaryUnverifiedCode, 1},
	} {
		if state.name == "stale" {
			commitSelfBinaryFiles(t, root, map[string]string{"self.go": "package self\n\nfunc Self() int { return 2 }\n"})
		}
		info := selfBinaryInfo(module, state.commit)
		ctx := context.Background()
		head := strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
		own := binaryStaleAgainst(ctx, root, info, "")
		if shared := binaryStaleAgainst(ctx, root, info, head); shared != own || own.Code != state.code {
			t.Fatalf("%s: the plan's HEAD judged %+v, its own read %+v, want code %q", state.name, shared, own, state.code)
		}
		planCode, _, plan := dryRun(unstamped)
		code, stderr, spawned := dryRun(info)
		if code != planCode || code != 3 {
			t.Fatalf("%s: test --dry-run exited %d, %d without a stamp: %s", state.name, code, planCode, stderr)
		}
		line := "corvint-test: DEGRADED " + own.Code + ": " + own.Detail + "\n"
		if (own.Code == "") == strings.Contains(stderr, "corvint-test: DEGRADED ") || own.Code != "" && !strings.Contains(stderr, line) {
			t.Fatalf("%s: stderr does not carry exactly the verdict %q:\n%s", state.name, line, stderr)
		}
		if len(spawned) != len(plan)+state.extra {
			t.Fatalf("%s: the check spawned %d Git processes beyond the plan's %d, want %d:\n%s", state.name, len(spawned)-len(plan), len(plan), state.extra, strings.Join(spawned, "\n"))
		}
		diffs := 0
		for _, arguments := range spawned {
			if strings.Contains(arguments, " diff --name-only -z --no-renames "+state.commit+" HEAD -- :(glob)**/*.go") {
				diffs++
			}
		}
		if diffs != state.extra {
			t.Fatalf("%s: the check spawned %d diffs, want %d:\n%s", state.name, diffs, state.extra, strings.Join(spawned, "\n"))
		}
	}
}
