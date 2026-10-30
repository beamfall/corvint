package transaction

import (
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

func (f *priorityFixture) hold(member, minutes string) leaseContext {
	return f.waitRequest(&LeaseRequest{Verb: LeasePoolHold, Pool: "lanes", Member: member, Holder: "ops", LeaseMinutes: wire.Size(minutes), Reason: "run a schema migration by hand"})
}

func (f *priorityFixture) entry(t *testing.T, member string) snapshot.PoolEntry {
	t.Helper()
	for _, en := range f.st.pools.Entries {
		if en.MemberID == member {
			return en
		}
	}
	t.Fatalf("no entry for %s in %+v", member, f.st.pools.Entries)
	return snapshot.PoolEntry{}
}

func planned(c leaseContext) leaseOutcome { return planLease(c.r, c.in, c.st) }

// PSR-V0-268, PSR-V0-269: a hold journals a HELD entry for a FREE member,
// and a dispatched claim is handed another member while it stands.
func TestPSRV0268_HoldOccupiesTheMemberAndClaimsSkipIt(t *testing.T) {
	rec := priorityTicket("A", "P1", "lanes")
	f := newPriorityFixture(t, priorityPolicy(t, []string{"a", "b"}, ""), []*ticket.Record{rec})
	c := f.hold("a", "60")
	out := planned(c)
	f.apply(t, out)
	en := f.entry(t, "a")
	if en.State != "HELD" || en.Holder != "ops" || en.Hold == nil || en.Hold.HeldAt != timestamp || en.Hold.ExpiresAt != addMinutes(timestamp, "60") {
		t.Fatalf("held entry %+v hold %+v", en, en.Hold)
	}
	if en.AllocationID != PoolHoldAllocation(string(c.r.QueueID), c.r.RequestID, "a") || !strings.Contains(out.detail, "HELD: a:") {
		t.Fatalf("allocation %s detail %q", en.AllocationID, out.detail)
	}
	a := f.admitted(t, f.claim(rec.TicketID.Raw, "lanes"))
	if a.PoolAllocation == nil || a.PoolAllocation.MemberID != "b" {
		t.Fatalf("claim got %+v, want member b", a.PoolAllocation)
	}
	waitRefused(t, planned(f.hold("a", "60")), wire.CodeResourceCollision, "only a FREE member is held")
	waitRefused(t, planned(f.hold("b", "60")), wire.CodeResourceCollision, "member b is")
}

// PSR-V0-269, PSR-V0-272: an expired hold still occupies its member for
// claim-next and pool acquire, and keeps the definition the policy-update
// guard compares.
func TestPSRV0269_ExpiredHoldStaysOccupied(t *testing.T) {
	nx, wt := priorityTicket("NX-01", "P1", "lanes"), priorityTicket("WT-01", "P2", "")
	f := newPriorityFixture(t, priorityPolicy(t, []string{"a", "b"}, ""), []*ticket.Record{nx, wt})
	f.apply(t, planned(f.hold("a", "5")))
	if en := f.entry(t, "a"); en.DefinitionSha256 != f.st.policy.MemberDefinition("lanes", "a") {
		t.Fatalf("held entry definition %s", en.DefinitionSha256)
	}
	next := planClaimNext(at(f.claim("", "lanes"), 3600))
	f.apply(t, next)
	got := attemptOf(t, next)
	if got.TicketID.Raw != nx.TicketID.Raw || got.PoolAllocation == nil || got.PoolAllocation.MemberID != "b" {
		t.Fatalf("claim-next after expiry got %s on %+v, want member b", got.TicketID.Raw, got.PoolAllocation)
	}
	a := f.admitted(t, at(f.claim(wt.TicketID.Raw, ""), 3600))
	waitRefused(t, planPoolAcquire(at(f.acquire(a, "lanes"), 3600)), wire.CodeResourceCollision, "no eligible free member")
	if en := f.entry(t, "a"); en.State != "HELD" {
		t.Fatalf("expired hold converted to %s", en.State)
	}
}

// PSR-V0-268: hold is operator-attested, paused by a barrier, and names a
// policy member.
func TestPSRV0268_HoldRefusals(t *testing.T) {
	f := newPriorityFixture(t, priorityPolicy(t, []string{"a", "b"}, ""), nil)
	c := f.hold("a", "60")
	c.r.Actor.Role = "WORKER"
	waitRefused(t, planned(c), wire.CodeUnsupported, "operator attestation")
	c = f.hold("a", "60")
	c.st.barrier = &snapshot.Barrier{QueueID: f.st.queue.QueueID, Scope: "ALL", Reason: "OPERATOR"}
	waitRefused(t, planned(c), wire.CodePaused, "barrier")
	if out := planned(f.hold("zz", "60")); out.result == nil || out.result.Outcome.Codes[0] != wire.CodeMalformed {
		t.Fatalf("unknown member %+v", out.result)
	}
	for name, l := range map[string]*LeaseRequest{
		"no reason":   {Verb: LeasePoolHold, Pool: "lanes", Member: "a", Holder: "ops", LeaseMinutes: "60"},
		"long reason": {Verb: LeasePoolHold, Pool: "lanes", Member: "a", Holder: "ops", LeaseMinutes: "60", Reason: strings.Repeat("x", MaxPoolRecoverReasonBytes+1)},
		"no minutes":  {Verb: LeasePoolHold, Pool: "lanes", Member: "a", Holder: "ops", Reason: "job"},
		"attempt":     {Verb: LeasePoolHold, Pool: "lanes", Member: "a", Holder: "ops", LeaseMinutes: "60", Reason: "job", AttemptID: "attempt:acme:main:" + strings.Repeat("0", 32)},
		"renew pool":  {Verb: LeasePoolHoldRenew, Pool: "lanes", Member: "a", Allocation: "x", LeaseMinutes: "60"},
		"long hold":   {Verb: LeasePoolHold, Pool: "lanes", Member: "a", Holder: "ops", LeaseMinutes: "1441", Reason: "job"},
	} {
		if _, e := leaseValue(l, f.st.queue.QueueID); e == nil {
			t.Errorf("%s: shape accepted", name)
		}
	}
	if _, e := leaseValue(&LeaseRequest{Verb: LeasePoolHold, Pool: "lanes", Member: "a", Holder: "ops", LeaseMinutes: "60", Reason: "run a job"}, f.st.queue.QueueID); e != nil {
		t.Fatalf("valid hold refused: %v", e)
	}
}

// PSR-V0-270: renew extends an unexpired hold within MaxPoolHoldMinutes
// of heldAt; past the bound it is LIMIT_EXCEEDED, after expiry FENCED.
func TestPSRV0270_RenewIsBounded(t *testing.T) {
	f := newPriorityFixture(t, priorityPolicy(t, []string{"a", "b"}, ""), nil)
	f.apply(t, planned(f.hold("a", "60")))
	alloc := string(f.entry(t, "a").AllocationID)
	renew := func(seconds int64, minutes string) leaseContext {
		return at(f.waitRequest(&LeaseRequest{Verb: LeasePoolHoldRenew, Member: "a", Allocation: alloc, LeaseMinutes: wire.Size(minutes)}), seconds)
	}
	f.apply(t, planned(renew(1800, "120")))
	if got := f.entry(t, "a").Hold; got.HeldAt != timestamp || got.ExpiresAt != addMinutes(addSeconds(timestamp, 1800), "120") {
		t.Fatalf("renewed hold %+v", got)
	}
	out := planned(renew(3600, "1440"))
	if out.result == nil || out.result.Outcome.Codes[0] != wire.CodeLimitExceeded || !strings.Contains(out.result.Detail, "at most 1440 minutes") {
		t.Fatalf("over the bound %+v", out.result)
	}
	waitRefused(t, planned(renew(1800+120*60, "60")), wire.CodeFenced, "hold expired")
	other := f.waitRequest(&LeaseRequest{Verb: LeasePoolHoldRenew, Member: "a", Allocation: "other", LeaseMinutes: "60"})
	waitRefused(t, planned(other), wire.CodeFenced, "allocation differs")
	if !PoolHoldExpired(f.entry(t, "a"), mustTime(t, addSeconds(timestamp, 1800+120*60))) || PoolHoldExpired(f.entry(t, "a"), mustTime(t, timestamp)) {
		t.Fatal("PoolHoldExpired disagrees with the window")
	}
}

// PSR-V0-271: release, expired or not, quarantines the member with its
// quarantine time stamped and passes an ALL barrier.
func TestPSRV0271_ReleaseQuarantines(t *testing.T) {
	f := newPriorityFixture(t, priorityPolicy(t, []string{"a", "b"}, ""), nil)
	f.apply(t, planned(f.hold("a", "5")))
	alloc := string(f.entry(t, "a").AllocationID)
	c := at(f.waitRequest(&LeaseRequest{Verb: LeasePoolHoldRelease, Member: "a", Allocation: alloc}), 3600)
	if !Cancels(c.r) {
		t.Fatal("hold release does not pass an ALL barrier")
	}
	f.apply(t, planned(c))
	en := f.entry(t, "a")
	if en.State != "QUARANTINED" || en.Hold != nil || en.Reason != PoolHoldReleasedReason || en.QuarantinedAt != addSeconds(timestamp, 3600) {
		t.Fatalf("released entry %+v", en)
	}
	again := f.waitRequest(&LeaseRequest{Verb: LeasePoolHoldRelease, Member: "a", Allocation: alloc})
	waitRefused(t, planned(again), wire.CodeResourceCollision, "not an operator hold")
	if Cancels(f.hold("b", "60").r) {
		t.Fatal("a fresh hold passes an ALL barrier")
	}
}

func mustTime(t *testing.T, ts wire.Timestamp) time.Time {
	t.Helper()
	at, err := time.Parse(timestampLayout, string(ts))
	if err != nil {
		t.Fatal(err)
	}
	return at
}
