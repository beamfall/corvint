//go:build darwin || linux

package journal

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// stubStageSleep replaces the stage wait's sleep for one test, recording
// each pause and running then (if set) in place of sleeping.
func stubStageSleep(t *testing.T, then func(n int)) *[]time.Duration {
	t.Helper()
	pauses := &[]time.Duration{}
	saved := stageSleep
	stageSleep = func(d time.Duration) {
		*pauses = append(*pauses, d)
		if then != nil {
			then(len(*pauses))
		}
	}
	t.Cleanup(func() { stageSleep = saved })
	return pauses
}

// CTS-V0-008: a native writer starts its publish while an unlocked audit is
// reading. Its descriptor-less slot appears between the first attempt's two
// captures (SNAPSHOT_MOVED), stays put across the second attempt's captures
// as it does while the writer fsyncs or keeps a linked slot, and is removed by
// the writer's deferred RemoveStage. The audit waits for it and succeeds; it
// does not return the MALFORMED "unassigned stage slot" of V1-0825. The
// interleaving is driven by the capture and sleep hooks, not by timing.
func TestCTSV0008_AuditWaitsForWriterHeldStageSlot(t *testing.T) {
	repo, r := setup(t)
	appendReceipt(t, repo, "MUTATION", map[string][]byte{ticketPath("A"): fixture.Ticket("A").Encode()}, "", true, true, false)
	slot := filepath.Join(repo.StateDir, "staging", "a00")
	captures := 0
	r.afterCapture = func() {
		if captures++; captures == 1 {
			fixture.Write(t, slot, fixture.Ticket("A").Encode())
		}
	}
	pauses := stubStageSleep(t, func(int) {
		if e := os.Remove(slot); e != nil {
			t.Fatal(e)
		}
	})
	if _, err := r.Audit(); err != nil {
		t.Fatalf("audit beside a writer-held slot: %v", err)
	}
	if len(*pauses) != 1 || (*pauses)[0] != stagePause {
		t.Fatalf("pauses %v; want one of %v", *pauses, stagePause)
	}
	if captures != 3 {
		t.Fatalf("attempts %d; want moved, waited, settled", captures)
	}
}

// CTS-V0-008: the wait is bounded. A killed writer's orphan slot, which only
// the next writer removes (CAL-V0-019), still earns the same MALFORMED at the
// same path once stagePatience is spent, as a plain *wire.Error.
func TestCTSV0008_OrphanStageSlotStillMalformedAfterBoundedWait(t *testing.T) {
	repo, r := setup(t)
	appendReceipt(t, repo, "MUTATION", map[string][]byte{ticketPath("A"): fixture.Ticket("A").Encode()}, "", true, true, false)
	for _, s := range []string{"a03", "a00"} {
		fixture.Write(t, filepath.Join(repo.StateDir, "staging", s), []byte("x"))
	}
	pauses := stubStageSleep(t, nil)
	_, err := r.Audit()
	requireRefusal(t, err, wire.CodeMalformed, "staging/a00")
	if err.Error() != wire.Errorf(wire.CodeMalformed, "staging/a00", "unassigned stage slot").Error() {
		t.Fatalf("refusal changed: %v", err)
	}
	var total time.Duration
	for i, d := range *pauses {
		if d <= 0 || d > stagePauseCeiling || (i > 0 && d > 2*(*pauses)[i-1]) {
			t.Fatalf("pause %d of %v outside the backoff", i, *pauses)
		}
		total += d
	}
	if total != stagePatience || len(*pauses) > 16 {
		t.Fatalf("waited %v in %d pauses; want exactly %v", total, len(*pauses), stagePatience)
	}
	_, err = r.Audit(ticketPath("A"))
	requireRefusal(t, err, wire.CodeMalformed, "staging/a00")
}

// CTS-V0-008: only the descriptor-less shape is waited for. A slot beside a
// stage descriptor or a descriptor temp, which no native writer publishes,
// is refused at once.
func TestCTSV0008_UnassignedSlotBesideDescriptorRefusedAtOnce(t *testing.T) {
	for _, name := range []string{"active.json", "active.json.tmp"} {
		t.Run(name, func(t *testing.T) {
			repo, r := setup(t)
			appendReceipt(t, repo, "MUTATION", map[string][]byte{ticketPath("A"): fixture.Ticket("A").Encode()}, "", true, true, false)
			dir := filepath.Join(repo.StateDir, "staging")
			fixture.Write(t, filepath.Join(dir, name), stageDescriptor(t, repo, false))
			fixture.Write(t, filepath.Join(dir, "a07"), []byte("x"))
			pauses := stubStageSleep(t, nil)
			_, err := r.Audit()
			requireRefusal(t, err, wire.CodeMalformed, "staging/a07")
			if len(*pauses) != 0 {
				t.Fatalf("waited %v", *pauses)
			}
		})
	}
}
