package transaction

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

const timestamp wire.Timestamp = "2026-09-06T00:00:00Z"

func admin(op, id string) Request {
	return Request{Operation: op, QueueID: fixture.QueueID, RequestID: id, Actor: mutation.Binding{ID: "operator", Role: "OPERATOR"}}
}
func initialized(t *testing.T) (Input, *Plan) {
	t.Helper()
	inv, e := NewInventory(nil, nil)
	if e != nil {
		t.Fatal(e)
	}
	r := admin(Init, "init")
	r.Queue = fixture.QueueBytes()
	r.Policy = fixture.PolicyBytes()
	r.PrimaryWorktree = "/fixture"
	in := Input{Inventory: inv, Premise: FixtureNoRuntime, Replay: ReplayObservation{State: "ABSENT"}, Branch: "main", RecordedAt: timestamp}
	result := Model(r, in)
	if result.Kind != "Transaction" {
		t.Fatalf("INIT: %+v", result)
	}
	c, e := CheckCapacity(result.Plan)
	if e != nil {
		t.Fatal(e)
	}
	return Input{Inventory: c.Final, Head: result.Plan.Head(), Queue: r.Queue, Policy: r.Policy, Reservations: emptyReservations(fixture.QueueID), Premise: FixtureNoRuntime, Branch: "main", Replay: ReplayObservation{State: "ABSENT"}, RecordedAt: timestamp}, result.Plan
}
func commitModel(t *testing.T, in Input, p *Plan) Input {
	t.Helper()
	c, e := CheckCapacity(p)
	if e != nil {
		t.Fatal(e)
	}
	in.Inventory = c.Final
	in.Head = p.Head()
	if b, ok := p.posts["barrier.json"]; ok {
		in.Barrier = bytes.Clone(b)
	}
	for x, raw := range in.CanonicalTickets {
		rec, _ := ticket.Decode(raw)
		path := "intent/tickets/" + rec.TicketID.Local + ".json"
		if b, ok := p.posts[path]; ok {
			in.CanonicalTickets[x] = bytes.Clone(b)
		}
	}
	return in
}
func withTicket(t *testing.T, in Input, c *ticket.Record, physical []byte) Input {
	t.Helper()
	in.Inventory = in.Inventory.clone()
	in.Inventory.files["intent/tickets/"+c.TicketID.Local+".json"] = bytesEntry("intent/tickets/"+c.TicketID.Local+".json", physical)
	in.CanonicalTickets = [][]byte{c.Encode()}
	return in
}
func assertZero(t *testing.T, r Result, kind string) {
	t.Helper()
	if r.Kind != kind || r.Plan != nil {
		t.Fatalf("expected zero-growth %s: %+v", kind, r)
	}
	if r.Coverage != coverage() {
		t.Fatalf("coverage: %+v", r.Coverage)
	}
}
func rewriteRequestRecord(t *testing.T, raw []byte, edit func(wire.Value)) []byte {
	t.Helper()
	v, e := wire.Parse(raw)
	if e != nil {
		t.Fatal(e)
	}
	edit(v)
	return wire.EncodeFile(v)
}
func TestTMV0006_AS03_AdministrativeDigestsReplayAndRefusal(t *testing.T) {
	in, init := initialized(t)
	for _, op := range []string{Init, Pause, Unpause, KeepJournal, AdoptFile} {
		r := admin(op, "request")
		if op == Init {
			r.Queue = fixture.QueueBytes()
			r.Policy = fixture.PolicyBytes()
			r.PrimaryWorktree = "/fixture"
		}
		if op == KeepJournal || op == AdoptFile {
			r.TargetID = fixture.TicketID("T-1")
			r.File = []byte("discard")
			if op == KeepJournal {
				r.CanonicalSha256 = wire.Sum([]byte("canonical"))
			}
		}
		base, e := Digest(r)
		if e != nil {
			t.Fatal(e)
		}
		for _, change := range []func(*Request){func(x *Request) { x.Actor.ID = "other" }, func(x *Request) { x.Actor.Role = "OWNER" }, func(x *Request) { x.RequestID = "other" }} {
			x := cloneRequest(r)
			change(&x)
			d, e := Digest(x)
			if e != nil || d == base {
				t.Fatalf("digest %s %v", op, e)
			}
		}
		if op == AdoptFile {
			id, _ := wire.ParseTicketID("", r.TargetID)
			if base != mutation.AdoptDigest(r.Actor, mustQueue(r.QueueID), id, r.RequestID, r.File) {
				t.Fatal("ADOPT digest parity")
			}
		}
	}
	reqPath, _ := snapshot.RequestPath("init")
	original := init.posts[reqPath]
	replayIn := Input{Replay: ReplayObservation{State: "FOUND", Record: original}}
	got := Model(init.request, replayIn)
	assertZero(t, got, "Replay")
	if !got.Outcome.Replayed || got.Outcome.ReceiptSeq == nil || *got.Outcome.ReceiptSeq != "1" {
		t.Fatal(got)
	}
	changed := cloneRequest(init.request)
	changed.Actor.ID = "other"
	got = Model(changed, replayIn)
	assertZero(t, got, "Refused")
	if got.Outcome.Outcome != mutation.OutcomeRequestIDConflict {
		t.Fatal(got)
	}
	for _, state := range []string{"", "UNKNOWN", "ERROR"} {
		in.Replay = ReplayObservation{State: state}
		assertZero(t, Model(admin(Pause, "p"), in), "Refused")
	}
	in.Replay = ReplayObservation{State: "ABSENT"}
	assertZero(t, Model(init.request, in), "Refused")
	corrupt := bytes.Replace(original, []byte(`"requestId":"init"`), []byte(`"requestId":"oops"`), 1)
	replayIn.Replay.Record = corrupt
	assertZero(t, Model(init.request, replayIn), "Refused")
}
func TestTMV0006_AS03_ReplayConflictPrecedesOperationShape(t *testing.T) {
	in, _ := initialized(t)
	adminRequest := admin(Pause, "admin-to-ticket")
	pause := Model(adminRequest, in)
	if pause.Kind != "Transaction" {
		t.Fatal(pause)
	}
	pausePath, _ := snapshot.RequestPath(adminRequest.RequestID)
	pauseRecord := pause.Plan.posts[pausePath]

	c := fixture.Ticket("T-1")
	in = withTicket(t, in, c, []byte("divergent"))
	keepRequest := admin(KeepJournal, adminRequest.RequestID)
	keepRequest.TargetID = c.TicketID.Raw
	keepRequest.File = []byte("divergent")
	keepRequest.CanonicalSha256 = c.FileDigest()
	got := Model(keepRequest, Input{Replay: ReplayObservation{State: "FOUND", Record: pauseRecord}})
	assertZero(t, got, "Refused")
	if got.Outcome.Outcome != mutation.OutcomeRequestIDConflict || !got.Outcome.HasCode(wire.CodeRequestIDConflict) {
		t.Fatal("administrative-to-reconciliation conflict", got)
	}

	keepRequest.RequestID = "ticket-to-admin"
	keep := Model(keepRequest, in)
	if keep.Kind != "Transaction" {
		t.Fatal(keep)
	}
	keepPath, _ := snapshot.RequestPath(keepRequest.RequestID)
	keepRecord := keep.Plan.posts[keepPath]
	conflictingPause := admin(Pause, keepRequest.RequestID)
	got = Model(conflictingPause, Input{Replay: ReplayObservation{State: "FOUND", Record: keepRecord}})
	assertZero(t, got, "Refused")
	if got.Outcome.Outcome != mutation.OutcomeRequestIDConflict || !got.Outcome.HasCode(wire.CodeRequestIDConflict) {
		t.Fatal("reconciliation-to-administrative conflict", got)
	}

	badPauseShape := rewriteRequestRecord(t, pauseRecord, func(v wire.Value) {
		out, _ := v.Obj.Get("outcome")
		out.Obj.Set("resultingRevision", s("1"))
		out.Obj.Set("resultingAcceptanceRevision", s("1"))
	})
	got = Model(adminRequest, Input{Replay: ReplayObservation{State: "FOUND", Record: badPauseShape}})
	assertZero(t, got, "Refused")
	if got.Outcome.Outcome != mutation.OutcomeValidationFailed || !got.Outcome.HasCode(wire.CodeMalformed) {
		t.Fatal("same-digest administrative shape accepted", got)
	}

	badKeepShape := rewriteRequestRecord(t, keepRecord, func(v wire.Value) {
		out, _ := v.Obj.Get("outcome")
		out.Obj.Set("resultingRevision", wire.Null())
		out.Obj.Set("resultingAcceptanceRevision", wire.Null())
	})
	got = Model(keepRequest, Input{Replay: ReplayObservation{State: "FOUND", Record: badKeepShape}})
	assertZero(t, got, "Refused")
	if got.Outcome.Outcome != mutation.OutcomeValidationFailed || !got.Outcome.HasCode(wire.CodeMalformed) {
		t.Fatal("same-digest reconciliation shape accepted", got)
	}

	for name, corrupt := range map[string][]byte{
		"malformed":    []byte("{"),
		"noncanonical": bytes.TrimSuffix(pauseRecord, []byte("\n")),
		"identity": rewriteRequestRecord(t, pauseRecord, func(v wire.Value) {
			v.Obj.Set("requestId", s("other-request"))
			out, _ := v.Obj.Get("outcome")
			out.Obj.Set("requestId", s("other-request"))
		}),
		"sequence": rewriteRequestRecord(t, pauseRecord, func(v wire.Value) {
			v.Obj.Set("seq", s("1000001"))
			out, _ := v.Obj.Get("outcome")
			out.Obj.Set("receiptSeq", s("1000001"))
		}),
	} {
		t.Run(name, func(t *testing.T) {
			changed := admin(Pause, adminRequest.RequestID)
			changed.Actor.ID = "different-actor"
			got := Model(changed, Input{Replay: ReplayObservation{State: "FOUND", Record: corrupt}})
			assertZero(t, got, "Refused")
			if got.Outcome.Outcome != mutation.OutcomeValidationFailed || !got.Outcome.HasCode(wire.CodeMalformed) {
				t.Fatal("corrupt source classified as digest conflict", got)
			}
		})
	}
}
func TestTMV0007_AS35_UnpauseDivergenceValidationBoundary(t *testing.T) {
	in := paused(t)
	c := fixture.Ticket("T-1")
	in = withTicket(t, in, c, []byte("divergent projection"))
	if got := Model(admin(Unpause, "allowed-divergence"), in); got.Kind != "Transaction" {
		t.Fatal("well-formed divergent inventory refused", got)
	}

	otherOperation := Model(admin(Pause, "still-strict"), in)
	assertZero(t, otherOperation, "Refused")
	if otherOperation.Outcome.Outcome != mutation.OutcomeValidationFailed || !otherOperation.Outcome.HasCode(wire.CodeMalformed) {
		t.Fatal("non-UNPAUSE projection validation weakened", otherOperation)
	}

	absent := in
	absent.Inventory = in.Inventory.clone()
	delete(absent.Inventory.files, "intent/tickets/T-1.json")
	got := Model(admin(Unpause, "absent-projection"), absent)
	assertZero(t, got, "Refused")
	if got.Outcome.Outcome != mutation.OutcomeValidationFailed || !got.Outcome.HasCode(wire.CodeMalformed) {
		t.Fatal("absent physical projection admitted", got)
	}

	extra := in
	extra.Inventory = in.Inventory.clone()
	extra.Inventory.files["intent/tickets/T-2.json"] = bytesEntry("intent/tickets/T-2.json", []byte{})
	got = Model(admin(Unpause, "incomplete-canonical"), extra)
	assertZero(t, got, "Refused")
	if got.Outcome.Outcome != mutation.OutcomeValidationFailed || !got.Outcome.HasCode(wire.CodeMalformed) {
		t.Fatal("incomplete canonical inventory admitted", got)
	}

	malformedCanonical := in
	malformedCanonical.CanonicalTickets = [][]byte{[]byte("{")}
	got = Model(admin(Unpause, "malformed-canonical"), malformedCanonical)
	assertZero(t, got, "Refused")
	if got.Outcome.Outcome != mutation.OutcomeValidationFailed || !got.Outcome.HasCode(wire.CodeMalformed) {
		t.Fatal("malformed canonical record admitted", got)
	}
}
func TestTMV0016_AS27_BarrierTemplatesExemptionsAndNoChange(t *testing.T) {
	in, _ := initialized(t)
	assertZero(t, Model(admin(Unpause, "absent"), in), "NoChange")
	pause := Model(admin(Pause, "pause"), in)
	if pause.Kind != "Transaction" {
		t.Fatal(pause)
	}
	in = commitModel(t, in, pause.Plan)
	assertZero(t, Model(admin(Pause, "again"), in), "NoChange")
	for _, scope := range []string{"ADMISSION", "ALL"} {
		for _, reason := range []string{"OPERATOR", "CUTOVER", "EMERGENCY", "DRAIN"} {
			x := in
			x.Inventory = in.Inventory.clone()
			v, _ := wire.Parse(in.Barrier)
			v.Obj.Set("scope", s(scope))
			v.Obj.Set("reason", s(reason))
			x.Barrier = wire.EncodeFile(v)
			x.Inventory.files["barrier.json"] = bytesEntry("barrier.json", x.Barrier)
			if scope != "ADMISSION" || reason != "OPERATOR" {
				r := Model(admin(Pause, "blocked"), x)
				assertZero(t, r, "Refused")
				if !r.Outcome.HasCode(wire.CodePaused) {
					t.Fatal(r)
				}
			}

			c := fixture.Ticket("T-1")
			x = withTicket(t, x, c, c.Encode())
			m := admin(Mutate, "ticket-"+scope+"-"+reason)
			m.Envelope = wire.EncodeFile(object("profile", s(mutation.Profile), "requestId", s(m.RequestID), "actor", object("id", s(m.Actor.ID), "role", s(m.Actor.Role)), "queueId", s(fixture.QueueID), "targetId", s(c.TicketID.Raw), "expectedRevision", s("1"), "operation", s(mutation.OpPrioritize), "payload", object("priority", s("P1"), "order", s("0")), "issuedAt", s(string(timestamp))))
			changed := Model(m, x)
			if scope == "ALL" {
				assertZero(t, changed, "Refused")
				if !changed.Outcome.HasCode(wire.CodePaused) {
					t.Fatal(changed)
				}
			} else {
				if changed.Kind != "Transaction" {
					t.Fatal(changed)
				}
				path, _ := snapshot.RequestPath(m.RequestID)
				replayInput := Input{Replay: ReplayObservation{State: "FOUND", Record: changed.Plan.posts[path]}}
				m.Actor.ID = "other"
				denied := Model(m, replayInput)
				if denied.Outcome.Outcome != mutation.OutcomeUnauthorized {
					t.Fatal("replay bypassed actor binding", denied)
				}
			}
			u := Model(admin(Unpause, "unpause"), x)
			if u.Kind != "Transaction" {
				t.Fatal(u)
			}
			rc, e := snapshot.DecodeReceipt(u.Plan.Receipt())
			if e != nil {
				t.Fatal(e)
			}
			if len(rc.Post) != 2 || rc.Post[0].Path != "barrier.json" || rc.Post[0].Sha256 != nil || *rc.Pre[0].Sha256 != wire.Sum(x.Barrier) {
				t.Fatal("paired deletion")
			}
			for _, op := range []string{KeepJournal, AdoptFile} {
				c := fixture.Ticket("T-1")
				x = withTicket(t, x, c, c.Encode())
				r := admin(op, "reconcile-"+op)
				r.TargetID = c.TicketID.Raw
				r.File = c.Encode()
				if op == KeepJournal {
					r.CanonicalSha256 = c.FileDigest()
				}
				result := Model(r, x)
				want := "Transaction"
				if op == KeepJournal {
					want = "NoChange"
				}
				if result.Kind != want {
					t.Fatalf("barrier exemption %s %s %s: %+v", scope, reason, op, result)
				}
			}
		}
	}
}
func TestTMV0007_AS35_KeepPhysicalCanonicalAndEmptyEvidence(t *testing.T) {
	for _, discard := range [][]byte{[]byte{}, []byte("{malformed"), []byte(strings.Repeat("x", 70000))} {
		in, _ := initialized(t)
		c := fixture.Ticket("T-1")
		in = withTicket(t, in, c, discard)
		r := admin(KeepJournal, "keep")
		r.TargetID = c.TicketID.Raw
		r.File = discard
		r.CanonicalSha256 = c.FileDigest()
		result := Model(r, in)
		if result.Kind != "Transaction" {
			t.Fatal(result)
		}
		p := result.Plan
		rc, e := snapshot.DecodeReceipt(p.Receipt())
		if e != nil {
			t.Fatal(e)
		}
		path := "intent/tickets/T-1.json"
		if !bytes.Equal(p.posts[path], c.Encode()) {
			t.Fatal("KEEP changed canonical")
		}
		for j, post := range rc.Post {
			if post.Path == path && *rc.Pre[j].Sha256 != wire.Sum(discard) {
				t.Fatal("physical pre is not D")
			}
		}
		ep := "evidence/" + string(wire.Sum(discard))
		if p.posts[ep] == nil || !bytes.Equal(p.posts[ep], discard) {
			t.Fatal("empty evidence lost")
		}
		occurrences := 0
		for _, a := range p.artifacts {
			if a.Target == ep {
				occurrences++
				if a.Role != "POST" {
					t.Fatal("cross-role dedup")
				}
			}
		}
		if occurrences != 1 {
			t.Fatal(occurrences)
		}
		if *result.Outcome.ResultingRevision != c.Revision || *result.Outcome.ResultingAcceptanceRevision != c.AcceptanceRevision {
			t.Fatal("KEEP revisions")
		}
		in = commitModel(t, in, p)
		rp, _ := snapshot.RequestPath(r.RequestID)
		in.Replay = ReplayObservation{State: "FOUND", Record: p.posts[rp]}
		assertZero(t, Model(r, in), "Replay")
		in.Replay = ReplayObservation{State: "ABSENT"}
		r.RequestID = "fresh"
		assertZero(t, Model(r, in), "Refused")
	}
}
func TestTMV0007_AS35_AdoptReducerParityAndAcceptance(t *testing.T) {
	for _, acceptance := range []bool{false, true} {
		in, _ := initialized(t)
		c := fixture.Ticket("T-1")
		offered := fixture.Ticket("T-1")
		if acceptance {
			offered.AcceptanceCriteria = []string{"changed acceptance"}
		}
		in = withTicket(t, in, c, offered.Encode())
		r := admin(AdoptFile, "adopt")
		r.TargetID = c.TicketID.Raw
		r.File = offered.Encode()
		result := Model(r, in)
		if result.Kind != "Transaction" {
			t.Fatal(result)
		}
		st, e := validateInput(r, in)
		if e != nil {
			t.Fatal(e)
		}
		oracle := mutation.Adopt(mutation.Context{Binding: r.Actor, Queue: st.queue, Policy: st.policy, Inventory: st.tickets, Requests: absentIndex{}, Attempts: zeroAttempts{}, Now: timestamp}, r.RequestID, c, r.File)
		got := result.Plan.posts["intent/tickets/T-1.json"]
		if !bytes.Equal(got, oracle.Post.Encode()) {
			t.Fatal("ADOPT reducer drift")
		}
		post, _ := ticket.Decode(got)
		if post.Revision != "2" || *post.PreviousRecordSha256 != c.FileDigest() {
			t.Fatal("chain")
		}
		want := wire.Count("1")
		if acceptance {
			want = "2"
		}
		if post.AcceptanceRevision != want {
			t.Fatal("acceptance revision")
		}
		if *result.Outcome.ResultingRevision != post.Revision || *result.Outcome.ResultingAcceptanceRevision != post.AcceptanceRevision {
			t.Fatal("outcome revisions")
		}
	}
	// Existing reducer policy restrictions remain the source of refusal.
	in, _ := initialized(t)
	c := fixture.Ticket("T-1")
	offered := fixture.Ticket("T-1")
	offered.Title = "edited"
	in = withTicket(t, in, c, offered.Encode())
	v, _ := wire.Parse(in.Policy)
	v.Obj.Set("roles", object("OPERATOR", wire.Array()))
	in.Policy = wire.EncodeFile(v)
	in.Inventory.files["intent/policy.json"] = bytesEntry("intent/policy.json", in.Policy)
	r := admin(AdoptFile, "denied")
	r.TargetID = c.TicketID.Raw
	r.File = offered.Encode()
	result := Model(r, in)
	assertZero(t, result, "Refused")
	if result.Outcome.Outcome != mutation.OutcomeUnauthorized {
		t.Fatal(result)
	}
}
func TestTMV0002_AS10_ImmutableInputsAndExplicitScope(t *testing.T) {
	in, _ := initialized(t)
	before := in.Inventory.Files()
	r := admin(Pause, "pause")
	result := Model(r, in)
	if result.Kind != "Transaction" {
		t.Fatal(result)
	}
	if len(in.Inventory.Files()) != len(before) {
		t.Fatal("input inventory mutated")
	}
	raw := result.Plan.Receipt()
	raw[0] = '!'
	if result.Plan.Receipt()[0] != '{' {
		t.Fatal("receipt alias")
	}
	a := result.Plan.Artifacts()
	a[0].Data[0] = '!'
	if bytes.Equal(a[0].Data, result.Plan.Artifacts()[0].Data) {
		t.Fatal("artifact alias")
	}
	for _, change := range []func(*Input){func(x *Input) { x.Premise = "" }, func(x *Input) { x.Reservations = []byte("unknown") }, func(x *Input) { x.Inventory = nil }, func(x *Input) { x.Barrier = []byte{} }, func(x *Input) {
		v, _ := wire.Parse(x.Queue)
		v.Obj.Set("fixture", wire.Bool(false))
		x.Queue = wire.EncodeFile(v)
	}} {
		x := in
		change(&x)
		assertZero(t, Model(r, x), "Refused")
	}
}

func TestTMV0006_AS03_AllPreimageFieldsAndOriginalChoices(t *testing.T) {
	c := fixture.Ticket("T-1")
	keep := admin(KeepJournal, "request")
	keep.TargetID = c.TicketID.Raw
	keep.File = []byte("discard")
	keep.CanonicalSha256 = c.FileDigest()
	base, e := Digest(keep)
	if e != nil {
		t.Fatal(e)
	}
	for _, change := range []func(*Request){func(r *Request) { r.CanonicalSha256 = wire.Sum([]byte("other canonical")) }, func(r *Request) { r.File = []byte("other physical") }, func(r *Request) { r.TargetID = fixture.TicketID("T-2") }, func(r *Request) { r.QueueID = "queue:acme:other"; r.TargetID = "ticket:acme:other:T-1" }} {
		r := cloneRequest(keep)
		change(&r)
		d, e := Digest(r)
		if e != nil || d == base {
			t.Fatal("KEEP binding", e)
		}
	}
	in, p := initialized(t)
	base, e = Digest(p.request)
	if e != nil {
		t.Fatal(e)
	}
	for _, change := range []func(*Request){func(r *Request) { r.PrimaryWorktree = "/other" }, func(r *Request) {
		v, _ := wire.Parse(r.Queue)
		v.Obj.Set("nextSerial", s("3"))
		r.Queue = wire.EncodeFile(v)
	}, func(r *Request) {
		v, _ := wire.Parse(r.Policy)
		v.Obj.Set("policyVersion", s("2"))
		r.Policy = wire.EncodeFile(v)
	}} {
		r := cloneRequest(p.request)
		change(&r)
		d, e := Digest(r)
		if e != nil || d == base {
			t.Fatal("INIT binding", e)
		}
	}
	r := admin(Pause, "new")
	a := Model(r, in)
	in.RecordedAt = "2026-09-07T00:00:00Z"
	b := Model(r, in)
	if a.Kind != "Transaction" || b.Kind != "Transaction" {
		t.Fatal(a, b)
	}
	da, _ := DecodeDescriptor(a.Plan.Descriptor())
	db, _ := DecodeDescriptor(b.Plan.Descriptor())
	if da.RequestSha256 != db.RequestSha256 || bytes.Equal(a.Plan.Descriptor(), b.Plan.Descriptor()) {
		t.Fatal("timestamp frozen only in plan")
	}
	o := stageObservation(a.Plan)
	if _, e = ClassifyStage(o, b.Plan); e == nil {
		t.Fatal("retry regenerated different artifacts")
	}
	r.Queue = []byte("extra")
	if _, e = Digest(r); e == nil {
		t.Fatal("closed request fields")
	}
}
func TestTMV0009_AS11_InitRetainsOrphansAndUsesSixNullPreimages(t *testing.T) {
	_, p := initialized(t)
	inv, e := NewInventory(nil, []string{"staging", "evidence"})
	if e != nil {
		t.Fatal(e)
	}
	for _, a := range p.Artifacts() {
		if a.Role == "EVIDENCE" {
			if e = inv.put(entry(a), true); e != nil {
				t.Fatal(e)
			}
		}
	}
	in := Input{Inventory: inv, Premise: FixtureNoRuntime, Replay: ReplayObservation{State: "ABSENT"}, RecordedAt: timestamp, Branch: "main"}
	result := Model(p.request, in)
	if result.Kind != "Transaction" {
		t.Fatal(result)
	}
	rc, e := snapshot.DecodeReceipt(result.Plan.Receipt())
	if e != nil {
		t.Fatal(e)
	}
	if len(rc.Pre) != 6 || len(rc.Post) != 6 {
		t.Fatal("indexed genesis shape")
	}
	for _, pre := range rc.Pre {
		if pre.Sha256 != nil {
			t.Fatal("genesis pre must be absent")
		}
	}
	for _, a := range result.Plan.Artifacts() {
		if a.Role == "EVIDENCE" {
			t.Fatal("already linked blob staged again")
		}
	}
}
