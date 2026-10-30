package store

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/archive"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// migrateHistory is a settled store long enough for `journal seal` to seal
// two segments (1..1,024) and keep 676 receipts active.
const migrateHistory = 1700

// historyExport returns repo's verified archive stream.
func historyExport(t *testing.T, repo *intent.Repository) []byte {
	t.Helper()
	var out bytes.Buffer
	if _, err := archive.Export(archive.ExportOptions{Repo: repo, Staging: t.TempDir(), Stdout: &out}); err != nil {
		t.Fatalf("archive export: %v", err)
	}
	return out.Bytes()
}

func mustSameTree(t *testing.T, repo *intent.Repository, want map[string]string, label string) {
	t.Helper()
	got := stateTree(t, repo)
	if reflect.DeepEqual(want, got) {
		return
	}
	for p := range want {
		if want[p] != got[p] {
			t.Errorf("%s: differs at %s", label, p)
		}
	}
	for p := range got {
		if _, ok := want[p]; !ok {
			t.Errorf("%s: left %s", label, p)
		}
	}
	t.FailNow()
}

// TestJSGV0013_JournalSealAndUnseal runs the operator verbs end to end: an
// unseal of an unsegmented store writes nothing, seal converts the store and
// seals two segments, a second seal finds nothing to do, the archive of the
// sealed store is byte-identical to the archive of the unsegmented one
// (JSG-V0-012), and unseal restores the byte-identical prior tree.
func TestJSGV0013_JournalSealAndUnseal(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := historyStore(t, migrateHistory)
	before := stateTree(t, repo)
	r, err := UnsealJournal(ctx, repo)
	if err != nil || *r != (JournalSealReport{HeadSeq: migrateHistory}) {
		t.Fatalf("unseal of an unsegmented store: %+v %v", r, err)
	}
	mustSameTree(t, repo, before, "unseal of an unsegmented store")
	plain := historyExport(t, repo)
	mustSameTree(t, repo, before, "archive export")

	r, err = SealJournal(ctx, repo)
	if err != nil || *r != (JournalSealReport{HeadSeq: migrateHistory, ThroughAfter: 2 * snapshot.SegmentReceipts, SegmentsAfter: 2}) {
		t.Fatalf("seal: %+v %v", r, err)
	}
	mustSegmentAudit(t, repo, migrateHistory)
	sealedTree := stateTree(t, repo)
	r, err = SealJournal(ctx, repo)
	if err != nil || *r != (JournalSealReport{HeadSeq: migrateHistory, ThroughBefore: 2 * snapshot.SegmentReceipts, ThroughAfter: 2 * snapshot.SegmentReceipts, SegmentsAfter: 2}) {
		t.Fatalf("second seal: %+v %v", r, err)
	}
	mustSameTree(t, repo, sealedTree, "second seal")
	if sealed := historyExport(t, repo); !bytes.Equal(sealed, plain) {
		t.Fatalf("the archive of the sealed store (%d bytes) differs from the unsegmented archive (%d bytes)", len(sealed), len(plain))
	}

	r, err = UnsealJournal(ctx, repo)
	if err != nil || *r != (JournalSealReport{HeadSeq: migrateHistory, ThroughBefore: 2 * snapshot.SegmentReceipts}) {
		t.Fatalf("unseal: %+v %v", r, err)
	}
	mustSameTree(t, repo, before, "unseal")
	mustSegmentAudit(t, repo, migrateHistory)
}

// TestJSGV0013_MigrationRefusesAFault checks that seal and unseal refuse a
// store the complete audit does not prove and write nothing.
func TestJSGV0013_MigrationRefusesAFault(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	for _, tc := range []struct {
		name   string
		sealed bool
		damage func(t *testing.T, repo *intent.Repository)
		verb   func(context.Context, *intent.Repository) (*JournalSealReport, error)
	}{
		{"seal over a changed receipt", false, func(t *testing.T, repo *intent.Repository) {
			path := segmentPath(repo, receiptPath(9))
			raw := mustRead(t, path)
			raw[len(raw)-2] ^= 1
			historyWrite(t, path, raw)
		}, SealJournal},
		{"unseal over a changed segment", true, func(t *testing.T, repo *intent.Repository) {
			path := segmentPath(repo, snapshot.SegmentName(1, snapshot.SegmentReceipts))
			raw := mustRead(t, path)
			raw[len(raw)/2] ^= 1
			historyWrite(t, path, raw)
		}, UnsealJournal},
		{"unseal over a stray request afterimage", true, func(t *testing.T, repo *intent.Repository) {
			p, err := readLogical(repo, receiptPath(1), wire.MaxReceiptFileBytes)
			if err != nil || len(p) == 0 {
				t.Fatal(err)
			}
			req := firstSealedLogicalRequest(t, repo)
			historyWrite(t, segmentPath(repo, req), []byte("{}\n"))
		}, UnsealJournal},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			repo := historyStore(t, segmentHistory)
			if tc.sealed {
				if _, err := SealJournal(ctx, repo); err != nil {
					t.Fatal(err)
				}
			}
			tc.damage(t, repo)
			before := stateTree(t, repo)
			if _, err := tc.verb(ctx, repo); err == nil {
				t.Fatal("a migration accepted a damaged store")
			}
			mustSameTree(t, repo, before, "refused migration")
		})
	}
}

// firstSealedLogicalRequest returns a request afterimage the first segment
// seals, read through the logical view.
func firstSealedLogicalRequest(t *testing.T, repo *intent.Repository) string {
	t.Helper()
	for seq := uint64(1); seq <= snapshot.SegmentReceipts; seq++ {
		raw, err := readLogical(repo, receiptPath(seq), wire.MaxReceiptFileBytes)
		if err != nil {
			t.Fatal(err)
		}
		rc, _, _, err := snapshot.DecodeRetainedReceipt(raw)
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

// TestJSGV0013_UnsealStopsSettle stops an unseal of the second of two
// segments after each stage: before its head publish the next writer rolls
// it back to the sealed store, after it the writer rolls it forward, and a
// rerun of unseal then restores the byte-identical unsegmented tree.
func TestJSGV0013_UnsealStopsSettle(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	for _, tc := range []struct {
		stage   string
		through uint64
	}{{"marker", 2 * snapshot.SegmentReceipts}, {"segment", 2 * snapshot.SegmentReceipts}, {"head", snapshot.SegmentReceipts}, {"dropped", snapshot.SegmentReceipts}} {
		t.Run(tc.stage, func(t *testing.T) {
			t.Parallel()
			repo := historyStore(t, migrateHistory)
			before := stateTree(t, repo)
			if _, err := SealJournal(ctx, repo); err != nil {
				t.Fatal(err)
			}
			withSealStop(t, repo, tc.stage)
			if _, err := UnsealJournal(ctx, repo); !errors.Is(err, errSegmentSealStop) {
				t.Fatalf("unseal: %v", err)
			}
			if _, err := segmentAudit(repo); wire.CodeOf(err) != wire.CodeRedoPending {
				t.Fatalf("audit with a pending unseal: %v, want REDO_PENDING", err)
			}
			clearSealStop(repo)
			if err := settleSegmentSeal(repo); err != nil {
				t.Fatal(err)
			}
			if got := segmentThrough(t, repo); got != tc.through {
				t.Fatalf("settled through %d, want %d", got, tc.through)
			}
			if receiptLinked(t, repo, tc.through) || !receiptLinked(t, repo, tc.through+1) {
				t.Fatal("settle left the restored receipts in the wrong state")
			}
			if _, err := os.Lstat(segmentPath(repo, snapshot.SealMarkerPath)); !os.IsNotExist(err) {
				t.Fatalf("settle left the marker: %v", err)
			}
			mustSegmentAudit(t, repo, migrateHistory)
			if _, err := UnsealJournal(ctx, repo); err != nil {
				t.Fatal(err)
			}
			mustSameTree(t, repo, before, "unseal after a settled stop")
		})
	}
}

// TestJSGV0013_SegmentedStoreRefusesRetireAndUnretire checks that retire and
// unretire refuse a segmented store with nothing written: unseal first.
func TestJSGV0013_SegmentedStoreRefusesRetireAndUnretire(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := historyStore(t, segmentHistory)
	if _, err := SealJournal(ctx, repo); err != nil {
		t.Fatal(err)
	}
	before := stateTree(t, repo)
	if _, err := RetirePrefix(ctx, repo, "", true); wire.CodeOf(err) != wire.CodeUnsupported {
		t.Fatalf("retire: %v, want UNSUPPORTED", err)
	}
	if _, err := UnretirePrefix(ctx, repo, filepath.Join(t.TempDir(), "absent.tar")); wire.CodeOf(err) != wire.CodeUnsupported || !strings.Contains(err.Error(), "unseal") {
		t.Fatalf("unretire: %v, want UNSUPPORTED naming unseal", err)
	}
	mustSameTree(t, repo, before, "refused retire")
}

// TestJSGV0012_CompactionOnASealedStore compacts a store whose sealed
// prefix holds superseded heartbeats: the sealed receipts stay whole in
// their segment, superseded active ones are elided, the audit and the
// archive still verify, and unseal restores the sealed receipts unchanged.
func TestJSGV0012_CompactionOnASealedStore(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	const created = 400
	repo := historyStore(t, created)
	holdLeasePolicy(t, repo)
	root := filepath.Join(t.TempDir(), "worktree")
	holdGit(t, root, "init", "-q", "-b", "main")
	holdGit(t, root, "-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-q", "--allow-empty", "-m", "base")
	entries, err := os.ReadDir(filepath.Join(repo.PrimaryWorktree, intent.Dir, "tickets"))
	if err != nil {
		t.Fatal(err)
	}
	lease := func(id string, l transaction.LeaseRequest) *Report {
		t.Helper()
		r, err := Lease(ctx, repo, historyActor, LeaseChoice{QueueID: fixture.QueueID, RequestID: id, Root: root, Lease: l}, WallClock())
		if err != nil || r.Outcome.Outcome != mutation.OutcomeCompleted {
			t.Fatalf("%s: %+v %v", id, r, err)
		}
		return r
	}
	c := lease("jsg-claim", transaction.LeaseRequest{Verb: transaction.LeaseClaim, TicketID: fixture.TicketID(strings.TrimSuffix(entries[0].Name(), ".json")), Holder: "agent-1", LeaseMinutes: "60"})
	heartbeats := 0
	beatUntil := func(seq uint64) {
		t.Helper()
		for {
			head, err := readHead(repo)
			if err != nil {
				t.Fatal(err)
			}
			if head.LastSeq.Uint64() >= seq {
				return
			}
			lease(fmt.Sprintf("jsg-hb-%d", heartbeats), transaction.LeaseRequest{Verb: transaction.LeaseHeartbeat, AttemptID: c.AttemptID, Generation: c.Generation})
			heartbeats++
		}
	}
	// Heartbeats span the first segment boundary, so the sealed prefix and
	// the active tail both hold superseded heartbeats; synthetic edits of an
	// unleased ticket then carry head toward the second boundary (V1-2402)
	// and a last few heartbeats supersede the active ones.
	beatUntil(snapshot.SegmentReceipts + 20)
	padPath := filepath.Join(repo.PrimaryWorktree, intent.Dir, "tickets", entries[1].Name())
	padRaw, err := os.ReadFile(padPath)
	if err != nil {
		t.Fatal(err)
	}
	pad, err := wire.Parse(padRaw)
	if err != nil {
		t.Fatal(err)
	}
	for {
		head, err := readHead(repo)
		if err != nil {
			t.Fatal(err)
		}
		seq := head.LastSeq.Uint64() + 1
		if seq > 2*snapshot.SegmentReceipts+16 {
			break
		}
		id := fmt.Sprintf("jsg-pad-%d", seq)
		reqPath, _ := snapshot.RequestPath(id)
		pad.Obj.Set("body", wire.String(fmt.Sprintf("padding receipt %d", seq)))
		historyAppend(t, repo, map[string][]byte{
			reqPath:                               historyRequest(id, seq),
			"intent/tickets/" + entries[1].Name(): wire.EncodeFile(pad),
		}, id)
	}
	beatUntil(2*snapshot.SegmentReceipts + 20)
	head, err := readHead(repo)
	if err != nil {
		t.Fatal(err)
	}
	last := head.LastSeq.Uint64()
	original := map[uint64][]byte{}
	for seq := uint64(1); seq <= last; seq++ {
		original[seq] = mustRead(t, segmentPath(repo, receiptPath(seq)))
	}
	if _, err := SealJournal(ctx, repo); err != nil {
		t.Fatal(err)
	}
	through := segmentThrough(t, repo)
	if through != snapshot.SegmentReceipts {
		t.Fatalf("sealed through %d", through)
	}
	segment := mustRead(t, segmentPath(repo, snapshot.SegmentName(1, through)))
	report, err := CompactLiveness(ctx, repo, false)
	if err != nil {
		t.Fatal(err)
	}
	if report.Elided == 0 {
		t.Fatalf("compaction elided nothing: %+v", report)
	}
	if !bytes.Equal(mustRead(t, segmentPath(repo, snapshot.SegmentName(1, through))), segment) {
		t.Fatal("compaction changed a sealed segment")
	}
	elided := uint64(0)
	for seq := through + 1; seq <= last; seq++ {
		raw := mustRead(t, segmentPath(repo, receiptPath(seq)))
		if !bytes.Equal(raw, original[seq]) {
			elided++
		}
	}
	if elided != report.Elided {
		t.Fatalf("%d active receipts changed, report %d", elided, report.Elided)
	}
	mustSegmentAudit(t, repo, last)
	historyExport(t, repo)
	if _, err := UnsealJournal(ctx, repo); err != nil {
		t.Fatal(err)
	}
	for seq := uint64(1); seq <= through; seq++ {
		if !bytes.Equal(mustRead(t, segmentPath(repo, receiptPath(seq))), original[seq]) {
			t.Fatalf("sealed receipt %d changed", seq)
		}
	}
	mustSegmentAudit(t, repo, last)
}

// TestJSGV0012_ArchiveExportRefusesADamagedSegment checks that export of a
// sealed store whose segment bytes changed refuses and writes no archive.
func TestJSGV0012_ArchiveExportRefusesADamagedSegment(t *testing.T) {
	t.Parallel()
	repo := historyStore(t, segmentHistory)
	if _, err := SealJournal(context.Background(), repo); err != nil {
		t.Fatal(err)
	}
	path := segmentPath(repo, snapshot.SegmentName(1, snapshot.SegmentReceipts))
	raw := mustRead(t, path)
	raw[len(raw)/2] ^= 1
	historyWrite(t, path, raw)
	var out bytes.Buffer
	if _, err := archive.Export(archive.ExportOptions{Repo: repo, Staging: t.TempDir(), Stdout: &out}); wire.CodeOf(err) != wire.CodeJournalForked || out.Len() != 0 {
		t.Fatalf("export of a damaged segment: %v (%d bytes written)", err, out.Len())
	}
}
