package cli

import (
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// ATR-V0-062: SESSION_CHURN names the ticket, the count, the median length
// and the first-seen time of a ticket's newest run of short sessions; a
// long session or one whose start precedes the scan ends the run.
func TestATRV0062_DoctorSessionChurn(t *testing.T) {
	t0 := time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC)
	scan := &doctorScan{}
	seq := uint64(0)
	add := func(ticket, attempt, gen, phase string, fresh bool, at time.Time) {
		seq++
		a := &snapshot.Attempt{AttemptID: attempt, TicketID: wire.TicketID{Raw: ticket}, Generation: wire.Size(gen), Phase: phase}
		if phase == "FAILED" {
			a.RetryAccounting = &snapshot.RetryAccounting{Disposition: "RELEASED"}
		}
		scan.events = append(scan.events, doctorEvent{seq: seq, at: at, attempts: []*snapshot.Attempt{a}, fresh: []bool{fresh}})
	}
	// t1: a long session, then three short ones (60s, 120s, 30s), the
	// last two as new generations of one attempt.
	add("t1", "a0", "1", "RUNNING", true, t0)
	add("t1", "a0", "1", "FAILED", false, t0.Add(time.Hour))
	add("t1", "a1", "1", "RUNNING", true, t0.Add(2*time.Hour))
	add("t1", "a1", "1", "FAILED", false, t0.Add(2*time.Hour+time.Minute))
	add("t1", "a2", "1", "RUNNING", true, t0.Add(3*time.Hour))
	add("t1", "a2", "1", "FAILED", false, t0.Add(3*time.Hour+2*time.Minute))
	add("t1", "a2", "2", "RUNNING", false, t0.Add(4*time.Hour))
	add("t1", "a2", "2", "FAILED", false, t0.Add(4*time.Hour+30*time.Second))
	// t2: three short endings, the first of unknown length.
	add("t2", "b0", "1", "FAILED", false, t0.Add(time.Minute))
	add("t2", "b1", "1", "RUNNING", true, t0.Add(2*time.Minute))
	add("t2", "b1", "1", "FAILED", false, t0.Add(3*time.Minute))
	add("t2", "b2", "1", "RUNNING", true, t0.Add(4*time.Minute))
	add("t2", "b2", "1", "FAILED", false, t0.Add(5*time.Minute))
	all := func(string) bool { return true }
	got := doctorSessionChurn(scan, all, nil)
	if len(got) != 1 || got[0].kind != "SESSION_CHURN" || got[0].who != "t1" {
		t.Fatalf("findings %+v", got)
	}
	f := got[0]
	if !f.firstSeen.Equal(t0.Add(2*time.Hour)) || len(f.seqs) != 3 || f.remedy == "" {
		t.Fatalf("finding %+v", f)
	}
	for _, want := range []string{"3 consecutive sessions", "median 60s", "first seen 2026-10-17T14:00:00Z"} {
		if !strings.Contains(f.detail, want) {
			t.Fatalf("detail %q lacks %q", f.detail, want)
		}
	}
	if got := doctorSessionChurn(scan, func(string) bool { return false }, nil); len(got) != 0 {
		t.Fatalf("a non-candidate ticket was reported: %+v", got)
	}
	if got := doctorSessionChurn(scan, all, []doctorFinding{{kind: "NO_PROGRESS_HANDOFF", who: "t1"}}); len(got) != 0 {
		t.Fatalf("a NO_PROGRESS_HANDOFF ticket was reported again: %+v", got)
	}
}
