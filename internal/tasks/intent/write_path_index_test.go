// SPDX-License-Identifier: Apache-2.0

package intent_test

import (
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
