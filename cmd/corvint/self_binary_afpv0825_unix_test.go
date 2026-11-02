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
	"time"

	"github.com/Beamfall/corvint/internal/execfile"
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
	if err := execfile.WriteFile(filepath.Join(directory, "git"), []byte(shim), 0o755); err != nil {
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
		own := binaryStaleAgainst(ctx, root, info, nil)
		if shared := binaryStaleAgainst(ctx, root, info, &headRead{revision: head}); shared != own || own.Code != state.code {
			t.Fatalf("%s: the plan's HEAD judged %+v, its own read %+v, want code %q", state.name, shared, own, state.code)
		}
		planCode, _, plan := dryRun(unstamped)
		code, stderr, spawned := dryRun(info)
		if code != planCode || code != 3 {
			t.Fatalf("%s: test --dry-run exited %d, %d without a stamp: %s", state.name, code, planCode, stderr)
		}
		line := "corvint-test: DEGRADED " + own.Code + ": " + selfBinaryNotice(own) + "\n"
		if (own.Code == "") == strings.Contains(stderr, "corvint-test: DEGRADED ") || own.Code != "" && !strings.Contains(stderr, line) {
			t.Fatalf("%s: stderr does not carry exactly the verdict %q:\n%s", state.name, line, stderr)
		}
		if len(spawned) != len(plan)+state.extra {
			t.Fatalf("%s: the check spawned %d Git processes beyond the plan's %d, want %d:\n%s", state.name, len(spawned)-len(plan), len(plan), state.extra, strings.Join(spawned, "\n"))
		}
		diffs := 0
		for _, arguments := range spawned {
			if strings.Contains(arguments, " diff --name-only -z --no-renames "+state.commit+" "+head+" -- :(glob)**/*.go") {
				diffs++
			}
		}
		if diffs != state.extra {
			t.Fatalf("%s: the check spawned %d diffs, want %d:\n%s", state.name, diffs, state.extra, strings.Join(spawned, "\n"))
		}
	}
}

// AFP-V0-825: the check judges the binary against the revision the plan read,
// not against whatever HEAD has moved to since. A binary built at A, read
// against a plan revision B that changed Go source, is stale even after HEAD
// moves back to A; a read that failed or used up the check's deadline is
// unverified even when it names the binary's own commit.
func TestAFPV0825StaleCheckJudgesTheReadRevision(t *testing.T) {
	ctx := context.Background()
	root, built := selfBinaryRepository(t)
	info := selfBinaryInfo("example.test/self", built)
	commitSelfBinaryFiles(t, root, map[string]string{"self.go": "package self\n\nfunc Self() int { return 2 }\n"})
	planned := strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
	affectedGit(t, root, "checkout", "--quiet", "--detach", built)
	if freshness := binaryStaleAgainst(ctx, root, info, &headRead{revision: planned}); freshness.Code != selfBinaryStaleCode || !strings.Contains(freshness.Detail, "1 Go source paths differ at HEAD: self.go;") {
		t.Fatalf("a binary built at %s, judged against the plan's %s after HEAD moved back, is %+v", built, planned, freshness)
	}
	for name, read := range map[string]*headRead{
		"failed read":         {},
		"read at the bound":   {revision: built, elapsed: selfBinaryDiffDeadline},
		"read past the bound": {revision: planned, elapsed: selfBinaryDiffDeadline + time.Second},
	} {
		if freshness := binaryStaleAgainst(ctx, root, info, read); freshness.Code != selfBinaryUnverifiedCode || !strings.Contains(freshness.Detail, "built from commit "+shortRevision(built)+", which could not be compared with HEAD") {
			t.Errorf("%s: judged %+v, want %s", name, freshness, selfBinaryUnverifiedCode)
		}
	}
}

// firstHeadReadShim puts a Git shim first on PATH that runs action before the
// first `rev-parse --verify --quiet HEAD^{commit}` after each arm call, and
// otherwise runs Git. The hermetic Git environment carries no PATH, so action
// names its commands absolutely.
func firstHeadReadShim(t *testing.T, action string) (arm func()) {
	t.Helper()
	gitExecutable, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	mark := filepath.Join(directory, "read")
	shim := "#!/bin/sh\ncase \"$*\" in\n*\"rev-parse --verify --quiet HEAD^{commit}\")\n  if [ ! -e '" + mark + "' ]; then : > '" + mark + "'; " + action + "; fi;;\nesac\nexec '" + gitExecutable + "' \"$@\"\n"
	if err := execfile.WriteFile(filepath.Join(directory, "git"), []byte(shim), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", directory+string(os.PathListSeparator)+os.Getenv("PATH"))
	return func() {
		if err := os.Remove(mark); err != nil && !errors.Is(err, os.ErrNotExist) {
			t.Fatal(err)
		}
	}
}

// AFP-V0-825: when the shared HEAD read fails once or outlasts the check's
// own bound, `corvint test` reports the binary unverified and still captures
// its source, exactly as its own failed or expired read did before.
func TestAFPV0825FailedOrSlowHEADReadIsUnverified(t *testing.T) {
	for _, read := range []struct{ name, action string }{
		{"failed once", "exit 1"},
		{"slower than the bound", "/bin/sleep 3 </dev/null >/dev/null 2>&1"},
	} {
		t.Run(read.name, func(t *testing.T) {
			root, built := selfBinaryRepository(t)
			useAffectedUserCacheDir(t, "", errors.New("no cache in this run"))
			arm := firstHeadReadShim(t, read.action)
			stubSelfBuildInfo(t, selfBinaryInfo("example.test/self", built))
			arm()
			var stdout, stderr bytes.Buffer
			code := runContext(context.Background(), []string{"--root", root, "test", "--base", built, "--dry-run"}, strings.NewReader(""), &stdout, &stderr)
			line := "corvint-test: DEGRADED " + selfBinaryUnverifiedCode + ": this binary was built from commit " + shortRevision(built) + ", which could not be compared with HEAD here"
			if code != 3 || !strings.Contains(stderr.String(), line) {
				t.Fatalf("test --dry-run exited %d, want 3 with %q:\n%s", code, line, stderr.String())
			}
		})
	}
}
