// SPDX-License-Identifier: Apache-2.0

package appmap

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/procgroup"
)

// AMAP-V0-8780: process failures are not evidence that a revision is invalid, and Git's verified
// "no such commit" (rev-parse --quiet --verify exit 1) is not a source failure.
func TestResolveRetainsGitFailureClass(t *testing.T) {
	saved := runGit
	t.Cleanup(func() { runGit = saved })
	for _, test := range []struct {
		name        string
		observation procgroup.Observation
		code, want  string
	}{
		{"exit-128", procgroup.Observation{ExitStatus: 128, Stderr: []byte("fatal: private diagnostic")}, "appmap-source-unavailable", "application map Git source unavailable: exit status 128"},
		{"exit-129", procgroup.Observation{ExitStatus: 129}, "appmap-source-unavailable", "application map Git source unavailable: exit status 129"},
		{"start", procgroup.Observation{ExitStatus: -1, Err: errors.New("process-start-failed")}, "appmap-source-unavailable", "application map Git source unavailable: process-start-failed"},
		{"timeout", procgroup.Observation{Err: context.DeadlineExceeded, TimedOut: true}, "appmap-source-unavailable", "application map Git source unavailable: context deadline exceeded"},
		{"cancelled", procgroup.Observation{Err: context.Canceled, Cancelled: true}, "appmap-source-unavailable", "application map Git source unavailable: context canceled"},
		{"overflow", procgroup.Observation{Err: errors.New("process-output-overflow"), OutputOverflow: true}, "appmap-source-unavailable", "application map Git source unavailable: process-output-overflow"},
		{"platform", procgroup.Observation{Err: procgroup.ErrPlatformUnsupported}, "process-platform-unsupported", "application map Git source unavailable: process-platform-unsupported"},
	} {
		t.Run(test.name, func(t *testing.T) {
			runGit = func(context.Context, procgroup.Spec) procgroup.Observation { return test.observation }
			r := repo{ctx: t.Context(), root: t.TempDir()}
			revision, err := r.resolve("HEAD")
			if revision != "" || err == nil || err.Error() != test.want || codeOf(err) != test.code {
				t.Fatalf("resolve = %q, %v (code %q); want %q with code %q", revision, err, codeOf(err), test.want, test.code)
			}
			if test.observation.Err != nil && test.name != "platform" && !errors.Is(err, test.observation.Err) {
				t.Fatalf("lost process cause: %v", err)
			}
			if test.name != "platform" {
				_, other := r.git(4096, nil, "status")
				if other == nil || other.Error() != "application map Git source unavailable" || codeOf(other) != "" {
					t.Fatalf("unrelated Git reader diagnostic changed: %v", other)
				}
			}
		})
	}
	// Exit 1 without a process error is Git's verified answer that no such commit exists.
	runGit = func(context.Context, procgroup.Spec) procgroup.Observation {
		return procgroup.Observation{ExitStatus: 1}
	}
	r := repo{ctx: t.Context(), root: t.TempDir()}
	if revision, err := r.resolve("refs/heads/missing"); revision != "" || !errors.Is(err, errRevisionNotCommit) || codeOf(err) != "" {
		t.Fatalf("verified missing revision = %q, %v; want errRevisionNotCommit", revision, err)
	}
	// Exit 128 from the lookup is a revision defect only when the repository probe
	// (rev-parse --git-dir) exits 0; the exit-128 row above, whose probe also exits 128, stays a
	// source failure. A probe that cannot run (timeout) keeps it a source failure too.
	for _, probe := range []struct {
		name        string
		observation procgroup.Observation
		invalid     bool
	}{
		{"readable", procgroup.Observation{Stdout: []byte(".git\n")}, true},
		{"unreadable", procgroup.Observation{ExitStatus: 128}, false},
		{"timeout", procgroup.Observation{Err: context.DeadlineExceeded, TimedOut: true}, false},
	} {
		var probed bool
		runGit = func(_ context.Context, spec procgroup.Spec) procgroup.Observation {
			if slices.Contains(spec.Argv, "--git-dir") {
				probed = true
				return probe.observation
			}
			return procgroup.Observation{ExitStatus: 128, Stderr: []byte("fatal: log for 'HEAD' only has 1 entries")}
		}
		revision, err := r.resolve("HEAD@{1}")
		if !probed {
			t.Fatalf("%s: exit 128 did not probe the repository", probe.name)
		}
		if probe.invalid && (revision != "" || !errors.Is(err, errRevisionNotCommit) || codeOf(err) != "") {
			t.Fatalf("%s: exit 128 in a readable repository = %q, %v; want errRevisionNotCommit", probe.name, revision, err)
		}
		if !probe.invalid && (revision != "" || codeOf(err) != "appmap-source-unavailable" || err.Error() != "application map Git source unavailable: exit status 128") {
			t.Fatalf("%s: exit 128 without a readable repository = %q, %v (code %q)", probe.name, revision, err, codeOf(err))
		}
	}
	var argv []string
	runGit = func(_ context.Context, spec procgroup.Spec) procgroup.Observation {
		argv = spec.Argv
		return procgroup.Observation{Stdout: []byte(strings.Repeat("a", 40) + "\n")}
	}
	if revision, err := r.resolve("HEAD"); err != nil || revision != strings.Repeat("a", 40) {
		t.Fatalf("successful resolution = %q, %v", revision, err)
	}
	if !slices.Contains(argv, "--quiet") || !slices.Contains(argv, "--verify") || argv[len(argv)-1] != "HEAD^{commit}" {
		t.Fatalf("resolve argv %q must verify quietly so exit 1 alone means no such commit", argv)
	}
	for _, revision := range []string{"", "--bad"} {
		if got, err := r.resolve(revision); got != "" || err == nil || err.Error() != "invalid revision" {
			t.Fatalf("invalid input %q = %q, %v", revision, got, err)
		}
	}
}

// foreignRepo is an empty repository: Git reads it and verifies that no map revision is a commit
// there (exit 1), unlike a non-repository directory, which Git cannot read (AMAP-V0-8780).
func foreignRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	gitTest(t, root, "init", "-q")
	return root
}

// stubRevisionGit answers the worktree probe, resolves ok, and observes failure for every other
// Git read, so each caller reaches its revision lookup with the injected outcome.
func stubRevisionGit(t *testing.T, ok string, failure procgroup.Observation) {
	t.Helper()
	saved := runGit
	t.Cleanup(func() { runGit = saved })
	runGit = func(_ context.Context, spec procgroup.Spec) procgroup.Observation {
		switch {
		case slices.Contains(spec.Argv, "--is-inside-work-tree"):
			return procgroup.Observation{Stdout: []byte("true\n\n")}
		case slices.Contains(spec.Argv, "--verify") && spec.Argv[len(spec.Argv)-1] == ok+"^{commit}":
			return procgroup.Observation{Stdout: []byte(strings.Repeat("a", 40) + "\n")}
		}
		return failure
	}
}

// AMAP-V0-8780: every production caller of a revision lookup surfaces the retained Git failure
// class under appmap-source-unavailable, and keeps its own refusal for a verified missing commit.
func TestRevisionCallersSurfaceGitSourceFailure(t *testing.T) {
	ctx := context.Background()
	root, other := t.TempDir(), t.TempDir()
	broken := &Map{Revision: "broken"}
	callers := []struct {
		name, ok, fallback string
		call               func() error
	}{
		{"build", "", "appmap-invalid-revision", func() error { _, err := Build(ctx, root, "appmap.json", "broken"); return err }},
		{"build-from", "", "appmap-invalid-revision", func() error {
			_, _, err := BuildFrom(ctx, root, "appmap.json", "broken", Roots{}, &Map{})
			return err
		}},
		{"impact-head", "", "appmap-invalid-query", func() error {
			_, err := Impact(ctx, &Map{}, nil, "HEAD", Options{Root: root, Revision: "broken"})
			return err
		}},
		{"impact-base", "HEAD", "appmap-invalid-query", func() error { _, err := Impact(ctx, &Map{}, nil, "broken", Options{Root: root}); return err }},
		{"root-stated", "", "appmap-root-unavailable", func() error { _, _, err := openRoot(ctx, root, "app", other, "broken"); return err }},
		{"root-head", "", "appmap-root-unavailable", func() error { _, _, err := openRoot(ctx, root, "app", other, ""); return err }},
		{"branches", "", "appmap-invalid-revision", func() error {
			_, err := QueryBranches(ctx, broken, []string{"app/models/user.rb:1"}, Options{Root: root})
			return err
		}},
		{"actions", "", "appmap-invalid-revision", func() error { _, err := CheckActions(ctx, broken, Options{Root: root}); return err }},
		{"blocker", "", "appmap-invalid-revision", func() error {
			_, err := CheckBlocker(ctx, broken, "404 GET /orders", Options{Root: root})
			return err
		}},
		{"page-objects", "", "appmap-invalid-revision", func() error {
			_, err := PageObjects(ctx, broken, []string{"e2e"}, Options{Root: root})
			return err
		}},
	}
	for _, failure := range []struct {
		name        string
		observation procgroup.Observation
		class       string
		cause       error
	}{
		{"exit-128", procgroup.Observation{ExitStatus: 128, Stderr: []byte("fatal: private diagnostic")}, "exit status 128", nil},
		{"timeout", procgroup.Observation{Err: context.DeadlineExceeded, TimedOut: true}, "context deadline exceeded", context.DeadlineExceeded},
	} {
		for _, caller := range callers {
			t.Run(failure.name+"/"+caller.name, func(t *testing.T) {
				stubRevisionGit(t, caller.ok, failure.observation)
				err := caller.call()
				if codeOf(err) != "appmap-source-unavailable" || err.Error() != "application map Git source unavailable: "+failure.class {
					t.Fatalf("want appmap-source-unavailable naming %q, got code %q: %v", failure.class, codeOf(err), err)
				}
				if failure.cause != nil && !errors.Is(err, failure.cause) {
					t.Fatalf("lost process cause %v: %v", failure.cause, err)
				}
			})
		}
	}
	for _, caller := range callers {
		t.Run("verified-missing/"+caller.name, func(t *testing.T) {
			stubRevisionGit(t, caller.ok, procgroup.Observation{ExitStatus: 1})
			err := caller.call()
			if codeOf(err) != caller.fallback || strings.Contains(err.Error(), "source unavailable") {
				t.Fatalf("verified missing commit: want %s, got code %q: %v", caller.fallback, codeOf(err), err)
			}
		})
	}
}

// AMAP-V0-8780: exercise real Git. A missing revision is a verified exit 1 and stays an invalid
// revision; a repository Git cannot read is a source failure. The bounded stderr goes to the test
// log only: run this with -v on hosted macos-15 to diagnose V1-4838; a Linux result is not
// evidence for the hosted failure in run 41968984800.
func TestResolveHostGitDiagnostics(t *testing.T) {
	root, head := fixtureRepo(t)
	saved := runGit
	t.Cleanup(func() { runGit = saved })
	runGit = func(ctx context.Context, spec procgroup.Spec) procgroup.Observation {
		observation := saved(ctx, spec)
		if observation.Err != nil || observation.ExitStatus != 0 {
			t.Logf("git argv=%q exit=%d err=%v timed_out=%t cancelled=%t overflow=%t stderr=%q",
				spec.Argv, observation.ExitStatus, observation.Err, observation.TimedOut,
				observation.Cancelled, observation.OutputOverflow, observation.Stderr)
		}
		return observation
	}
	ctx := t.Context()
	r := repo{ctx: ctx, root: root}
	if got, err := r.resolve("HEAD"); err != nil || got != head {
		t.Fatalf("fixture HEAD = %q, %v; want %s", got, err, head)
	}
	if got, err := r.resolve("refs/heads/definitely-missing-fixture-ref"); got != "" || !errors.Is(err, errRevisionNotCommit) {
		t.Fatalf("missing revision = %q, %v; want errRevisionNotCommit", got, err)
	}
	if _, err := Build(ctx, root, "appmap.json", "refs/heads/definitely-missing-fixture-ref"); codeOf(err) != "appmap-invalid-revision" || err.Error() != "evaluated revision is not a commit" {
		t.Fatalf("Build of a missing revision = code %q: %v", codeOf(err), err)
	}
	// A malformed repository config makes Git exit 128 after it has found this repository, so no
	// enclosing directory can answer instead.
	unreadable := t.TempDir()
	gitTest(t, unreadable, "init", "-q")
	if err := os.WriteFile(filepath.Join(unreadable, ".git", "config"), []byte("[core\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got, err := (repo{ctx: ctx, root: unreadable}).resolve("HEAD"); got != "" || codeOf(err) != "appmap-source-unavailable" || err.Error() != "application map Git source unavailable: exit status 128" {
		t.Fatalf("unreadable repository = %q, %v (code %q)", got, err, codeOf(err))
	}
	if _, err := Build(ctx, unreadable, "appmap.json", "HEAD"); codeOf(err) != "appmap-source-unavailable" || strings.Contains(err.Error(), "not a commit") {
		t.Fatalf("Build of an unreadable repository = code %q: %v", codeOf(err), err)
	}
	// Git exits 128 for a reflog entry a one-commit repository lacks and for @{u} with no upstream
	// configured. The repository is readable, so both are invalid revisions, never source failures.
	single := t.TempDir()
	gitTest(t, single, "init", "-q")
	if err := os.WriteFile(filepath.Join(single, "a.txt"), []byte("a\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	commitAll(t, single, "one")
	for _, revision := range []string{"HEAD@{1}", "HEAD@{5}", "@{u}"} {
		if got, err := (repo{ctx: ctx, root: single}).resolve(revision); got != "" || !errors.Is(err, errRevisionNotCommit) || codeOf(err) != "" {
			t.Fatalf("%s in a one-commit repository = %q, %v (code %q); want errRevisionNotCommit", revision, got, err, codeOf(err))
		}
		if _, err := Build(ctx, single, "appmap.json", revision); codeOf(err) != "appmap-invalid-revision" || err.Error() != "evaluated revision is not a commit" {
			t.Fatalf("Build of %s = code %q: %v", revision, codeOf(err), err)
		}
	}
	// A plain directory outside every repository: Git cannot read it, so it stays a source failure.
	plain := t.TempDir()
	if probe := exec.Command("git", "-C", plain, "rev-parse", "--git-dir"); probe.Run() == nil {
		t.Skipf("%s lies inside a Git repository; the plain-directory row needs a temporary directory outside one", plain)
	}
	if got, err := (repo{ctx: ctx, root: plain}).resolve("HEAD"); got != "" || codeOf(err) != "appmap-source-unavailable" || err.Error() != "application map Git source unavailable: exit status 128" {
		t.Fatalf("plain directory = %q, %v (code %q)", got, err, codeOf(err))
	}
}
