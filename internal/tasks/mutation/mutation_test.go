package mutation_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// now is the logical timestamp every test supplies; it is later than the
// fixture's record timestamp so updatedAt visibly changes.
const now = wire.Timestamp("2026-09-06T13:00:00Z")

var (
	owner    = mutation.Binding{ID: "russell", Role: "OWNER"}
	operator = mutation.Binding{ID: "ops", Role: "OPERATOR"}
	worker   = mutation.Binding{ID: "lane-1", Role: "WORKER"}
	reviewer = mutation.Binding{ID: "rev-1", Role: "REVIEWER"}
	importer = mutation.Binding{ID: "importer", Role: "IMPORTER"}
	system   = mutation.Binding{ID: "atm", Role: "SYSTEM"}
)

// attempts is a test attempt oracle: listed tickets have a live attempt,
// every other ticket is observed not to.
type attempts map[string]bool

func (a attempts) LiveAttempt(id string) ticket.Observation {
	if a[id] {
		return ticket.Satisfied
	}
	return ticket.Unsatisfied
}

func obj(kv ...interface{}) wire.Value {
	o := wire.NewObject()
	for i := 0; i+1 < len(kv); i += 2 {
		o.Set(kv[i].(string), kv[i+1].(wire.Value))
	}
	return wire.ObjectValue(o)
}

func str(s string) wire.Value { return wire.String(s) }

func strOrNull(s string) wire.Value {
	if s == "" {
		return wire.Null()
	}
	return str(s)
}

func newCtx(t *testing.T, b mutation.Binding, live attempts, recs ...*ticket.Record) mutation.Context {
	t.Helper()
	return ctxWithPolicy(t, b, live, fixture.PolicyBytes(), recs...)
}

func ctxWithPolicy(t *testing.T, b mutation.Binding, live attempts, policy []byte, recs ...*ticket.Record) mutation.Context {
	t.Helper()
	q, err := intent.DecodeQueue(fixture.QueueBytes())
	if err != nil {
		t.Fatalf("queue: %v", err)
	}
	p, err := intent.DecodePolicy(policy)
	if err != nil {
		t.Fatalf("policy: %v", err)
	}
	qid, _ := wire.ParseQueueID("", fixture.QueueID)
	inv, err := ticket.NewInventory(qid, recs)
	if err != nil {
		t.Fatalf("inventory: %v", err)
	}
	if live == nil {
		live = attempts{}
	}
	return mutation.Context{Binding: b, Queue: q, Policy: p, Inventory: inv, Attempts: live, Requests: mutation.NewMemoryIndex(), Now: now}
}

// envelope renders a canonical envelope. target "" and rev "" render null
// (the CREATE form).
func envelope(reqID string, actor mutation.Binding, target, rev, op string, payload wire.Value) []byte {
	tid := wire.Null()
	if target != "" {
		tid = str(fixture.TicketID(target))
	}
	return wire.EncodeFile(obj(
		"profile", str(mutation.Profile),
		"requestId", str(reqID),
		"actor", obj("id", str(actor.ID), "role", str(actor.Role)),
		"queueId", str(fixture.QueueID),
		"targetId", tid,
		"expectedRevision", strOrNull(rev),
		"operation", str(op),
		"payload", payload,
		"issuedAt", str(string(now)),
	))
}

func createPayload(local string, kind string, criteria []string, sourceKind string) wire.Value {
	src := obj("kind", str("NATIVE"), "sourceQueueId", str(fixture.QueueID), "sourceItemId", wire.Null(), "sourceRevisionSha256", wire.Null())
	if sourceKind == "IMPORT" {
		src = obj("kind", str("IMPORT"), "sourceQueueId", str("queue:ext:src"), "sourceItemId", str("X-1"), "sourceRevisionSha256", wire.Null())
	}
	o := wire.NewObject()
	if local != "" {
		o.Set("localToken", str(local))
	}
	o.Set("title", str("Created ticket"))
	o.Set("body", wire.Null())
	o.Set("kind", str(kind))
	o.Set("owner", wire.Null())
	o.Set("milestone", wire.Null())
	o.Set("priority", str("P2"))
	o.Set("order", str("0"))
	o.Set("labels", wire.Strings(nil))
	o.Set("dependencies", wire.Array())
	o.Set("acceptanceCriteria", wire.Strings(criteria))
	o.Set("requirementRefs", wire.Strings(nil))
	o.Set("source", src)
	o.Set("effects", obj("coverage", str("QUALIFIED"), "touchPaths", wire.Strings(nil), "resources", wire.Array(), "externalUnbounded", wire.Bool(false)))
	o.Set("capabilities", wire.Strings(nil))
	o.Set("requiredGates", wire.Strings(nil))
	o.Set("executionClass", str("AUTONOMOUS"))
	o.Set("dueDate", wire.Null())
	o.Set("estimateMinutes", wire.Null())
	o.Set("supersedes", wire.Null())
	o.Set("supersededBy", wire.Null())
	return wire.ObjectValue(o)
}

func depValue(local, obligation, gate string) wire.Value {
	return obj("ticketId", str(fixture.TicketID(local)), "obligation", str(obligation), "gateId", strOrNull(gate))
}

func apply(t *testing.T, ctx mutation.Context, raw []byte) *mutation.Plan {
	t.Helper()
	env, err := mutation.Decode(raw)
	if err != nil {
		t.Fatalf("decode: %v\n%s", err, raw)
	}
	return mutation.Apply(ctx, env)
}

func want(t *testing.T, plan *mutation.Plan, outcome, code string) {
	t.Helper()
	if plan.Outcome.Outcome != outcome {
		t.Fatalf("outcome %s (codes %v, detail %q); want %s", plan.Outcome.Outcome, plan.Outcome.Codes, plan.Detail, outcome)
	}
	if code == "" {
		if len(plan.Outcome.Codes) != 0 {
			t.Fatalf("codes %v; want none", plan.Outcome.Codes)
		}
	} else if !plan.Outcome.HasCode(code) {
		t.Fatalf("codes %v (detail %q); want %s", plan.Outcome.Codes, plan.Detail, code)
	}
	if outcome != mutation.OutcomeCompleted {
		if plan.Post != nil || plan.QueuePost != nil {
			t.Fatalf("%s carries a post record: nothing may be written", outcome)
		}
		if plan.Outcome.ResultingRevision != nil || plan.Outcome.ReceiptSeq != nil {
			t.Fatalf("%s carries resulting revision or receipt", outcome)
		}
	} else if !plan.Outcome.Replayed {
		if plan.Post == nil {
			t.Fatalf("COMPLETED without a post record")
		}
		if plan.Outcome.ReceiptSeq != nil {
			t.Fatalf("a planned result must not carry a receiptSeq")
		}
		if _, err := ticket.Decode(plan.Post.Encode()); err != nil {
			t.Fatalf("post record does not validate: %v", err)
		}
	}
}

// chain checks that post is exactly one revision after pre and chained to
// pre's file digest, with the acceptance revision as stated.
func chain(t *testing.T, pre, post *ticket.Record, acc string) {
	t.Helper()
	if post.Revision.Int() != pre.Revision.Int()+1 {
		t.Fatalf("revision %s after %s", post.Revision, pre.Revision)
	}
	if post.PreviousRecordSha256 == nil || *post.PreviousRecordSha256 != pre.FileDigest() {
		t.Fatalf("previousRecordSha256 is not the canonical record's digest")
	}
	if string(post.AcceptanceRevision) != acc {
		t.Fatalf("acceptanceRevision %s; want %s", post.AcceptanceRevision, acc)
	}
	if post.UpdatedAt != now || post.UpdatedBy == "" {
		t.Fatalf("updatedAt/updatedBy not set from the context: %s/%s", post.UpdatedAt, post.UpdatedBy)
	}
}

// TestTMV0002_AS01_EnvelopeClosedSchema covers unknown, missing and wrong
// keys of the envelope and its payloads, the unsupported profile version,
// the nullity rule, bounds and the closed operation set.
func TestTMV0002_AS01_EnvelopeClosedSchema(t *testing.T) {
	good := string(envelope("r1", owner, "AT-01", "1", mutation.OpRefine, obj("title", str("x"))))
	if _, err := mutation.Decode([]byte(good)); err != nil {
		t.Fatalf("good envelope: %v", err)
	}
	edits := []struct {
		name string
		edit func(string) string
		want string
	}{
		{"unknown envelope key", func(s string) string { return strings.Replace(s, `"issuedAt":`, `"extra":"x","issuedAt":`, 1) }, wire.CodeMalformed},
		{"missing envelope key", func(s string) string { return strings.Replace(s, `"issuedAt":"`+string(now)+`",`, ``, 1) }, wire.CodeMalformed},
		{"unknown payload key", func(s string) string { return strings.Replace(s, `{"title":"x"}`, `{"colour":"x","title":"x"}`, 1) }, wire.CodeMalformed},
		{"empty REFINE payload", func(s string) string { return strings.Replace(s, `{"title":"x"}`, `{}`, 1) }, wire.CodeMalformed},
		{"unsupported version", func(s string) string { return strings.Replace(s, "taskman-mutation/0", "taskman-mutation/1", 1) }, wire.CodeUnsupportedVersion},
		{"wrong profile", func(s string) string { return strings.Replace(s, "taskman-mutation/0", "taskman-ticket/0", 1) }, wire.CodeMalformed},
		{"unknown operation", func(s string) string { return strings.Replace(s, `"operation":"REFINE"`, `"operation":"NUKE"`, 1) }, wire.CodeMalformed},
		{"unknown role", func(s string) string { return strings.Replace(s, `"role":"OWNER"`, `"role":"ROOT"`, 1) }, wire.CodeMalformed},
		{"null expectedRevision for REFINE", func(s string) string {
			return strings.Replace(s, `"expectedRevision":"1"`, `"expectedRevision":null`, 1)
		}, wire.CodeMalformed},
		{"JSON number", func(s string) string { return strings.Replace(s, `"expectedRevision":"1"`, `"expectedRevision":1`, 1) }, wire.CodeMalformed},
		{"target outside queue", func(s string) string {
			return strings.Replace(s, `"targetId":"ticket:acme:main:AT-01"`, `"targetId":"ticket:acme:other:AT-01"`, 1)
		}, wire.CodeMalformed},
		{"requestId over 64 bytes", func(s string) string {
			return strings.Replace(s, `"requestId":"r1"`, `"requestId":"`+strings.Repeat("r", 65)+`"`, 1)
		}, wire.CodeLimitExceeded},
		{"non-canonical whitespace", func(s string) string { return strings.Replace(s, `"title":"x"`, `"title": "x"`, 1) }, wire.CodeMalformed},
		{"missing trailing LF", func(s string) string { return strings.TrimSuffix(s, "\n") }, wire.CodeMalformed},
	}
	for _, c := range edits {
		_, err := mutation.Decode([]byte(c.edit(good)))
		if wire.CodeOf(err) != c.want {
			t.Errorf("%s: got %v; want %s", c.name, err, c.want)
		}
	}
	cases := []struct {
		name string
		raw  []byte
		want string
	}{
		{"CREATE with targetId", envelope("r", owner, "AT-01", "", mutation.OpCreate, createPayload("", "FEATURE", []string{"ac"}, "NATIVE")), wire.CodeMalformed},
		{"CREATE with expectedRevision", envelope("r", owner, "", "1", mutation.OpCreate, createPayload("", "FEATURE", []string{"ac"}, "NATIVE")), wire.CodeMalformed},
		{"CREATE missing payload key", envelope("r", owner, "", "", mutation.OpCreate, obj("title", str("x"))), wire.CodeMalformed},
		{"invalid priority", envelope("r", owner, "AT-01", "1", mutation.OpPrioritize, obj("priority", str("P9"), "order", str("1"))), wire.CodeInvalidPriority},
		{"order with leading zero", envelope("r", owner, "AT-01", "1", mutation.OpPrioritize, obj("priority", str("P1"), "order", str("01"))), wire.CodeMalformed},
		{"order over Count max", envelope("r", owner, "AT-01", "1", mutation.OpPrioritize, obj("priority", str("P1"), "order", str("2147483648"))), wire.CodeMalformed},
		{"HOLD payload extra key", envelope("r", owner, "AT-01", "1", mutation.OpHold, obj("holdId", str("h"), "reason", str("r"), "actor", str("x"))), wire.CodeMalformed},
		{"RELEASE_HOLD payload wrong key", envelope("r", owner, "AT-01", "1", mutation.OpReleaseHold, obj("reason", str("r"))), wire.CodeMalformed},
		{"GRANT with revoked key", envelope("r", owner, "AT-01", "1", mutation.OpGrantApproval, obj("grantId", str("g"), "actor", str("russell"), "operation", str("RUN"), "targetRevision", str("1"), "scope", wire.Strings(nil), "revoked", wire.Bool(false))), wire.CodeMalformed},
		{"GRANT bad operation", envelope("r", owner, "AT-01", "1", mutation.OpGrantApproval, obj("grantId", str("g"), "actor", str("russell"), "operation", str("DEPLOY"), "targetRevision", str("1"), "scope", wire.Strings(nil))), wire.CodeMalformed},
		{"COMPLETE_MANUAL bad digest", envelope("r", owner, "AT-01", "1", mutation.OpCompleteManual, obj("reason", str("done"), "evidence", wire.Strings([]string{"abc"}))), wire.CodeMalformed},
		{"SET_DEPENDENCIES gateId without GATE_PASSED is a record rule, but a bad obligation is closed", envelope("r", owner, "AT-01", "1", mutation.OpSetDependencies, obj("dependencies", wire.Array(depValue("AT-02", "DONE", "")))), wire.CodeMalformed},
		{"REFINE title over 512 bytes", envelope("r", owner, "AT-01", "1", mutation.OpRefine, obj("title", str(strings.Repeat("t", 513)))), wire.CodeLimitExceeded},
		{"REFINE labels unsorted", envelope("r", owner, "AT-01", "1", mutation.OpRefine, obj("labels", wire.Strings([]string{"b", "a"}))), wire.CodeMalformed},
		{"REFINE hostile code point", envelope("r", owner, "AT-01", "1", mutation.OpRefine, obj("title", str("bad"+string(rune(0x202E))+"text"))), wire.CodeMalformed},
		{"envelope over 256 KiB", envelope("r", owner, "AT-01", "1", mutation.OpRefine, obj("body", str(strings.Repeat("b", 257*1024)))), wire.CodeLimitExceeded},
	}
	for _, c := range cases {
		_, err := mutation.Decode(c.raw)
		if wire.CodeOf(err) != c.want {
			t.Errorf("%s: got %v; want %s", c.name, err, c.want)
		}
	}
}

// TestTMV0002_EnvelopeRoundTrip proves Decode and Value are inverses for a
// CREATE with localToken and a REFINE subset.
func TestTMV0002_EnvelopeRoundTrip(t *testing.T) {
	for _, raw := range [][]byte{
		envelope("r1", owner, "", "", mutation.OpCreate, createPayload("AT-07", "FEATURE", []string{"ac"}, "NATIVE")),
		envelope("r2", owner, "AT-01", "3", mutation.OpRefine, obj("body", wire.Null(), "labels", wire.Strings([]string{"x"}), "title", str("t"))),
		envelope("r3", owner, "AT-01", "3", mutation.OpCompleteManual, obj("reason", str("ok"), "evidence", wire.Strings([]string{string(wire.Sum([]byte("e")))}))),
	} {
		env, err := mutation.Decode(raw)
		if err != nil {
			t.Fatalf("decode: %v", err)
		}
		if got := wire.EncodeFile(env.Value()); string(got) != string(raw) {
			t.Fatalf("round trip differs:\n%s\n%s", raw, got)
		}
		if env.Sha256() != wire.Sum(raw) {
			t.Fatalf("envelope digest is not the digest of its bytes")
		}
	}
}

// TestTMV0005_AS02_CreateAndRefine: native CREATE allocates the serial from
// the explicit queue input and returns the incremented manifest as output;
// explicit tokens are honoured; case-folded collisions refuse; REFINE bumps
// revision, chains, and bumps acceptanceRevision only for relevant fields.
func TestTMV0005_AS02_CreateAndRefine(t *testing.T) {
	at01 := fixture.Ticket("AT-01")
	ctx := newCtx(t, owner, nil, at01)

	plan := apply(t, ctx, envelope("c1", owner, "", "", mutation.OpCreate, createPayload("", "FEATURE", []string{"ac"}, "NATIVE")))
	want(t, plan, mutation.OutcomeCompleted, "")
	if plan.Post.TicketID.Local != "AT-0002" {
		t.Fatalf("allocated %s; want AT-0002 from nextSerial 2", plan.Post.TicketID.Local)
	}
	if plan.QueuePost == nil || plan.QueuePost.NextSerial != "3" {
		t.Fatalf("queue post must carry nextSerial 3")
	}
	if ctx.Queue.NextSerial != "2" {
		t.Fatalf("input queue manifest was mutated")
	}
	if plan.Post.Revision != "1" || plan.Post.AcceptanceRevision != "1" || plan.Post.PreviousRecordSha256 != nil {
		t.Fatalf("new record must start at revision 1 with a null chain")
	}
	if plan.Post.Status != ticket.StatusOpen || plan.Post.CreatedAt != now || plan.Post.UpdatedBy != "russell" {
		t.Fatalf("new record status/time/actor wrong: %s %s %s", plan.Post.Status, plan.Post.CreatedAt, plan.Post.UpdatedBy)
	}
	if plan.Outcome.ResultingRevision == nil || *plan.Outcome.ResultingRevision != "1" {
		t.Fatalf("outcome resulting revision")
	}
	if !plan.Planned() {
		t.Fatalf("a fresh COMPLETED plan is Planned")
	}

	plan = apply(t, ctx, envelope("c2", owner, "", "", mutation.OpCreate, createPayload("AT-07", "FEATURE", []string{"ac"}, "NATIVE")))
	want(t, plan, mutation.OutcomeCompleted, "")
	if plan.Post.TicketID.Local != "AT-07" || plan.QueuePost != nil {
		t.Fatalf("explicit token must not allocate a serial")
	}

	plan = apply(t, ctx, envelope("c3", owner, "", "", mutation.OpCreate, createPayload("at-01", "FEATURE", []string{"ac"}, "NATIVE")))
	want(t, plan, mutation.OutcomeValidationFailed, wire.CodeDuplicateID)

	// A tombstone keeps its token.
	tomb := fixture.Ticket("AT-02")
	from := ticket.StatusOpen
	tomb.Status = ticket.StatusArchived
	tomb.ArchivedFrom = &from
	ctx2 := newCtx(t, owner, nil, at01, tomb)
	plan = apply(t, ctx2, envelope("c4", owner, "", "", mutation.OpCreate, createPayload("AT-02", "FEATURE", []string{"ac"}, "NATIVE")))
	want(t, plan, mutation.OutcomeValidationFailed, wire.CodeDuplicateID)

	// REFINE routine field.
	plan = apply(t, ctx, envelope("r1", owner, "AT-01", "1", mutation.OpRefine, obj("title", str("Renamed"))))
	want(t, plan, mutation.OutcomeCompleted, "")
	chain(t, at01, plan.Post, "1")
	if plan.Post.Title != "Renamed" || plan.Post.Revision != "2" {
		t.Fatalf("refine did not apply")
	}
	if *plan.Outcome.ResultingRevision != "2" || *plan.Outcome.ResultingAcceptanceRevision != "1" {
		t.Fatalf("outcome revisions %s/%s", *plan.Outcome.ResultingRevision, *plan.Outcome.ResultingAcceptanceRevision)
	}
	// REFINE acceptance-relevant field.
	plan = apply(t, ctx, envelope("r2", owner, "AT-01", "1", mutation.OpRefine, obj("acceptanceCriteria", wire.Strings([]string{"a", "b"}))))
	want(t, plan, mutation.OutcomeCompleted, "")
	chain(t, at01, plan.Post, "2")
	// REFINE with the same acceptance value is routine.
	plan = apply(t, ctx, envelope("r3", owner, "AT-01", "1", mutation.OpRefine, obj("kind", str("FEATURE"))))
	want(t, plan, mutation.OutcomeCompleted, "")
	chain(t, at01, plan.Post, "1")
	// Every operation's post decodes and the input record is untouched.
	if at01.Title != "Fixture ticket AT-01" || at01.Revision != "1" {
		t.Fatalf("input record mutated")
	}
}

// TestTMV0005_AS02_SerialAllocationSkipsOccupied: an explicit localToken that
// took the next serial (exactly or under case folding, live or tombstoned)
// never wedges later automatic CREATEs: allocation skips occupied serials,
// returns the advanced manifest, and refuses LIMIT_EXCEEDED before the Count
// range would overflow, leaving the manifest untouched.
func TestTMV0005_AS02_SerialAllocationSkipsOccupied(t *testing.T) {
	at01 := fixture.Ticket("AT-01")
	ctx := newCtx(t, owner, nil, at01)
	// Explicit token equal to the next serial while nextSerial is 2.
	explicit := apply(t, ctx, envelope("e1", owner, "", "", mutation.OpCreate, createPayload("AT-0002", "FEATURE", []string{"ac"}, "NATIVE")))
	want(t, explicit, mutation.OutcomeCompleted, "")
	if explicit.QueuePost != nil {
		t.Fatalf("explicit token allocates no serial")
	}
	// Commit the explicit record (pure equivalent) and allocate again.
	ctx = newCtx(t, owner, nil, at01, explicit.Post)
	auto := apply(t, ctx, envelope("a1", owner, "", "", mutation.OpCreate, createPayload("", "FEATURE", []string{"ac"}, "NATIVE")))
	want(t, auto, mutation.OutcomeCompleted, "")
	if auto.Post.TicketID.Local != "AT-0003" || auto.QueuePost == nil || auto.QueuePost.NextSerial != "4" {
		t.Fatalf("allocated %s with nextSerial %v; want AT-0003 and nextSerial 4", auto.Post.TicketID.Local, auto.QueuePost)
	}
	if ctx.Queue.NextSerial != "2" {
		t.Fatalf("input manifest mutated")
	}
	// Case-folded occupant and a tombstone are both skipped.
	folded := fixture.Ticket("at-0002")
	tomb := fixture.Ticket("AT-0003")
	from := ticket.StatusOpen
	tomb.Status = ticket.StatusArchived
	tomb.ArchivedFrom = &from
	ctx = newCtx(t, owner, nil, at01, folded, tomb)
	auto = apply(t, ctx, envelope("a2", owner, "", "", mutation.OpCreate, createPayload("", "FEATURE", []string{"ac"}, "NATIVE")))
	want(t, auto, mutation.OutcomeCompleted, "")
	if auto.Post.TicketID.Local != "AT-0004" || auto.QueuePost.NextSerial != "5" {
		t.Fatalf("allocated %s / %s; want AT-0004 / 5", auto.Post.TicketID.Local, auto.QueuePost.NextSerial)
	}
	// The Count range is a hard bound: no allocation at the maximum, and no
	// skip past it.
	ctx = newCtx(t, owner, nil, at01)
	ctx.Queue.NextSerial = wire.CountOf(wire.MaxCountValue)
	want(t, apply(t, ctx, envelope("a3", owner, "", "", mutation.OpCreate, createPayload("", "FEATURE", []string{"ac"}, "NATIVE"))), mutation.OutcomeValidationFailed, wire.CodeLimitExceeded)
	last := fixture.Ticket(fmt.Sprintf("AT-%d", wire.MaxCountValue-1))
	ctx = newCtx(t, owner, nil, at01, last)
	ctx.Queue.NextSerial = wire.CountOf(wire.MaxCountValue - 1)
	want(t, apply(t, ctx, envelope("a4", owner, "", "", mutation.OpCreate, createPayload("", "FEATURE", []string{"ac"}, "NATIVE"))), mutation.OutcomeValidationFailed, wire.CodeLimitExceeded)
	// An explicit token is still checked against occupants.
	ctx = newCtx(t, owner, nil, at01, folded)
	want(t, apply(t, ctx, envelope("e2", owner, "", "", mutation.OpCreate, createPayload("AT-0002", "FEATURE", []string{"ac"}, "NATIVE"))), mutation.OutcomeValidationFailed, wire.CodeDuplicateID)
}

// TestTMV0005_AS02_StaleExpectedRevision: a stale expectedRevision is
// REVISION_CONFLICT with no post record and byte-identical inputs.
func TestTMV0005_AS02_StaleExpectedRevision(t *testing.T) {
	at01 := fixture.Ticket("AT-01")
	before := string(at01.Encode())
	ctx := newCtx(t, owner, nil, at01)
	plan := apply(t, ctx, envelope("r1", owner, "AT-01", "5", mutation.OpRefine, obj("title", str("x"))))
	want(t, plan, mutation.OutcomeRevisionConflict, "")
	if string(at01.Encode()) != before {
		t.Fatalf("record changed on conflict")
	}
	plan = apply(t, ctx, envelope("r2", owner, "AT-99", "1", mutation.OpRefine, obj("title", str("x"))))
	want(t, plan, mutation.OutcomeValidationFailed, wire.CodeMalformed)
}

// TestTMV0005_TrustedBindingRequired: the envelope actor is a claim. Apply
// refuses without a binding, with a mismatched role or id, and with a
// binding outside the role vocabulary; it never derives one.
func TestTMV0005_TrustedBindingRequired(t *testing.T) {
	at01 := fixture.Ticket("AT-01")
	raw := envelope("r1", owner, "AT-01", "1", mutation.OpRefine, obj("title", str("x")))

	ctx := newCtx(t, mutation.Binding{}, nil, at01)
	want(t, apply(t, ctx, raw), mutation.OutcomeUnauthorized, "")

	// Envelope claims OWNER; the process is bound as WORKER with the same id.
	ctx = newCtx(t, mutation.Binding{ID: "russell", Role: "WORKER"}, nil, at01)
	want(t, apply(t, ctx, raw), mutation.OutcomeUnauthorized, "")

	// Envelope claims another OWNER id.
	ctx = newCtx(t, mutation.Binding{ID: "someone", Role: "OWNER"}, nil, at01)
	want(t, apply(t, ctx, raw), mutation.OutcomeUnauthorized, "")

	// Binding outside the vocabulary or malformed.
	ctx = newCtx(t, mutation.Binding{ID: "russell", Role: "ROOT"}, nil, at01)
	want(t, apply(t, ctx, raw), mutation.OutcomeUnauthorized, "")
	ctx = newCtx(t, mutation.Binding{ID: "", Role: "OWNER"}, nil, at01)
	want(t, apply(t, ctx, raw), mutation.OutcomeUnauthorized, "")

	// Matching binding proceeds.
	ctx = newCtx(t, owner, nil, at01)
	want(t, apply(t, ctx, raw), mutation.OutcomeCompleted, "")

	// A worker spoofing OWNER in the envelope is refused even for an
	// operation the worker could issue.
	spoof := envelope("r2", owner, "AT-01", "1", mutation.OpRefine, obj("body", str("b")))
	ctx = newCtx(t, worker, nil, at01)
	want(t, apply(t, ctx, spoof), mutation.OutcomeUnauthorized, "")
}

// TestTMV0005_RoleMatrix covers the §3.2 rows and the restrictions inside
// them, and policy row removal.
func TestTMV0005_RoleMatrix(t *testing.T) {
	at01 := fixture.Ticket("AT-01")

	ctx := newCtx(t, worker, nil, at01)
	want(t, apply(t, ctx, envelope("w1", worker, "AT-01", "1", mutation.OpRefine, obj("body", str("notes")))), mutation.OutcomeCompleted, "")
	want(t, apply(t, ctx, envelope("w2", worker, "AT-01", "1", mutation.OpRefine, obj("title", str("t")))), mutation.OutcomeUnauthorized, "")
	want(t, apply(t, ctx, envelope("w3", worker, "AT-01", "1", mutation.OpRefine, obj("body", str("b"), "title", str("t")))), mutation.OutcomeUnauthorized, "")
	want(t, apply(t, ctx, envelope("w4", worker, "AT-01", "1", mutation.OpRefine, obj("acceptanceCriteria", wire.Strings([]string{"x"})))), mutation.OutcomeUnauthorized, "")
	forbidden := []struct {
		op      string
		payload wire.Value
	}{
		{mutation.OpPrioritize, obj("priority", str("P0"), "order", str("1"))},
		{mutation.OpSetDependencies, obj("dependencies", wire.Array())},
		{mutation.OpSetGates, obj("requiredGates", wire.Strings([]string{"verify"}))},
		{mutation.OpSetEffects, obj("effects", obj("coverage", str("QUALIFIED"), "touchPaths", wire.Strings(nil), "resources", wire.Array(), "externalUnbounded", wire.Bool(false)), "capabilities", wire.Strings(nil), "executionClass", str("AUTONOMOUS"))},
		{mutation.OpHold, obj("holdId", str("h"), "reason", str("r"))},
		{mutation.OpReleaseHold, obj("holdId", str("h"))},
		{mutation.OpReopen, obj("reason", str("r"))},
		{mutation.OpArchive, obj("reason", str("r"))},
		{mutation.OpRestore, obj("reason", str("r"))},
		{mutation.OpCompleteManual, obj("reason", str("r"), "evidence", wire.Strings(nil))},
		{mutation.OpGrantApproval, obj("grantId", str("g"), "actor", str("lane-1"), "operation", str("RUN"), "targetRevision", str("1"), "scope", wire.Strings(nil))},
		{mutation.OpRevokeApproval, obj("grantId", str("g"), "reason", str("r"))},
	}
	for _, f := range forbidden {
		want(t, apply(t, ctx, envelope("wf", worker, "AT-01", "1", f.op, f.payload)), mutation.OutcomeUnauthorized, "")
	}
	want(t, apply(t, ctx, envelope("wc", worker, "", "", mutation.OpCreate, createPayload("", "FEATURE", []string{"ac"}, "NATIVE"))), mutation.OutcomeUnauthorized, "")

	// REVIEWER: nothing.
	ctx = newCtx(t, reviewer, nil, at01)
	want(t, apply(t, ctx, envelope("v1", reviewer, "AT-01", "1", mutation.OpRefine, obj("body", str("b")))), mutation.OutcomeUnauthorized, "")

	// SYSTEM: HOLD ESCALATED only; actor and time from the context.
	ctx = newCtx(t, system, nil, at01)
	plan := apply(t, ctx, envelope("s1", system, "AT-01", "1", mutation.OpHold, obj("holdId", str("ESCALATED"), "reason", str("review escalated"))))
	want(t, plan, mutation.OutcomeCompleted, "")
	if plan.Post.Status != ticket.StatusHeld || len(plan.Post.Holds) != 1 || plan.Post.Holds[0].Actor != "atm" || plan.Post.Holds[0].PlacedAt != now {
		t.Fatalf("SYSTEM hold not composed from the context: %+v", plan.Post.Holds)
	}
	want(t, apply(t, ctx, envelope("s2", system, "AT-01", "1", mutation.OpHold, obj("holdId", str("manual"), "reason", str("r")))), mutation.OutcomeUnauthorized, "")
	want(t, apply(t, ctx, envelope("s3", system, "AT-01", "1", mutation.OpReleaseHold, obj("holdId", str("ESCALATED")))), mutation.OutcomeUnauthorized, "")
	want(t, apply(t, ctx, envelope("s4", system, "AT-01", "1", mutation.OpRefine, obj("body", str("b")))), mutation.OutcomeUnauthorized, "")

	// OPERATOR: everything but GRANT_APPROVAL for ADJUDICATE.
	ctx = newCtx(t, operator, nil, at01)
	grant := func(op string) wire.Value {
		return obj("grantId", str("g-"+op), "actor", str("ops"), "operation", str(op), "targetRevision", str("1"), "scope", wire.Strings(nil))
	}
	want(t, apply(t, ctx, envelope("o1", operator, "AT-01", "1", mutation.OpGrantApproval, grant("ADJUDICATE"))), mutation.OutcomeUnauthorized, "")
	want(t, apply(t, ctx, envelope("o2", operator, "AT-01", "1", mutation.OpGrantApproval, grant("RUN"))), mutation.OutcomeCompleted, "")
	want(t, apply(t, ctx, envelope("o3", operator, "AT-01", "1", mutation.OpSetGates, obj("requiredGates", wire.Strings([]string{"verify"})))), mutation.OutcomeCompleted, "")
	ctx = newCtx(t, owner, nil, at01)
	og := obj("grantId", str("g"), "actor", str("russell"), "operation", str("ADJUDICATE"), "targetRevision", str("1"), "scope", wire.Strings(nil))
	want(t, apply(t, ctx, envelope("o4", owner, "AT-01", "1", mutation.OpGrantApproval, og)), mutation.OutcomeCompleted, "")

	// Policy row removal: WORKER with no operations.
	pv := fixture.PolicyValue()
	pv.Obj.Set("roles", obj("WORKER", wire.Strings(nil)))
	ctx = ctxWithPolicy(t, worker, nil, wire.EncodeFile(pv), at01)
	want(t, apply(t, ctx, envelope("p1", worker, "AT-01", "1", mutation.OpRefine, obj("body", str("b")))), mutation.OutcomeUnauthorized, "")
	// Removal of one row leaves the others at their defaults.
	ctx = ctxWithPolicy(t, owner, nil, wire.EncodeFile(pv), at01)
	want(t, apply(t, ctx, envelope("p2", owner, "AT-01", "1", mutation.OpRefine, obj("body", str("b")))), mutation.OutcomeCompleted, "")
	// OPERATOR limited to REFINE and PRIORITIZE by policy.
	pv.Obj.Set("roles", obj("OPERATOR", wire.Strings([]string{"PRIORITIZE", "REFINE"})))
	ctx = ctxWithPolicy(t, operator, nil, wire.EncodeFile(pv), at01)
	want(t, apply(t, ctx, envelope("p3", operator, "AT-01", "1", mutation.OpHold, obj("holdId", str("h"), "reason", str("r")))), mutation.OutcomeUnauthorized, "")
	want(t, apply(t, ctx, envelope("p4", operator, "AT-01", "1", mutation.OpPrioritize, obj("priority", str("P0"), "order", str("1")))), mutation.OutcomeCompleted, "")
}

// TestTMV0005_ImporterRules: IMPORTER may CREATE and REFINE only records
// whose source.kind is IMPORT.
func TestTMV0005_ImporterRules(t *testing.T) {
	native := fixture.Ticket("AT-01")
	imported := fixture.Ticket("IMP-1")
	item := "X-1"
	imported.Source = ticket.Source{Kind: "IMPORT", SourceQueueID: "queue:ext:src", SourceItemID: &item}
	ctx := newCtx(t, importer, nil, native, imported)

	want(t, apply(t, ctx, envelope("i1", importer, "", "", mutation.OpCreate, createPayload("IMP-2", "FEATURE", []string{"ac"}, "NATIVE"))), mutation.OutcomeUnauthorized, "")
	plan := apply(t, ctx, envelope("i2", importer, "", "", mutation.OpCreate, createPayload("IMP-2", "FEATURE", []string{"ac"}, "IMPORT")))
	want(t, plan, mutation.OutcomeCompleted, "")
	if plan.Post.Source.Kind != "IMPORT" || plan.Post.ShadowOverlay {
		t.Fatalf("imported create wrong: %+v", plan.Post.Source)
	}
	want(t, apply(t, ctx, envelope("i3", importer, "AT-01", "1", mutation.OpRefine, obj("body", str("b")))), mutation.OutcomeUnauthorized, "")
	want(t, apply(t, ctx, envelope("i4", importer, "IMP-1", "1", mutation.OpRefine, obj("body", str("b"), "title", str("t")))), mutation.OutcomeCompleted, "")
	want(t, apply(t, ctx, envelope("i5", importer, "IMP-1", "1", mutation.OpPrioritize, obj("priority", str("P0"), "order", str("1")))), mutation.OutcomeUnauthorized, "")
	want(t, apply(t, ctx, envelope("i6", importer, "IMP-1", "1", mutation.OpHold, obj("holdId", str("h"), "reason", str("r")))), mutation.OutcomeUnauthorized, "")
}

// TestTMV0005_AS04_DependencyAndGateValidation: missing dependency, self,
// direct, transitive and gate-edge cycles, unknown gates and unknown
// supersession all refuse without a post record.
func TestTMV0005_AS04_DependencyAndGateValidation(t *testing.T) {
	at01 := fixture.Ticket("AT-01")
	at02 := fixture.Ticket("AT-02")
	at02.Dependencies = []ticket.Dependency{fixture.Dep("AT-01")}
	at03 := fixture.Ticket("AT-03")
	at03.Dependencies = []ticket.Dependency{fixture.GateDep("AT-02", "verify")}
	at04 := fixture.Ticket("AT-04")
	ctx := newCtx(t, owner, nil, at01, at02, at03, at04)
	deps := func(vs ...wire.Value) wire.Value { return obj("dependencies", wire.Array(vs...)) }

	want(t, apply(t, ctx, envelope("d1", owner, "AT-01", "1", mutation.OpSetDependencies, deps(depValue("AT-99", "COMPLETED", "")))), mutation.OutcomeValidationFailed, wire.CodeDependencyMissing)
	want(t, apply(t, ctx, envelope("d2", owner, "AT-01", "1", mutation.OpSetDependencies, deps(depValue("AT-01", "COMPLETED", "")))), mutation.OutcomeValidationFailed, wire.CodeCycle)
	want(t, apply(t, ctx, envelope("d3", owner, "AT-01", "1", mutation.OpSetDependencies, deps(depValue("AT-02", "COMPLETED", "")))), mutation.OutcomeValidationFailed, wire.CodeCycle)
	want(t, apply(t, ctx, envelope("d4", owner, "AT-01", "1", mutation.OpSetDependencies, deps(depValue("AT-03", "COMPLETED", "")))), mutation.OutcomeValidationFailed, wire.CodeCycle)
	want(t, apply(t, ctx, envelope("d5", owner, "AT-01", "1", mutation.OpSetDependencies, deps(depValue("AT-03", "GATE_PASSED", "verify")))), mutation.OutcomeValidationFailed, wire.CodeCycle)
	want(t, apply(t, ctx, envelope("d6", owner, "AT-04", "1", mutation.OpSetDependencies, deps(depValue("AT-01", "GATE_PASSED", "nope")))), mutation.OutcomeValidationFailed, wire.CodeGateUnknown)
	want(t, apply(t, ctx, envelope("d7", owner, "AT-04", "1", mutation.OpSetDependencies, deps(depValue("AT-01", "COMPLETED", "verify")))), mutation.OutcomeValidationFailed, wire.CodeMalformed)
	want(t, apply(t, ctx, envelope("d8", owner, "AT-04", "1", mutation.OpSetDependencies, deps(depValue("AT-01", "COMPLETED", ""), depValue("AT-01", "COMPLETED", "")))), mutation.OutcomeValidationFailed, wire.CodeDuplicateID)
	plan := apply(t, ctx, envelope("d9", owner, "AT-04", "1", mutation.OpSetDependencies, deps(depValue("AT-03", "GATE_PASSED", "verify"), depValue("AT-01", "COMPLETED", ""))))
	want(t, plan, mutation.OutcomeCompleted, "")
	chain(t, at04, plan.Post, "2")
	if len(plan.Post.Dependencies) != 2 || plan.Post.Dependencies[0].TicketID.Local != "AT-03" {
		t.Fatalf("dependencies keep semantic order")
	}

	want(t, apply(t, ctx, envelope("g1", owner, "AT-01", "1", mutation.OpSetGates, obj("requiredGates", wire.Strings([]string{"nope"})))), mutation.OutcomeValidationFailed, wire.CodeGateUnknown)
	plan = apply(t, ctx, envelope("g2", owner, "AT-01", "1", mutation.OpSetGates, obj("requiredGates", wire.Strings([]string{"verify"}))))
	want(t, plan, mutation.OutcomeCompleted, "")
	chain(t, at01, plan.Post, "2")

	want(t, apply(t, ctx, envelope("s1", owner, "AT-01", "1", mutation.OpRefine, obj("supersedes", str(fixture.TicketID("AT-99"))))), mutation.OutcomeValidationFailed, wire.CodeDependencyMissing)
	want(t, apply(t, ctx, envelope("s2", owner, "AT-01", "1", mutation.OpRefine, obj("supersedes", str(fixture.TicketID("AT-01"))))), mutation.OutcomeValidationFailed, wire.CodeMalformed)
	plan = apply(t, ctx, envelope("s3", owner, "AT-01", "1", mutation.OpRefine, obj("supersedes", str(fixture.TicketID("AT-04")))))
	want(t, plan, mutation.OutcomeCompleted, "")
	chain(t, at01, plan.Post, "2")

	// CREATE: self dependency and missing dependency.
	cp := createPayload("AT-09", "FEATURE", []string{"ac"}, "NATIVE")
	cp.Obj.Set("dependencies", wire.Array(depValue("AT-09", "COMPLETED", "")))
	want(t, apply(t, ctx, envelope("c1", owner, "", "", mutation.OpCreate, cp)), mutation.OutcomeValidationFailed, wire.CodeCycle)
	cp.Obj.Set("dependencies", wire.Array(depValue("AT-99", "COMPLETED", "")))
	want(t, apply(t, ctx, envelope("c2", owner, "", "", mutation.OpCreate, cp)), mutation.OutcomeValidationFailed, wire.CodeDependencyMissing)
	cp.Obj.Set("dependencies", wire.Array(depValue("AT-03", "GATE_PASSED", "verify")))
	cp.Obj.Set("requiredGates", wire.Strings([]string{"verify"}))
	want(t, apply(t, ctx, envelope("c3", owner, "", "", mutation.OpCreate, cp)), mutation.OutcomeCompleted, "")

	// SET_EFFECTS is acceptance-relevant and validates its closed body.
	eff := obj("effects", obj("coverage", str("INCOMPLETE"), "touchPaths", wire.Strings([]string{"cmd/"}), "resources", wire.Array(obj("class", str("PATH"), "key", str("cmd/"))), "externalUnbounded", wire.Bool(false)),
		"capabilities", wire.Strings([]string{"git"}), "executionClass", str("APPROVAL_REQUIRED"))
	plan = apply(t, ctx, envelope("e1", owner, "AT-01", "1", mutation.OpSetEffects, eff))
	want(t, plan, mutation.OutcomeCompleted, "")
	chain(t, at01, plan.Post, "2")
	if plan.Post.ExecutionClass != "APPROVAL_REQUIRED" || plan.Post.Effects.Coverage != "INCOMPLETE" {
		t.Fatalf("effects not applied")
	}
}

// TestTMV0003_AttemptLiveRule: with a live attempt, routine mutations bump
// revision only; acceptance-relevant ones are BLOCKED/ATTEMPT_LIVE; a
// NOT_OBSERVED liveness fails closed.
func TestTMV0003_AttemptLiveRule(t *testing.T) {
	at01 := fixture.Ticket("AT-01")
	at02 := fixture.Ticket("AT-02")
	ctx := newCtx(t, owner, attempts{fixture.TicketID("AT-01"): true}, at01, at02)
	routine := []struct {
		op      string
		payload wire.Value
	}{
		{mutation.OpRefine, obj("title", str("t"), "body", str("b"), "owner", str("o"), "milestone", str("m"), "labels", wire.Strings([]string{"l"}), "dueDate", str("2026-12-31"), "estimateMinutes", str("5"))},
		{mutation.OpRefine, obj("kind", str("FEATURE"))},
		{mutation.OpPrioritize, obj("priority", str("P0"), "order", str("9"))},
		{mutation.OpHold, obj("holdId", str("h"), "reason", str("r"))},
		{mutation.OpGrantApproval, obj("grantId", str("g"), "actor", str("russell"), "operation", str("RUN"), "targetRevision", str("1"), "scope", wire.Strings(nil))},
	}
	for _, r := range routine {
		plan := apply(t, ctx, envelope("l", owner, "AT-01", "1", r.op, r.payload))
		want(t, plan, mutation.OutcomeCompleted, "")
		chain(t, at01, plan.Post, "1")
	}
	relevant := []struct {
		op      string
		payload wire.Value
	}{
		{mutation.OpRefine, obj("acceptanceCriteria", wire.Strings([]string{"changed"}))},
		{mutation.OpRefine, obj("kind", str("BUG"))},
		{mutation.OpRefine, obj("requirementRefs", wire.Strings([]string{"ATCP-V0-003"}))},
		{mutation.OpSetDependencies, obj("dependencies", wire.Array(depValue("AT-02", "COMPLETED", "")))},
		{mutation.OpSetGates, obj("requiredGates", wire.Strings([]string{"verify"}))},
		{mutation.OpCompleteManual, obj("reason", str("done"), "evidence", wire.Strings(nil))},
	}
	for _, r := range relevant {
		want(t, apply(t, ctx, envelope("l", owner, "AT-01", "1", r.op, r.payload)), mutation.OutcomeBlocked, wire.CodeAttemptLive)
	}
	// SET_DEPENDENCIES with an identical value changes nothing and is routine.
	plan := apply(t, ctx, envelope("l2", owner, "AT-01", "1", mutation.OpSetDependencies, obj("dependencies", wire.Array())))
	want(t, plan, mutation.OutcomeCompleted, "")
	chain(t, at01, plan.Post, "1")

	// Unknown liveness: fail closed for relevant, proceed for routine.
	unknown := newCtx(t, owner, nil, at01)
	unknown.Attempts = ticket.NoEvidence{}
	want(t, apply(t, unknown, envelope("u1", owner, "AT-01", "1", mutation.OpRefine, obj("acceptanceCriteria", wire.Strings([]string{"x"})))), mutation.OutcomeBlocked, wire.CodeAttemptLive)
	want(t, apply(t, unknown, envelope("u2", owner, "AT-01", "1", mutation.OpRefine, obj("title", str("x")))), mutation.OutcomeCompleted, "")
	unknown.Attempts = nil
	want(t, apply(t, unknown, envelope("u3", owner, "AT-01", "1", mutation.OpSetGates, obj("requiredGates", wire.Strings([]string{"verify"})))), mutation.OutcomeBlocked, wire.CodeAttemptLive)
}

// TestTMV0006_AS03_ReplayAndConflict: against the explicit in-memory index,
// an identical retry replays the recorded outcome with replayed:true and no
// post record; the same request ID with different bytes conflicts; a
// recorded refusal replays as that refusal. The index is a fake, so none
// of this is commit evidence.
func TestTMV0006_AS03_ReplayAndConflict(t *testing.T) {
	at01 := fixture.Ticket("AT-01")
	ctx := newCtx(t, owner, nil, at01)
	idx := mutation.NewMemoryIndex()
	ctx.Requests = idx

	raw := envelope("req-1", owner, "AT-01", "1", mutation.OpRefine, obj("title", str("first")))
	first := apply(t, ctx, raw)
	want(t, first, mutation.OutcomeCompleted, "")
	if first.MutationSha256 != wire.Sum(raw) {
		t.Fatalf("mutation digest is not the SHA-256 of the envelope bytes")
	}
	if err := idx.Record(mutation.IndexEntry{RequestID: "req-1", MutationSha256: first.MutationSha256, Outcome: first.Outcome}); err != nil {
		t.Fatalf("record: %v", err)
	}
	if err := idx.Record(mutation.IndexEntry{RequestID: "req-1", MutationSha256: first.MutationSha256, Outcome: first.Outcome}); wire.CodeOf(err) != wire.CodeRequestIDConflict {
		t.Fatalf("index is append-only: %v", err)
	}

	again := apply(t, ctx, raw)
	if again.Outcome.Outcome != mutation.OutcomeCompleted || !again.Outcome.Replayed {
		t.Fatalf("identical retry: %+v", again.Outcome)
	}
	if again.Post != nil || again.Planned() {
		t.Fatalf("a replay carries no post record and plans nothing")
	}
	if *again.Outcome.ResultingRevision != "2" || again.Outcome.RequestID != "req-1" {
		t.Fatalf("replay must return the original outcome")
	}
	if len(again.Outcome.Codes) != 0 {
		t.Fatalf("replay codes %v", again.Outcome.Codes)
	}
	if raw2, err := again.Outcome.Encode(); err != nil || !strings.Contains(string(raw2), `"replayed":true`) {
		t.Fatalf("replayed outcome encode: %v %s", err, raw2)
	}

	altered := envelope("req-1", owner, "AT-01", "1", mutation.OpRefine, obj("title", str("second")))
	conflict := apply(t, ctx, altered)
	want(t, conflict, mutation.OutcomeRequestIDConflict, wire.CodeRequestIDConflict)
	if conflict.Outcome.Replayed {
		t.Fatalf("a conflict is not a replay")
	}
	// A different request ID with the same bytes otherwise is fresh.
	fresh := apply(t, ctx, envelope("req-2", owner, "AT-01", "1", mutation.OpRefine, obj("title", str("first"))))
	want(t, fresh, mutation.OutcomeCompleted, "")
	if fresh.Outcome.Replayed {
		t.Fatalf("fresh request replayed")
	}

	// A recorded refusal replays as that refusal.
	stale := envelope("req-3", owner, "AT-01", "7", mutation.OpRefine, obj("title", str("x")))
	p := apply(t, ctx, stale)
	want(t, p, mutation.OutcomeRevisionConflict, "")
	if err := idx.Record(mutation.IndexEntry{RequestID: "req-3", MutationSha256: p.MutationSha256, Outcome: p.Outcome}); err != nil {
		t.Fatalf("record: %v", err)
	}
	p = apply(t, ctx, stale)
	if p.Outcome.Outcome != mutation.OutcomeRevisionConflict || !p.Outcome.Replayed || p.Post != nil {
		t.Fatalf("refusal replay: %+v", p.Outcome)
	}
	// The trust boundary precedes replay: a replay is never handed to an
	// unbound caller.
	unbound := ctx
	unbound.Binding = mutation.Binding{}
	want(t, apply(t, unbound, raw), mutation.OutcomeUnauthorized, "")
	if idx.Len() != 2 {
		t.Fatalf("index has %d entries; the library never records", idx.Len())
	}
}

// step applies one operation to the current record and returns the post
// record as the next canonical record, checking the chain.
func step(t *testing.T, b mutation.Binding, live attempts, cur *ticket.Record, op string, payload wire.Value, acc string, others ...*ticket.Record) *ticket.Record {
	t.Helper()
	ctx := newCtx(t, b, live, append([]*ticket.Record{cur}, others...)...)
	plan := apply(t, ctx, envelope("s-"+op, b, cur.TicketID.Local, string(cur.Revision), op, payload))
	want(t, plan, mutation.OutcomeCompleted, "")
	chain(t, cur, plan.Post, acc)
	return plan.Post
}

func refused(t *testing.T, b mutation.Binding, cur *ticket.Record, op string, payload wire.Value, outcome, code string) {
	t.Helper()
	ctx := newCtx(t, b, nil, cur)
	want(t, apply(t, ctx, envelope("x-"+op, b, cur.TicketID.Local, string(cur.Revision), op, payload)), outcome, code)
}

// TestTMV0004_AS05_HoldsArchiveRestoreReopen walks the §3.2 transition table
// and refuses every unenumerated transition.
func TestTMV0004_AS05_HoldsArchiveRestoreReopen(t *testing.T) {
	cur := fixture.Ticket("AT-01")
	hold := func(id, reason string) wire.Value { return obj("holdId", str(id), "reason", str(reason)) }
	reason := obj("reason", str("because"))

	cur = step(t, owner, nil, cur, mutation.OpHold, hold("review", "needs review"), "1")
	if cur.Status != ticket.StatusHeld || len(cur.Holds) != 1 || cur.Holds[0].Actor != "russell" || cur.Holds[0].PlacedAt != now || cur.Holds[0].Reason != "needs review" {
		t.Fatalf("hold: %s %+v", cur.Status, cur.Holds)
	}
	refused(t, owner, cur, mutation.OpHold, hold("review", "again"), mutation.OutcomeValidationFailed, wire.CodeDuplicateID)
	refused(t, owner, cur, mutation.OpCompleteManual, obj("reason", str("r"), "evidence", wire.Strings(nil)), mutation.OutcomeBlocked, wire.CodeTicketState)
	refused(t, owner, cur, mutation.OpRestore, reason, mutation.OutcomeBlocked, wire.CodeTicketState)
	refused(t, owner, cur, mutation.OpReopen, reason, mutation.OutcomeBlocked, wire.CodeTicketState)
	cur = step(t, owner, nil, cur, mutation.OpHold, hold("legal", "legal check"), "1")
	if len(cur.Holds) != 2 || cur.Holds[1].HoldID != "legal" {
		t.Fatalf("holds keep semantic order: %+v", cur.Holds)
	}
	cur = step(t, owner, nil, cur, mutation.OpReleaseHold, obj("holdId", str("review")), "1")
	if cur.Status != ticket.StatusHeld || len(cur.Holds) != 1 || cur.Holds[0].HoldID != "legal" {
		t.Fatalf("release one of two: %s %+v", cur.Status, cur.Holds)
	}
	refused(t, owner, cur, mutation.OpReleaseHold, obj("holdId", str("nope")), mutation.OutcomeValidationFailed, wire.CodeMalformed)

	// Archive a HELD ticket: tombstone keeps the holds and archivedFrom.
	cur = step(t, owner, nil, cur, mutation.OpArchive, reason, "1")
	if cur.Status != ticket.StatusArchived || cur.ArchivedFrom == nil || *cur.ArchivedFrom != ticket.StatusHeld || len(cur.Holds) != 1 {
		t.Fatalf("archive: %s %v %d holds", cur.Status, cur.ArchivedFrom, len(cur.Holds))
	}
	for _, f := range []struct {
		op      string
		payload wire.Value
	}{
		{mutation.OpRefine, obj("title", str("t"))},
		{mutation.OpArchive, reason},
		{mutation.OpReopen, reason},
		{mutation.OpHold, hold("x", "y")},
		{mutation.OpReleaseHold, obj("holdId", str("legal"))},
		{mutation.OpPrioritize, obj("priority", str("P0"), "order", str("1"))},
	} {
		refused(t, owner, cur, f.op, f.payload, mutation.OutcomeBlocked, wire.CodeTicketState)
	}
	// Restore returns to HELD and bumps acceptanceRevision.
	cur = step(t, owner, nil, cur, mutation.OpRestore, reason, "2")
	if cur.Status != ticket.StatusHeld || cur.ArchivedFrom != nil || len(cur.Holds) != 1 {
		t.Fatalf("restore: %s %v", cur.Status, cur.ArchivedFrom)
	}
	cur = step(t, owner, nil, cur, mutation.OpReleaseHold, obj("holdId", str("legal")), "2")
	if cur.Status != ticket.StatusOpen || len(cur.Holds) != 0 {
		t.Fatalf("release last: %s", cur.Status)
	}
	refused(t, owner, cur, mutation.OpReleaseHold, obj("holdId", str("legal")), mutation.OutcomeBlocked, wire.CodeTicketState)

	// Manual completion (AS-06): labelled MANUAL, no manifest, actor and
	// time from the context, acceptanceRevision bumps.
	ev := string(wire.Sum([]byte("evidence")))
	cur = step(t, owner, nil, cur, mutation.OpCompleteManual, obj("reason", str("done by hand"), "evidence", wire.Strings([]string{ev})), "3")
	if cur.Status != ticket.StatusCompleted || cur.Completion == nil || cur.Completion.Kind != "MANUAL" || cur.Completion.ManifestSha256 != nil {
		t.Fatalf("manual completion: %s %+v", cur.Status, cur.Completion)
	}
	if cur.Completion.Actor != "russell" || cur.Completion.RecordedAt != now || len(cur.Completion.Evidence) != 1 || *cur.Completion.Reason != "done by hand" {
		t.Fatalf("completion attribution: %+v", cur.Completion)
	}
	refused(t, owner, cur, mutation.OpCompleteManual, obj("reason", str("r"), "evidence", wire.Strings(nil)), mutation.OutcomeBlocked, wire.CodeTicketState)
	refused(t, owner, cur, mutation.OpHold, hold("x", "y"), mutation.OutcomeBlocked, wire.CodeTicketState)
	refused(t, owner, cur, mutation.OpRefine, obj("acceptanceCriteria", wire.Strings([]string{"new"})), mutation.OutcomeBlocked, wire.CodeTicketState)
	refused(t, owner, cur, mutation.OpSetGates, obj("requiredGates", wire.Strings([]string{"verify"})), mutation.OutcomeBlocked, wire.CodeTicketState)
	// Routine edits on a COMPLETED ticket are fine.
	cur = step(t, owner, nil, cur, mutation.OpRefine, obj("title", str("renamed after completion")), "3")
	if cur.Status != ticket.StatusCompleted {
		t.Fatalf("routine refine changed status")
	}
	// Reopen clears the completion in the new OPEN record and bumps
	// acceptanceRevision; the prior completion stays in the chained prior
	// record, which is untouched.
	prior := cur
	priorBytes := string(prior.Encode())
	cur = step(t, owner, nil, cur, mutation.OpReopen, reason, "4")
	if cur.Status != ticket.StatusOpen || cur.Completion != nil {
		t.Fatalf("reopen: %s %+v", cur.Status, cur.Completion)
	}
	if string(prior.Encode()) != priorBytes || prior.Completion == nil || prior.Completion.Kind != "MANUAL" {
		t.Fatalf("reopen altered the prior record or its completion")
	}
	if cur.PreviousRecordSha256 == nil || *cur.PreviousRecordSha256 != prior.FileDigest() {
		t.Fatalf("reopened record is not chained to the completed record")
	}
	// Archive from OPEN and from COMPLETED.
	arch := step(t, owner, nil, cur, mutation.OpArchive, reason, "4")
	if *arch.ArchivedFrom != ticket.StatusOpen {
		t.Fatalf("archivedFrom %s", *arch.ArchivedFrom)
	}
	done := step(t, owner, nil, cur, mutation.OpCompleteManual, obj("reason", str("again"), "evidence", wire.Strings(nil)), "5")
	arch = step(t, owner, nil, done, mutation.OpArchive, reason, "5")
	if *arch.ArchivedFrom != ticket.StatusCompleted || arch.Completion == nil {
		t.Fatalf("archived completed ticket keeps its completion")
	}
	back := step(t, owner, nil, arch, mutation.OpRestore, reason, "6")
	if back.Status != ticket.StatusCompleted {
		t.Fatalf("restore to %s", back.Status)
	}
	// Six revisions later the record still decodes; the chain is intact.
	if back.Revision != "13" {
		t.Fatalf("revision %s", back.Revision)
	}
}

// TestTMV0005_Approvals: grants bind the current acceptanceRevision, name
// the invoking actor, are unique, and revocation keeps the entry.
func TestTMV0005_Approvals(t *testing.T) {
	cur := fixture.Ticket("AT-01")
	grant := func(id, actor, rev string) wire.Value {
		return obj("grantId", str(id), "actor", str(actor), "operation", str("RUN"), "targetRevision", str(rev), "scope", wire.Strings([]string{"a", "b"}))
	}
	refused(t, owner, cur, mutation.OpGrantApproval, grant("g1", "mallory", "1"), mutation.OutcomeUnauthorized, "")
	refused(t, owner, cur, mutation.OpGrantApproval, grant("g1", "russell", "2"), mutation.OutcomeValidationFailed, wire.CodeMalformed)
	cur = step(t, owner, nil, cur, mutation.OpGrantApproval, grant("g1", "russell", "1"), "1")
	if len(cur.Approvals) != 1 || cur.Approvals[0].Revoked || cur.Approvals[0].GrantedAt != now || cur.Approvals[0].Actor != "russell" || len(cur.Approvals[0].Scope) != 2 {
		t.Fatalf("grant: %+v", cur.Approvals)
	}
	refused(t, owner, cur, mutation.OpGrantApproval, grant("g1", "russell", "1"), mutation.OutcomeValidationFailed, wire.CodeDuplicateID)
	refused(t, owner, cur, mutation.OpRevokeApproval, obj("grantId", str("nope"), "reason", str("r")), mutation.OutcomeValidationFailed, wire.CodeMalformed)
	cur = step(t, owner, nil, cur, mutation.OpRevokeApproval, obj("grantId", str("g1"), "reason", str("changed my mind")), "1")
	if len(cur.Approvals) != 1 || !cur.Approvals[0].Revoked {
		t.Fatalf("revocation keeps the entry: %+v", cur.Approvals)
	}
	refused(t, owner, cur, mutation.OpRevokeApproval, obj("grantId", str("g1"), "reason", str("r")), mutation.OutcomeValidationFailed, wire.CodeMalformed)
	// After an acceptance bump the old grant no longer matches; a new one
	// must name the new value.
	cur = step(t, owner, nil, cur, mutation.OpRefine, obj("acceptanceCriteria", wire.Strings([]string{"v2"})), "2")
	refused(t, owner, cur, mutation.OpGrantApproval, grant("g2", "russell", "1"), mutation.OutcomeValidationFailed, wire.CodeMalformed)
	cur = step(t, owner, nil, cur, mutation.OpGrantApproval, grant("g2", "russell", "2"), "2")
	if len(cur.Approvals) != 2 {
		t.Fatalf("second grant")
	}
}

// TestTMV0003_DraftOpensByRefine: CREATE without acceptance criteria is
// DRAFT for autonomous kinds and OPEN for MANUAL/EXTERNAL; a REFINE that
// supplies criteria opens the draft.
func TestTMV0003_DraftOpensByRefine(t *testing.T) {
	ctx := newCtx(t, owner, nil)
	plan := apply(t, ctx, envelope("c1", owner, "", "", mutation.OpCreate, createPayload("D-1", "FEATURE", nil, "NATIVE")))
	want(t, plan, mutation.OutcomeCompleted, "")
	if plan.Post.Status != ticket.StatusDraft {
		t.Fatalf("status %s; want DRAFT", plan.Post.Status)
	}
	draft := plan.Post
	cur := step(t, owner, nil, draft, mutation.OpRefine, obj("title", str("still a draft")), "1")
	if cur.Status != ticket.StatusDraft {
		t.Fatalf("title refine opened the draft")
	}
	refused(t, owner, cur, mutation.OpHold, obj("holdId", str("h"), "reason", str("r")), mutation.OutcomeBlocked, wire.CodeTicketState)
	cur = step(t, owner, nil, cur, mutation.OpRefine, obj("acceptanceCriteria", wire.Strings([]string{"it works"})), "2")
	if cur.Status != ticket.StatusOpen {
		t.Fatalf("status %s; want OPEN after criteria", cur.Status)
	}
	plan = apply(t, ctx, envelope("c2", owner, "", "", mutation.OpCreate, createPayload("M-1", "MANUAL", nil, "NATIVE")))
	want(t, plan, mutation.OutcomeCompleted, "")
	if plan.Post.Status != ticket.StatusOpen {
		t.Fatalf("MANUAL kind status %s; want OPEN", plan.Post.Status)
	}
	// A DRAFT can be archived and restored to DRAFT.
	arch := step(t, owner, nil, draft, mutation.OpArchive, obj("reason", str("r")), "1")
	back := step(t, owner, nil, arch, mutation.OpRestore, obj("reason", str("r")), "2")
	if back.Status != ticket.StatusDraft {
		t.Fatalf("restored to %s", back.Status)
	}
}

// TestTMV0005_InputsNeverMutated: records, the queue manifest, the policy
// bytes and the envelope bytes are byte-identical after every kind of plan.
func TestTMV0005_InputsNeverMutated(t *testing.T) {
	at01 := fixture.Ticket("AT-01")
	at01.Labels = []string{"keep"}
	at01.Holds = []ticket.Hold{{HoldID: "h", Actor: "russell", Reason: "r", PlacedAt: fixture.Timestamp}}
	at01.Status = ticket.StatusHeld
	at01.Approvals = []ticket.Approval{{GrantID: "g", Actor: "russell", Operation: "RUN", TargetRevision: "1", Scope: []string{}, GrantedAt: fixture.Timestamp}}
	ctx := newCtx(t, owner, nil, at01)
	recBefore := string(at01.Encode())
	queueBefore := string(wire.EncodeFile(ctx.Queue.Value()))
	envs := [][]byte{
		envelope("m1", owner, "", "", mutation.OpCreate, createPayload("", "FEATURE", []string{"ac"}, "NATIVE")),
		envelope("m2", owner, "AT-01", "1", mutation.OpRefine, obj("labels", wire.Strings([]string{"a", "b"}), "title", str("t"))),
		envelope("m3", owner, "AT-01", "1", mutation.OpReleaseHold, obj("holdId", str("h"))),
		envelope("m4", owner, "AT-01", "1", mutation.OpHold, obj("holdId", str("h2"), "reason", str("r2"))),
		envelope("m5", owner, "AT-01", "1", mutation.OpRevokeApproval, obj("grantId", str("g"), "reason", str("r"))),
		envelope("m6", owner, "AT-01", "1", mutation.OpArchive, obj("reason", str("r"))),
		envelope("m7", owner, "AT-01", "1", mutation.OpSetDependencies, obj("dependencies", wire.Array(depValue("AT-01", "COMPLETED", "")))),
	}
	for _, raw := range envs {
		rawBefore := string(raw)
		env, err := mutation.Decode(raw)
		if err != nil {
			t.Fatalf("decode: %v", err)
		}
		plan := mutation.Apply(ctx, env)
		if plan.Post != nil {
			plan.Post.Labels = append(plan.Post.Labels, "poison")
			plan.Post.Holds = append(plan.Post.Holds, ticket.Hold{HoldID: "poison", Actor: "x", Reason: "y", PlacedAt: now})
			plan.Post.Approvals = append(plan.Post.Approvals, ticket.Approval{GrantID: "poison"})
		}
		if plan.QueuePost != nil {
			plan.QueuePost.NextSerial = "999"
		}
		if string(raw) != rawBefore || string(env.Raw) != rawBefore {
			t.Fatalf("envelope bytes changed")
		}
	}
	if string(at01.Encode()) != recBefore {
		t.Fatalf("canonical record changed:\n%s\n%s", recBefore, at01.Encode())
	}
	if string(wire.EncodeFile(ctx.Queue.Value())) != queueBefore {
		t.Fatalf("queue manifest changed")
	}
	if got, _ := ctx.Inventory.Get(fixture.TicketID("AT-01")); string(got.Encode()) != recBefore {
		t.Fatalf("inventory record changed")
	}
}

// TestTMV0002_OutcomeSchema: taskman-outcome/0 round trip, closed codes,
// closed outcomes and the COMPLETED/refused field relationships.
func TestTMV0002_OutcomeSchema(t *testing.T) {
	rev := wire.Count("2")
	acc := wire.Count("1")
	done := &mutation.Outcome{RequestID: "r", Outcome: mutation.OutcomeCompleted, ResultingRevision: &rev, ResultingAcceptanceRevision: &acc, Codes: []string{}}
	raw, err := done.Encode()
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	back, err := mutation.DecodeOutcome(raw)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if raw2, _ := back.Encode(); string(raw2) != string(raw) {
		t.Fatalf("round trip differs")
	}
	if !strings.Contains(string(raw), `"receiptSeq":null`) {
		t.Fatalf("planned outcome must carry a null receiptSeq: %s", raw)
	}
	refusedOut := &mutation.Outcome{RequestID: "r", Outcome: mutation.OutcomeBlocked, Codes: []string{wire.CodeTicketState, wire.CodeAttemptLive, wire.CodeAttemptLive}}
	raw, err = refusedOut.Encode()
	if err != nil {
		t.Fatalf("encode refused: %v", err)
	}
	if !strings.Contains(string(raw), `"codes":["ATTEMPT_LIVE","TICKET_STATE"]`) {
		t.Fatalf("codes must be sorted unique: %s", raw)
	}
	good := string(raw)
	for _, c := range []struct {
		name string
		edit func(string) string
		want string
	}{
		{"unknown code", func(s string) string { return strings.Replace(s, `"ATTEMPT_LIVE"`, `"ATTEMPT_LIVE","BOGUS"`, 1) }, wire.CodeMalformed},
		{"unknown outcome", func(s string) string { return strings.Replace(s, `"BLOCKED"`, `"MAYBE"`, 1) }, wire.CodeMalformed},
		{"version", func(s string) string { return strings.Replace(s, `taskman-outcome/0`, `taskman-outcome/2`, 1) }, wire.CodeUnsupportedVersion},
		{"refused with revision", func(s string) string {
			return strings.Replace(s, `"resultingRevision":null`, `"resultingRevision":"2"`, 1)
		}, wire.CodeMalformed},
		{"receiptSeq as Count leading zero", func(s string) string { return strings.Replace(s, `"receiptSeq":null`, `"receiptSeq":"01"`, 1) }, wire.CodeMalformed},
		{"unknown key", func(s string) string { return strings.Replace(s, `"codes":`, `"a":"b","codes":`, 1) }, wire.CodeMalformed},
	} {
		if _, err := mutation.DecodeOutcome([]byte(c.edit(good))); wire.CodeOf(err) != c.want {
			t.Errorf("%s: %v; want %s", c.name, err, c.want)
		}
	}
	bad := &mutation.Outcome{RequestID: "r", Outcome: mutation.OutcomeCompleted, ResultingRevision: &rev}
	if _, err := bad.Encode(); wire.CodeOf(err) != wire.CodeMalformed {
		t.Fatalf("COMPLETED with mixed revision nullness must not encode: %v", err)
	}
}

// TestTMV0005_ContextInputsRequired: the pure validator refuses without its
// explicit inputs rather than defaulting anything.
func TestTMV0005_ContextInputsRequired(t *testing.T) {
	at01 := fixture.Ticket("AT-01")
	raw := envelope("r", owner, "AT-01", "1", mutation.OpRefine, obj("title", str("x")))
	base := newCtx(t, owner, nil, at01)
	c := base
	c.Now = "not a time"
	want(t, apply(t, c, raw), mutation.OutcomeValidationFailed, wire.CodeMalformed)
	c = base
	c.Policy = nil
	want(t, apply(t, c, raw), mutation.OutcomeValidationFailed, wire.CodeMalformed)
	c = base
	c.Queue = nil
	want(t, apply(t, c, raw), mutation.OutcomeValidationFailed, wire.CodeMalformed)
	c = base
	c.Inventory = nil
	want(t, apply(t, c, raw), mutation.OutcomeValidationFailed, wire.CodeMalformed)
	// A missing request index is an explicit refusal, not an empty index: a
	// context without one would replay nothing and commit every retry twice.
	c = base
	c.Requests = nil
	want(t, apply(t, c, raw), mutation.OutcomeValidationFailed, wire.CodeMalformed)
	want(t, mutation.Adopt(c, "adopt-x", at01, at01.Encode()), mutation.OutcomeValidationFailed, wire.CodeMalformed)
	// An envelope for another queue is refused against this context.
	other := strings.Replace(string(raw), `"queueId":"queue:acme:main"`, `"queueId":"queue:acme:side"`, 1)
	other = strings.Replace(other, `"targetId":"ticket:acme:main:AT-01"`, `"targetId":"ticket:acme:side:AT-01"`, 1)
	want(t, apply(t, base, []byte(other)), mutation.OutcomeValidationFailed, wire.CodeMalformed)
	if mutation.Apply(base, nil).Outcome.Outcome != mutation.OutcomeValidationFailed {
		t.Fatalf("nil envelope")
	}
}

func TestTMV0006_AS03_TypedNilIndexRequired(t *testing.T) {
	rec := fixture.Ticket("AT-01")
	ctx := newCtx(t, owner, nil, rec)
	ctx.Requests = (*mutation.MemoryIndex)(nil)
	raw := envelope("nil-index", owner, "AT-01", "1", mutation.OpRefine, obj("title", str("new title")))
	want(t, apply(t, ctx, raw), mutation.OutcomeValidationFailed, wire.CodeMalformed)
	plan := mutation.Adopt(ctx, "adopt-nil", rec, rec.Encode())
	want(t, plan, mutation.OutcomeValidationFailed, wire.CodeMalformed)
	stillDiverged(t, plan, rec, string(rec.Encode()))
}
