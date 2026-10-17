//go:build darwin || linux

package cli_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

func drainWarned(r *wire.Result) bool {
	return slices.ContainsFunc(r.Warnings, func(w string) bool { return strings.HasPrefix(w, "DRAIN_PENDING since 2026-10-17T09:00:00Z: ") })
}

// ATR-V0-059: while the signal named by CORVINT_DISPATCH_DRAIN_FILE is
// present, a claim's result, admitted, replayed, refused or detached,
// carries the drain notice; without the signal it carries none.
func TestATRV0059_ClaimCarriesDrainNotice(t *testing.T) {
	root, _, id := detachClaimStore(t)
	signal := filepath.Join(t.TempDir(), "drain-pending.json")
	t.Setenv("CORVINT_DISPATCH_DRAIN_FILE", signal)
	first := atm(t, root, nil, "claim", id, "--holder", "first", "--request-id", "claim-first")
	if first.code != 0 || drainWarned(first.res) {
		t.Fatalf("claim without a signal: %s", first.stdout)
	}
	if err := os.WriteFile(signal, []byte(`{"profile":"taskman-dispatch-drain-pending/0","program":"prog","since":"2026-10-17T09:00:00Z","notice":"finish and end"}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	replay := atm(t, root, nil, "claim", id, "--holder", "first", "--request-id", "claim-first")
	if replay.code != 0 || !drainWarned(replay.res) {
		t.Fatalf("replayed claim during the drain: %s", replay.stdout)
	}
	refused := cliRun(t, root, "claim", id, "--holder", "second", "--request-id", "claim-second")
	if refused.res.Outcome == wire.OutcomeOK || !drainWarned(refused.res) {
		t.Fatalf("refused claim during the drain: %s", refused.stdout)
	}
	detached := cliRun(t, root, "claim", id, "--holder", "second", "--request-id", "claim-detached", "--detach", "--wait", "60")
	if !drainWarned(detached.res) {
		t.Fatalf("detached claim during the drain: %s", detached.stdout)
	}
	if err := os.Remove(signal); err != nil {
		t.Fatal(err)
	}
	if again := atm(t, root, nil, "claim", id, "--holder", "first", "--request-id", "claim-first"); drainWarned(again.res) {
		t.Fatalf("claim after the drain lifted: %s", again.stdout)
	}
}

// ATR-V0-059: every envelope a detached claim prints carries the drain
// notice: PENDING, a refused attach, and a replay of a result the child
// kept before the drain began, which carries it exactly once.
func TestATRV0059_DetachedEnvelopesCarryDrainNotice(t *testing.T) {
	root, repo, id := detachClaimStore(t)
	signal := filepath.Join(t.TempDir(), "drain-pending.json")
	t.Setenv("CORVINT_DISPATCH_DRAIN_FILE", signal)
	drain := func() {
		t.Helper()
		if err := os.WriteFile(signal, []byte(`{"profile":"taskman-dispatch-drain-pending/0","program":"prog","since":"2026-10-17T09:00:00Z","notice":"finish and end"}`+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	notices := func(r *wire.Result) int {
		n := 0
		for _, w := range r.Warnings {
			if strings.HasPrefix(w, "DRAIN_PENDING ") {
				n++
			}
		}
		return n
	}
	drain()
	held := holdPreparation(t, repo)
	released := false
	t.Cleanup(func() {
		if !released {
			_ = held.Close()
		}
	})
	claim := []string{"claim", id, "--holder", "agent", "--request-id", "claim-pending"}
	if x := cliRun(t, root, append(claim, "--detach")...); x.code != 75 || field(x.res.Items[0], "state").Str != "PENDING" || !drainWarned(x.res) {
		t.Fatalf("PENDING detached claim during the drain: code %d %s", x.code, x.stdout)
	}
	if x := cliRun(t, root, "claim", "--attach", "--request-id", "claim-unknown"); x.res.Outcome == wire.OutcomeOK || !drainWarned(x.res) {
		t.Fatalf("refused attach during the drain: %s", x.stdout)
	}
	// The child commits after the drain lifts, so its kept result has no notice.
	if err := os.Remove(signal); err != nil {
		t.Fatal(err)
	}
	released = true
	if err := held.Close(); err != nil {
		t.Fatal(err)
	}
	if x := cliRun(t, root, "claim", "--attach", "--request-id", "claim-pending", "--wait", "60"); x.code != 0 || notices(x.res) != 0 {
		t.Fatalf("attach without a drain: code %d %s", x.code, x.stdout)
	}
	drain()
	if x := cliRun(t, root, "claim", "--attach", "--request-id", "claim-pending"); x.code != 0 || notices(x.res) != 1 || !drainWarned(x.res) {
		t.Fatalf("replayed detached claim during the drain: code %d %s", x.code, x.stdout)
	}
}
