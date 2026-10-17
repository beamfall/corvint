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
