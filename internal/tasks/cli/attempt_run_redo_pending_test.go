//go:build darwin || linux

package cli_test

import (
	"bytes"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/cli"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestCALV0260_AttemptRunSettlesHeldPendingReceipt is the V1-1353
// reproduction for `attempt run`: its initial probe, which only names the
// queue, returned REDO_PENDING, so a held pending receipt refused the run
// before launch. Now the run's first heartbeat settles the receipt under the
// store lock, the command runs, and the envelope reports the redo.
func TestCALV0260_AttemptRunSettlesHeldPendingReceipt(t *testing.T) {
	root, a := attemptStore(t)
	t.Setenv("CORVINT_TASKS_ACTOR", "holder")
	ref := attemptRef{attempt: a.AttemptID, generation: gen(a)}
	seq := holdPending(t, root, heartbeatOnce(t, root, ref, "beat-run"))
	r := runAttempt(t, root, a, gen(a), []string{"--timeout", "30"}, "/bin/sh", "-c", "exit 0")
	if r.code != 0 || r.res.Outcome != wire.OutcomeOK || hasCode(r.res, wire.CodeRedoPending) {
		t.Fatalf("attempt run over a held pending receipt: %s\n%s", r.stdout, r.stderr)
	}
	if !bytes.Contains(r.stdout, []byte(redoWarning)) {
		t.Fatalf("attempt run does not report the redo: %s", r.stdout)
	}
	if o := recordedOutcome(t, root, r); o.Heartbeats != 2 {
		t.Fatalf("outcome heartbeats %d, want 2", o.Heartbeats)
	}
	// The pending receipt, the pre-launch heartbeat and renew as the lease
	// needs, the final heartbeat and the outcome all committed.
	if got := auditedHeadSeq(t, root); got < seq+4 {
		t.Fatalf("head seq %d, want at least %d", got, seq+4)
	}
	if _, err := cli.ObserveTickets(root); err != nil {
		t.Fatalf("read after the run: %v", err)
	}
}

// TestCALV0260_AttemptRunRefusalReportsSettledRedo: when the run's first
// heartbeat settles a pending release of its own attempt, the heartbeat is
// then refused and nothing starts, but the refusal still reports the
// completed redo.
func TestCALV0260_AttemptRunRefusalReportsSettledRedo(t *testing.T) {
	root, a := attemptStore(t)
	t.Setenv("CORVINT_TASKS_ACTOR", "holder")
	holdPending(t, root, func() {
		t.Helper()
		if x := atm(t, root, nil, "release", "--attempt", a.AttemptID, "--generation", gen(a), "--request-id", "release-held"); x.code != 0 {
			t.Fatalf("release: %s", x.stdout)
		}
	})
	r := runAttempt(t, root, a, gen(a), []string{"--timeout", "30"}, "/bin/sh", "-c", "exit 0")
	if r.code == 0 || r.res.Outcome != wire.OutcomeRefused || hasCode(r.res, wire.CodeRedoPending) {
		t.Fatalf("attempt run after a held pending release: %s\n%s", r.stdout, r.stderr)
	}
	if !bytes.Contains(r.stdout, []byte(redoWarning)) {
		t.Fatalf("refused attempt run does not report the redo: %s", r.stdout)
	}
	if _, err := cli.ObserveTickets(root); err != nil {
		t.Fatalf("read after the refusal: %v", err)
	}
}
