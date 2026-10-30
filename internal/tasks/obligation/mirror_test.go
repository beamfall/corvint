package obligation

import (
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

func mirrorRef(t *testing.T, owner, id string) *ticket.ObligationMirrorRef {
	t.Helper()
	tid, err := wire.ParseTicketID("/ticketId", owner)
	if err != nil {
		t.Fatal(err)
	}
	return &ticket.ObligationMirrorRef{TicketID: tid, ID: id}
}

// TestTOLV0121_ResolveMirrors: a pending mirror whose owner entry is
// WITNESSED or DEFERRED derives that state with the owner's witness event
// as its source and no review of its own; an owner ticket, ledger or
// obligation that is missing, an unreadable owner ledger and a pending
// owner entry each keep the mirror open with its reason; a mirror whose own
// state is already terminal is not derived; resolution is one hop, so an
// owner that is itself a decided mirror does not decide this one.
func TestTOLV0121_ResolveMirrors(t *testing.T) {
	const b, c = "ticket:acme:main:B-1", "ticket:acme:main:C-1"
	ev := wire.Digest(strings.Repeat("e", 64))
	owner := &ticket.ObligationLedger{Entries: []ticket.ObligationEntry{
		{ID: "BC-1", State: ticket.ObligationWitnessed, Evidence: &ticket.ObligationEvidence{Source: "DECLARED", EventSha256: ev}},
		{ID: "BC-2", State: ticket.ObligationDeferred},
		{ID: "BC-3", State: ticket.ObligationBlocked},
		{ID: "BC-4", State: ticket.ObligationOpen, SatisfiedBy: mirrorRef(t, c, "CC-1")},
	}}
	third := &ticket.ObligationLedger{Entries: []ticket.ObligationEntry{{ID: "CC-1", State: ticket.ObligationWitnessed, Evidence: &ticket.ObligationEvidence{EventSha256: ev}}}}
	lookup := func(id string) MirrorOwner {
		switch id {
		case b:
			return MirrorOwner{Ledger: owner}
		case c:
			return MirrorOwner{Ledger: third}
		case "ticket:acme:main:NO-LEDGER":
			return MirrorOwner{Missing: MirrorOwnerLedgerMissing}
		case "ticket:acme:main:BROKEN":
			return MirrorOwner{Missing: MirrorOwnerLedgerUnknown}
		}
		return MirrorOwner{Missing: MirrorOwnerTicketMissing}
	}
	l := &ticket.ObligationLedger{Entries: []ticket.ObligationEntry{
		{ID: "AC-1", State: ticket.ObligationOpen, SatisfiedBy: mirrorRef(t, b, "BC-1")},
		{ID: "AC-2", State: ticket.ObligationBlocked, SatisfiedBy: mirrorRef(t, b, "BC-2")},
		{ID: "AC-3", State: ticket.ObligationOpen, SatisfiedBy: mirrorRef(t, b, "BC-3")},
		{ID: "AC-4", State: ticket.ObligationOpen, SatisfiedBy: mirrorRef(t, b, "BC-9")},
		{ID: "AC-5", State: ticket.ObligationOpen, SatisfiedBy: mirrorRef(t, "ticket:acme:main:GONE", "GC-1")},
		{ID: "AC-6", State: ticket.ObligationOpen, SatisfiedBy: mirrorRef(t, "ticket:acme:main:NO-LEDGER", "NC-1")},
		{ID: "AC-7", State: ticket.ObligationOpen, SatisfiedBy: mirrorRef(t, "ticket:acme:main:BROKEN", "XC-1")},
		{ID: "AC-8", State: ticket.ObligationDeferred, SatisfiedBy: mirrorRef(t, b, "BC-1")},
		{ID: "AC-9", State: ticket.ObligationOpen, SatisfiedBy: mirrorRef(t, b, "BC-4")},
		{ID: "AD-1", State: ticket.ObligationOpen},
	}}
	got := map[string]string{}
	rs := ResolveMirrors(l, lookup)
	for _, r := range rs {
		got[r.ID] = r.State + "/" + r.Reason
		if r.Decided != (r.ID == "AC-1" || r.ID == "AC-2") {
			t.Errorf("%s decided = %v", r.ID, r.Decided)
		}
	}
	for id, want := range map[string]string{
		"AC-1": "WITNESSED/", "AC-2": "DEFERRED/", "AC-3": "OPEN/" + MirrorOwnerUndecided,
		"AC-4": "OPEN/" + MirrorOwnerObligationMissing, "AC-5": "OPEN/" + MirrorOwnerTicketMissing,
		"AC-6": "OPEN/" + MirrorOwnerLedgerMissing, "AC-7": "OPEN/" + MirrorOwnerLedgerUnknown,
		"AC-8": "DEFERRED/", "AC-9": "OPEN/" + MirrorOwnerUndecided,
	} {
		if got[id] != want {
			t.Errorf("%s = %s, want %s", id, got[id], want)
		}
	}
	if len(rs) != 9 {
		t.Fatalf("%d resolutions for 9 mirrors", len(rs))
	}
	v := string(wire.Encode(MirrorValue(rs[0])))
	if !strings.Contains(v, `"derived":true`) || !strings.Contains(v, `"localState":"OPEN"`) ||
		!strings.Contains(v, `"source":{"eventSha256":"`+string(ev)+`","id":"BC-1","state":"WITNESSED","ticketId":"`+b+`"}`) || strings.Contains(v, "review") {
		t.Fatalf("witnessed mirror value: %s", v)
	}
	if v := string(wire.Encode(MirrorValue(rs[1]))); !strings.Contains(v, `"eventSha256":null`) {
		t.Fatalf("deferred mirror value: %s", v)
	}
	if v := string(wire.Encode(MirrorValue(rs[3]))); strings.Contains(v, "source") || !strings.Contains(v, `"reason":"`+MirrorOwnerObligationMissing+`"`) {
		t.Fatalf("missing owner value: %s", v)
	}
	if ResolveMirrors(&ticket.ObligationLedger{Entries: []ticket.ObligationEntry{{ID: "AD-1", State: ticket.ObligationOpen}}}, lookup) != nil {
		t.Fatal("a ledger without mirrors resolved some")
	}
}

// TestTOLV0122_MirrorFindings: decided mirrors form one
// MIRROR-DECIDED-ELSEWHERE finding; missing or unreadable owners form one
// MIRROR-OWNER-UNRESOLVED finding; an undecided owner is no finding.
func TestTOLV0122_MirrorFindings(t *testing.T) {
	rs := []MirrorResolution{
		{ID: "AC-1", Owner: *mirrorRef(t, "ticket:acme:main:B-1", "BC-1"), LocalState: "OPEN", OwnerState: "WITNESSED", Decided: true, State: "WITNESSED"},
		{ID: "AC-2", Owner: *mirrorRef(t, "ticket:acme:main:B-1", "BC-2"), LocalState: "OPEN", State: "OPEN", Reason: MirrorOwnerUndecided},
		{ID: "AC-3", Owner: *mirrorRef(t, "ticket:acme:main:B-1", "BC-9"), LocalState: "OPEN", State: "OPEN", Reason: MirrorOwnerObligationMissing},
	}
	fs := MirrorFindings("ticket:acme:main:A-1", rs, false)
	if len(fs) != 2 {
		t.Fatalf("findings: %d", len(fs))
	}
	d, u := string(wire.Encode(fs[0])), string(wire.Encode(fs[1]))
	if !strings.Contains(d, `"code":"MIRROR-DECIDED-ELSEWHERE","completable":false`) || !strings.Contains(d, `"id":"AC-1"`) || strings.Contains(d, `"AC-2"`) {
		t.Fatalf("decided finding: %s", d)
	}
	if !strings.Contains(u, `"code":"MIRROR-OWNER-UNRESOLVED"`) || !strings.Contains(u, `"id":"AC-3"`) || strings.Contains(u, `"AC-2"`) {
		t.Fatalf("unresolved finding: %s", u)
	}
	if fs := MirrorFindings("ticket:acme:main:A-1", rs[1:2], false); fs != nil {
		t.Fatalf("an undecided owner is a finding: %v", fs)
	}
	in := heldInput("T-1", "P1", "AC-1", "a.spec.ts", 1, "2026-10-01T00:00:00Z")
	in.MirrorDecided = true
	if class, reason := ClassifyWorklist(in); class != WorklistExcluded || reason != WorklistMirrorDecided {
		t.Fatalf("decided mirror in the worklist: %s/%s", class, reason)
	}
}

// TestTOLV0123_MirrorsCompletable: a ledger is completable only when it has
// a decided mirror and every pending entry is one; WITNESSED and DEFERRED
// entries do not hold it back.
func TestTOLV0123_MirrorsCompletable(t *testing.T) {
	l := &ticket.ObligationLedger{Entries: []ticket.ObligationEntry{
		{ID: "AC-1", State: ticket.ObligationOpen}, {ID: "AC-2", State: ticket.ObligationWitnessed},
		{ID: "AC-3", State: ticket.ObligationDeferred}, {ID: "AC-4", State: ticket.ObligationBlocked},
	}}
	decided := []MirrorResolution{{ID: "AC-1", Decided: true}, {ID: "AC-4", Decided: true}}
	if !MirrorsCompletable(l, decided) {
		t.Fatal("all pending entries decided is not completable")
	}
	if MirrorsCompletable(l, decided[:1]) {
		t.Fatal("a pending non-mirror is completable")
	}
	if MirrorsCompletable(l, []MirrorResolution{{ID: "AC-1"}, {ID: "AC-4"}}) || MirrorsCompletable(nil, decided) {
		t.Fatal("undecided mirrors or no ledger are completable")
	}
	done := &ticket.ObligationLedger{Entries: []ticket.ObligationEntry{{ID: "AC-2", State: ticket.ObligationWitnessed}}}
	if MirrorsCompletable(done, nil) {
		t.Fatal("a ledger without decided mirrors is completable")
	}
}
