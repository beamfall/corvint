package affected_test

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
)

// observationRepository builds a small module with a committed baseline and a
// dirty worktree that exercises every record shape the decoder handles: a
// modification, an addition, and a rename.
func observationRepository(t *testing.T) (string, string) {
	t.Helper()
	gitExecutable, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git is unavailable")
	}
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	write := func(relative, body string) {
		t.Helper()
		full := filepath.Join(root, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	git := func(argv ...string) {
		t.Helper()
		command := exec.Command(gitExecutable, append([]string{"-C", root}, argv...)...)
		command.Env = append(os.Environ(),
			"GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null",
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.test",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.test",
		)
		if output, err := command.CombinedOutput(); err != nil {
			t.Skipf("git %v is unavailable here: %v: %s", argv, err, output)
		}
	}
	write("go.mod", "module example.test/observed\n\ngo 1.27.0\n")
	write("core/core.go", "package core\n\nfunc Value() int { return 1 }\n")
	write("core/core_test.go", "package core\n\nimport \"testing\"\n\nfunc TestValue(t *testing.T) { _ = Value() }\n")
	write("client/client.go", "package client\n\nimport \"example.test/observed/core\"\n\nfunc Use() int { return core.Value() }\n")
	write("client/client_test.go", "package client\n\nimport \"testing\"\n\nfunc TestUse(t *testing.T) { _ = Use() }\n")
	write("core/moved.go", "package core\n\nfunc Moved() {}\n")
	git("init", "--quiet")
	git("add", "go.mod", "core", "client")
	git("commit", "--quiet", "-m", "baseline")

	write("core/core.go", "package core\n\nfunc Value() int { return 2 }\n")
	write("client/added.go", "package client\n\nfunc Added() {}\n")
	git("mv", "core/moved.go", "core/renamed.go")
	return gitExecutable, root
}

// TestPublishedObservationAndOwnCaptureProduceTheSamePlan is the consumption
// half of the change: the selector must not care whether the dirty set arrived
// from an authority that already observed the worktree or from its own capture.
// If those two paths could disagree, publishing the list would have bought a
// second source of truth rather than removed one.
func TestPublishedObservationAndOwnCaptureProduceTheSamePlan(t *testing.T) {
	gitExecutable, root := observationRepository(t)
	graph, err := affected.Build(root, golang.New())
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	// What an authority publishes: the decode of the exact status bytes it
	// hashed. Nothing here observes the worktree a second time.
	raw := captureStatus(t, gitExecutable, root)
	published, err := affected.DecodeStatus(raw)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	fromPublished, err := affected.DirtyPathsFor(context.Background(), gitExecutable, root, &affected.Observation{Paths: published})
	if err != nil {
		t.Fatalf("published: %v", err)
	}

	start := time.Now()
	fromCapture, err := affected.DirtyPathsFor(context.Background(), gitExecutable, root, nil)
	captureCost := time.Since(start)
	if err != nil {
		t.Fatalf("fallback: %v", err)
	}
	t.Logf("fallback capture cost removed when an observation is available: %s", captureCost.Round(time.Microsecond))

	if len(fromPublished) == 0 {
		t.Fatal("fixture produced no dirty paths")
	}
	planFromPublished, err := affected.Select(graph, fromPublished).Canonical()
	if err != nil {
		t.Fatal(err)
	}
	planFromCapture, err := affected.Select(graph, fromCapture).Canonical()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(planFromPublished, planFromCapture) {
		t.Fatalf("plans differ:\npublished=%s\ncapture=  %s", planFromPublished, planFromCapture)
	}
	// The rename must reach both endpoints through either path, so the unit that
	// lost the file is selected too.
	for _, want := range []string{"core/moved.go", "core/renamed.go", "core/core.go", "client/added.go"} {
		if !contains(fromPublished, want) {
			t.Fatalf("published dirty set %v omits %s", fromPublished, want)
		}
	}
}

// TestNilObservationFallsBackToACapture keeps the standalone path working: an
// absent observation is not an empty one.
func TestNilObservationFallsBackToACapture(t *testing.T) {
	gitExecutable, root := observationRepository(t)
	dirty, err := affected.DirtyPathsFor(context.Background(), gitExecutable, root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(dirty) == 0 {
		t.Fatal("nil observation captured nothing on a dirty worktree")
	}
	clean, err := affected.DirtyPathsFor(context.Background(), gitExecutable, root, &affected.Observation{})
	if err != nil {
		t.Fatal(err)
	}
	if len(clean) != 0 {
		t.Fatalf("an observation of a clean worktree was re-captured: %v", clean)
	}
}

func captureStatus(t *testing.T, gitExecutable, root string) []byte {
	t.Helper()
	command := exec.Command(gitExecutable,
		"--no-optional-locks", "-c", "core.fsmonitor=false", "-c", "core.untrackedCache=false",
		"-C", root, "status", "--porcelain=v1", "-z", "--untracked-files=all", "--ignored=no",
	)
	command.Env = []string{
		"GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null",
		"GIT_TERMINAL_PROMPT=0", "GIT_OPTIONAL_LOCKS=0", "LANG=C", "LC_ALL=C",
	}
	raw, err := command.Output()
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	return raw
}

// TestDirtyNonUTF8PathIsDisclosedNotRefused is V1-0314: a committed path whose
// Git bytes are Latin-1 and that is changed in the worktree enters the dirty
// set in its U+FFFD display form (IDX-SNAP-V0-024's model), and a committed
// range naming it decodes the same way, rather than the capture refusing as
// malformed. The path is committed through plumbing and absent from the
// worktree, because some filesystems refuse non-UTF-8 names; Git then reports
// it deleted, which is a worktree change like any other.
func TestDirtyNonUTF8PathIsDisclosedNotRefused(t *testing.T) {
	gitExecutable, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git is unavailable")
	}
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	git := func(stdin string, argv ...string) string {
		t.Helper()
		command := exec.Command(gitExecutable, append([]string{"-C", root}, argv...)...)
		command.Env = append(os.Environ(),
			"GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null",
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.test",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.test",
		)
		command.Stdin = bytes.NewBufferString(stdin)
		output, err := command.Output()
		if err != nil {
			t.Skipf("git %v is unavailable here: %v", argv, err)
		}
		return string(bytes.TrimSpace(output))
	}
	git("", "init", "--quiet")
	git("", "commit", "--quiet", "--allow-empty", "-m", "base")
	base := git("", "rev-parse", "HEAD")
	blob := git("latin\n", "hash-object", "-w", "--stdin")
	git("100644 "+blob+"\tdocs/caf\xe9.txt\n", "update-index", "--index-info")
	git("", "commit", "--quiet", "-m", "latin-1 path")

	want := "docs/caf\uFFFD.txt"
	dirty, err := affected.DirtyPaths(context.Background(), gitExecutable, root)
	if err != nil {
		t.Fatalf("dirty capture refused a non-UTF-8 path: %v", err)
	}
	if len(dirty) != 1 || dirty[0] != want {
		t.Fatalf("dirty=%q want [%q]", dirty, want)
	}
	committed, err := affected.RangePaths(context.Background(), gitExecutable, root, base)
	if err != nil {
		t.Fatalf("range capture refused a non-UTF-8 path: %v", err)
	}
	if len(committed) != 1 || committed[0] != want {
		t.Fatalf("range=%q want [%q]", committed, want)
	}
}
