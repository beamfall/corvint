package store_test

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/journal"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// noteEnvelope builds one NOTE_SET/NOTE_CLEAR taskman-mutation/0; an empty
// expected or supersedes is null.
func noteEnvelope(requestID, operation, target, expected, text, supersedes string) []byte {
	rev, sup := wire.Null(), wire.Null()
	if expected != "" {
		rev = str(expected)
	}
	if supersedes != "" {
		sup = str(supersedes)
	}
	payload := obj("supersedes", sup)
	if operation == mutation.OpNoteSet {
		payload = obj("supersedes", sup, "text", str(text))
	}
	return wire.EncodeFile(obj(
		"profile", str(mutation.Profile),
		"requestId", str(requestID),
		"actor", obj("id", str("tester"), "role", str("OWNER")),
		"queueId", str(fixture.QueueID),
		"targetId", str(target),
		"expectedRevision", rev,
		"operation", str(operation),
		"payload", payload,
		"issuedAt", str(issued),
	))
}

func loadRecord(t *testing.T, repo *intent.Repository, id string) *ticket.Record {
	t.Helper()
	loaded, err := intent.Load(repo.PrimaryWorktree)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	rec, ok := loaded.Inventory.Get(id)
	if !ok {
		t.Fatalf("%s missing", id)
	}
	return rec
}

// TestONV0006_NativeNoteSetClearReplayAndAudit is the first native fixture:
// SET posts the ticket and its event in one MUTATE stage, the reference and
// event agree, identical replay returns the original outcome, a stale
// supersedes conflicts, CLEAR keeps a tombstone, and an ordinary REFINE then
// preserves the reference through a full audit.
func TestONV0006_NativeNoteSetClearReplayAndAudit(t *testing.T) {
	t.Parallel()
	repo, _ := initialized(t)
	created := mutate(t, repo, envelope("req-create", mutation.OpCreate, "", "", createPayload("Noted ticket")))
	set := noteEnvelope("req-note-1", mutation.OpNoteSet, created.Ticket, "", "Use the staging database.", "0")
	first := mutate(t, repo, set)
	if first.Outcome.Outcome != mutation.OutcomeCompleted || first.Receipt == "" {
		t.Fatalf("set outcome = %s (%v) %s", first.Outcome.Outcome, first.Outcome.Codes, first.Detail)
	}
	rec := loadRecord(t, repo, created.Ticket)
	if rec.OperatorNote == nil || rec.OperatorNote.Revision != "1" || rec.OperatorNote.Current == nil || *rec.OperatorNote.Current != rec.OperatorNote.Head {
		t.Fatalf("reference after SET = %+v", rec.OperatorNote)
	}
	if rec.Revision != "2" || rec.AcceptanceRevision != "1" {
		t.Errorf("revisions = %s/%s, want 2/1", rec.Revision, rec.AcceptanceRevision)
	}
	raw, err := os.ReadFile(filepath.Join(repo.StateDir, "evidence", string(rec.OperatorNote.Head)))
	if err != nil {
		t.Fatalf("note event not stored: %v", err)
	}
	ev, err := ticket.ResolveOperatorNote(rec.TicketID, *rec.OperatorNote, raw)
	if err != nil || ev.Operation != "SET" || ev.TicketRevision != "2" {
		t.Fatalf("event = %+v, %v", ev, err)
	}

	before := storeDigest(t, repo)
	again := mutate(t, repo, set)
	if !again.Outcome.Replayed || again.Receipt != "" {
		t.Fatalf("identical NOTE_SET did not replay: %+v", again.Outcome)
	}
	stale := mutate(t, repo, noteEnvelope("req-note-stale", mutation.OpNoteSet, created.Ticket, "", "Other", "0"))
	if stale.Outcome.Outcome != mutation.OutcomeRevisionConflict || stale.Receipt != "" {
		t.Fatalf("stale supersedes = %s %s", stale.Outcome.Outcome, stale.Detail)
	}
	if storeDigest(t, repo) != before {
		t.Fatal("replay or conflict changed the store")
	}

	refined := mutate(t, repo, envelope("req-refine", mutation.OpRefine, created.Ticket, "2", obj("title", str("Refined"))))
	if refined.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("refine = %s %s", refined.Outcome.Outcome, refined.Detail)
	}
	after := loadRecord(t, repo, created.Ticket)
	if after.OperatorNote == nil || after.OperatorNote.Head != rec.OperatorNote.Head {
		t.Fatalf("REFINE dropped the reference: %+v", after.OperatorNote)
	}

	cleared := mutate(t, repo, noteEnvelope("req-note-2", mutation.OpNoteClear, created.Ticket, "3", "", "1"))
	if cleared.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("clear = %s (%v) %s", cleared.Outcome.Outcome, cleared.Outcome.Codes, cleared.Detail)
	}
	tomb := loadRecord(t, repo, created.Ticket)
	if tomb.OperatorNote == nil || tomb.OperatorNote.Revision != "2" || tomb.OperatorNote.Current != nil {
		t.Fatalf("reference after CLEAR = %+v", tomb.OperatorNote)
	}
}

// noteAudit runs the full receipt audit over the native store.
func noteAudit(t *testing.T, repo *intent.Repository) (*journal.Result, error) {
	t.Helper()
	q, _ := wire.ParseQueueID("", fixture.QueueID)
	return (journal.Reader{Source: journal.Native{StateDir: repo.StateDir, PrimaryWorktree: repo.PrimaryWorktree}, QueueID: q, PrimaryWorktree: repo.PrimaryWorktree}).Audit()
}

// forgeReceipt rewrites one retained receipt's pre/post pairs and relinks
// head.json when it names that receipt, so a counterexample fails on note
// semantics rather than on a stale outer hash. A rewritten ticket post is
// also written to its projection.
func forgeReceipt(t *testing.T, repo *intent.Repository, name string, edit func(path string, record wire.Value) (wire.Value, bool)) {
	t.Helper()
	forgeReceiptWith(t, repo, name, edit, nil)
}

// forgeReceiptWith is forgeReceipt plus add, which may rewrite the parsed
// receipt (its pre/post arrays and identity) before it is re-encoded.
func forgeReceiptWith(t *testing.T, repo *intent.Repository, name string, edit func(path string, record wire.Value) (wire.Value, bool), add func(receipt wire.Value)) {
	t.Helper()
	path := filepath.Join(repo.StateDir, "receipts", name)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	v, err := wire.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	pre, _ := v.Obj.Get("pre")
	post, _ := v.Obj.Get("post")
	var keptPre, keptPost []wire.Value
	for i, p := range post.Arr {
		dest, _ := p.Obj.Get("path")
		rec, _ := p.Obj.Get("record")
		next, keep := edit(dest.Str, rec)
		if !keep {
			continue
		}
		if next.Kind == wire.KindObject && rec.Kind == wire.KindObject {
			b := wire.EncodeFile(next)
			p.Obj.Set("record", next).Set("sha256", str(string(wire.Sum(b))))
			if strings.HasPrefix(dest.Str, "intent/") {
				fixture.Write(t, filepath.Join(repo.PrimaryWorktree, intent.Dir, strings.TrimPrefix(dest.Str, "intent/")), b)
			}
		}
		keptPre, keptPost = append(keptPre, pre.Arr[i]), append(keptPost, p)
	}
	v.Obj.Set("pre", wire.Array(keptPre...)).Set("post", wire.Array(keptPost...))
	if add != nil {
		add(v)
	}
	forged := wire.EncodeFile(v)
	fixture.Write(t, path, forged)
	hp := filepath.Join(repo.StateDir, "head.json")
	h, err := wire.Parse(mustRead(t, hp))
	if err != nil {
		t.Fatal(err)
	}
	if last, _ := h.Obj.Get("lastReceiptSha256"); last.Str == string(wire.Sum(raw)) {
		h.Obj.Set("lastReceiptSha256", str(string(wire.Sum(forged))))
		fixture.Write(t, hp, wire.EncodeFile(h))
	}
}

func mustRead(t *testing.T, p string) []byte {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// withField returns a copy of an object with key set, or removed when v is
// the zero Value.
func withField(o wire.Value, key string, v wire.Value) wire.Value {
	out := wire.NewObject()
	for _, k := range o.Obj.SortedKeys() {
		if k != key {
			old, _ := o.Obj.Get(k)
			out.Set(k, old)
		}
	}
	if v.Kind != 0 || v.Obj != nil || v.Str != "" {
		out.Set(key, v)
	}
	return wire.ObjectValue(out)
}

// TestONV0006_ReceiptAuditBindsNoteHistory: SET, an ordinary REFINE, CLEAR
// and a superseding SET all replay from audited pre-state, so a store whose
// only extension is the note profile audits with known semantic coverage.
func TestONV0006_ReceiptAuditBindsNoteHistory(t *testing.T) {
	t.Parallel()
	repo, _ := initialized(t)
	created := mutate(t, repo, envelope("req-create", mutation.OpCreate, "", "", createPayload("Noted ticket")))
	for _, env := range [][]byte{
		noteEnvelope("req-note-1", mutation.OpNoteSet, created.Ticket, "", "First.", "0"),
		envelope("req-refine", mutation.OpRefine, created.Ticket, "2", obj("title", str("Refined"))),
		noteEnvelope("req-note-2", mutation.OpNoteClear, created.Ticket, "3", "", "1"),
		noteEnvelope("req-note-3", mutation.OpNoteSet, created.Ticket, "", "Second.", "2"),
	} {
		if r := mutate(t, repo, env); r.Outcome.Outcome != mutation.OutcomeCompleted {
			t.Fatalf("mutate = %s (%v) %s", r.Outcome.Outcome, r.Outcome.Codes, r.Detail)
		}
	}
	result, err := noteAudit(t, repo)
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if result.SemanticCoverage == "UNKNOWN" {
		t.Errorf("semantic coverage = UNKNOWN for a note-only store")
	}
}

// TestONV0006_ForgedNoteHistoryIsJournalForked rewrites receipts with a
// consistent outer hash chain; each forgery must fail the note binding.
func TestONV0006_ForgedNoteHistoryIsJournalForked(t *testing.T) {
	t.Parallel()
	ticketPost := func(path string) bool { return strings.HasPrefix(path, "intent/tickets/") }
	cases := map[string]struct {
		refine bool
		edit   func(path string, rec wire.Value) (wire.Value, bool)
	}{
		"note post differs from replay": {edit: func(path string, rec wire.Value) (wire.Value, bool) {
			if ticketPost(path) {
				return withField(rec, "title", str("Forged")), true
			}
			return rec, true
		}},
		"note event removed": {edit: func(path string, rec wire.Value) (wire.Value, bool) {
			return rec, !strings.HasPrefix(path, "evidence/")
		}},
		"reference dropped without event": {refine: true, edit: func(path string, rec wire.Value) (wire.Value, bool) {
			if ticketPost(path) {
				return withField(rec, "operatorNote", wire.Value{}), true
			}
			return rec, true
		}},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			repo, _ := initialized(t)
			created := mutate(t, repo, envelope("req-create", mutation.OpCreate, "", "", createPayload("Noted ticket")))
			last := mutate(t, repo, noteEnvelope("req-note-1", mutation.OpNoteSet, created.Ticket, "", "First.", "0"))
			if c.refine {
				last = mutate(t, repo, envelope("req-refine", mutation.OpRefine, created.Ticket, "2", obj("title", str("Refined"))))
			}
			if _, err := noteAudit(t, repo); err != nil {
				t.Fatalf("audit before forgery: %v", err)
			}
			forgeReceipt(t, repo, last.Receipt, c.edit)
			_, err := noteAudit(t, repo)
			if wire.CodeOf(err) != wire.CodeJournalForked || !strings.Contains(err.Error(), "operator note") {
				t.Fatalf("audit after forgery = %v, want JOURNAL_FORKED from the note binding", err)
			}
		})
	}
}

// TestONV0006_RedoBindsAPendingNoteReceipt: a linked-in NOTE_SET whose head and
// projection were not yet written is redone through the bound audit, and the
// same pending receipt with a forged ticket post is refused.
func TestONV0006_RedoBindsAPendingNoteReceipt(t *testing.T) {
	t.Parallel()
	for _, forged := range []bool{false, true} {
		repo, _ := initialized(t)
		created := mutate(t, repo, envelope("req-create", mutation.OpCreate, "", "", createPayload("Noted ticket")))
		head := filepath.Join(repo.StateDir, "head.json")
		projection := filepath.Join(repo.PrimaryWorktree, intent.Dir, intent.TicketsDir, created.Ticket[len(created.Ticket)-7:]+".json")
		headBefore, projectionBefore := mustRead(t, head), mustRead(t, projection)
		set := mutate(t, repo, noteEnvelope("req-note-1", mutation.OpNoteSet, created.Ticket, "", "First.", "0"))
		fixture.Write(t, head, headBefore)
		fixture.Write(t, projection, projectionBefore)
		if forged {
			forgeReceipt(t, repo, set.Receipt, func(path string, rec wire.Value) (wire.Value, bool) {
				if strings.HasPrefix(path, "intent/tickets/") {
					return withField(rec, "title", str("Forged")), true
				}
				return rec, true
			})
			fixture.Write(t, projection, projectionBefore)
		}
		next, err := store.Mutate(context.Background(), repo, operator(), envelope("req-second", mutation.OpCreate, "", "", createPayload("Second")), now(t))
		if forged {
			if wire.CodeOf(err) != wire.CodeJournalForked || !strings.Contains(err.Error(), "operator note") {
				t.Fatalf("forged pending note receipt = %+v, %v; want JOURNAL_FORKED from the note binding", next, err)
			}
			if after := loadRecord(t, repo, created.Ticket); after.OperatorNote != nil || after.Title == "Forged" {
				t.Fatalf("forged redo published %+v", after)
			}
			continue
		}
		if err != nil || !next.Redone || next.Outcome.Outcome != mutation.OutcomeCompleted {
			t.Fatalf("redo = %+v, %v", next, err)
		}
		if after := loadRecord(t, repo, created.Ticket); after.OperatorNote == nil || after.OperatorNote.Revision != "1" {
			t.Fatalf("redo lost the note reference: %+v", after.OperatorNote)
		}
	}
}

// TestONV0006_CheckpointTailNeverReadsThePrefix: a checkpoint-resumed read
// keeps its tail for an ordinary REFINE of a noted ticket, and falls back to
// the complete audit rather than read a pre-checkpoint receipt when a note
// transition's pre-state precedes the checkpoint (CAL-V0-061).
func TestONV0006_CheckpointTailNeverReadsThePrefix(t *testing.T) {
	t.Parallel()
	repo, _ := initialized(t)
	created := mutate(t, repo, envelope("req-create", mutation.OpCreate, "", "", createPayload("Noted ticket")))
	mutate(t, repo, noteEnvelope("req-note-1", mutation.OpNoteSet, created.Ticket, "", "First.", "0"))
	full, err := noteAudit(t, repo)
	if err != nil {
		t.Fatal(err)
	}
	cp := full.Checkpoint()
	q, _ := wire.ParseQueueID("", fixture.QueueID)
	resumed := func() *journal.Result {
		t.Helper()
		res, err := (journal.Reader{Source: journal.Native{StateDir: repo.StateDir, PrimaryWorktree: repo.PrimaryWorktree}, QueueID: q, PrimaryWorktree: repo.PrimaryWorktree, Checkpoint: cp}).Audit()
		if err != nil {
			t.Fatalf("resumed audit: %v", err)
		}
		return res
	}
	mutate(t, repo, envelope("req-refine", mutation.OpRefine, created.Ticket, "2", obj("title", str("Refined"))))
	if res := resumed(); res.Mode != journal.ModeCheckpoint {
		t.Fatalf("REFINE tail mode = %s, want %s", res.Mode, journal.ModeCheckpoint)
	}
	mutate(t, repo, noteEnvelope("req-note-2", mutation.OpNoteClear, created.Ticket, "3", "", "1"))
	if res := resumed(); res.Mode != journal.ModeFull || res.SemanticCoverage == "UNKNOWN" {
		t.Fatalf("note tail = %s %s, want the complete audit's verdict", res.Mode, res.SemanticCoverage)
	}
}

// TestONV0006_NoteReceiptBindsOnlyItsOwnTransition: a rehashed note receipt
// that also posts an unrelated state path, or whose request index and receipt
// were consistently renamed away from the retained request's own ID, fails
// the note binding both in a settled audit and as a pending receipt offered
// to redo, which then publishes nothing.
func TestONV0006_NoteReceiptBindsOnlyItsOwnTransition(t *testing.T) {
	t.Parallel()
	keep := func(_ string, rec wire.Value) (wire.Value, bool) { return rec, true }
	extraPost := func(t *testing.T, repo *intent.Repository) func(wire.Value) {
		genesis, err := wire.Parse(mustRead(t, filepath.Join(repo.StateDir, "receipts", "000000000001.json")))
		if err != nil {
			t.Fatal(err)
		}
		gp, _ := genesis.Obj.Get("post")
		var reservations wire.Value
		for _, p := range gp.Arr {
			if dest, _ := p.Obj.Get("path"); dest.Str == "reservations.json" {
				reservations = p
			}
		}
		sum, _ := reservations.Obj.Get("sha256")
		return func(v wire.Value) {
			pre, _ := v.Obj.Get("pre")
			post, _ := v.Obj.Get("post")
			v.Obj.Set("pre", wire.Array(append(pre.Arr, obj("path", str("reservations.json"), "sha256", sum))...))
			v.Obj.Set("post", wire.Array(append(post.Arr, reservations)...))
		}
	}
	renamed := func(t *testing.T, repo *intent.Repository) func(wire.Value) {
		const to = "req-note-renamed"
		return func(v wire.Value) {
			newPath, err := snapshot.RequestPath(to)
			if err != nil {
				t.Fatal(err)
			}
			v.Obj.Set("requestId", str(to))
			pre, _ := v.Obj.Get("pre")
			post, _ := v.Obj.Get("post")
			for i, p := range post.Arr {
				dest, _ := p.Obj.Get("path")
				if !strings.HasPrefix(dest.Str, "requests/") {
					continue
				}
				rec, _ := p.Obj.Get("record")
				outcome, _ := rec.Obj.Get("outcome")
				rec = withField(rec, "requestId", str(to))
				rec = withField(rec, "outcome", withField(outcome, "requestId", str(to)))
				b := wire.EncodeFile(rec)
				p.Obj.Set("path", str(newPath)).Set("record", rec).Set("sha256", str(string(wire.Sum(b))))
				pre.Arr[i].Obj.Set("path", str(newPath))
				if err := os.Remove(filepath.Join(repo.StateDir, filepath.FromSlash(dest.Str))); err != nil && !os.IsNotExist(err) {
					t.Fatal(err)
				}
				fixture.Write(t, filepath.Join(repo.StateDir, filepath.FromSlash(newPath)), b)
			}
		}
	}
	for name, forge := range map[string]func(*testing.T, *intent.Repository) func(wire.Value){"unrelated post": extraPost, "renamed request": renamed} {
		for _, pending := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/pending=%v", name, pending), func(t *testing.T) {
				repo, _ := initialized(t)
				created := mutate(t, repo, envelope("req-create", mutation.OpCreate, "", "", createPayload("Noted ticket")))
				head := filepath.Join(repo.StateDir, "head.json")
				projection := filepath.Join(repo.PrimaryWorktree, intent.Dir, intent.TicketsDir, created.Ticket[len(created.Ticket)-7:]+".json")
				headBefore, projectionBefore := mustRead(t, head), mustRead(t, projection)
				reservations := filepath.Join(repo.StateDir, "reservations.json")
				reservationsBefore := mustRead(t, reservations)
				set := mutate(t, repo, noteEnvelope("req-note-1", mutation.OpNoteSet, created.Ticket, "", "First.", "0"))
				if pending {
					fixture.Write(t, head, headBefore)
				}
				forgeReceiptWith(t, repo, set.Receipt, keep, forge(t, repo))
				if !pending {
					_, err := noteAudit(t, repo)
					if wire.CodeOf(err) != wire.CodeJournalForked || !strings.Contains(err.Error(), "operator note") {
						t.Fatalf("audit = %v, want JOURNAL_FORKED from the note binding", err)
					}
					return
				}
				fixture.Write(t, projection, projectionBefore)
				next, err := store.Mutate(context.Background(), repo, operator(), envelope("req-second", mutation.OpCreate, "", "", createPayload("Second")), now(t))
				if wire.CodeOf(err) != wire.CodeJournalForked || !strings.Contains(err.Error(), "operator note") {
					t.Fatalf("redo = %+v, %v; want JOURNAL_FORKED from the note binding", next, err)
				}
				if after := loadRecord(t, repo, created.Ticket); after.OperatorNote != nil {
					t.Fatalf("refused redo published the note: %+v", after.OperatorNote)
				}
				if !bytes.Equal(mustRead(t, reservations), reservationsBefore) || !bytes.Equal(mustRead(t, head), headBefore) {
					t.Fatal("refused redo moved reservations or head")
				}
			})
		}
	}
}
