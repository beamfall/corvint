// SPDX-License-Identifier: Apache-2.0

package intent_test

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

var writePathTickets = []string{"tickets/A.json", "tickets/B.json", "tickets/C.json", "tickets/D.json"}

// writePathRepo writes OPEN B and D and COMPLETED A and C, ages every ticket
// file out of the racy window and writes the index.
func writePathRepo(t *testing.T) *fixture.Repo {
	t.Helper()
	r := fixture.TempRepo(t)
	reason := "done"
	var recs []*ticket.Record
	for _, local := range []string{"A", "B", "C", "D"} {
		rec := fixture.Ticket(local)
		if local == "A" || local == "C" {
			rec.Status = ticket.StatusCompleted
			rec.Completion = &ticket.Completion{Kind: "MANUAL", Actor: fixture.Actor, Reason: &reason, Evidence: []wire.Digest{}, RecordedAt: fixture.Timestamp}
		}
		recs = append(recs, rec)
	}
	recs[1].Dependencies = []ticket.Dependency{fixture.Dep("A")}
	fixture.WriteState(t, r)
	fixture.WriteIntent(t, r, recs...)
	old := time.Now().Add(-time.Hour)
	for _, rec := range recs {
		if err := os.Chtimes(writePathTicket(r, rec.TicketID.Local), old, old); err != nil {
			t.Fatal(err)
		}
	}
	if err := intent.WriteClosedIndex(r.Root, r.StateDir); err != nil {
		t.Fatalf("write index: %v", err)
	}
	return r
}

func writePathTicket(r *fixture.Repo, local string) string {
	return filepath.Join(r.IntentDir, intent.TicketsDir, local+".json")
}

// TestTCXV0845IndexAdmitsOnlyReencodedBytes: the format 3 index holds every
// aged ticket, open and closed, and leaves out a ticket read by a rewrite
// whose record does not re-encode to exactly its bytes.
func TestTCXV0845IndexAdmitsOnlyReencodedBytes(t *testing.T) {
	r := writePathRepo(t)
	idx := intent.ReadClosedIndex(r.StateDir)
	if idx == nil {
		t.Fatal("index does not read back")
	}
	if got := intent.IndexedTicketPaths(idx); !slices.Equal(got, writePathTickets) {
		t.Fatalf("indexed %v, want every ticket %v", got, writePathTickets)
	}
	restore := intent.SetReencode(func(rec *ticket.Record) []byte {
		raw := rec.Encode()
		if rec.TicketID.Local == "D" {
			raw = append(raw, ' ')
		}
		return raw
	})
	defer restore()
	// A new stat key makes the rewrite read D again instead of keeping its
	// entry, so D must pass the re-encode check.
	older := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(writePathTicket(r, "D"), older, older); err != nil {
		t.Fatal(err)
	}
	if err := intent.WriteClosedIndex(r.Root, r.StateDir); err != nil {
		t.Fatal(err)
	}
	want := []string{"tickets/A.json", "tickets/B.json", "tickets/C.json"}
	if got := intent.IndexedTicketPaths(intent.ReadClosedIndex(r.StateDir)); !slices.Equal(got, want) {
		t.Fatalf("indexed %v after a failed re-encode of D, want %v", got, want)
	}
}

// TestTCXV0845ReadsTrustOnlyClosedEntries: a read's indexed capture takes
// only the closed tickets from the index and reads every open one.
func TestTCXV0845ReadsTrustOnlyClosedEntries(t *testing.T) {
	r := writePathRepo(t)
	tree, err := intent.TreeDigestIndexed(r.Root, intent.ReadClosedIndex(r.StateDir))
	if err != nil {
		t.Fatal(err)
	}
	seen := 0
	for _, f := range tree.Files {
		if !slices.Contains(writePathTickets, f.Path) {
			continue
		}
		seen++
		closed := f.Path == "tickets/A.json" || f.Path == "tickets/C.json"
		if closed != (f.Raw == nil) {
			t.Errorf("%s: read=%v, want a read exactly for an open ticket", f.Path, f.Raw != nil)
		}
	}
	if seen != len(writePathTickets) {
		t.Fatalf("capture listed %d tickets, want %d", seen, len(writePathTickets))
	}
}

func indexBody(t *testing.T, r *fixture.Repo) []byte {
	t.Helper()
	raw, err := os.ReadFile(intent.ClosedIndexPath(r.StateDir))
	if err != nil {
		t.Fatal(err)
	}
	return raw[bytes.IndexByte(raw, '\n')+1:]
}

// TestTCXV0849IncrementalRewriteMatchesFullEncode: after one ticket changes,
// the incremental rewrite's entry lines are byte-identical to those a full
// rewrite with no previous index writes.
func TestTCXV0849IncrementalRewriteMatchesFullEncode(t *testing.T) {
	r := writePathRepo(t)
	path := writePathTicket(r, "B")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, bytes.Replace(raw, []byte(`"title":"`), []byte(`"title":"changed `), 1), 0o644); err != nil {
		t.Fatal(err)
	}
	older := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(path, older, older); err != nil {
		t.Fatal(err)
	}
	if err := intent.WriteClosedIndex(r.Root, r.StateDir); err != nil {
		t.Fatal(err)
	}
	incremental := indexBody(t, r)
	if err := os.Remove(intent.ClosedIndexPath(r.StateDir)); err != nil {
		t.Fatal(err)
	}
	if err := intent.WriteClosedIndex(r.Root, r.StateDir); err != nil {
		t.Fatal(err)
	}
	if full := indexBody(t, r); !bytes.Equal(incremental, full) || !bytes.Contains(full, []byte("changed ")) {
		t.Fatalf("incremental body:\n%s\nfull body:\n%s", incremental, full)
	}
}

// TestTCXV0849UnchangedIndexNotRewritten: a rewrite with no changed entry
// leaves the file as it is.
func TestTCXV0849UnchangedIndexNotRewritten(t *testing.T) {
	r := writePathRepo(t)
	before, err := os.Stat(intent.ClosedIndexPath(r.StateDir))
	if err != nil {
		t.Fatal(err)
	}
	if err := intent.WriteClosedIndex(r.Root, r.StateDir); err != nil {
		t.Fatal(err)
	}
	after, err := os.Stat(intent.ClosedIndexPath(r.StateDir))
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(before, after) || !before.ModTime().Equal(after.ModTime()) {
		t.Fatal("an unchanged index was rewritten")
	}
}

// TestTCXV0849ParsedIndexReusedPerVersion: a second read of the same index
// version reuses the parse; a rewritten index is parsed again.
func TestTCXV0849ParsedIndexReusedPerVersion(t *testing.T) {
	r := writePathRepo(t)
	first := intent.ReadClosedIndex(r.StateDir)
	if first == nil || intent.ReadClosedIndex(r.StateDir) != first {
		t.Fatal("the same index version was parsed twice")
	}
	path := writePathTicket(r, "D")
	older := time.Now().Add(-3 * time.Hour)
	if err := os.Chtimes(path, older, older); err != nil {
		t.Fatal(err)
	}
	if err := intent.WriteClosedIndex(r.Root, r.StateDir); err != nil {
		t.Fatal(err)
	}
	second := intent.ReadClosedIndex(r.StateDir)
	if second == nil || second == first {
		t.Fatal("a rewritten index was not parsed again")
	}
	if got := intent.IndexedTicketPaths(second); !slices.Equal(got, writePathTickets) {
		t.Fatalf("reparsed index holds %v", got)
	}
}

// TestTCXV0849SameStatRewriteReparsed: an index rewritten in place at the
// same length with its modification time restored, as on a filesystem whose
// change time is coarse, is parsed again; an invalid replacement is refused,
// never served from the earlier parse.
func TestTCXV0849SameStatRewriteReparsed(t *testing.T) {
	r := writePathRepo(t)
	path := intent.ClosedIndexPath(r.StateDir)
	if intent.ReadClosedIndex(r.StateDir) == nil {
		t.Fatal("index does not read back")
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	forged := bytes.Clone(raw)
	forged[len(forged)-2] ^= 0x01 // one body byte; the checksum no longer holds
	f, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteAt(forged, 0); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}
	if after, err := os.Stat(path); err != nil || after.Size() != info.Size() || !after.ModTime().Equal(info.ModTime()) || !os.SameFile(info, after) {
		t.Fatalf("fixture: rewrite changed size, time or file: %v", err)
	}
	if idx := intent.ReadClosedIndex(r.StateDir); idx != nil {
		t.Fatal("an invalid same-stat replacement was served from the earlier parse")
	}
}
