package store_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/supervisor"
)

// leaderFault, when set by a platform test file, replaces the lane leader
// while CORVINT_TEST_LEADER_FAULT is set in its environment.
var leaderFault func(dir string)

// TestMain lets supervised-workflow tests use the test binary itself as the
// pinned lane leader (`self lane-leader --directory DIR --capsule DIGEST`).
func TestMain(m *testing.M) {
	if len(os.Args) == 2 && os.Args[1] == "output-holder" {
		time.Sleep(3 * time.Second)
		os.Exit(0)
	}
	if len(os.Args) == 6 && os.Args[1] == "lane-leader" {
		if leaderFault != nil && os.Getenv("CORVINT_TEST_LEADER_FAULT") != "" {
			leaderFault(os.Args[3])
		}
		if e := supervisor.Leader(context.Background(), os.Args[3], os.Args[5]); e != nil {
			fmt.Fprintln(os.Stderr, e)
			os.Exit(1)
		}
		os.Exit(0)
	}
	// safeopen refuses symlinked ancestors, and the macOS default temp root
	// sits under /var -> /private/var, so test dirs come from its resolved
	// path (V1-0753).
	if dir, err := filepath.EvalSymlinks(os.TempDir()); err == nil {
		os.Setenv("TMPDIR", dir)
	}
	os.Exit(m.Run())
}
