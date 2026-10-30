package cli_test

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/cli"
	"github.com/Beamfall/corvint/internal/tasks/dispatch"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// noteHistoryRepo creates one ticket and writes the given note operations
// ("SET:text" or "CLEAR") through the CLI, each superseding the previous.
func noteHistoryRepo(t *testing.T, ops ...string) (*fixture.Repo, string) {
	t.Helper()
	r := fixture.TempRepo(t)
	fixture.Write(t, filepath.Join(r.IntentDir, "queue.json"), fixture.QueueBytes())
	fixture.Write(t, filepath.Join(r.IntentDir, "policy.json"), fixture.PolicyBytes())
	atm(t, r.Root, nil, "init")
	created := atm(t, r.Root, nil, "ticket", "create",
		"--request-id", "req-1", "--issued-at", "2026-09-07T12:00:00Z", "--payload", createPayloadJSON)
	if created.res.Outcome != wire.OutcomeOK {
		t.Fatalf("ticket create: %+v", created.res)
	}
	id := field(created.res.Items[0], "ticketId").Str
	for i, op := range ops {
		noteWrite(t, r, id, i, op)
	}
	return r, id
}

func noteWrite(t *testing.T, r *fixture.Repo, id string, prior int, op string) {
	t.Helper()
	args := []string{"ticket", "note", "clear", id}
	if text, ok := strings.CutPrefix(op, "SET:"); ok {
		args = []string{"ticket", "note", "set", id, "--text", text}
	}
	args = append(args, "--request-id", "note-"+strconv.Itoa(prior+1), "--issued-at", "2026-10-05T12:00:00Z",
		"--supersedes", strconv.Itoa(prior))
	if x := atm(t, r.Root, nil, args...); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("note %s: %+v", op, x.res)
	}
}

func noteHistoryPage(t *testing.T, r *fixture.Repo, args ...string) (wire.Value, *wire.Result) {
	t.Helper()
	x := atm(t, r.Root, nil, append([]string{"ticket", "note", "history"}, args...)...)
	if x.res.Outcome != wire.OutcomeOK {
		return wire.Value{}, x.res
	}
	if len(x.res.Items) != 1 || !x.res.Untrusted || x.res.Page == nil {
		t.Fatalf("history result shape: %+v", x.res)
	}
	return x.res.Items[0], x.res
}

func entryRevisions(v wire.Value) []string {
	var out []string
	for _, e := range field(v, "entries").Arr {
		out = append(out, field(e, "operation").Str+":"+field(e, "revision").Str)
	}
	return out
}

// TestONV0008_NoteHistoryPagesAnchoredChain drives `ticket note history`:
// a never-noted ticket has an empty history, superseded and cleared notes
// stay readable newest first, pages follow an opaque cursor bound to their
// anchor even after a concurrent replacement, every read leaves the state
// and intent trees byte-identical, and a tampered or foreign cursor refuses.
func TestONV0008_NoteHistoryPagesAnchoredChain(t *testing.T) {
	r, id := noteHistoryRepo(t)
	empty, res := noteHistoryPage(t, r, id)
	if empty.Kind == wire.KindNull || len(field(empty, "entries").Arr) != 0 || field(empty, "anchor").Kind != wire.KindNull ||
		field(empty, "nextCursor").Kind != wire.KindNull || res.Page.Total == nil || *res.Page.Total != "0" {
		t.Fatalf("never-noted history: %s %+v", wire.Encode(empty), res.Page)
	}

	for i, op := range []string{"SET:first", "SET:second", "CLEAR", "SET:third", "SET:fourth"} {
		noteWrite(t, r, id, i, op)
	}
	stateBefore, intentBefore := fixture.TreeSnapshot(t, r.StateDir), fixture.TreeSnapshot(t, r.IntentDir)

	all, res := noteHistoryPage(t, r, id)
	if got := strings.Join(entryRevisions(all), " "); got != "SET:5 SET:4 CLEAR:3 SET:2 SET:1" {
		t.Fatalf("full history = %s", got)
	}
	if res.Page.Truncated || field(all, "nextCursor").Kind != wire.KindNull || *res.Page.Total != "5" {
		t.Fatalf("full history page: %+v %s", res.Page, wire.Encode(all))
	}
	entries := field(all, "entries").Arr
	if !field(entries[0], "current").Bool || field(entries[1], "current").Bool || field(entries[0], "text").Str != "fourth" ||
		field(entries[2], "text").Kind != wire.KindNull || field(entries[4], "previous").Kind != wire.KindNull ||
		field(entries[3], "requestId").Str != "note-2" || field(field(entries[3], "actor"), "role").Str != "OWNER" {
		t.Fatalf("history entries: %s", wire.Encode(all))
	}
	for i := 0; i+1 < len(entries); i++ {
		if field(entries[i], "previous").Str != field(entries[i+1], "sha256").Str {
			t.Fatalf("entry %d does not link to its predecessor: %s", i, wire.Encode(all))
		}
	}

	first, res := noteHistoryPage(t, r, id, "--limit", "2")
	if got := strings.Join(entryRevisions(first), " "); got != "SET:5 SET:4" || !res.Page.Truncated || res.Page.Offset != "0" {
		t.Fatalf("first page = %s %+v", got, res.Page)
	}
	cursor := field(first, "nextCursor").Str
	if cursor == "" || strings.ContainsAny(cursor, "/.") {
		t.Fatalf("cursor is not opaque: %q", cursor)
	}
	if !fixture.SameTree(stateBefore, fixture.TreeSnapshot(t, r.StateDir)) || !fixture.SameTree(intentBefore, fixture.TreeSnapshot(t, r.IntentDir)) {
		t.Fatal("a history read changed the store")
	}

	// A concurrent replacement after the first page does not move the
	// following pages off their anchor.
	noteWrite(t, r, id, 5, "SET:fifth")
	second, res := noteHistoryPage(t, r, id, "--limit", "2", "--cursor", cursor)
	if got := strings.Join(entryRevisions(second), " "); got != "CLEAR:3 SET:2" || res.Page.Offset != "2" || *res.Page.Total != "5" {
		t.Fatalf("second page = %s %+v", got, res.Page)
	}
	if field(field(second, "anchor"), "revision").Str != "5" || field(field(second, "committedHead"), "revision").Str != "6" {
		t.Fatalf("second page anchor: %s", wire.Encode(second))
	}
	third, res := noteHistoryPage(t, r, id, "--limit", "2", "--cursor", field(second, "nextCursor").Str)
	if got := strings.Join(entryRevisions(third), " "); got != "SET:1" || res.Page.Truncated || field(third, "nextCursor").Kind != wire.KindNull {
		t.Fatalf("third page = %s %+v", got, res.Page)
	}

	refuses := func(name string, args ...string) {
		t.Helper()
		if _, res := noteHistoryPage(t, r, args...); res == nil || res.Outcome == wire.OutcomeOK || len(res.Codes) == 0 || res.Codes[0] != wire.CodeMalformed {
			t.Fatalf("%s was not refused MALFORMED: %+v", name, res)
		}
	}
	refuses("limit 51", id, "--limit", "51")
	refuses("limit 0", id, "--limit", "0")
	refuses("garbage cursor", id, "--cursor", "not-a-cursor")
	refuses("path cursor", id, "--cursor", "../evidence/x")
	c, err := store.DecodeOperatorNoteCursor(cursor)
	if err != nil {
		t.Fatal(err)
	}
	forged := *c
	forged.Next = forged.Anchor
	refuses("cursor off the chain", id, "--cursor", forged.Encode())
	forged = *c
	forged.AnchorRevision = "7"
	refuses("cursor anchored past the head", id, "--cursor", forged.Encode())
	forged = *c
	forged.TicketID = c.TicketID + "x"
	refuses("cursor of another ticket", id, "--cursor", forged.Encode())

	other := atm(t, r.Root, nil, "ticket", "create", "--request-id", "req-2", "--issued-at", "2026-09-07T12:00:00Z", "--payload", createPayloadJSON)
	if other.res.Outcome != wire.OutcomeOK {
		t.Fatalf("second ticket: %+v", other.res)
	}
	refuses("cursor on a never-noted ticket", field(other.res.Items[0], "ticketId").Str, "--cursor", cursor)
}

// TestONV0008_NoteHistoryRefusesBrokenEvidence checks that a missing or
// rewritten middle event refuses the read instead of shortening history, and
// that the 1 MiB page cut stops before the bound with a next cursor while a
// single entry always fits.
func TestONV0008_NoteHistoryRefusesBrokenEvidence(t *testing.T) {
	r, id := noteHistoryRepo(t, "SET:one", "SET:two", "SET:three")
	all, _ := noteHistoryPage(t, r, id)
	entries := field(all, "entries").Arr
	middle := filepath.Join(r.StateDir, "evidence", field(entries[1], "sha256").Str)
	saved, err := os.ReadFile(middle)
	if err != nil {
		t.Fatal(err)
	}

	repo, err := intent.Resolve(r.Root)
	if err != nil {
		t.Fatal(err)
	}
	tid, err := wire.ParseTicketID("ticket", id)
	if err != nil {
		t.Fatal(err)
	}
	head := wire.Digest(field(entries[0], "sha256").Str)
	ref := ticket.OperatorNoteReference{Revision: "3", Current: &head, Head: head}
	big := func(store.OperatorNoteHistoryEntry) int { return store.OperatorNoteHistoryBytes/2 + 1 }
	page, err := store.OperatorNoteHistory(repo, tid, ref, nil, 50, big)
	if err != nil || len(page.Entries) != 1 || page.Next == nil || page.Next.NextRevision != "2" {
		t.Fatalf("byte cut: %+v %v", page, err)
	}
	huge := func(store.OperatorNoteHistoryEntry) int { return store.OperatorNoteHistoryBytes + 1 }
	if page, err = store.OperatorNoteHistory(repo, tid, ref, nil, 50, huge); err != nil || len(page.Entries) != 1 {
		t.Fatalf("one entry must always fit: %+v %v", page, err)
	}

	if err := os.Remove(middle); err != nil {
		t.Fatal(err)
	}
	if _, res := noteHistoryPage(t, r, id); res == nil || res.Outcome == wire.OutcomeOK || len(res.Codes) == 0 ||
		(res.Codes[0] != wire.CodeMissingEvidence && res.Codes[0] != wire.CodeJournalForked) {
		t.Fatalf("missing middle event was not refused: %+v", res)
	}
	if _, err := store.OperatorNoteHistory(repo, tid, ref, nil, 50, big); err == nil || !strings.Contains(err.Error(), "not in the evidence store") {
		t.Fatalf("store read of a missing middle event: %v", err)
	}

	// A rewritten event stored under the original digest fails its digest.
	if err := os.WriteFile(middle, append(append([]byte(nil), saved[:len(saved)-1]...), ' ', '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := store.OperatorNoteHistory(repo, tid, ref, nil, 50, big); err == nil {
		t.Fatal("a rewritten middle event was accepted")
	}
}

// TestONV0011_DispatchObservesTheCurrentNote checks the native dispatcher
// observation behind the {operatorNote} placeholder: a never-noted ticket
// has no note view, a SET is CURRENT with its text and provenance, and a
// CLEAR is CLEARED without text. The observation writes nothing.
func TestONV0011_DispatchObservesTheCurrentNote(t *testing.T) {
	r, id := noteHistoryRepo(t)
	view := func() *dispatch.NoteView {
		t.Helper()
		obs, err := cli.ObserveDispatch(r.Root)
		if err != nil {
			t.Fatal(err)
		}
		for _, tk := range obs.Tickets {
			if tk.ID == id {
				return tk.OperatorNote
			}
		}
		t.Fatalf("ticket %s not observed", id)
		return nil
	}
	if n := view(); n != nil {
		t.Fatalf("never-noted ticket observed a note: %+v", n)
	}
	noteWrite(t, r, id, 0, "SET:rerun the flaky gate once")
	before := fixture.TreeSnapshot(t, r.StateDir)
	n := view()
	if n == nil || n.State != "CURRENT" || n.Text != "rerun the flaky gate once" || n.Revision != "1" || n.ActorRole != "OWNER" || n.RecordedAt == "" {
		t.Fatalf("current note view: %+v", n)
	}
	if !fixture.SameTree(before, fixture.TreeSnapshot(t, r.StateDir)) {
		t.Fatal("the dispatcher observation changed the store")
	}
	noteWrite(t, r, id, 1, "CLEAR")
	cleared := view()
	if cleared == nil || cleared.State != "CLEARED" || cleared.Text != "" || cleared.Revision != "2" {
		t.Fatalf("cleared note view: %+v", cleared)
	}
	// A head event missing from the evidence store is UNAVAILABLE with its
	// code, never no note, and does not fail the rest of the observation.
	if err := os.Remove(filepath.Join(r.StateDir, "evidence", cleared.Head)); err != nil {
		t.Fatal(err)
	}
	obs, err := cli.ObserveDispatch(r.Root)
	if err != nil {
		t.Fatalf("observation with a missing note event: %v", err)
	}
	for _, tk := range obs.Tickets {
		if tk.ID == id && (tk.OperatorNote == nil || tk.OperatorNote.State != "UNAVAILABLE" || tk.OperatorNote.Code != "MISSING_EVIDENCE") {
			t.Fatalf("missing head event view: %+v", tk.OperatorNote)
		}
	}
}
