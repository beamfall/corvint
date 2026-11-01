//go:build darwin || linux

package contextindex

import (
	"context"
	"errors"
	"github.com/Beamfall/corvint/internal/execfile"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

// DCW-V0-075 (V1-1652): the per-phase Git deadline is 30 seconds and has no
// override.
func TestDCWV0075GitDeadlineIsThirtySeconds(t *testing.T) {
	if gitDeadline != 30*time.Second {
		t.Fatalf("gitDeadline = %v, want 30s (DCW-V0-075 documents this value)", gitDeadline)
	}
}

// DCW-V0-075 (V1-1652): a Git slower than its deadline, as on a loaded host,
// refuses with the stable uncoded deadline error that dogfood change retries
// once and then admits, and the build returns within a bound.
func TestDCWV0075SlowGitOverrunsTheDeadline(t *testing.T) {
	root := testRepository(t)
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	if realGit, err = filepath.Abs(realGit); err != nil {
		t.Fatal(err)
	}
	wrapper := filepath.Join(t.TempDir(), "git")
	body := "#!/bin/sh\n/bin/sleep 2\nexec " + strconv.Quote(realGit) + " \"$@\"\n"
	if err := execfile.WriteFile(wrapper, []byte(body), 0o700); err != nil {
		t.Fatal(err)
	}
	// A caller deadline shorter than gitDeadline ends the same per-phase
	// context, so the overrun is reproduced without shortening the constant.
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	started := time.Now()
	_, err = BuildWithGitExecution(ctx, root, wrapper, qualifiedGitTestEnvironment(t))
	var failure *Error
	if !errors.As(err, &failure) || failure.Code != "" || failure.Message != "Git repository index exceeded its deadline" || !errors.Is(failure.Cause, context.DeadlineExceeded) {
		t.Fatalf("slow Git = %#v, want the uncoded deadline refusal", err)
	}
	if elapsed := time.Since(started); elapsed > 10*time.Second {
		t.Fatalf("slow Git build returned after %v, want a bounded overrun", elapsed)
	}
}

// IDX-SNAP-V0-912: only a context the refresh writer marked drops the shared
// 30-second deadline, keeping its own; an unmarked context keeps 30 seconds
// even under a longer parent deadline.
func TestWriterBoundsReplaceTheGitDeadline_IDXSNAPV0912(t *testing.T) {
	parent, cancelParent := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancelParent()
	ordinary, cancel := withGitDeadline(parent)
	defer cancel()
	if deadline, ok := ordinary.Deadline(); !ok || time.Until(deadline) > gitDeadline {
		t.Fatalf("unmarked deadline %v (%t), want at most %s", deadline, ok, gitDeadline)
	}
	writer, cancelWriter := withGitDeadline(WithWriterBounds(parent))
	defer cancelWriter()
	parentDeadline, _ := parent.Deadline()
	if deadline, ok := writer.Deadline(); !ok || !deadline.Equal(parentDeadline) {
		t.Fatalf("writer deadline %v (%t), want the parent's %v", deadline, ok, parentDeadline)
	}
	cancelParent()
	<-writer.Done()
}
