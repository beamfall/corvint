package cli_test

import (
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/cli"
	"github.com/Beamfall/corvint/internal/tasks/dispatch"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

const ownerSeed = `{"prefix":"BC","obligations":[{"id":"BC-1","title":"shared login","core":true},{"id":"BC-2","title":"shared logout","core":false}]}`

// mirrorRepo is obligationRepo plus a second seeded ledger ticket, the
// owner; it returns the repo root, the mirroring ticket, the owner and HEAD.
func mirrorRepo(t *testing.T) (string, string, string, string) {
	t.Helper()
	r, a, head := obligationRepo(t)
	seedObligations(t, r.Root, a)
	b := planTicket(t, r.Root, "owner", "P2", `["e2e/"]`)
	if x := atm(t, r.Root, nil, "ticket", "obligations", "seed", "--target", b, "--expected-revision", obligationRevision(t, r.Root, b),
		"--request-id", "seed-b", "--issued-at", "2026-10-08T12:00:00Z", "--payload", ownerSeed); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("owner seed: %s", x.stdout)
	}
	return r.Root, a, b, head
}

func mirrorSet(t *testing.T, root, id, req, changes string) run {
	t.Helper()
	return atm(t, root, nil, "ticket", "obligations", "set", "--target", id, "--expected-revision", obligationRevision(t, root, id),
		"--request-id", req, "--payload", `{"changes":`+changes+`}`)
}

// mirrorItems maps each resolved mirror of show's mirrors member to
// state/derived/reason.
func mirrorItems(v wire.Value) map[string]string {
	out := map[string]string{}
	for _, m := range field(field(v, "mirrors"), "items").Arr {
		d := "local"
		if field(m, "derived").Bool {
			d = "derived"
		}
		out[field(m, "id").Str] = field(m, "state").Str + "/" + d + "/" + field(m, "reason").Str
	}
	return out
}

func statusMirrors(t *testing.T, root string) []wire.Value {
	t.Helper()
	x := atm(t, root, nil, "queue", "status")
	if x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("queue status: %s", x.stdout)
	}
	return field(x.res.Items[0], "obligationMirrors").Arr
}

// TestTOLV0120_MirrorReadsAndAcceptance covers the GitHub #1214 acceptance:
// a ledger without a mirror carries no new member in show, worklist or
// queue status; a self mirror refuses; with the owner witnessed the mirror
// reports WITNESSED with the owner as its source and no review of its own;
// a missing owner obligation is reported, not silently open; once every
// pending item is a decided mirror the ticket is completable and the
// dispatcher's plan defers it as MIRRORS_DECIDED_ELSEWHERE (TOL-V0-120..123).
func TestTOLV0120_MirrorReadsAndAcceptance(t *testing.T) {
	root, a, b, head := mirrorRepo(t)
	if v := obligationsShow(t, root, a); field(v, "mirrors").Kind != 0 {
		t.Fatalf("a ledger without mirrors carries mirrors: %s", wire.Encode(v))
	}
	if x := atm(t, root, nil, "queue", "status"); strings.Contains(string(x.stdout), "obligationMirrors") {
		t.Fatalf("queue status carries obligationMirrors: %s", x.stdout)
	}
	if v := worklistRead(t, root); field(v, "findings").Kind != 0 {
		t.Fatalf("worklist carries findings: %s", wire.Encode(v))
	}
	if x := mirrorSet(t, root, a, "self", `[{"id":"AC-1","state":null,"core":null,"reason":"self","satisfiedBy":{"ticketId":"`+a+`","id":"AC-2"}}]`); x.res.Outcome == wire.OutcomeOK || !strings.Contains(string(x.stdout), "OBLIGATION_MIRROR_SELF:") {
		t.Fatalf("self mirror: %s", x.stdout)
	}
	if x := mirrorSet(t, root, a, "mirror-1", `[{"id":"AC-1","state":null,"core":null,"reason":"shared with the owner","satisfiedBy":{"ticketId":"`+b+`","id":"BC-1"}},`+
		`{"id":"AC-2","state":null,"core":null,"reason":"renamed on the owner","satisfiedBy":{"ticketId":"`+b+`","id":"BC-9"}},`+
		`{"id":"AC-3","state":null,"core":null,"reason":"shared","satisfiedBy":{"ticketId":"`+b+`","id":"BC-2"}}]`); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("mirror set: %s", x.stdout)
	}
	if got := mirrorItems(obligationsShow(t, root, a)); got["AC-1"] != "OPEN/local/OWNER_UNDECIDED" || got["AC-2"] != "OPEN/local/OWNER_OBLIGATION_MISSING" || got["AC-3"] != "OPEN/local/OWNER_UNDECIDED" {
		t.Fatalf("before the owner decides: %v", got)
	}
	if x := atm(t, root, nil, declareArgs(b, "w-b", obligationRevision(t, root, b), head, "BC-1")...); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("owner witness: %s", x.stdout)
	}
	v := obligationsShow(t, root, a)
	if st := entryStates(v); st["AC-1"] != "OPEN" {
		t.Fatalf("the mirror's own state changed: %v", st)
	}
	var m wire.Value
	for _, it := range field(field(v, "mirrors"), "items").Arr {
		if field(it, "id").Str == "AC-1" {
			m = it
		}
	}
	src := field(m, "source")
	if field(m, "state").Str != "WITNESSED" || !field(m, "derived").Bool || field(src, "ticketId").Str != b || field(src, "id").Str != "BC-1" ||
		field(src, "state").Str != "WITNESSED" || field(src, "eventSha256").Kind != wire.KindString || strings.Contains(string(wire.Encode(m)), "review") {
		t.Fatalf("witnessed mirror: %s", wire.Encode(m))
	}
	if field(field(v, "mirrors"), "completable").Bool {
		t.Fatal("a ledger with other open items is completable")
	}
	fs := statusMirrors(t, root)
	if len(fs) != 2 || field(fs[0], "code").Str != "MIRROR-DECIDED-ELSEWHERE" || field(fs[0], "ticketId").Str != a || field(fs[0], "completable").Bool ||
		field(fs[1], "code").Str != "MIRROR-OWNER-UNRESOLVED" || len(field(fs[1], "items").Arr) != 1 || field(field(fs[1], "items").Arr[0], "id").Str != "AC-2" {
		t.Fatalf("queue status findings: %s", wire.Encode(wire.Array(fs...)))
	}
	if wl := worklistRead(t, root); len(field(wl, "findings").Arr) != 2 {
		t.Fatalf("worklist findings: %s", wire.Encode(wl))
	}

	// Decide the rest: the owner defers BC-2, AC-2 drops its mirror and is
	// deferred with the other local items. Only decided mirrors remain.
	if x := mirrorSet(t, root, b, "defer-b", `[{"id":"BC-2","state":"DEFERRED","core":null,"reason":"later"}]`); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("owner defer: %s", x.stdout)
	}
	rest := `[{"id":"AC-2","state":"DEFERRED","core":null,"reason":"later","satisfiedBy":null}`
	for _, id := range []string{"AC-4", "AC-5", "AC-6", "AC-7", "AC-8", "AC-9"} {
		rest += `,{"id":"` + id + `","state":"DEFERRED","core":null,"reason":"later"}`
	}
	if x := mirrorSet(t, root, a, "defer-a", rest+`]`); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("local defer: %s", x.stdout)
	}
	v = obligationsShow(t, root, a)
	if got := mirrorItems(v); len(got) != 2 || got["AC-1"] != "WITNESSED/derived/" || got["AC-3"] != "DEFERRED/derived/" || !field(field(v, "mirrors"), "completable").Bool {
		t.Fatalf("completable mirrors: %v %s", got, wire.Encode(field(v, "mirrors")))
	}
	if fs := statusMirrors(t, root); len(fs) != 1 || !field(fs[0], "completable").Bool {
		t.Fatalf("completable finding: %s", wire.Encode(wire.Array(fs...)))
	}
	if dispatch.PlanReasonMirrorsDecided != transaction.PlanReasonMirrorsDecided {
		t.Fatal("dispatch and plan reasons differ")
	}
	tickets, err := cli.ObserveTickets(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, tk := range tickets {
		want := ""
		if tk.ID == a {
			want = dispatch.PlanReasonMirrorsDecided
		}
		if (tk.PlanReason == dispatch.PlanReasonMirrorsDecided) != (want != "") {
			t.Fatalf("%s plan %s/%s", tk.ID, tk.Plan, tk.PlanReason)
		}
	}
	// The plan preview is not the dispatcher: it does not defer the ticket.
	if x := atm(t, root, nil, "plan", "preview"); x.res.Outcome != wire.OutcomeOK || !strings.Contains(string(x.stdout), a) || strings.Contains(string(x.stdout), dispatch.PlanReasonMirrorsDecided) {
		t.Fatalf("plan preview defers mirrors: %s", x.stdout)
	}
}
