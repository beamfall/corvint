package transaction

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/rand"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// priorityN1Pinned is priorityN1Digest over the policy without
// priorityAdmission, recomputed on the pre-CAL-V0-101 source (662051a0).
const priorityN1Pinned = "78b5005e6119a5a6c2cae99c8b53a2aebf0782168a298823423dde2f5e8efcff"

// yieldTarget is the ticket a refused claim yielded to, or "".
func yieldTarget(out leaseOutcome) string {
	if out.result == nil || !strings.Contains(out.result.Detail, " priority admission: ") {
		return ""
	}
	_, to, _ := strings.Cut(out.result.Detail, "; yields to ")
	return to
}

// CAL-V0-101: with priorityAdmission, an explicit pooled claim yields the
// last free member to a waiting higher-priority ticket, BLOCKED
// RESOURCE_COLLISION naming it, before any member is chosen; the waiting
// ticket and claim-next still admit, and a live competitor does not count.
func TestCALV0101_ExplicitPooledClaimYields(t *testing.T) {
	aa, hi, fr, lo := priorityScenario()
	recs := []*ticket.Record{aa, hi, fr, lo}
	// Three members: after AA-01 takes one, two free members exceed the one
	// waiting ticket, so LO-01 is admitted.
	wide := newPriorityFixture(t, priorityPolicy(t, []string{"a", "b", "c"}, "true"), recs)
	wide.apply(t, planClaim(wide.claim(aa.TicketID.Raw, "lanes")))
	if out := planClaim(wide.claim(lo.TicketID.Raw, "lanes")); out.result != nil {
		t.Fatalf("two free members, one waiting: %+v", *out.result)
	}

	f := newPriorityFixture(t, priorityPolicy(t, []string{"a", "b"}, "true"), recs)
	f.apply(t, planClaim(f.claim(aa.TicketID.Raw, "lanes")))
	for _, id := range []string{lo.TicketID.Raw, fr.TicketID.Raw} {
		out := planClaim(f.claim(id, "lanes"))
		if out.result == nil || out.result.Outcome.Outcome != "BLOCKED" || len(out.result.Outcome.Codes) != 1 || out.result.Outcome.Codes[0] != wire.CodeResourceCollision || len(out.posts) != 0 {
			t.Fatalf("%s: want yield refusal, got %+v", id, out)
		}
		if want := "pool lanes priority admission: 1 higher-priority waiting ticket(s) for 1 free eligible member(s); yields to " + hi.TicketID.Raw; out.result.Detail != want {
			t.Fatalf("detail %q", out.result.Detail)
		}
	}
	// Unpooled claims and the waiting ticket itself are unaffected.
	if out := planClaim(f.claim(fr.TicketID.Raw, "")); out.result != nil {
		t.Fatalf("unpooled claim: %+v", *out.result)
	}
	if out := planClaimNext(f.claim("", "lanes")); out.result != nil {
		t.Fatalf("claim-next: %+v", *out.result)
	} else if a := attemptOf(t, out); a.TicketID.Raw != hi.TicketID.Raw {
		t.Fatalf("claim-next admitted %s", a.TicketID.Raw)
	}
	f.apply(t, planClaim(f.claim(hi.TicketID.Raw, "lanes")))
	// HI-01 is live on the last member: nothing waits and nothing is free, so
	// LO-01 meets the ordinary no-free-member refusal, not a yield.
	if out := planClaim(f.claim(lo.TicketID.Raw, "lanes")); out.result == nil || yieldTarget(out) != "" {
		t.Fatalf("live competitor: %+v", out)
	}
}

func attemptOf(t *testing.T, out leaseOutcome) *snapshot.Attempt {
	t.Helper()
	for path, raw := range out.posts {
		if strings.HasPrefix(path, "attempts/") {
			a, err := snapshot.DecodeAttempt(raw)
			if err != nil {
				t.Fatal(err)
			}
			return a
		}
	}
	t.Fatal("no attempt posted")
	return nil
}

// CAL-V0-101: without the flag, or with it false, claim bytes, refusals,
// plans and recorded claimability equal the pre-change source; the policy
// bytes round-trip unchanged and member definitions ignore the flag.
func TestCALV0101_FlagOffMatchesNMinusOne(t *testing.T) {
	members := []string{"a", "b"}
	absent, off, on := priorityPolicy(t, members, ""), priorityPolicy(t, members, "false"), priorityPolicy(t, members, "true")
	if got := priorityN1Digest(t, absent); got != priorityN1Pinned {
		t.Fatalf("absent flag digest %s, pinned %s", got, priorityN1Pinned)
	}
	// An explicit false changes only the policy bytes, so only the policy
	// identity the attempt binds may differ.
	if got, want := priorityN1Transcript(t, off), priorityN1Transcript(t, absent); policyNeutral(t, got, off) != policyNeutral(t, want, absent) {
		t.Fatal("false flag changed claim bytes or decisions beyond the policy identity")
	}
	if got := priorityN1Digest(t, on); got == priorityN1Pinned {
		t.Fatal("the scenario does not exercise priority admission")
	}
	var defs []wire.Digest
	for _, raw := range [][]byte{absent, off, on} {
		p, err := intent.DecodePolicy(raw)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(p.Raw, raw) || p.PolicySha256() != (&intent.Policy{Raw: raw}).PolicySha256() {
			t.Fatal("policy bytes or identity changed on decode")
		}
		if p.Pools[0].PriorityAdmission != bytes.Contains(raw, []byte(`"priorityAdmission":true`)) {
			t.Fatalf("flag decoded %v", p.Pools[0].PriorityAdmission)
		}
		defs = append(defs, p.MemberDefinition("lanes", "a"))
	}
	if defs[0] == "" || defs[0] != defs[1] || defs[0] != defs[2] {
		t.Fatalf("member definitions differ: %v", defs)
	}
	if bytes.Contains(absent, []byte("priorityAdmission")) {
		t.Fatal("absent policy carries the key")
	}
	for _, bad := range []string{"null", `"true"`, "[]", "{}"} {
		if _, err := intent.DecodePolicy(priorityPolicy(t, members, bad)); wire.CodeOf(err) != wire.CodeMalformed {
			t.Fatalf("priorityAdmission %s: %v", bad, err)
		}
	}
}

// policyNeutral replaces the policy identities (the policy:sha256 digest
// and the SHA-256 of its bytes) in a transcript.
func policyNeutral(t *testing.T, transcript string, raw []byte) string {
	t.Helper()
	p, err := intent.DecodePolicy(raw)
	if err != nil {
		t.Fatal(err)
	}
	raw256 := sha256.Sum256(raw)
	for _, d := range []string{string(p.PolicySha256()), hex.EncodeToString(raw256[:])} {
		transcript = strings.ReplaceAll(transcript, d, "POLICY")
		if i := strings.LastIndex(d, ":"); i >= 0 {
			transcript = strings.ReplaceAll(transcript, d[i+1:], "POLICY")
		}
	}
	return transcript
}

// CAL-V0-101 (acceptance 3): over generated pool and ticket states, the
// --pool plan, the default plan, an explicit claim and claim-next agree on
// every yield, and a SELECTED --pool entry is admitted by an explicit claim.
func TestCALV0101_PlanClaimAndClaimNextAgree(t *testing.T) {
	rng := rand.New(rand.NewSource(101))
	yields := 0
	for iter := 0; iter < 400; iter++ {
		flag := "true"
		if rng.Intn(5) == 0 {
			flag = ""
		}
		members := []string{"a", "b", "c"}[:1+rng.Intn(3)]
		recs := []*ticket.Record{priorityTicket("DP-00", "P3", "")}
		for i := 0; i < 2+rng.Intn(6); i++ {
			pool := ""
			if rng.Intn(10) < 7 {
				pool = "lanes"
			}
			rec := priorityTicket(fmt.Sprintf("T-%02d", i), fmt.Sprintf("P%d", rng.Intn(4)), pool)
			rec.Order = wire.Count(fmt.Sprint(rng.Intn(3)))
			if rng.Intn(7) == 0 {
				rec.Effects.TouchPaths = []string{"shared/"}
			}
			if rng.Intn(7) == 0 {
				gate := "ci"
				rec.Dependencies = []ticket.Dependency{{TicketID: recs[0].TicketID, Obligation: "GATE_PASSED", GateID: &gate}}
			}
			recs = append(recs, rec)
		}
		f := newPriorityFixture(t, priorityPolicy(t, members, flag), recs)
		for _, rec := range recs[1:] {
			if rng.Intn(4) == 0 {
				pool := rec.RequiresPool
				if pool == "" && rng.Intn(2) == 0 {
					pool = "lanes"
				}
				if out := planClaim(f.claim(rec.TicketID.Raw, pool)); out.result == nil {
					f.apply(t, out)
				}
			}
		}
		where := fmt.Sprintf("iter %d flag %q members %d", iter, flag, len(members))

		pooled := PriorityFirst(f.planInput("lanes"))
		ahead := []string{}
		for _, e := range pooled.Entries {
			if e.State == PlanBlocked {
				continue
			}
			// The plan's running waiting list is the direct predicate.
			if direct, _ := priorityWaiting(f.planInput("lanes"), e.Ticket, "lanes"); fmt.Sprint(direct) != fmt.Sprint(ahead) {
				t.Fatalf("%s: %s waiting %v, plan order %v", where, e.Ticket.TicketID.Raw, direct, ahead)
			}
			if e.Ticket.RequiresPool == "lanes" {
				ahead = append(ahead, e.Ticket.TicketID.Raw)
			}
			out := planClaim(f.claim(e.Ticket.TicketID.Raw, "lanes"))
			if got := yieldTarget(out); got != e.yieldTo {
				t.Fatalf("%s: --pool plan %s yieldTo %q, explicit claim %q (%+v)", where, e.Ticket.TicketID.Raw, e.yieldTo, got, out.result)
			}
			if e.yieldTo != "" {
				yields++
				if flag == "" || e.State != PlanDeferred || e.Reason != wire.CodeResourceCollision || len(e.Blockers) != 1 || e.Blockers[0] != e.yieldTo {
					t.Fatalf("%s: yield entry %+v", where, e)
				}
			}
			if e.State == PlanSelected && out.result != nil {
				t.Fatalf("%s: SELECTED %s refused %+v", where, e.Ticket.TicketID.Raw, *out.result)
			}
		}
		next := planClaimNext(f.claim("", "lanes"))
		if yieldTarget(next) != "" {
			t.Fatalf("%s: claim-next yielded %+v", where, *next.result)
		}
		if chosen := pooled.ClaimNext("lanes"); chosen == nil {
			if next.result == nil {
				t.Fatalf("%s: claim-next admitted from an empty plan", where)
			}
		} else if next.result != nil || attemptOf(t, next).TicketID.Raw != chosen.Ticket.TicketID.Raw {
			t.Fatalf("%s: claim-next %+v, plan chose %s", where, next.result, chosen.Ticket.TicketID.Raw)
		}

		def := PriorityFirst(f.planInput(""))
		for _, e := range def.Entries {
			if e.State == PlanBlocked || e.Ticket.RequiresPool == "" {
				continue
			}
			out := planClaim(f.claim(e.Ticket.TicketID.Raw, "lanes"))
			if got := yieldTarget(out); got != e.yieldTo {
				t.Fatalf("%s: default plan %s yieldTo %q, explicit claim %q", where, e.Ticket.TicketID.Raw, e.yieldTo, got)
			}
			if e.yieldTo != "" && (!e.poolDeferred || len(e.Blockers) != 1 || e.Blockers[0] != e.yieldTo) {
				t.Fatalf("%s: default yield entry %+v", where, e)
			}
		}
	}
	if yields == 0 {
		t.Fatal("generator produced no yield")
	}
}

// CAL-V0-101: a competitor whose claim eligibility is NOT_OBSERVED never
// causes a refusal, and the recorded claimability reports the gap.
func TestCALV0101_UnobservedCompetitorIsNotObserved(t *testing.T) {
	dep := priorityTicket("DP-00", "P3", "")
	hi, lo, lv := priorityTicket("HI-01", "P0", "lanes"), priorityTicket("LO-01", "P2", "lanes"), priorityTicket("LV-01", "P3", "lanes")
	gate := "ci"
	hi.Dependencies = []ticket.Dependency{{TicketID: dep.TicketID, Obligation: "GATE_PASSED", GateID: &gate}}
	f := newPriorityFixture(t, priorityPolicy(t, []string{"a", "b"}, "true"), []*ticket.Record{dep, hi, lo, lv})
	f.apply(t, planClaim(f.claim(lv.TicketID.Raw, "lanes")))
	in := f.planInput("")
	if waiting, unobserved := priorityWaiting(in, lo, "lanes"); len(waiting) != 0 || len(unobserved) != 1 || unobserved[0] != hi.TicketID.Raw {
		t.Fatalf("waiting %v unobserved %v", waiting, unobserved)
	}
	if out := planClaim(f.claim(lo.TicketID.Raw, "lanes")); out.result != nil {
		t.Fatalf("refused on an unobserved competitor: %+v", *out.result)
	}
	if v, code := RecordedClaimability(in, lo); v.Kind != wire.KindNull || code != string(ticket.NotObserved) {
		t.Fatalf("claimability %s %s", wire.Encode(v), code)
	}
	for _, e := range PriorityFirst(f.planInput("lanes")).Entries {
		if e.Ticket.TicketID.Raw == lo.TicketID.Raw && e.State != PlanSelected {
			t.Fatalf("--pool plan %+v", e)
		}
	}
	// Without the flag the same state is plainly claimable.
	f.st.policy.Pools[0].PriorityAdmission = false
	if v, code := RecordedClaimability(f.planInput(""), lo); v.Kind != wire.KindBool || !v.Bool || code != PlanSelected {
		t.Fatalf("flag off claimability %s %s", wire.Encode(v), code)
	}
}
