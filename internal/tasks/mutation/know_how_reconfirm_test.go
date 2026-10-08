package mutation_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

var (
	khDigestF = string(wire.Sum([]byte("func F() {}")))
	khDigestG = string(wire.Sum([]byte("func G() {}")))
	khCommit2 = strings.Repeat("d", 40)
)

func khSymbol(path, blob, symbol, digest string) wire.Value {
	return obj("blob", str(blob), "path", str(path), "symbol", str(symbol), "symbolSha256", str(digest))
}

func knowHowReconfirm(note, commit string, anchors ...wire.Value) wire.Value {
	return obj("note", str(note), "anchors", wire.Array(anchors...), "commit", str(commit), "attempt", wire.Null(), "generation", wire.Null())
}

// TestKHNV0016_SymbolAnchorsInPayloadAndRecord: an ADD may carry symbol
// anchors {blob, path, symbol, symbolSha256} beside file anchors, in (path,
// symbol) order; a file anchor's encoding is unchanged; a malformed symbol,
// a missing or malformed digest, a stray symbol key, two blobs for one path
// or an unsorted pair is refused by the payload decoder and the record codec.
func TestKHNV0016_SymbolAnchorsInPayloadAndRecord(t *testing.T) {
	post := step(t, owner, nil, fixture.Ticket("AT-01"), mutation.OpKnowHowAdd, knowHowAdd("F needs the lock", "", "",
		khAnchor("a.go", khBlobA), khSymbol("a.go", khBlobA, "F", khDigestF), khSymbol("a.go", khBlobA, "T.G", khDigestG)), "1")
	got := post.KnowHow[0].Anchors
	if len(got) != 3 || got[0].Symbol != "" || got[1].Symbol != "F" || got[1].SymbolSha256 != khDigestF || got[2].Symbol != "T.G" {
		t.Fatalf("anchors: %+v", got)
	}
	enc := string(post.Encode())
	if !strings.Contains(enc, `{"blob":"`+khBlobA+`","path":"a.go"}`) || !strings.Contains(enc, `"symbol":"F","symbolSha256":"`+khDigestF+`"`) {
		t.Fatalf("anchor encoding: %s", enc)
	}
	back, err := ticket.Decode(post.Encode())
	if err != nil || !bytes.Equal(back.Encode(), post.Encode()) {
		t.Fatalf("round trip: %v", err)
	}

	rev := string(fixture.Ticket("AT-01").Revision)
	cases := map[string]wire.Value{
		"space in symbol":  knowHowAdd("t", "", "", khSymbol("a.go", khBlobA, "F G", khDigestF)),
		"hash in symbol":   knowHowAdd("t", "", "", khSymbol("a.go", khBlobA, "F#G", khDigestF)),
		"long symbol":      knowHowAdd("t", "", "", khSymbol("a.go", khBlobA, strings.Repeat("F", wire.KnowHowMaxSymbolBytes+1), khDigestF)),
		"empty symbol":     knowHowAdd("t", "", "", khSymbol("a.go", khBlobA, "", khDigestF)),
		"bad digest":       knowHowAdd("t", "", "", khSymbol("a.go", khBlobA, "F", "00")),
		"symbol no digest": knowHowAdd("t", "", "", obj("blob", str(khBlobA), "path", str("a.go"), "symbol", str("F"))),
		"two blobs":        knowHowAdd("t", "", "", khAnchor("a.go", khBlobA), khSymbol("a.go", khBlobB, "F", khDigestF)),
		"unsorted symbols": knowHowAdd("t", "", "", khSymbol("a.go", khBlobA, "G", khDigestG), khSymbol("a.go", khBlobA, "F", khDigestF)),
		"file after sym":   knowHowAdd("t", "", "", khSymbol("a.go", khBlobA, "F", khDigestF), khAnchor("a.go", khBlobA)),
		"same symbol":      knowHowAdd("t", "", "", khSymbol("a.go", khBlobA, "F", khDigestF), khSymbol("a.go", khBlobA, "F", khDigestG)),
	}
	for name, p := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := mutation.Decode(envelope("p", owner, "AT-01", rev, mutation.OpKnowHowAdd, p)); err == nil {
				t.Fatalf("payload decoded: %s", wire.Encode(p))
			}
		})
	}
	bad := map[string]func(*ticket.Record){
		"record two blobs":  func(r *ticket.Record) { r.KnowHow[0].Anchors[1].Blob = khBlobB },
		"record bad symbol": func(r *ticket.Record) { r.KnowHow[0].Anchors[1].Symbol = "F G" },
		"record no digest":  func(r *ticket.Record) { r.KnowHow[0].Anchors[1].SymbolSha256 = "" },
	}
	for name, edit := range bad {
		t.Run(name, func(t *testing.T) {
			rec, _ := ticket.Decode(post.Encode())
			edit(rec)
			if _, err := ticket.Decode(rec.Encode()); err == nil || !strings.Contains(err.Error(), "/knowHow") {
				t.Fatalf("decoded %v", err)
			}
		})
	}
	secret := "AKIAABCDEFGHIJKLMNOP"
	ctx := newCtx(t, owner, nil, fixture.Ticket("AT-01"))
	plan := apply(t, ctx, envelope("sec", owner, "AT-01", rev, mutation.OpKnowHowAdd, knowHowAdd("ok", "", "", khSymbol("a.go", khBlobA, secret, khDigestF))))
	want(t, plan, mutation.OutcomeValidationFailed, wire.CodeSecretDetected)
	if !strings.HasPrefix(plan.Detail, mutation.KnowHowSecretDetail+":") || strings.Contains(plan.Detail, secret) {
		t.Fatalf("symbol secret detail %q", plan.Detail)
	}
}

// TestKHNV0018_ReconfirmRepinsWithProvenance: RECONFIRM appends one entry
// naming the note with its re-pinned anchors, commit, attempt, generation,
// actor and time; every earlier entry, the note's text and its active seq
// stay; the effective pins are the latest RECONFIRM's, and a second
// RECONFIRM supersedes the first while both stay in history. It counts
// toward the 32-entry bound, and OPERATOR needs an explicit policy row.
func TestKHNV0018_ReconfirmRepinsWithProvenance(t *testing.T) {
	add := step(t, owner, nil, fixture.Ticket("AT-01"), mutation.OpKnowHowAdd, knowHowAdd("F needs the lock", "", "",
		khAnchor("a.go", khBlobA), khSymbol("b.go", khBlobA, "F", khDigestF)), "1")
	first := string(wire.Encode(ticket.KnowHowValue(add.KnowHow)))
	p := knowHowReconfirm("1", khCommit2, khAnchor("a.go", khBlobB), khSymbol("b.go", khBlobB, "F", khDigestG))
	p.Obj.Set("attempt", str("att-home")).Set("generation", str("3"))
	// The named attempt passes the KHN-V0-008 provenance check; an unknown
	// attempt or an unobserved ledger refuses PROVENANCE_UNVERIFIED.
	for name, ledger := range map[string]mutation.AttemptLedger{"unobserved": nil, "unknown": {"att-other": khnLedger()["att-other"]}} {
		ctx := newCtx(t, owner, nil, add)
		ctx.KnowHowAttempts = ledger
		want(t, apply(t, ctx, envelope("re-"+name, owner, "AT-01", string(add.Revision), mutation.OpKnowHowReconfirm, p)), mutation.OutcomeValidationFailed, wire.CodeProvenanceUnverified)
	}
	ctx := newCtx(t, owner, nil, add)
	ctx.KnowHowAttempts = khnLedger()
	plan := apply(t, ctx, envelope("re", owner, "AT-01", string(add.Revision), mutation.OpKnowHowReconfirm, p))
	want(t, plan, mutation.OutcomeCompleted, "")
	chain(t, add, plan.Post, "1")
	re := plan.Post
	sameExceptKnowHow(t, add, re)
	if !strings.HasPrefix(string(wire.Encode(ticket.KnowHowValue(re.KnowHow))), strings.TrimSuffix(first, "]")) {
		t.Fatal("reconfirm rewrote the earlier entry")
	}
	e := re.KnowHow[1]
	if len(re.KnowHow) != 2 || e.Operation != ticket.KnowHowReconfirm || e.Seq != "2" || e.Note != "1" || e.Commit != khCommit2 ||
		e.ActorID != owner.ID || e.ActorRole != "OWNER" || e.RecordedAt != now || *e.Attempt != "att-home" || *e.Generation != "3" || e.Reason != nil || e.Text != "" {
		t.Fatalf("reconfirm entry: %+v", e)
	}
	eff := ticket.EffectiveKnowHow(re.KnowHow)
	if len(eff) != 1 || eff[0].Seq != "1" || eff[0].Text != "F needs the lock" || eff[0].Commit != khCommit2 ||
		eff[0].Anchors[0].Blob != khBlobB || eff[0].Anchors[1].SymbolSha256 != khDigestG || eff[0].Reconfirmed == nil || eff[0].Reconfirmed.Seq != "2" {
		t.Fatalf("effective: %+v", eff)
	}
	if act := ticket.ActiveKnowHow(re.KnowHow); len(act) != 1 || act[0].Anchors[0].Blob != khBlobA {
		t.Fatalf("active entries keep the ADD's own pins: %+v", act)
	}
	enc := string(wire.Encode(ticket.KnowHowValue(re.KnowHow[1:])))
	if !strings.Contains(enc, `"operation":"RECONFIRM"`) || strings.Contains(enc, "reason") || strings.Contains(enc, "text") {
		t.Fatalf("reconfirm encoding: %s", enc)
	}
	back, err := ticket.Decode(re.Encode())
	if err != nil || !bytes.Equal(back.Encode(), re.Encode()) {
		t.Fatalf("round trip: %v", err)
	}
	again := step(t, owner, nil, re, mutation.OpKnowHowReconfirm, knowHowReconfirm("1", khCommit, khAnchor("a.go", khBlobA), khSymbol("b.go", khBlobA, "F", khDigestF)), "1")
	if eff := ticket.EffectiveKnowHow(again.KnowHow); eff[0].Reconfirmed.Seq != "3" || eff[0].Anchors[0].Blob != khBlobA || len(again.KnowHow) != 3 {
		t.Fatalf("second reconfirm: %+v", eff)
	}
	sup := step(t, owner, nil, again, mutation.OpKnowHowAdd, knowHowAdd("replaced", "1", "rewrote", khAnchor("a.go", khBlobA)), "1")
	if eff := ticket.EffectiveKnowHow(sup.KnowHow); len(eff) != 1 || eff[0].Seq != "4" || eff[0].Reconfirmed != nil {
		t.Fatalf("superseded note keeps a reconfirm overlay: %+v", eff)
	}

	full := fixture.Ticket("AT-01")
	full.KnowHow = append(full.KnowHow, add.KnowHow...)
	for i := 1; i < wire.KnowHowMaxEntries; i++ {
		blob := khBlobA
		if i%2 == 1 {
			blob = khBlobB
		}
		full.KnowHow = append(full.KnowHow, ticket.KnowHowEntry{Seq: wire.CountOf(int64(i + 1)), Operation: ticket.KnowHowReconfirm, Note: "1",
			Anchors: []ticket.KnowHowAnchor{{Path: "a.go", Blob: blob}, {Path: "b.go", Blob: blob, Symbol: "F", SymbolSha256: map[bool]string{true: khDigestG, false: khDigestF}[i%2 == 1]}},
			Commit:  khCommit, ActorID: "russell", ActorRole: "OWNER", RecordedAt: now})
	}
	if _, err := ticket.Decode(full.Encode()); err != nil {
		t.Fatalf("thirty-two entries: %v", err)
	}
	refused(t, owner, full, mutation.OpKnowHowReconfirm, knowHowReconfirm("1", khCommit, khAnchor("a.go", khBlobA), khSymbol("b.go", khBlobA, "F", khDigestF)),
		mutation.OutcomeValidationFailed, wire.CodeLimitExceeded)

	for _, b := range []mutation.Binding{operator, worker, reviewer, importer, system} {
		refused(t, b, add, mutation.OpKnowHowReconfirm, p, mutation.OutcomeUnauthorized, "")
	}
	pv := fixture.PolicyValue()
	pv.Obj.Set("roles", obj("OPERATOR", wire.Strings([]string{"KNOWHOW_RECONFIRM"})))
	ctx = ctxWithPolicy(t, operator, nil, wire.EncodeFile(pv), add)
	ctx.KnowHowAttempts = khnLedger()
	plan = apply(t, ctx, envelope("op1", operator, "AT-01", string(add.Revision), mutation.OpKnowHowReconfirm, p))
	want(t, plan, mutation.OutcomeCompleted, "")
	if plan.Post.KnowHow[1].ActorRole != "OPERATOR" {
		t.Fatalf("operator reconfirm actor: %+v", plan.Post.KnowHow[1])
	}
}

// TestKHNV0019_ReconfirmOfANoteThatIsNotStaleIsRefused: a RECONFIRM whose
// pins equal the note's effective pins is refused VALIDATION_FAILED /
// MALFORMED with the KNOWHOW_NOT_STALE prefix, by the writer and by the
// record codec; one that moves, adds, drops or renames an anchor, or names
// an inactive note, is refused; the payload is closed.
func TestKHNV0019_ReconfirmOfANoteThatIsNotStaleIsRefused(t *testing.T) {
	add := step(t, owner, nil, fixture.Ticket("AT-01"), mutation.OpKnowHowAdd, knowHowAdd("n", "", "",
		khAnchor("a.go", khBlobA), khSymbol("b.go", khBlobA, "F", khDigestF)), "1")
	same := knowHowReconfirm("1", khCommit2, khAnchor("a.go", khBlobA), khSymbol("b.go", khBlobA, "F", khDigestF))
	ctx := newCtx(t, owner, nil, add)
	plan := apply(t, ctx, envelope("ns", owner, "AT-01", string(add.Revision), mutation.OpKnowHowReconfirm, same))
	want(t, plan, mutation.OutcomeValidationFailed, wire.CodeMalformed)
	if !strings.HasPrefix(plan.Detail, wire.KnowHowNotStale+":") {
		t.Fatalf("not-stale detail %q", plan.Detail)
	}
	// The symbol's file changed but its declaration did not: still not STALE.
	blobOnly := knowHowReconfirm("1", khCommit2, khAnchor("a.go", khBlobA), khSymbol("b.go", khBlobB, "F", khDigestF))
	plan = apply(t, newCtx(t, owner, nil, add), envelope("nb", owner, "AT-01", string(add.Revision), mutation.OpKnowHowReconfirm, blobOnly))
	if !strings.HasPrefix(plan.Detail, wire.KnowHowNotStale+":") {
		t.Fatalf("blob-only detail %q", plan.Detail)
	}
	for name, p := range map[string]wire.Value{
		"dropped anchor": knowHowReconfirm("1", khCommit2, khAnchor("a.go", khBlobB)),
		"moved anchor":   knowHowReconfirm("1", khCommit2, khSymbol("b.go", khBlobA, "F", khDigestG), khAnchor("c.go", khBlobB)),
		"renamed symbol": knowHowReconfirm("1", khCommit2, khAnchor("a.go", khBlobB), khSymbol("b.go", khBlobA, "G", khDigestG)),
		"to file anchor": knowHowReconfirm("1", khCommit2, khAnchor("a.go", khBlobB), khAnchor("b.go", khBlobB)),
		"unknown note":   knowHowReconfirm("9", khCommit2, khAnchor("a.go", khBlobB), khSymbol("b.go", khBlobA, "F", khDigestG)),
	} {
		t.Run(name, func(t *testing.T) {
			refused(t, owner, add, mutation.OpKnowHowReconfirm, p, mutation.OutcomeValidationFailed, wire.CodeMalformed)
		})
	}
	ret := step(t, owner, nil, add, mutation.OpKnowHowRetract, obj("note", str("1"), "reason", str("gone")), "1")
	refused(t, owner, ret, mutation.OpKnowHowReconfirm, knowHowReconfirm("1", khCommit2, khAnchor("a.go", khBlobB), khSymbol("b.go", khBlobA, "F", khDigestG)),
		mutation.OutcomeValidationFailed, wire.CodeMalformed)
	withReason := knowHowReconfirm("1", khCommit2, khAnchor("a.go", khBlobB))
	withReason.Obj.Set("reason", str("why"))
	if _, err := mutation.Decode(envelope("p", owner, "AT-01", string(add.Revision), mutation.OpKnowHowReconfirm, withReason)); err == nil {
		t.Fatal("a reconfirm with a reason decoded")
	}

	codec := map[string]ticket.KnowHowEntry{
		"unchanged pins": {Seq: "2", Operation: ticket.KnowHowReconfirm, Note: "1", Anchors: add.KnowHow[0].Anchors, Commit: khCommit2, ActorID: "russell", ActorRole: "OWNER", RecordedAt: now},
		"other anchors":  {Seq: "2", Operation: ticket.KnowHowReconfirm, Note: "1", Anchors: []ticket.KnowHowAnchor{{Path: "a.go", Blob: khBlobB}}, Commit: khCommit2, ActorID: "russell", ActorRole: "OWNER", RecordedAt: now},
		"forward note":   {Seq: "2", Operation: ticket.KnowHowReconfirm, Note: "2", Anchors: []ticket.KnowHowAnchor{{Path: "a.go", Blob: khBlobB}}, Commit: khCommit2, ActorID: "russell", ActorRole: "OWNER", RecordedAt: now},
	}
	for name, e := range codec {
		t.Run("codec "+name, func(t *testing.T) {
			rec, _ := ticket.Decode(add.Encode())
			rec.KnowHow = append(rec.KnowHow, e)
			if _, err := ticket.Decode(rec.Encode()); err == nil || !strings.Contains(err.Error(), "/knowHow") {
				t.Fatalf("decoded %v", err)
			}
		})
	}
}
