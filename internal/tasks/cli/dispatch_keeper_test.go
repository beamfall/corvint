package cli_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/cli"
	"github.com/Beamfall/corvint/internal/tasks/dispatch"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestCALV0313_NativeKeeperRenewsAndHeartbeatsFenced drives the
// dispatcher's native LeaseKeeper through the real CLI and store: a
// heartbeat leaves the lease as it was, a renew moves it to the requested
// minutes, and either write at a superseded generation or on a released
// attempt is refused as ErrFenced (CAL-V0-314) and moves nothing.
func TestCALV0313_NativeKeeperRenewsAndHeartbeatsFenced(t *testing.T) {
	root, claimed := leaseCLIStore(t, 1, time.Now().UTC().Add(-10*time.Minute).Truncate(time.Second))
	q := cli.DispatchQueueForTest(root)
	k, ok := q.(dispatch.LeaseKeeper)
	if !ok {
		t.Fatal("the native queue has no lease keeper")
	}
	ctx := context.Background()
	a := observedAttempt(t, q, claimed[0].AttemptID)
	if replayed, err := k.Heartbeat(ctx, a, "keep-beat"); err != nil || replayed {
		t.Fatalf("heartbeat: replayed %v %v", replayed, err)
	}
	if after := observedAttempt(t, q, a.ID); !after.LeaseExpires.Equal(a.LeaseExpires) || !after.Live {
		t.Fatalf("the heartbeat moved the lease: %+v", after)
	}
	before := time.Now().UTC().Truncate(time.Second)
	if replayed, err := k.Renew(ctx, a, 30, "keep-renew"); err != nil || replayed {
		t.Fatalf("renew: replayed %v %v", replayed, err)
	}
	renewed := observedAttempt(t, q, a.ID)
	if renewed.LeaseExpires.Before(before.Add(30*time.Minute)) || renewed.LeaseExpires.After(time.Now().Add(31*time.Minute)) {
		t.Fatalf("renewed lease %s, want about 30 minutes from %s", renewed.LeaseExpires, before)
	}
	// CAL-V0-318: the same request again replays and moves nothing.
	if replayed, err := k.Renew(ctx, a, 30, "keep-renew"); err != nil || !replayed {
		t.Fatalf("repeated renew: replayed %v %v", replayed, err)
	}
	if again := observedAttempt(t, q, a.ID); !again.LeaseExpires.Equal(renewed.LeaseExpires) {
		t.Fatalf("a replayed renew moved the lease %s to %s", renewed.LeaseExpires, again.LeaseExpires)
	}
	stale := a
	stale.Generation = "99"
	if _, err := k.Renew(ctx, stale, 30, "keep-stale-renew"); !errors.Is(err, dispatch.ErrFenced) {
		t.Fatalf("stale renew: %v", err)
	}
	if _, err := k.Heartbeat(ctx, stale, "keep-stale-beat"); !errors.Is(err, dispatch.ErrFenced) {
		t.Fatalf("stale heartbeat: %v", err)
	}
	release := atm(t, root, nil, "release", "--attempt", a.ID, "--generation", a.Generation, "--request-id", "release-first")
	if release.res.Outcome != wire.OutcomeOK {
		t.Fatalf("release: %+v", release.res)
	}
	if _, err := k.Renew(ctx, a, 30, "keep-late-renew"); !errors.Is(err, dispatch.ErrFenced) {
		t.Fatalf("renew after release: %v", err)
	}
	if after := observedAttempt(t, q, a.ID); after.Live {
		t.Fatalf("a fenced renew revived %+v", after)
	}
}
