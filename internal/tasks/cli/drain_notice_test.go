//go:build darwin || linux

package cli_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/supervisor"
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

// ATR-V0-8207 / V1-4637: a child can end without a kept result, as in the
// hosted failure. LOST and PENDING are transport results and still carry the
// current drain notice. This controls the state without scheduling a child.
func TestATRV08207_DetachedTransportCarriesDrainNotice(t *testing.T) {
	root, dir, _ := lostRequest(t, "drain-lost")
	signal := filepath.Join(t.TempDir(), "drain.json")
	if err := os.WriteFile(signal, []byte(`{"profile":"taskman-dispatch-drain-pending/0","program":"prog","since":"2026-10-17T09:00:00Z","notice":"finish and end"}`+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CORVINT_DISPATCH_DRAIN_FILE", signal)
	lost := cliRun(t, root, "claim", "--attach", "--request-id", "drain-lost")
	if !hasCode(lost.res, wire.CodeSupervisorLost) || !drainWarned(lost.res) {
		t.Fatalf("LOST: %s", lost.stdout)
	}
	identity, err := supervisor.ProcessIdentity(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	writeJSON(t, filepath.Join(dir, "child.json"), map[string]any{"pid": os.Getpid(), "identity": identity, "startedAt": "2026-10-17T09:00:00Z", "ceilingAt": "2026-10-17T09:10:00Z"})
	pending := cliRun(t, root, "claim", "--attach", "--request-id", "drain-lost")
	if pending.code != 75 || !drainWarned(pending.res) {
		t.Fatalf("PENDING: %s", pending.stdout)
	}
	if err = os.Remove(signal); err != nil {
		t.Fatal(err)
	}
	again := cliRun(t, root, "claim", "--attach", "--request-id", "drain-lost")
	if drainWarned(again.res) {
		t.Fatalf("lifted drain: %s", again.stdout)
	}
}
