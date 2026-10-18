package transaction

import (
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// progressAttempt is loopAttempt for no-tree implement hand-offs whose
// hand-off evidence references are refs, oldest first: "" records no
// reference and "?" a prior generation recorded without the progress
// signal (UNKNOWN under it).
func progressAttempt(rec *ticket.Record, refs ...string) *snapshot.Attempt {
	steps := make([]loopStep, len(refs))
	for i := range refs {
		steps[i] = handoff("implement", "")
	}
	a := loopAttempt(rec, steps...)
	for i, ref := range refs {
		if i == len(refs)-1 {
			if ref != "?" {
				a.HandoffEvidence = ref
			}
			break
		}
		if ref == "?" {
			continue
		}
		p := &snapshot.LoopProgress{Signal: intent.ProgressSignalHandoffEvidence}
		if ref != "" {
			r := ref
			p.Fingerprint = &r
		}
		a.PriorGenerations[i].History.Loop.Progress = p
	}
	return a
}

// TestATRV0082_LoopHoldHonoursProgressSignal: under a declared
// HANDOFF_EVIDENCE signal a hand-off whose fingerprint is new or changed is
// progress and ends the no-progress run; an unchanged or absent fingerprint
// with no tree, gate or review still counts; a generation recorded without
// the signal is UNKNOWN; without the declaration the same history holds as
// before; and a NO_PROGRESS hold names the signal (ATR-V0-083).
func TestATRV0082_LoopHoldHonoursProgressSignal(t *testing.T) {
	t.Run("ATR-V0-082 LoopHoldHonoursProgressSignal", func(t *testing.T) {
		rec := issue502Asking(t, "AT-01", 0)
		declared := loopPolicy("2", "2")
		declared.ProgressSignal = intent.ProgressSignalHandoffEvidence
		cases := []struct {
			name string
			refs []string
			run  string // the newest no-progress run; a run above 2 holds
		}{
			{"every fingerprint changes", []string{"local:a", "local:b", "local:c", "local:d"}, ""},
			{"unchanged fingerprint counts", []string{"local:a", "local:a", "local:a", "local:a"}, "2,3,4"},
			{"no fingerprint counts", []string{"", "", ""}, "1,2,3"},
			{"change ends the run", []string{"local:a", "local:a", "local:a", "local:b", "local:b", "local:b"}, "5,6"},
			{"change on the newest generation", []string{"local:a", "local:a", "local:a", "local:a", "local:b"}, ""},
			{"absent fingerprint is unchanged", []string{"local:a", "", "local:a", ""}, "2,3,4"},
			{"unknown history never counts", []string{"", "?", "", ""}, "3,4"},
			{"unknown makes a fingerprint incomparable", []string{"local:a", "?", "local:a", "local:a", "local:a"}, "4,5"},
		}
		for _, c := range cases {
			a := progressAttempt(rec, c.refs...)
			if run := strings.Join(noProgressRun(loopGenerations(a, declared.ProgressSignal), "", declared.ProgressSignal), ","); run != c.run {
				t.Fatalf("%s: run %q want %q", c.name, run, c.run)
			}
			got := LoopHoldOf(map[string]*snapshot.Attempt{"a": a}, rec, declared)
			if strings.Count(c.run, ",") < 2 {
				if got != nil {
					t.Fatalf("%s: held %+v", c.name, got)
				}
				continue
			}
			if got == nil || got.Signal != LoopNoProgress || strings.Join(got.Generations, ",") != c.run || got.ProgressSignal != intent.ProgressSignalHandoffEvidence {
				t.Fatalf("%s: got %+v want %s", c.name, got, c.run)
			}
			if !strings.HasSuffix(got.Detail(), " (progress signal HANDOFF_EVIDENCE)") {
				t.Fatalf("%s: detail %q does not name the signal", c.name, got.Detail())
			}
		}

		// ATR-V0-082(3): without the declaration changed fingerprints are
		// ignored and the hold names no signal, byte for byte as before.
		changing := progressAttempt(rec, "local:a", "local:b", "local:c", "local:d")
		got := LoopHoldOf(map[string]*snapshot.Attempt{"a": changing}, rec, loopPolicy("2", "2"))
		if got == nil || strings.Join(got.Generations, ",") != "1,2,3,4" || got.ProgressSignal != "" || strings.Contains(got.Detail(), "progress signal") {
			t.Fatalf("undeclared: %+v", got)
		}
		// The alternating-returns signal is not judged by the progress signal.
		alt := loopAttempt(rec, handoff("implement", "a"), returned("a"), handoff("implement", "b"), returned("b"), handoff("implement", "c"), returned("c"))
		if got := LoopHoldOf(map[string]*snapshot.Attempt{"a": alt}, rec, declared); got == nil || got.Signal != LoopAlternatingReturns || got.ProgressSignal != "" {
			t.Fatalf("alternating: %+v", got)
		}
	})
}

// TestATRV0081_ClaimRecordsProgressFingerprint: under loopDetection and a
// declared signal each claim records the ended generation's fingerprint as
// loopEvidence.progress, and the store-level scenario holds a ticket whose
// hand-offs repeat one reference but not one whose references change.
func TestATRV0081_ClaimRecordsProgressFingerprint(t *testing.T) {
	t.Run("ATR-V0-081 ClaimRecordsProgressFingerprint", func(t *testing.T) {
		run := func(signal string, refs []string) string {
			rec, other := priorityTicket("LP-01", "P1", ""), priorityTicket("OT-01", "P2", "")
			f := newPriorityFixture(t, priorityPolicy(t, []string{"a"}, ""), []*ticket.Record{rec, other})
			p := *f.st.policy
			p.LoopDetection = &intent.LoopDetection{MaxNoProgressGenerations: "2", MaxAlternatingReturns: "2"}
			p.ProgressSignal = signal
			f.st.policy = &p
			var parts []string
			for i, ref := range refs {
				c := f.claim(rec.TicketID.Raw, "")
				c.l.Stage = "implement"
				out := planClaim(c)
				parts = append(parts, outcomeTranscript(out))
				f.apply(t, out)
				f.st.head = &snapshot.Head{Generation: wire.SizeOf(uint64(i + 1)), LastSeq: f.st.head.LastSeq}
				loopN1Release(t, f, rec)
				for _, a := range f.st.attempts {
					if a.TicketID == rec.TicketID {
						a.HandoffEvidence = ref
					}
				}
			}
			c := f.claim(rec.TicketID.Raw, "")
			c.l.Stage = "implement"
			parts = append(parts, outcomeTranscript(planClaim(c)))
			return strings.Join(parts, "\n")
		}
		same := run(intent.ProgressSignalHandoffEvidence, []string{"local:a", "local:a", "local:a", "local:a"})
		if !strings.Contains(same, `"progress":{"fingerprint":"local:a","signal":"HANDOFF_EVIDENCE"}`) || !strings.Contains(same, wire.CodeLoopDetected) || !strings.Contains(same, "progress signal HANDOFF_EVIDENCE") {
			t.Fatalf("unchanged fingerprints did not record and hold:\n%s", same)
		}
		changed := run(intent.ProgressSignalHandoffEvidence, []string{"local:a", "local:b", "local:c"})
		if strings.Contains(changed, wire.CodeLoopDetected) || !strings.Contains(changed, `"fingerprint":"local:c"`) {
			t.Fatalf("changed fingerprints held or were not recorded:\n%s", changed)
		}
		plain := run("", []string{"local:a", "local:b", "local:c"})
		if strings.Contains(plain, `"progress"`) || !strings.Contains(plain, wire.CodeLoopDetected) {
			t.Fatalf("undeclared signal recorded progress or did not hold:\n%s", plain)
		}
	})
}
