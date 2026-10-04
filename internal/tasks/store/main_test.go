package store_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/supervisor"
)

// TestMain lets supervised-workflow tests use the test binary itself as the
// pinned lane leader (`self lane-leader --directory DIR --capsule DIGEST`).
func TestMain(m *testing.M) {
	if len(os.Args) == 6 && os.Args[1] == "lane-leader" {
		if e := supervisor.Leader(context.Background(), os.Args[3], os.Args[5]); e != nil {
			fmt.Fprintln(os.Stderr, e)
			os.Exit(1)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}
