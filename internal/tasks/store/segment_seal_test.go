package store

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/archive"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/journal"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// segmentHistory is a settled store 1,100 receipts long: one write past it
// seals receipts 1..512 (JSG-V0-008).
const segmentHistory = 1100

// stateTree returns every file under the state directory by relative path.
func stateTree(t *testing.T, repo *intent.Repository) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(repo.StateDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(repo.StateDir, p)
		if d.IsDir() {
			out[filepath.ToSlash(rel)+"/"] = ""
			return nil
		}
		raw, err := os.ReadFile(p)
		out[filepath.ToSlash(rel)] = string(raw)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func withoutHead(files []archive.FileEntry) []archive.FileEntry {
	out := []archive.FileEntry{}
	for _, f := range files {
		if f.Path != "head.json" {
			out = append(out, f)
		}
	}
	return out
}

func segmentAudit(repo *intent.Repository) (*journal.Result, error) {
	q, _ := wire.ParseQueueID("", fixture.QueueID)
	return (journal.Reader{Source: journal.Native{StateDir: repo.StateDir, PrimaryWorktree: repo.PrimaryWorktree}, QueueID: q, PrimaryWorktree: repo.PrimaryWorktree}).Audit()
}

func mustSegmentAudit(t *testing.T, repo *intent.Repository, last uint64) {
	t.Helper()
	res, err := segmentAudit(repo)
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if res.Mode != journal.ModeFull || res.Pending || res.LastSeq != wire.SizeOf(last) {
		t.Fatalf("audit mode %v pending %v last %v, want full settled %d", res.Mode, res.Pending, res.LastSeq, last)
	}
}

func segmentThrough(t *testing.T, repo *intent.Repository) uint64 {
	t.Helper()
	head, err := readHead(repo)
	if err != nil {
		t.Fatal(err)
	}
	if head.Segments == nil {
		return 0
	}
	return head.Segments.Through.Uint64()
}

func receiptLinked(t *testing.T, repo *intent.Repository, seq uint64) bool {
	t.Helper()
	on, err := present(segmentPath(repo, receiptPath(seq)))
	if err != nil {
		t.Fatal(err)
	}
	return on
}

// sealStops maps a store's state directory to the seal stage its test stops
// at, so parallel tests each stop only their own store's seal.
var sealStops sync.Map

func init() {
	segmentSealStage = func(repo *intent.Repository, s string) error {
		if pause, ok := sealPauses.Load(repo.StateDir); ok {
			pause.(func(string))(s)
		}
		if stage, ok := sealStops.Load(repo.StateDir); ok && stage == s {
			return errSegmentSealStop
		}
		return nil
	}
}

func withSealStop(t *testing.T, repo *intent.Repository, stage string) {
	t.Helper()
	sealStops.Store(repo.StateDir, stage)
	t.Cleanup(func() { clearSealStop(repo) })
}

func clearSealStop(repo *intent.Repository) { sealStops.Delete(repo.StateDir) }

// TestJSGV0005_SealUnsealRollback seals a settled store, checks the sealed
// journal audits, keeps its inventory and serves sealed files by logical
// path, then unseals it back to the byte-identical prior state directory.
func TestJSGV0005_SealUnsealRollback(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := historyStore(t, segmentHistory)
	before := stateTree(t, repo)
	filesBefore, dirsBefore, err := scan(repo)
	if err != nil {
		t.Fatal(err)
	}
	if err := sealSegmentsLocked(ctx, repo); err != nil {
		t.Fatal(err)
	}
	if got := segmentThrough(t, repo); got != snapshot.SegmentReceipts {
		t.Fatalf("sealed through %d, want %d", got, snapshot.SegmentReceipts)
	}
	if receiptLinked(t, repo, 1) || receiptLinked(t, repo, snapshot.SegmentReceipts) || !receiptLinked(t, repo, snapshot.SegmentReceipts+1) {
		t.Fatal("sealed receipts must be unlinked and the first active receipt kept")
	}
	sealedTree := stateTree(t, repo)
	var segFiles []string
	for p := range sealedTree {
		if strings.HasPrefix(p, snapshot.SegmentsDir+"/") && !strings.HasSuffix(p, "/") {
			segFiles = append(segFiles, p)
		}
	}
	if len(segFiles) != 2 {
		t.Fatalf("segments/ holds %v, want one segment and its manifest", segFiles)
	}
	mustSegmentAudit(t, repo, segmentHistory)
	filesAfter, dirsAfter, err := scan(repo)
	if err != nil {
		t.Fatal(err)
	}
	// The inventory is the same but for the head, which now binds the
	// manifest.
	if !reflect.DeepEqual(withoutHead(filesBefore), withoutHead(filesAfter)) || !reflect.DeepEqual(dirsBefore, dirsAfter) {
		t.Fatalf("a seal changed the store inventory: %d files, %d after; dirs %v, %v", len(filesBefore), len(filesAfter), dirsBefore, dirsAfter)
	}
	// Every sealed receipt and request afterimage reads by its logical path.
	for p, raw := range before {
		if !strings.HasPrefix(p, "receipts/") && !strings.HasPrefix(p, "requests/") || strings.HasSuffix(p, "/") {
			continue
		}
		if _, linked := sealedTree[p]; linked {
			continue
		}
		got, err := readLogical(repo, p, len(raw))
		if err != nil || string(got) != raw {
			t.Fatalf("logical read %s: %v", p, err)
		}
	}
	if _, err := UnsealJournal(ctx, repo); err != nil {
		t.Fatal(err)
	}
	if after := stateTree(t, repo); !reflect.DeepEqual(before, after) {
		for p := range before {
			if before[p] != after[p] {
				t.Errorf("unseal differs at %s", p)
			}
		}
		for p := range after {
			if _, ok := before[p]; !ok {
				t.Errorf("unseal left %s", p)
			}
		}
		t.FailNow()
	}
	mustSegmentAudit(t, repo, segmentHistory)
	// A write leaves the unsealed store unsegmented; sealing it again
	// converts it, and the writer keeps working on the segmented store.
	historyMutate(t, repo, "jsg-after-unseal")
	if got := segmentThrough(t, repo); got != 0 {
		t.Fatalf("a write converted an unsegmented store through %d", got)
	}
	if err := sealSegmentsLocked(ctx, repo); err != nil {
		t.Fatal(err)
	}
	historyMutate(t, repo, "jsg-on-segmented")
	if got := segmentThrough(t, repo); got != snapshot.SegmentReceipts {
		t.Fatalf("resealed through %d, want %d", got, snapshot.SegmentReceipts)
	}
	mustSegmentAudit(t, repo, segmentHistory+2)
}

// TestJSGV0008_WriterSealsOnlySegmentedStores shows that a write whose
// commit leaves 1,024 receipts after S seals the next segment on a
// segmented store, and that the same write leaves an unsegmented store as
// it is, so no write converts a store a prior binary reads.
func TestJSGV0008_WriterSealsOnlySegmentedStores(t *testing.T) {
	t.Parallel()
	const history = 3*snapshot.SegmentReceipts - 1
	plain := historyStore(t, history)
	historyMutate(t, plain, "jsg-unsegmented")
	if got := segmentThrough(t, plain); got != 0 {
		t.Fatalf("a write converted an unsegmented store through %d", got)
	}
	repo := historyStore(t, history)
	if err := sealSegmentsLocked(context.Background(), repo); err != nil {
		t.Fatal(err)
	}
	if got := segmentThrough(t, repo); got != snapshot.SegmentReceipts {
		t.Fatalf("sealed through %d, want %d", got, snapshot.SegmentReceipts)
	}
	historyMutate(t, repo, "jsg-next-segment")
	if got := segmentThrough(t, repo); got != 2*snapshot.SegmentReceipts {
		t.Fatalf("write sealed through %d, want %d", got, 2*snapshot.SegmentReceipts)
	}
	if receiptLinked(t, repo, 2*snapshot.SegmentReceipts) || !receiptLinked(t, repo, 2*snapshot.SegmentReceipts+1) {
		t.Fatal("the writer's seal left the wrong receipts linked")
	}
	mustSegmentAudit(t, repo, history+1)
}

// TestJSGV0008_SealStopsSettle stops a seal after each durable step. A
// lock-free audit reports the pending seal, and the next writer settles it:
// back to the prior head before the head is published, forward after.
func TestJSGV0008_SealStopsSettle(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		stage   string
		through uint64
	}{{"marker", 0}, {"segment", 0}, {"head", snapshot.SegmentReceipts}, {"dropped", snapshot.SegmentReceipts}} {
		t.Run(tc.stage, func(t *testing.T) {
			t.Parallel()
			repo := historyStore(t, segmentHistory)
			withSealStop(t, repo, tc.stage)
			if err := sealSegmentsLocked(context.Background(), repo); !errors.Is(err, errSegmentSealStop) {
				t.Fatalf("seal: %v", err)
			}
			if _, err := segmentAudit(repo); wire.CodeOf(err) != wire.CodeRedoPending {
				t.Fatalf("audit with a pending seal: %v, want REDO_PENDING", err)
			}
			clearSealStop(repo)
			if err := settleSegmentSeal(repo); err != nil {
				t.Fatal(err)
			}
			if got := segmentThrough(t, repo); got != tc.through {
				t.Fatalf("settled through %d, want %d", got, tc.through)
			}
			if receiptLinked(t, repo, 1) != (tc.through == 0) {
				t.Fatal("settle left receipt 1 in the wrong state")
			}
			tree := stateTree(t, repo)
			for p := range tree {
				if strings.HasPrefix(p, snapshot.SegmentsDir+"/") && !strings.HasSuffix(p, "/") && tc.through == 0 {
					t.Fatalf("a rolled-back seal left %s", p)
				}
			}
			if _, ok := tree[snapshot.SealMarkerPath]; ok {
				t.Fatal("settle left the marker")
			}
			mustSegmentAudit(t, repo, segmentHistory)
			historyMutate(t, repo, "jsg-after-stop")
			if got := segmentThrough(t, repo); got != tc.through {
				t.Fatalf("next write left the store sealed through %d, want %d", got, tc.through)
			}
			mustSegmentAudit(t, repo, segmentHistory+1)
		})
	}
}

// TestJSGV0008_SealSettleRefusesForeignState covers the settle refusals: a
// head that is neither the seal's prior nor next head, and a sealed file
// still linked with bytes that differ from the sealed ones. Nothing is
// removed in either case.
func TestJSGV0008_SealSettleRefusesForeignState(t *testing.T) {
	t.Parallel()
	t.Run("foreign head", func(t *testing.T) {
		t.Parallel()
		repo := historyStore(t, segmentHistory)
		withSealStop(t, repo, "segment")
		if err := sealSegmentsLocked(context.Background(), repo); !errors.Is(err, errSegmentSealStop) {
			t.Fatalf("seal: %v", err)
		}
		head, err := readHead(repo)
		if err != nil {
			t.Fatal(err)
		}
		head.Generation = wire.SizeOf(head.Generation.Uint64() + 1)
		historyWrite(t, statePath(repo, "head.json"), wire.EncodeFile(head.Value()))
		before := stateTree(t, repo)
		if err := settleSegmentSeal(repo); wire.CodeOf(err) != wire.CodeJournalForked {
			t.Fatalf("settle under a foreign head: %v, want JOURNAL_FORKED", err)
		}
		if !reflect.DeepEqual(before, stateTree(t, repo)) {
			t.Fatal("a refused settle changed the state directory")
		}
	})
	t.Run("linked file differs", func(t *testing.T) {
		t.Parallel()
		repo := historyStore(t, segmentHistory)
		withSealStop(t, repo, "head")
		if err := sealSegmentsLocked(context.Background(), repo); !errors.Is(err, errSegmentSealStop) {
			t.Fatalf("seal: %v", err)
		}
		path := segmentPath(repo, receiptPath(7))
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		raw[len(raw)-2] ^= 1
		historyWrite(t, path, raw)
		before := stateTree(t, repo)
		if err := settleSegmentSeal(repo); wire.CodeOf(err) != wire.CodeJournalForked {
			t.Fatalf("settle over a changed linked file: %v, want JOURNAL_FORKED", err)
		}
		if !reflect.DeepEqual(before, stateTree(t, repo)) {
			t.Fatal("a refused settle changed the state directory")
		}
	})
}

// TestJSGV0008_SealDeclinesUnprovenInput checks that a seal writes nothing
// when a receipt to seal does not chain or a request afterimage differs from
// the digest its receipt posted.
func TestJSGV0008_SealDeclinesUnprovenInput(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		code string
		edit func(t *testing.T, repo *intent.Repository)
	}{
		{"receipt chain", wire.CodeJournalForked, func(t *testing.T, repo *intent.Repository) {
			historyWrite(t, segmentPath(repo, receiptPath(9)), mustRead(t, segmentPath(repo, receiptPath(10))))
		}},
		{"request afterimage", wire.CodeUnsupported, func(t *testing.T, repo *intent.Repository) {
			path := segmentPath(repo, firstSealedRequest(t, repo))
			historyWrite(t, path, append(mustRead(t, path), ' '))
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			repo := historyStore(t, segmentHistory)
			tc.edit(t, repo)
			before := stateTree(t, repo)
			if err := sealSegmentsLocked(context.Background(), repo); wire.CodeOf(err) != tc.code {
				t.Fatalf("seal: %v, want %s", err, tc.code)
			}
			if !reflect.DeepEqual(before, stateTree(t, repo)) {
				t.Fatal("a declined seal changed the state directory")
			}
		})
	}
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

// firstSealedRequest returns a request afterimage posted by a receipt the
// first segment seals.
func firstSealedRequest(t *testing.T, repo *intent.Repository) string {
	t.Helper()
	for seq := uint64(1); seq <= snapshot.SegmentReceipts; seq++ {
		rc, _, _, err := snapshot.DecodeRetainedReceipt(mustRead(t, segmentPath(repo, receiptPath(seq))))
		if err != nil {
			t.Fatal(err)
		}
		for _, p := range rc.Post {
			if strings.HasPrefix(p.Path, "requests/") {
				return p.Path
			}
		}
	}
	t.Fatal("no sealed request")
	return ""
}

// TestJSGV0006_SegmentTamperAndMovedReads checks that a changed segment or
// manifest refuses the audit and every writer, and that a logical read whose
// head names a manifest a later seal replaced is a moved observation.
func TestJSGV0006_SegmentTamperAndMovedReads(t *testing.T) {
	t.Parallel()
	seal := func(t *testing.T) *intent.Repository {
		repo := historyStore(t, segmentHistory)
		if err := sealSegmentsLocked(context.Background(), repo); err != nil {
			t.Fatal(err)
		}
		return repo
	}
	for _, name := range []string{snapshot.SegmentName(1, snapshot.SegmentReceipts), snapshot.SegmentManifestName(snapshot.SegmentReceipts)} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			repo := seal(t)
			path := segmentPath(repo, name)
			raw := mustRead(t, path)
			raw[len(raw)/2] ^= 1
			historyWrite(t, path, raw)
			if _, err := segmentAudit(repo); wire.CodeOf(err) != wire.CodeJournalForked {
				t.Fatalf("audit of a changed %s: %v, want JOURNAL_FORKED", name, err)
			}
			if _, err := Mutate(context.Background(), repo, historyActor, historyCreate("jsg-tampered", "tampered"), WallClock()); err == nil {
				t.Fatal("a writer accepted a changed segment")
			}
		})
	}
	t.Run("moved manifest", func(t *testing.T) {
		t.Parallel()
		repo := seal(t)
		name := snapshot.SegmentManifestName(snapshot.SegmentReceipts)
		if err := os.Rename(segmentPath(repo, name), segmentPath(repo, snapshot.SegmentManifestName(snapshot.SegmentReceipts*2))); err != nil {
			t.Fatal(err)
		}
		if _, err := readLogical(repo, receiptPath(1), wire.MaxReceiptFileBytes); wire.CodeOf(err) != wire.CodeSnapshotMoved {
			t.Fatalf("logical read under a replaced manifest: %v, want SNAPSHOT_MOVED", err)
		}
		if _, _, err := scan(repo); wire.CodeOf(err) != wire.CodeSnapshotMoved {
			t.Fatalf("inventory under a replaced manifest: %v, want SNAPSHOT_MOVED", err)
		}
	})
}

// TestJSGV0007_SegmentLayoutRefusals covers the layout checks a walk applies
// to segments/: a sealed receipt still linked, a file the manifest does not
// bind, and segments beside an unsegmented head are each a fork that both a
// reader and a writer report.
func TestJSGV0007_SegmentLayoutRefusals(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		seal   bool
		damage func(t *testing.T, repo *intent.Repository)
	}{
		{"sealed receipt still linked", true, func(t *testing.T, repo *intent.Repository) {
			raw, err := readLogical(repo, receiptPath(7), wire.MaxReceiptFileBytes)
			if err != nil {
				t.Fatal(err)
			}
			historyWrite(t, filepath.Join(repo.StateDir, receiptPath(7)), raw)
		}},
		{"unexpected name in segments", true, func(t *testing.T, repo *intent.Repository) {
			historyWrite(t, segmentPath(repo, snapshot.SegmentsDir+"/extra"), []byte("x"))
		}},
		{"unbound segment", true, func(t *testing.T, repo *intent.Repository) {
			historyWrite(t, segmentPath(repo, snapshot.SegmentName(snapshot.SegmentReceipts+1, 2*snapshot.SegmentReceipts)), []byte("x"))
		}},
		{"segments beside an unsegmented head", false, func(t *testing.T, repo *intent.Repository) {
			historyWrite(t, segmentPath(repo, snapshot.SegmentName(1, snapshot.SegmentReceipts)), []byte("x"))
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			repo := historyStore(t, segmentHistory)
			if tc.seal {
				if err := sealSegmentsLocked(context.Background(), repo); err != nil {
					t.Fatal(err)
				}
			}
			tc.damage(t, repo)
			before := stateTree(t, repo)
			if _, err := segmentAudit(repo); wire.CodeOf(err) != wire.CodeJournalForked {
				t.Fatalf("audit: %v, want JOURNAL_FORKED", err)
			}
			if _, err := Mutate(context.Background(), repo, historyActor, historyCreate("jsg-layout", "layout"), WallClock()); wire.CodeOf(err) != wire.CodeJournalForked {
				t.Fatalf("writer: %v, want JOURNAL_FORKED", err)
			}
			if !reflect.DeepEqual(stateTree(t, repo), before) {
				t.Fatal("a refused walk changed the state")
			}
		})
	}
}
