// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// tcxRecords is a store with closed dependency targets: A COMPLETED, C
// ARCHIVED from COMPLETED, E COMPLETED, and OPEN B, D and F that depend on
// or sit beside them.
func tcxRecords() []*ticket.Record {
	reason := "done"
	closed := func(local string) *ticket.Record {
		rec := fixture.Ticket(local)
		rec.Status = ticket.StatusCompleted
		rec.Completion = &ticket.Completion{Kind: "MANUAL", Actor: fixture.Actor, Reason: &reason, Evidence: []wire.Digest{}, RecordedAt: fixture.Timestamp}
		return rec
	}
	a, c, e := closed("A"), closed("C"), closed("E")
	from := ticket.StatusCompleted
	c.Status, c.ArchivedFrom = ticket.StatusArchived, &from
	e.Labels = []string{"x"}
	a.RequiredGates = []string{"verify"}
	c.Title = `archived "quoted" ünïcode \ title`
	b, d, f := fixture.Ticket("B"), fixture.Ticket("D"), fixture.Ticket("F")
	b.Dependencies = []ticket.Dependency{fixture.Dep("A")}
	d.Dependencies = []ticket.Dependency{fixture.Dep("C")}
	f.Title = "archive speed"
	return []*ticket.Record{a, b, c, d, e, f}
}

// tcxRepo writes tcxRecords, journaled or not, ages every ticket file out of
// the racy window when aged is set, and writes the closed-ticket index.
func tcxRepo(t *testing.T, journal, aged bool) *fixture.Repo {
	t.Helper()
	r := fixture.TempRepo(t)
	recs := tcxRecords()
	if journal {
		fixture.WriteState(t, r)
	}
	fixture.WriteIntent(t, r, recs...)
	if journal {
		posts := map[string][]byte{}
		for _, rec := range recs {
			posts["intent/tickets/"+rec.TicketID.Local+".json"] = rec.Encode()
		}
		fixture.CommitPosts(t, r, "MUTATION", "", posts)
	}
	if aged {
		old := time.Now().Add(-time.Hour)
		for _, rec := range recs {
			if err := os.Chtimes(tcxTicketPath(r, rec.TicketID.Local), old, old); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := intent.WriteClosedIndex(r.Root, r.StateDir); err != nil {
		t.Fatalf("write index: %v", err)
	}
	return r
}

func tcxTicketPath(r *fixture.Repo, local string) string {
	return filepath.Join(r.IntentDir, intent.TicketsDir, local+".json")
}

// tcxRewrite replaces old with new in a ticket file and keeps its
// modification time, so only a read of the bytes can tell.
func tcxRewrite(t *testing.T, r *fixture.Repo, local, old, new string) {
	t.Helper()
	path := tcxTicketPath(r, local)
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil || !bytes.Contains(raw, []byte(old)) {
		t.Fatalf("read %s: %v", path, err)
	}
	fixture.Write(t, path, bytes.Replace(raw, []byte(old), []byte(new), 1))
	if err := os.Chtimes(path, info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}
}

func tcxRun(env Env) []byte {
	var out bytes.Buffer
	env.Stdout, env.Stderr = &out, &out
	Run(env)
	return out.Bytes()
}

// tcxClock masks queue status's wall-clock observation time.
func tcxClock(out []byte) []byte {
	return tcxObservedAt.ReplaceAll(out, []byte(`"observedAt":""`))
}

var tcxObservedAt = regexp.MustCompile(`"observedAt":"[^"]*"`)

func tcxReads(journal bool) [][]string {
	reads := [][]string{
		{"ticket", "search", "--status", "OPEN"},
		{"ticket", "search", "--status", "OPEN", "--facets"},
		{"ticket", "search", "--status", "OPEN", "--count"},
		{"ticket", "search", "--status", "COMPLETED", "--count"},
		{"ticket", "search", "--label", "x"},
		{"ticket", "search", "--text", "archive"},
		{"roadmap"},
		{"queue", "status"},
		{"ticket", "show", "B"},
		{"ticket", "show", "D"},
		{"ticket", "show", "A"},
		{"ticket", "show", "C"},
		{"ticket", "blockers", "B"},
	}
	if journal {
		reads = append(reads, []string{"receipt", "audit"})
	}
	return reads
}

func tcxWithoutIndex(t *testing.T) {
	t.Helper()
	useClosedIndex = false
	t.Cleanup(func() { useClosedIndex = true })
}

// TestTCXV0_ClosedIndexRoundTrip: a written index decodes, vouches for the
// aged closed files, and leaves the intent tree digest unchanged.
func TestTCXV0_ClosedIndexRoundTrip(t *testing.T) {
	r := tcxRepo(t, false, true)
	idx := intent.ReadClosedIndex(r.StateDir)
	if idx == nil {
		t.Fatal("index does not read back")
	}
	full, err := intent.TreeDigest(r.Root)
	if err != nil {
		t.Fatal(err)
	}
	indexed, err := intent.TreeDigestIndexed(r.Root, idx)
	if err != nil || indexed.Sha256 != full.Sha256 {
		t.Fatalf("indexed digest %v %v; want %s", indexed.Sha256, err, full.Sha256)
	}
	// Rewriting reuses the trusted entries and yields the same body.
	first, _ := os.ReadFile(intent.ClosedIndexPath(r.StateDir))
	if err := intent.WriteClosedIndex(r.Root, r.StateDir); err != nil {
		t.Fatal(err)
	}
	second, _ := os.ReadFile(intent.ClosedIndexPath(r.StateDir))
	if len(first) == 0 || bytes.Count(first, []byte("\n")) != bytes.Count(second, []byte("\n")) {
		t.Fatalf("rewrite changed the entry set:\n%s\n%s", first, second)
	}
}

// TestTCXV0_LazyInventoryMatchesFull: the indexed store's inventory orders,
// diagnoses and materializes every ticket exactly as the full load does.
func TestTCXV0_LazyInventoryMatchesFull(t *testing.T) {
	r := tcxRepo(t, false, true)
	idx := intent.ReadClosedIndex(r.StateDir)
	tree, err := intent.TreeDigest(r.Root)
	if err != nil {
		t.Fatal(err)
	}
	full, err := intent.LoadTree(r.Root, tree, "")
	if err != nil {
		t.Fatal(err)
	}
	itree, err := intent.TreeDigestIndexed(r.Root, idx)
	if err != nil {
		t.Fatal(err)
	}
	lazy, err := intent.LoadTreeIndexed(r.Root, itree, "", idx)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(full.Inventory.Sorted(), ",") != strings.Join(lazy.Inventory.Sorted(), ",") {
		t.Fatalf("order %v; want %v", lazy.Inventory.Sorted(), full.Inventory.Sorted())
	}
	for _, id := range full.Inventory.IDs() {
		if len(full.Inventory.Problems(id)) != len(lazy.Inventory.Problems(id)) ||
			len(full.Inventory.UnmetCompletedDependencies(id)) != len(lazy.Inventory.UnmetCompletedDependencies(id)) {
			t.Fatalf("%s diagnoses differ", id)
		}
		want, _ := full.Inventory.Get(id)
		got, ok := lazy.Inventory.Get(id)
		if !ok || !bytes.Equal(want.Encode(), got.Encode()) {
			t.Fatalf("%s materializes differently", id)
		}
	}
}

// TestTCXV0_IndexedReadParity: every opted-in read, and receipt audit, is
// byte-identical with and without the index, on journaled and journal-absent
// stores; reads never write the index and a non-read command rewrites it.
func TestTCXV0_IndexedReadParity(t *testing.T) {
	for _, journal := range []bool{false, true} {
		r := tcxRepo(t, journal, true)
		path := intent.ClosedIndexPath(r.StateDir)
		before, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		reads := tcxReads(journal)
		indexed := make([][]byte, len(reads))
		for i, args := range reads {
			indexed[i] = tcxRun(Env{Cwd: r.Root, Args: args})
		}
		if after, _ := os.ReadFile(path); !bytes.Equal(before, after) {
			t.Fatalf("journal=%v: a read rewrote the index", journal)
		}
		useClosedIndex = false
		for i, args := range reads {
			if got := tcxRun(Env{Cwd: r.Root, Args: args}); !bytes.Equal(tcxClock(indexed[i]), tcxClock(got)) {
				useClosedIndex = true
				t.Fatalf("journal=%v %v:\nindexed %s\nfull    %s", journal, args, indexed[i], got)
			}
		}
		useClosedIndex = true
		if !journal {
			continue
		}
		for i, args := range reads {
			if res, err := wire.DecodeResult(indexed[i]); err != nil || res.Outcome != wire.OutcomeOK {
				t.Fatalf("%v: %s", args, indexed[i])
			}
		}
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
		// A refused write leaves no index; a committed one rewrites it.
		tcxRun(Env{Cwd: r.Root, Args: []string{"ticket", "archive", "--target", fixture.TicketID("B")}})
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("a refused command wrote the index: %v", err)
		}
		refresh := closedIndexRefresher(Env{Cwd: r.Root})
		head := filepath.Join(r.StateDir, "head.json")
		later := time.Now().Add(time.Minute)
		if err := os.Chtimes(head, later, later); err != nil {
			t.Fatal(err)
		}
		refresh()
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("a committed mutation did not rewrite the index: %v", err)
		}
	}
}

// TestTCXV0_ChangedFileNotTrusted: an indexed closed file rewritten with a
// new size or modification time is read, so the read shows its new bytes.
func TestTCXV0_ChangedFileNotTrusted(t *testing.T) {
	r := tcxRepo(t, false, true)
	path := tcxTicketPath(r, "A")
	raw, _ := os.ReadFile(path)
	fixture.Write(t, path, bytes.Replace(raw, []byte("Fixture ticket A"), []byte("Fixture ticket A renamed"), 1))
	got := tcxRun(Env{Cwd: r.Root, Args: []string{"ticket", "show", "A"}})
	if !bytes.Contains(got, []byte("Fixture ticket A renamed")) {
		t.Fatalf("show A after rewrite: %s", got)
	}
	tcxWithoutIndex(t)
	if full := tcxRun(Env{Cwd: r.Root, Args: []string{"ticket", "show", "A"}}); !bytes.Equal(full, got) {
		t.Fatalf("indexed %s\nfull %s", got, full)
	}
}

// TestTCXV0_CorruptIndexFullScan: an index that fails its checksum is
// ignored and reads are the full scan's.
func TestTCXV0_CorruptIndexFullScan(t *testing.T) {
	r := tcxRepo(t, false, true)
	path := intent.ClosedIndexPath(r.StateDir)
	raw, _ := os.ReadFile(path)
	raw[len(raw)-2] ^= 1
	fixture.Write(t, path, raw)
	if intent.ReadClosedIndex(r.StateDir) != nil {
		t.Fatal("corrupt index decoded")
	}
	fixture.Write(t, path, []byte("not an index\n"))
	if intent.ReadClosedIndex(r.StateDir) != nil {
		t.Fatal("foreign file decoded")
	}
	// A same-size rewrite now shows, because nothing vouches for the old bytes.
	tcxRewrite(t, r, "A", "Fixture ticket A", "Fixture ticket Z")
	if got := tcxRun(Env{Cwd: r.Root, Args: []string{"ticket", "show", "A"}}); !bytes.Contains(got, []byte("Fixture ticket Z")) {
		t.Fatalf("show A: %s", got)
	}
}

// TestTCXV0_MovedClosedRecordFails: a closed file the index vouches for but
// whose bytes changed is caught when decoded on demand: SNAPSHOT_MOVED, never
// the stale summary's record. Reads that do not decode it are unaffected.
func TestTCXV0_MovedClosedRecordFails(t *testing.T) {
	r := tcxRepo(t, false, true)
	tcxRewrite(t, r, "A", "Fixture ticket A", "Fixture ticket Z")
	res, err := wire.DecodeResult(tcxRun(Env{Cwd: r.Root, Args: []string{"ticket", "show", "A"}}))
	if err != nil || res.Outcome == wire.OutcomeOK || !slices.Contains(res.Codes, wire.CodeSnapshotMoved) {
		t.Fatalf("show A: %+v %v", res, err)
	}
	if res, err := wire.DecodeResult(tcxRun(Env{Cwd: r.Root, Args: []string{"ticket", "show", "B"}})); err != nil || res.Outcome != wire.OutcomeOK {
		t.Fatalf("show B: %+v %v", res, err)
	}
}

// TestTCXV0_PolicyMismatchNotTrusted: after the policy changes no entry is
// trusted, so a same-size rewrite of an indexed file is read, not refused.
func TestTCXV0_PolicyMismatchNotTrusted(t *testing.T) {
	r := tcxRepo(t, false, true)
	policy := fixture.PolicyValue()
	budgets, _ := policy.Obj.Get("budgets")
	budgets.Obj.Set("requireEnforcedFields", wire.Strings(nil))
	fixture.Write(t, filepath.Join(r.IntentDir, intent.PolicyFile), wire.EncodeFile(policy))
	tcxRewrite(t, r, "A", "Fixture ticket A", "Fixture ticket Z")
	if got := tcxRun(Env{Cwd: r.Root, Args: []string{"ticket", "show", "A"}}); !bytes.Contains(got, []byte("Fixture ticket Z")) {
		t.Fatalf("show A: %s", got)
	}
}

// TestTCXV0_RacyEntryLeftOut: files modified within the racy window of the
// capture are not indexed, so a same-size same-time rewrite is still read.
func TestTCXV0_RacyEntryLeftOut(t *testing.T) {
	r := tcxRepo(t, false, false)
	tcxRewrite(t, r, "A", "Fixture ticket A", "Fixture ticket Z")
	if got := tcxRun(Env{Cwd: r.Root, Args: []string{"ticket", "show", "A"}}); !bytes.Contains(got, []byte("Fixture ticket Z")) {
		t.Fatalf("show A: %s", got)
	}
}
