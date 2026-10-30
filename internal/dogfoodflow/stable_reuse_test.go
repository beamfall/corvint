package dogfoodflow

import (
	"context"
	"slices"
	"testing"

	"github.com/Beamfall/corvint/internal/lrfrepo"
)

// TestOCMV0027ChildrenShareOnlyTheirRunStore: Exec's children never inherit a
// caller's stable verdict store, and get exactly the run's own when it has one.
func TestOCMV0027ChildrenShareOnlyTheirRunStore(t *testing.T) {
	t.Setenv(lrfrepo.StableReuseEnv, "/inherited/store")
	inherited := lrfrepo.StableReuseEnv + "=/inherited/store"
	plain := childEnvironment(context.Background())
	if slices.Contains(plain, inherited) || !slices.Contains(plain, "LC_ALL=C") {
		t.Fatalf("child environment kept an inherited store or lost LC_ALL: %v", plain)
	}
	run := childEnvironment(withStableReuse(context.Background(), "/run/store"))
	stores := 0
	for _, entry := range run {
		if len(entry) > len(lrfrepo.StableReuseEnv) && entry[:len(lrfrepo.StableReuseEnv)+1] == lrfrepo.StableReuseEnv+"=" {
			stores++
		}
	}
	if stores != 1 || !slices.Contains(run, lrfrepo.StableReuseEnv+"=/run/store") {
		t.Fatalf("child environment does not name exactly the run store: %v", run)
	}
}
