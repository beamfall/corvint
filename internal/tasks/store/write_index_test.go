// SPDX-License-Identifier: Apache-2.0

package store_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// writeIndexRepo is a journaled store with COMPLETED A and D and OPEN B
// (depending on A), C and E.
func writeIndexRepo(t *testing.T) (*fixture.Repo, *intent.Repository) {
	t.Helper()
	r := fixture.TempRepo(t)
	fixture.WriteWritableState(t, r)
	reason := "done"
	var recs []*ticket.Record
	posts := map[string][]byte{}
	for _, local := range []string{"A", "B", "C", "D", "E"} {
		rec := fixture.Ticket(local)
		if local == "A" || local == "D" {
			rec.Status = ticket.StatusCompleted
			rec.Completion = &ticket.Completion{Kind: "MANUAL", Actor: fixture.Actor, Reason: &reason, Evidence: []wire.Digest{}, RecordedAt: fixture.Timestamp}
		}
		if local == "B" {
			rec.Dependencies = []ticket.Dependency{fixture.Dep("A")}
		}
		recs = append(recs, rec)
		posts["intent/tickets/"+local+".json"] = rec.Encode()
	}
	fixture.WriteIntent(t, r, recs...)
	fixture.CommitPosts(t, r, "MUTATION", "", posts)
	repo, err := intent.Resolve(r.Root)
	if err != nil {
		t.Fatal(err)
	}
	return r, repo
}

// indexAged ages every ticket file out of the racy window and rewrites the
// index, so it holds an entry for every ticket the writer will read.
func indexAged(t *testing.T, r *fixture.Repo) int {
	t.Helper()
	old := time.Now().Add(-time.Hour)
	names, err := os.ReadDir(filepath.Join(r.IntentDir, intent.TicketsDir))
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range names {
		if err := os.Chtimes(filepath.Join(r.IntentDir, intent.TicketsDir, n.Name()), old, old); err != nil {
			t.Fatal(err)
		}
	}
	if err := intent.WriteClosedIndex(r.Root, r.StateDir); err != nil {
		t.Fatal(err)
	}
	policy, err := os.ReadFile(filepath.Join(r.IntentDir, intent.PolicyFile))
	if err != nil {
		t.Fatal(err)
	}
	return len(intent.ReadClosedIndex(r.StateDir).WriterSummaries(fixture.QueueID, wire.Sum(policy)))
}

// files reads every file of the intent store and the state directory. The
// index is a sibling of the state directory, so it is not among them.
func files(t *testing.T, r *fixture.Repo) map[string][]byte {
	t.Helper()
	out := map[string][]byte{}
	for _, root := range []string{r.IntentDir, r.StateDir} {
		err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			raw, err := os.ReadFile(p)
			out[p] = raw
			return err
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	return out
}

// restore makes the intent store and the state directory exactly saved.
func restore(t *testing.T, r *fixture.Repo, saved map[string][]byte) {
	t.Helper()
	for p := range files(t, r) {
		if _, ok := saved[p]; !ok {
			if err := os.Remove(p); err != nil {
				t.Fatal(err)
			}
		}
	}
	for p, raw := range saved {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, raw, 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

type writeStep struct {
	name                        string
	operation, target, expected string
	payload                     wire.Value
}

// runIndexedAndFull commits each step twice from the same state, once with
// an index holding every ticket and once with none, at one fixed recordedAt,
// and returns each step's two reports and file sets.
func runIndexedAndFull(t *testing.T, steps []writeStep) (reports [][2]*store.Report, after [][2]map[string][]byte) {
	t.Helper()
	r, repo := writeIndexRepo(t)
	at, err := wire.ParseTimestamp("recordedAt", "2026-11-02T12:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range steps {
		if held := indexAged(t, r); held == 0 {
			t.Fatalf("%s: the index governs no ticket", s.name)
		}
		before := files(t, r)
		env := envelope(s.name, s.operation, s.target, s.expected, s.payload)
		indexed, err := store.Mutate(context.Background(), repo, operator(), env, at)
		if err != nil {
			t.Fatalf("%s indexed: %v", s.name, err)
		}
		withIndex := files(t, r)
		restore(t, r, before)
		if err := os.Remove(intent.ClosedIndexPath(r.StateDir)); err != nil {
			t.Fatal(err)
		}
		full, err := store.Mutate(context.Background(), repo, operator(), env, at)
		if err != nil {
			t.Fatalf("%s full: %v", s.name, err)
		}
		reports = append(reports, [2]*store.Report{indexed, full})
		after = append(after, [2]map[string][]byte{withIndex, files(t, r)})
	}
	return reports, after
}

func writeSteps() []writeStep {
	id := func(local string) string { return fixture.Ticket(local).TicketID.Raw }
	return []writeStep{
		{"create-first", mutation.OpCreate, "", "", createPayload("first")},
		{"prioritize-b", mutation.OpPrioritize, id("B"), "1", obj("priority", str("P1"), "order", str("3"))},
		{"complete-c", mutation.OpCompleteManual, id("C"), "1", obj("evidence", wire.Array(), "reason", str("done"))},
		{"archive-d", mutation.OpArchive, id("D"), "1", obj("reason", str("gone"))},
		{"create-second", mutation.OpCreate, "", "", createPayload("second")},
	}
}

// TestTCXV0846ModelOutputMatchesFullDecode: every step's writer report is the
// same with the index as with a full decode of every ticket.
func TestTCXV0846ModelOutputMatchesFullDecode(t *testing.T) {
	reports, _ := runIndexedAndFull(t, writeSteps())
	for i, pair := range reports {
		a, b := pair[0], pair[1]
		if a.Outcome.Outcome != mutation.OutcomeCompleted || !sameJSON(t, a.Outcome, b.Outcome) || !sameJSON(t, a.MutationEffects, b.MutationEffects) || a.Receipt != b.Receipt || a.Ticket != b.Ticket || a.Kind != b.Kind {
			t.Fatalf("step %d: indexed %+v, full %+v", i, a, b)
		}
	}
}

// TestTCXV0848JournalByteIdenticalWithIndex: after every step the journal,
// the state directory and the intent store are byte-identical with and
// without the index.
func TestTCXV0848JournalByteIdenticalWithIndex(t *testing.T) {
	_, after := runIndexedAndFull(t, writeSteps())
	for i, pair := range after {
		a, b := pair[0], pair[1]
		if len(a) != len(b) {
			t.Fatalf("step %d: %d files with the index, %d without", i, len(a), len(b))
		}
		for p, raw := range a {
			if !bytes.Equal(raw, b[p]) {
				t.Fatalf("step %d: %s differs with the index", i, p)
			}
		}
	}
}

func sameJSON(t *testing.T, a, b any) bool {
	t.Helper()
	ja, errA := json.Marshal(a)
	jb, errB := json.Marshal(b)
	if errA != nil || errB != nil {
		t.Fatalf("marshal: %v %v", errA, errB)
	}
	return bytes.Equal(ja, jb)
}
