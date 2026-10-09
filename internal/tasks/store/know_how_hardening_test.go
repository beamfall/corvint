package store_test

import (
	"archive/tar"
	"bytes"
	"context"
	"io"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/archive"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// knowHowAddPayload is a KNOWHOW_ADD payload with one pinned anchor and no
// provenance; the store does not re-resolve pins, the CLI composes them.
func knowHowAddPayload(text string) wire.Value {
	return obj("text", str(text), "anchors", wire.Array(obj("blob", str(strings.Repeat("a", 40)), "path", str("src/a.go"))),
		"routes", wire.Strings([]string{}), "commit", str(strings.Repeat("c", 40)), "supersedes", wire.Null(), "reason", wire.Null(),
		"attempt", wire.Null(), "generation", wire.Null(), "evidencePath", wire.Null())
}

// knowHowStore is an initialized store with one created ticket; it returns
// the ticket ID and its projection path.
func knowHowStore(t *testing.T) (*intent.Repository, string, string) {
	t.Helper()
	repo, _ := initialized(t)
	created := mutate(t, repo, envelope("req-create", mutation.OpCreate, "", "", createPayload("Know-how home")))
	projection := filepath.Join(repo.PrimaryWorktree, intent.Dir, intent.TicketsDir, created.Ticket[len(created.Ticket)-7:]+".json")
	return repo, created.Ticket, projection
}

// TestKHNV0013_CompetingWritersOneWinner: two writers compose KNOWHOW_ADD
// against the same expected revision before either takes the writer lock,
// the only window in which they overlap. The lock serializes them in a
// fixed order: the first commits note 1; the second is REVISION_CONFLICT
// with nothing written, and its identical retry repeats that refusal. Once
// rebuilt at the new revision it commits note 2, and note 1 stays
// byte-identical.
func TestKHNV0013_CompetingWritersOneWinner(t *testing.T) {
	repo, id, _ := knowHowStore(t)
	first := envelope("kh-a", mutation.OpKnowHowAdd, id, "1", knowHowAddPayload("writer a"))
	second := envelope("kh-b", mutation.OpKnowHowAdd, id, "1", knowHowAddPayload("writer b"))
	if r := mutate(t, repo, first); r.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("first writer: %+v", r.Outcome)
	}
	won := loadRecord(t, repo, id)
	before := storeDigest(t, repo)
	for i := 0; i < 2; i++ {
		lost := mutate(t, repo, second)
		if lost.Outcome.Outcome != mutation.OutcomeRevisionConflict {
			t.Fatalf("second writer, try %d: %+v %s", i, lost.Outcome, lost.Detail)
		}
		if storeDigest(t, repo) != before {
			t.Fatalf("the losing writer changed the store (try %d)", i)
		}
	}
	rebuilt := mutate(t, repo, envelope("kh-b2", mutation.OpKnowHowAdd, id, "2", knowHowAddPayload("writer b")))
	if rebuilt.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("rebuilt writer: %+v", rebuilt.Outcome)
	}
	after := loadRecord(t, repo, id)
	if len(after.KnowHow) != 2 || after.KnowHow[1].Seq != "2" || after.KnowHow[1].Text != "writer b" ||
		!reflect.DeepEqual(after.KnowHow[0], won.KnowHow[0]) {
		t.Fatalf("ledger after the race: %+v", after.KnowHow)
	}
}

// TestKHNV0013_RedoBindsAPendingKnowHowReceipt: a KNOWHOW_ADD receipt that
// was linked in before its head and projection were written is redone by
// the next writer, which publishes the note exactly once.
func TestKHNV0013_RedoBindsAPendingKnowHowReceipt(t *testing.T) {
	repo, id, projection := knowHowStore(t)
	head := filepath.Join(repo.StateDir, "head.json")
	headBefore, projectionBefore := mustRead(t, head), mustRead(t, projection)
	mutate(t, repo, envelope("kh-pending", mutation.OpKnowHowAdd, id, "1", knowHowAddPayload("pending note")))
	fixture.Write(t, head, headBefore)
	fixture.Write(t, projection, projectionBefore)
	next, err := store.Mutate(context.Background(), repo, operator(), envelope("req-second", mutation.OpCreate, "", "", createPayload("Second")), now(t))
	if err != nil || !next.Redone || next.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("redo = %+v, %v", next, err)
	}
	after := loadRecord(t, repo, id)
	if len(after.KnowHow) != 1 || after.KnowHow[0].Text != "pending note" || after.Revision != "2" {
		t.Fatalf("redo lost or doubled the note: %+v", after.KnowHow)
	}
	replay := mutate(t, repo, envelope("kh-pending", mutation.OpKnowHowAdd, id, "1", knowHowAddPayload("pending note")))
	if !replay.Outcome.Replayed || len(loadRecord(t, repo, id).KnowHow) != 1 {
		t.Fatalf("retry of the redone request did not replay: %+v", replay.Outcome)
	}
}

// TestKHNV0012_ArchiveRoundTripKeepsKnowHow: archive export of a store whose
// ticket carries a know-how ledger verifies, and the archived ticket record
// is byte-identical to the projection and decodes to the same ledger.
func TestKHNV0012_ArchiveRoundTripKeepsKnowHow(t *testing.T) {
	repo, id, projection := knowHowStore(t)
	mutate(t, repo, envelope("kh-1", mutation.OpKnowHowAdd, id, "1", knowHowAddPayload("archived note")))
	mutate(t, repo, envelope("kh-2", mutation.OpKnowHowRetract, id, "2", obj("note", str("1"), "reason", str("superseded upstream"))))
	var stream bytes.Buffer
	if _, err := archive.Export(archive.ExportOptions{Repo: repo, Staging: t.TempDir(), Stdout: &stream}); err != nil {
		t.Fatal(err)
	}
	if _, err := archive.Verify(bytes.NewReader(stream.Bytes())); err != nil {
		t.Fatal(err)
	}
	live := mustRead(t, projection)
	want := "intent/tickets/" + filepath.Base(projection)
	tr := tar.NewReader(bytes.NewReader(stream.Bytes()))
	for {
		h, err := tr.Next()
		if err == io.EOF {
			t.Fatalf("archive lacks %s", want)
		}
		if err != nil {
			t.Fatal(err)
		}
		if h.Name != want {
			continue
		}
		raw, err := io.ReadAll(tr)
		if err != nil {
			t.Fatal(err)
		}
		rec, err := ticket.Decode(raw)
		if err != nil || !bytes.Equal(raw, live) {
			t.Fatalf("archived record differs: %v", err)
		}
		if len(rec.KnowHow) != 2 || rec.KnowHow[0].Text != "archived note" || rec.KnowHow[1].Operation != ticket.KnowHowRetract ||
			len(ticket.KnowHowActiveSeqs(rec.KnowHow)) != 0 {
			t.Fatalf("archived ledger: %+v", rec.KnowHow)
		}
		return
	}
}
