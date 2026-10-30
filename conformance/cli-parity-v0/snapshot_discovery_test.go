//go:build darwin || linux

package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

const snapshotCombinedCall = "rev-parse --path-format=absolute --git-dir --git-common-dir"
const snapshotGitDirCall = "rev-parse --path-format=absolute --git-dir"
const snapshotCommonDirCall = "rev-parse --path-format=absolute --git-common-dir"

func TestSnapshotGitDirectoriesRealRepositories(t *testing.T) {
	for _, fixture := range []struct {
		name, format string
		linked       bool
		calls        []string
	}{
		{"ordinary", "sha1", false, nil},
		{"sha256", "sha256", false, nil},
		{"spaces et café", "sha1", false, nil},
		{"newline\nparent", "sha1", false, nil},
		{"linked", "sha1", true, []string{snapshotCombinedCall}},
		{"linked newline\ncommon", "sha1", true, []string{snapshotCombinedCall, snapshotGitDirCall, snapshotCommonDirCall}},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			ctx := context.Background()
			root := filepath.Join(t.TempDir(), fixture.name)
			if err := os.Mkdir(root, 0o755); err != nil {
				t.Fatal(err)
			}
			git, err := newSanitizedGit(ctx, root)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := git.run(ctx, "init", "--quiet", "--object-format="+fixture.format); err != nil {
				t.Fatal(err)
			}
			if fixture.linked {
				if _, err := git.run(ctx, "commit", "--quiet", "--allow-empty", "-m", "fixture"); err != nil {
					t.Fatal(err)
				}
				linked := filepath.Join(t.TempDir(), "linked worktree")
				if _, err := git.run(ctx, "worktree", "add", "--quiet", "--detach", linked); err != nil {
					t.Fatal(err)
				}
				git.dir = linked
			}
			wantGit, err := git.string(ctx, "rev-parse", "--path-format=absolute", "--git-dir")
			if err != nil {
				t.Fatal(err)
			}
			wantCommon, err := git.string(ctx, "rev-parse", "--path-format=absolute", "--git-common-dir")
			if err != nil {
				t.Fatal(err)
			}
			git, directory := snapshotLoggedGit(t, git, "exec "+snapshotShellQuote(git.executable)+" \"$@\"\n")
			gotGit, gotCommon, err := snapshotGitDirectories(ctx, git)
			if err != nil || gotGit != wantGit || gotCommon != wantCommon {
				t.Fatalf("directories=(%q,%q,%v), legacy=(%q,%q)", gotGit, gotCommon, err, wantGit, wantCommon)
			}
			snapshotAssertCalls(t, directory, fixture.calls)
			if fixture.linked && gotGit == gotCommon {
				t.Fatal("linked worktree lost its distinct common directory")
			}
		})
	}
}

func TestSnapshotGitDirectoriesMalformedOutputFallsBack(t *testing.T) {
	for _, fixture := range []struct {
		name, combined string
		failed         bool
	}{
		{"empty", "", false},
		{"one", "/one\n", false},
		{"extra", "/one\n/two\n/three\n", false},
		{"blank", "/one\n \t\n", false},
		{"relative-first", "relative\n/two\n", false},
		{"relative-second", "/one\nrelative\n", false},
		{"extra-terminator", "/one\n/two\n\n", false},
		{"embedded-newline", "/one\npart\n/two\n", false},
		{"failed-combined", "/one\n/two\n", true},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			git, directory := snapshotFakeGit(t)
			snapshotWrite(t, directory, "combined", fixture.combined)
			if fixture.failed {
				snapshotWrite(t, directory, "fail-combined", "1")
			}
			gotGit, gotCommon, err := snapshotGitDirectories(context.Background(), git)
			if err != nil || gotGit != "/legacy git\ninside" || gotCommon != "/legacy café" {
				t.Fatalf("fallback directories=(%q,%q,%v)", gotGit, gotCommon, err)
			}
			snapshotAssertCalls(t, directory, []string{snapshotCombinedCall, snapshotGitDirCall, snapshotCommonDirCall})
		})
	}
}

func TestSnapshotGitDirectoriesTrimsEachAbsoluteValue(t *testing.T) {
	for _, output := range []string{
		" /git café \t\n\t/common with spaces \n",
		"/git café\n/common with spaces",
		"\u2003/git café\u2003\n\u2003/common with spaces\u2003\n",
		"/git café\r\n/common with spaces\r\n",
	} {
		t.Run(strconv.Quote(output), func(t *testing.T) {
			git, directory := snapshotFakeGit(t)
			snapshotWrite(t, directory, "combined", output)
			gotGit, gotCommon, err := snapshotGitDirectories(context.Background(), git)
			if err != nil || gotGit != "/git café" || gotCommon != "/common with spaces" {
				t.Fatalf("trimmed directories=(%q,%q,%v)", gotGit, gotCommon, err)
			}
			snapshotAssertCalls(t, directory, []string{snapshotCombinedCall})
		})
	}
}

func TestSnapshotGitDirectoriesPreservesLegacyErrors(t *testing.T) {
	for _, fixture := range []struct {
		stage          string
		combinedFailed bool
		newlineRoot    bool
	}{
		{"git-dir", false, false}, {"common-dir", false, false},
		{"git-dir", true, false}, {"common-dir", true, false},
		{"git-dir", false, true}, {"common-dir", false, true},
	} {
		t.Run(fixture.stage+"/combined-failed="+strconv.FormatBool(fixture.combinedFailed)+"/newline-root="+strconv.FormatBool(fixture.newlineRoot), func(t *testing.T) {
			stage := fixture.stage
			git, directory := snapshotFakeGit(t)
			if fixture.newlineRoot {
				git = snapshotNewlineGitRoot(t, git)
			}
			snapshotWrite(t, directory, "fail-"+stage, "1")
			if fixture.combinedFailed {
				snapshotWrite(t, directory, "fail-combined", "1")
			}
			// A first legacy error must win even when the second would also fail.
			if stage == "git-dir" {
				snapshotWrite(t, directory, "fail-common-dir", "1")
			}
			_, _, err := snapshotGitDirectories(context.Background(), git)
			operation := snapshotCommonDirCall
			if stage == "git-dir" {
				operation = snapshotGitDirCall
			}
			want := "git " + operation + ": exit status 17: stderr=\"legacy " + stage + " failure\\n\""
			if err == nil || err.Error() != want {
				t.Fatalf("legacy error=%v, want %s", err, want)
			}
			wantCalls := []string{snapshotCombinedCall, snapshotGitDirCall}
			if fixture.newlineRoot {
				wantCalls = []string{snapshotGitDirCall}
			}
			if stage == "common-dir" {
				wantCalls = append(wantCalls, snapshotCommonDirCall)
			}
			snapshotAssertCalls(t, directory, wantCalls)
		})
	}
}

func TestSnapshotGitDirectoriesDeadlineCleansChild(t *testing.T) {
	for _, fixture := range []struct {
		stage       string
		newlineRoot bool
	}{
		{"combined", false}, {"git-dir", false}, {"git-dir", true},
	} {
		t.Run(fixture.stage+"/newline-root="+strconv.FormatBool(fixture.newlineRoot), func(t *testing.T) {
			// The 2 s caller deadline is the behavior under test; the 60 s window
			// is a hang detector (decision 0082). An attempt whose fake Git never
			// showed a live child before that deadline proves nothing, so only
			// that unmet precondition starts a fresh fixture.
			end := time.Now().Add(60 * time.Second)
			for attempt := 1; !snapshotDeadlineCleansChild(t, fixture.stage, fixture.newlineRoot); attempt++ {
				if time.Now().After(end) {
					t.Fatalf("fake Git showed no live child before the deadline in %d attempts over 60s", attempt)
				}
			}
		})
	}
}

// snapshotDeadlineCleansChild reports false only when the fake Git child was
// not observed live before the caller deadline expired.
func snapshotDeadlineCleansChild(t *testing.T, stage string, newlineRoot bool) bool {
	t.Helper()
	git, directory := snapshotFakeGit(t)
	if newlineRoot {
		git = snapshotNewlineGitRoot(t, git)
	}
	snapshotWrite(t, directory, "sleep-"+stage, "1")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	t.Cleanup(cancel)
	done := make(chan error, 1)
	started := time.Now()
	go func() {
		_, _, err := snapshotGitDirectories(ctx, git)
		done <- err
	}()
	t.Cleanup(func() {
		cancel()
		select {
		case <-done:
		case <-time.After(4 * time.Second):
			t.Error("discovery did not stop during cleanup")
		}
	})
	pid, published := snapshotWaitChild(t, ctx, directory)
	if published {
		if err := syscall.Kill(pid, 0); err != nil {
			if ctx.Err() == nil {
				t.Fatalf("child was not live before deadline: %v", err)
			}
			published = false
		}
	}
	select {
	case err := <-done:
		done <- err // Leave the completion available to cleanup.
		if !published {
			return false
		}
		if ctx.Err() != context.DeadlineExceeded || err == nil {
			t.Fatalf("deadline error=%v", err)
		}
		if !errors.Is(err, context.DeadlineExceeded) && !strings.Contains(err.Error(), "process-cancelled") {
			t.Fatalf("unexpected adapter cancellation error=%v", err)
		}
	case <-time.After(4 * time.Second):
		t.Fatal("discovery exceeded caller deadline and bounded shutdown")
	}
	if elapsed := time.Since(started); elapsed > 4*time.Second {
		t.Fatalf("discovery took %s", elapsed)
	}
	if err := syscall.Kill(pid, 0); !errors.Is(err, syscall.ESRCH) {
		t.Fatalf("child remains after discovery: pid=%d error=%v", pid, err)
	}
	wantCalls := []string{snapshotCombinedCall}
	if stage == "git-dir" {
		wantCalls = append(wantCalls, snapshotGitDirCall)
	}
	if newlineRoot {
		wantCalls = []string{snapshotGitDirCall}
	}
	snapshotAssertCalls(t, directory, wantCalls)
	return true
}

func snapshotNewlineGitRoot(t *testing.T, git sanitizedGit) sanitizedGit {
	t.Helper()
	git.dir = filepath.Join(git.dir, "newline\nroot")
	if err := os.Mkdir(git.dir, 0o700); err != nil {
		t.Fatal(err)
	}
	return git
}

func snapshotFakeGit(t *testing.T) (sanitizedGit, string) {
	t.Helper()
	git, err := newSanitizedGit(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	git, directory := snapshotLoggedGit(t, git, `stage=combined
if [ "$#" = 3 ]; then
  case "$3" in
    --git-dir) stage=git-dir ;;
    --git-common-dir) stage=common-dir ;;
    *) exit 99 ;;
  esac
fi
if [ -f "$discovery/sleep-$stage" ]; then
  child=
  cleanup() {
    trap - EXIT INT TERM
    if [ -n "$child" ]; then
      kill "$child" 2>/dev/null || :
      wait "$child" 2>/dev/null || :
    fi
  }
  trap cleanup EXIT INT TERM
  sleep 30 &
  child=$!
  # Publish by rename so a reader never sees the redirect's empty file.
  printf '%s\n' "$child" > "$discovery/child.tmp"
  mv "$discovery/child.tmp" "$discovery/child"
  wait "$child"
  exit 0
fi
if [ -f "$discovery/fail-$stage" ]; then
  printf 'legacy %s failure\n' "$stage" >&2
  exit 17
fi
cat "$discovery/$stage"
`)
	snapshotWrite(t, directory, "combined", "ambiguous\n")
	snapshotWrite(t, directory, "git-dir", " \t/legacy git\ninside \n")
	snapshotWrite(t, directory, "common-dir", " /legacy café \n")
	return git, directory
}

func snapshotLoggedGit(t *testing.T, git sanitizedGit, body string) (sanitizedGit, string) {
	t.Helper()
	directory := t.TempDir()
	script := "#!/bin/sh\ndiscovery=" + snapshotShellQuote(directory) + "\nprintf '%s\\n' \"$*\" >> \"$discovery/calls\"\n" + body
	path := filepath.Join(directory, "git")
	if err := os.WriteFile(path, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	git.executable = path
	return git, directory
}

func snapshotShellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

func snapshotWrite(t *testing.T, directory, name, value string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(directory, name), []byte(value), 0o600); err != nil {
		t.Fatal(err)
	}
}

func snapshotAssertCalls(t *testing.T, directory string, want []string) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(directory, "calls"))
	if errors.Is(err, os.ErrNotExist) && len(want) == 0 {
		return
	}
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n")
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Git launches=%q, want %q", got, want)
	}
}

func snapshotWaitChild(t *testing.T, ctx context.Context, directory string) (int, bool) {
	t.Helper()
	for ctx.Err() == nil {
		raw, err := os.ReadFile(filepath.Join(directory, "child"))
		if err == nil {
			pid, err := strconv.Atoi(strings.TrimSpace(string(raw)))
			if err != nil || pid <= 0 {
				t.Fatalf("invalid child PID %q: %v", raw, err)
			}
			return pid, true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return 0, false
}
