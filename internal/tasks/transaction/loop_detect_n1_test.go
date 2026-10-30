package transaction

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// This file uses only API that predates CAL-V0-102, so loopN1Pinned can be
// recomputed on the pre-change source (base 58d089327178d834762f7fb83df3c013749ec386).
const loopN1Pinned = "5e3dff8cb0fd96775ba652c4f1e879a9cb33f1b0ac43f254ce4545a0f3d296cf"

// loopN1Transcript claims LP-01 at stage implement, ends each generation as
// RELEASE HANDOFF writes a clean no-tree hand-off, three times, then claims
// it once more: under a policy with loopDetection bound 2 that last claim is
// held. It records every posted byte, every ended
// attempt, both plans, claim-next, recorded claimability and ticket views.
// edit, when not nil, adjusts the decoded fixture policy first.
func loopN1Transcript(t *testing.T, edit func(*priorityFixture)) string {
	t.Helper()
	rec, other := priorityTicket("LP-01", "P1", ""), priorityTicket("OT-01", "P2", "")
	f := newPriorityFixture(t, priorityPolicy(t, []string{"a"}, ""), []*ticket.Record{rec, other})
	if edit != nil {
		edit(f)
	}
	var parts []string
	for i := 1; i <= 3; i++ {
		c := f.claim(rec.TicketID.Raw, "")
		c.l.Stage = "implement"
		out := planClaim(c)
		parts = append(parts, outcomeTranscript(out))
		f.apply(t, out)
		f.st.head = &snapshot.Head{Generation: wire.SizeOf(uint64(i)), LastSeq: f.st.head.LastSeq}
		parts = append(parts, loopN1Release(t, f, rec))
	}
	c := f.claim(rec.TicketID.Raw, "")
	c.l.Stage = "implement"
	parts = append(parts, outcomeTranscript(planClaim(c)))
	in := f.planInput("")
	parts = append(parts, planDigest(PriorityFirst(in)), outcomeTranscript(planClaimNext(f.claim("", ""))))
	for _, r := range []*ticket.Record{rec, other} {
		v, code := RecordedClaimability(in, r)
		view, _ := f.st.tickets.View(r.TicketID.Raw, ticket.Context{CanonicalWriter: f.st.queue.CanonicalWriter})
		parts = append(parts, string(wire.Encode(v))+" "+code, string(wire.Encode(view.Value(false))))
	}
	return strings.Join(parts, "\n")
}

// loopN1Release ends rec's live attempt as a clean no-tree HANDOFF with
// evidence and drops its reservation, returning the ended attempt's bytes.
func loopN1Release(t *testing.T, f *priorityFixture, rec *ticket.Record) string {
	t.Helper()
	for id, a := range f.st.attempts {
		if a.TicketID != rec.TicketID || !a.Live() {
			continue
		}
		next := *a
		accounting := *a.RetryAccounting
		accounting.Disposition = wire.CodeHandoff
		reason := wire.CodeHandoff
		next.RetryAccounting, next.HandoffEvidence, next.Cause = &accounting, "local:no-change", &reason
		next.Phase, next.PhaseSinceSeq, next.Quiescence = "CANCELLED", wire.Size(fmt.Sprint(f.seq)), "FENCED"
		raw, err := next.Encode()
		if err != nil {
			t.Fatal(err)
		}
		f.st.attempts[id] = &next
		kept := []snapshot.ReservationEntry{}
		for _, en := range f.st.reservations.Entries {
			if en.AttemptID != id {
				kept = append(kept, en)
			}
		}
		set := *f.st.reservations
		set.Entries = kept
		f.st.reservations = &set
		f.seq++
		return string(raw)
	}
	t.Fatal("no live attempt to release")
	return ""
}

// D8 (CAL-V0-102): with no loopDetection in the policy, claim and
// claim-next bytes, ended and re-claimed attempt bytes, plans, recorded
// claimability and ticket views equal the pre-change source.
func TestCALV0102_PolicyAbsentMatchesNMinusOne(t *testing.T) {
	t.Run("CAL-V0-102 PolicyAbsentMatchesNMinusOne", func(t *testing.T) {
		transcript := loopN1Transcript(t, nil)
		if !strings.Contains(transcript, `"priorGenerations":[{`) {
			t.Fatal("the scenario records no prior generation")
		}
		sum := sha256.Sum256([]byte(transcript))
		if got := hex.EncodeToString(sum[:]); got != loopN1Pinned {
			t.Fatalf("policy-absent digest %s, pinned %s", got, loopN1Pinned)
		}
	})
}
