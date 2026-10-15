package cli_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestCALV0231_RenewKeepsHolderFresh: a holder whose last signal is older
// than the default 600-second TTL reads STALE_HOLDER under a live lease; a
// current-generation `renew`, with no `attempt heartbeat`, makes it
// FRESH_HOLDER. A superseded-generation renew is refused FENCED and moves
// neither the lease nor the signal. `renew --help --verbose` states both.
func TestCALV0231_RenewKeepsHolderFresh(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	// leaseCLIStore claims at base+10m (now-11m) with a five-minute lease.
	root, claims := leaseCLIStore(t, 1, now.Add(-21*time.Minute))
	c := claims[0]
	repo, e := intent.Resolve(root)
	if e != nil {
		t.Fatal(e)
	}
	renewAt, err := wire.ParseTimestamp("recordedAt", now.Add(-630*time.Second).Format("2006-01-02T15:04:05Z"))
	if err != nil {
		t.Fatal(err)
	}
	renew := transaction.LeaseRequest{Verb: transaction.LeaseRenew, AttemptID: c.AttemptID, Generation: c.Generation, LeaseMinutes: "30"}
	if report, err := store.Lease(context.Background(), repo, mutation.Binding{ID: "tester", Role: "OWNER"}, store.LeaseChoice{QueueID: fixture.QueueID, RequestID: "renew-old", Root: root, Lease: renew, Derive: store.NoScopeDeriver}, renewAt); err != nil || report.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("renew: %+v %v", report, err)
	}
	show := func(want string) (string, string) {
		t.Helper()
		x := handoffCLI(t, root, "attempt", "show", c.AttemptID)
		item := x.res.Items[0]
		if x.code != 0 || field(item, "holderStatus").Str != want {
			t.Fatalf("want %s: %s", want, x.stdout)
		}
		return field(field(item, "lease"), "expiresAt").Str, field(item, "lastHeartbeatAt").Str
	}
	staleExpires, staleSignal := show("STALE_HOLDER")
	old := handoffCLI(t, root, "renew", "--attempt", c.AttemptID, "--generation", string(wire.SizeOf(c.Generation.Uint64()+1)), "--request-id", "renew-wrong-generation", "--lease-minutes", "30")
	if !hasCode(old.res, wire.CodeFenced) {
		t.Fatalf("wrong-generation renew: %s", old.stdout)
	}
	if x, s := show("STALE_HOLDER"); x != staleExpires || s != staleSignal {
		t.Fatalf("fenced renew moved the lease: %s %s -> %s %s", staleExpires, staleSignal, x, s)
	}
	if r := handoffCLI(t, root, "renew", "--attempt", c.AttemptID, "--generation", string(c.Generation), "--request-id", "renew-now", "--lease-minutes", "30"); r.code != 0 {
		t.Fatalf("renew: %s", r.stdout)
	}
	if x, s := show("FRESH_HOLDER"); x == staleExpires || s == staleSignal {
		t.Fatalf("renew did not move lease and signal: %s %s", x, s)
	}
	help := handoffCLI(t, root, "renew", "--help", "--verbose")
	note := field(help.res.Items[0], "note").Str
	for _, want := range []string{"lastHeartbeatAt", "STALE_HOLDER", "REVISION_CONFLICT FENCED", "fresh request ID"} {
		if !strings.Contains(note, want) {
			t.Fatalf("renew note lacks %q: %s", want, note)
		}
	}
	if terse := handoffCLI(t, root, "renew", "--help"); field(terse.res.Items[0], "verboseHelp").Str == "" {
		t.Fatalf("terse renew help lacks verboseHelp: %s", terse.stdout)
	}
}
