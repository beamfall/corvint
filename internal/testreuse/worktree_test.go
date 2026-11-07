// SPDX-License-Identifier: Apache-2.0

package testreuse

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/execfile"
)

// TestCheckoutAttributeTransformationRuns covers AFP-V0-2321 and
// AFP-V0-2322: a root .gitattributes change that converts a scoped fixture to
// CRLF on checkout keeps its blob and a clean status, yet the test reads
// different bytes and fails. The raw worktree bytes no longer hash to the
// blob, so the package runs (worktree-bytes) and a harness path is refused.
// An attribute change that transforms no input is a negative control.
func TestCheckoutAttributeTransformationRuns(t *testing.T) {
	root := fixtureRepo(t)
	goTest(t, root, true, "./a")
	record := recordAt(t, root, Source{Kind: SourceLocal}, allPassed)
	writeFixture(t, root, ".gitattributes", "*.md -diff\n")
	commitAll(t, root, "attributes without a transformation")
	wantUnits(t, "untransformed inputs", decideAt(t, root, localPolicy, record), map[string]string{"a": ReasonRecordDigestEqual, "b": ReasonRecordDigestEqual})
	writeFixture(t, root, ".gitattributes", "*.md -diff\ndata/a.txt text eol=crlf\n")
	commitAll(t, root, "checkout transformation")
	if err := os.Remove(filepath.Join(root, "data/a.txt")); err != nil {
		t.Fatal(err)
	}
	git(t, root, "checkout", "-q", "--", "data/a.txt")
	if status := git(t, root, "status", "--porcelain", "--untracked-files=all"); status != "" {
		t.Fatalf("the transformed checkout must be clean: %q", status)
	}
	if body, err := os.ReadFile(filepath.Join(root, "data/a.txt")); err != nil || string(body) != "ok\r\n" {
		t.Fatalf("checkout bytes %q %v, want CRLF", body, err)
	}
	units := decideAt(t, root, localPolicy, record)
	wantUnits(t, "transformed scope file", units, map[string]string{"a": ReasonDigestError, "b": ReasonRecordDigestEqual})
	if !strings.HasPrefix(units["a"].Detail, "worktree-bytes: data/a.txt") {
		t.Errorf("missing worktree-bytes detail: %+v", units["a"])
	}
	snapshot, err := Observe(context.Background(), ObserveOptions{Root: root, Env: fixtureEnv()})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := observeConfined(context.Background(), snapshot, "", fixtureEnv(), fixtureArgv, []string{"data"}); err == nil || !strings.Contains(err.Error(), "worktree-bytes") {
		t.Errorf("a transformed harness file: %v", err)
	}
	goTest(t, root, false, "./a")
}

// TestConcurrentEditDuringListingIsDrift covers AFP-V0-2321: a tracked scoped
// fixture edited while go list runs (a controlled edit from a go wrapper)
// leaves the first status read clean. The observation rereads HEAD, tree,
// status and index flags after listing and refuses the drift
// (snapshot-drift) rather than digesting the stale blob into a reuse.
func TestConcurrentEditDuringListingIsDrift(t *testing.T) {
	root := fixtureRepo(t)
	record := recordAt(t, root, Source{Kind: SourceLocal}, allPassed)
	if runtime.GOOS == "windows" {
		t.Skip("the controlled edit uses a POSIX shell go wrapper")
	}
	realGo, err := exec.LookPath("go")
	if err != nil {
		t.Skip("go not on PATH")
	}
	wrapper := filepath.Join(t.TempDir(), "go")
	script := "#!/bin/sh\nfor word in \"$@\"; do\n  if [ \"$word\" = \"-deps\" ]; then printf \"broken\\n\" > \"" + filepath.Join(root, "data/a.txt") + "\"; fi\ndone\nexec \"" + realGo + "\" \"$@\"\n"
	if err := execfile.WriteFile(wrapper, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	snapshot, err := Observe(context.Background(), ObserveOptions{Root: root, Go: wrapper, Env: fixtureEnv()})
	if err == nil {
		identity, identityErr := observeConfined(context.Background(), snapshot, "", fixtureEnv(), fixtureArgv, nil)
		if identityErr != nil {
			t.Fatal(identityErr)
		}
		unit := reasonFor(t, snapshot, identity, record, "a")
		t.Fatalf("an observation spanning a concurrent edit succeeded with dirty %v; a: %s %s", snapshot.Dirty, unit.Reason, unit.Detail)
	}
	if !strings.Contains(err.Error(), "snapshot-drift") {
		t.Fatalf("concurrent edit: %v", err)
	}
	goTest(t, root, false, "./a")
}

// TestRecheckRefusesLaterDrift covers AFP-V0-2321: the recheck a caller runs
// after its own reads (the CLI's, after the affected graph and the identity)
// compares the observed HEAD, tree, status and index flags, not timestamps.
// An unchanged checkout and an edit undone before the recheck pass (negative
// controls); a pending edit or a new commit is snapshot-drift.
func TestRecheckRefusesLaterDrift(t *testing.T) {
	root := fixtureRepo(t)
	ctx := context.Background()
	snapshot, err := Observe(ctx, ObserveOptions{Root: root, Env: fixtureEnv()})
	if err != nil {
		t.Fatal(err)
	}
	if err := snapshot.Recheck(ctx); err != nil {
		t.Fatalf("unchanged checkout: %v", err)
	}
	original, err := os.ReadFile(filepath.Join(root, "data/a.txt"))
	if err != nil {
		t.Fatal(err)
	}
	writeFixture(t, root, "data/a.txt", "broken\n")
	if err := snapshot.Recheck(ctx); err == nil || !strings.HasPrefix(err.Error(), ErrorSnapshotDrift+": ") {
		t.Errorf("pending edit: %v", err)
	}
	writeFixture(t, root, "data/a.txt", string(original))
	if err := snapshot.Recheck(ctx); err != nil {
		t.Errorf("undone edit: %v", err)
	}
	writeFixture(t, root, "data/b.txt", "moved\n")
	commitAll(t, root, "move HEAD")
	if err := snapshot.Recheck(ctx); err == nil || !strings.HasPrefix(err.Error(), ErrorSnapshotDrift+": ") {
		t.Errorf("new commit: %v", err)
	}
	if err := (&Snapshot{}).Recheck(ctx); err == nil || !strings.HasPrefix(err.Error(), ErrorSnapshotDrift+": ") {
		t.Errorf("unobserved snapshot: %v", err)
	}
}

// TestHiddenModeChangeRuns covers AFP-V0-2321: with core.fileMode=false a
// worktree file whose executable bit differs from its blob's mode is clean to
// status. The raw worktree check refuses it (worktree-bytes); the same
// checkout before the change is the negative control.
func TestHiddenModeChangeRuns(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows has no executable bit")
	}
	root := fixtureRepo(t)
	record := recordAt(t, root, Source{Kind: SourceLocal}, allPassed)
	git(t, root, "config", "core.fileMode", "false")
	wantUnits(t, "mode unchanged", decideAt(t, root, localPolicy, record), map[string]string{"a": ReasonRecordDigestEqual, "b": ReasonRecordDigestEqual})
	if err := os.Chmod(filepath.Join(root, "data/a.txt"), 0o755); err != nil {
		t.Fatal(err)
	}
	if status := git(t, root, "status", "--porcelain", "--untracked-files=all"); status != "" {
		t.Fatalf("the mode change must be hidden from status: %q", status)
	}
	units := decideAt(t, root, localPolicy, record)
	wantUnits(t, "hidden mode change", units, map[string]string{"a": ReasonDigestError, "b": ReasonRecordDigestEqual})
	if !strings.HasPrefix(units["a"].Detail, ErrorWorktreeBytes+": data/a.txt") {
		t.Errorf("missing worktree-bytes detail: %+v", units["a"])
	}
}
