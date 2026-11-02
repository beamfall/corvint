package affected_test

import (
	"context"
	"errors"
	"os/exec"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// TestAFPV0548DeadlineExpiryIsGitTimeout pins V1-3414: a status, range or
// ignored-path read that ends because its deadline elapsed reports
// cemcode.GitTimeout, while a plain cancellation keeps cemcode.GitCancelled.
func TestAFPV0548DeadlineExpiryIsGitTimeout(t *testing.T) {
	gitExecutable, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git is not installed")
	}
	root := t.TempDir()
	reads := map[string]func(context.Context) error{
		"DirtyPaths": func(ctx context.Context) error {
			_, err := affected.DirtyPaths(ctx, gitExecutable, root)
			return err
		},
		"RangePaths": func(ctx context.Context) error {
			_, err := affected.RangePaths(ctx, gitExecutable, root, "HEAD")
			return err
		},
		"IgnoredPaths": func(ctx context.Context) error {
			_, err := affected.IgnoredPaths(ctx, gitExecutable, root)
			return err
		},
	}
	for name, read := range reads {
		expired, cancelExpired := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
		err := read(expired)
		cancelExpired()
		if !errors.Is(err, affected.ErrStatusUnavailable) || cemcode.CodeOf(err) != cemcode.GitTimeout {
			t.Errorf("%s with an expired deadline: code %q, err %v; want %s", name, cemcode.CodeOf(err), err, cemcode.GitTimeout)
		}
		cancelled, cancel := context.WithCancel(context.Background())
		cancel()
		err = read(cancelled)
		if !errors.Is(err, affected.ErrStatusUnavailable) || cemcode.CodeOf(err) == cemcode.GitTimeout {
			t.Errorf("%s cancelled: code %q, err %v; want a cancellation, not %s", name, cemcode.CodeOf(err), err, cemcode.GitTimeout)
		}
	}
}
