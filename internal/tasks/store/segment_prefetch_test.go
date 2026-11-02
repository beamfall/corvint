package store

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/journal"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
)

// LGL-V0-813: a complete audit of a segmented store that reads sealed
// receipts ahead from their verified segment returns exactly what the
// sequential sealed walk returns -- verdicts, records, the physical files it
// consumed, and every refusal -- and is refused the same way when a segment
// or an active receipt is tampered with.
func TestLGLV0813_SegmentPrefetchMatchesTheSequentialSealedWalk(t *testing.T) {
	// Not parallel: it switches the process-wide segment read-ahead.
	repo := historyStore(t, 4*snapshot.SegmentReceipts+40)
	if err := sealSegmentsLocked(context.Background(), repo); err != nil {
		t.Fatal(err)
	}
	if got := segmentThrough(t, repo); got < 3*snapshot.SegmentReceipts {
		t.Fatalf("sealed through %d", got)
	}
	head, err := readHead(repo)
	if err != nil {
		t.Fatal(err)
	}
	type walk struct {
		res      *journal.Result
		physical []string
		err      string
	}
	run := func(on bool) (plain, observed walk) {
		t.Helper()
		defer journal.SetSegmentPrefetchForTest(on)()
		r := journalReader(repo, head)
		r.SelectState = true
		res, err := r.Audit()
		plain = walk{res: res, err: fmt.Sprint(err)}
		ores, obs, err := journalReader(repo, head).AuditForWriteObserved()
		observed = walk{res: ores, err: fmt.Sprint(err)}
		for p := range obs.Files {
			observed.physical = append(observed.physical, p)
		}
		return plain, observed
	}
	same := func(label string, a, b walk) {
		t.Helper()
		if a.err != b.err {
			t.Fatalf("%s: errors differ: sequential %s, prefetched %s", label, a.err, b.err)
		}
		if a.res == nil || b.res == nil {
			return
		}
		x, y := a.res, b.res
		if x.Mode != y.Mode || x.LastSeq != y.LastSeq || x.LastReceiptSha256 != y.LastReceiptSha256 || x.Identity != y.Identity || x.StructuralConsistency != y.StructuralConsistency || x.ProjectionAgreement != y.ProjectionAgreement || x.SemanticCoverage != y.SemanticCoverage || !reflect.DeepEqual(x.Records, y.Records) || !reflect.DeepEqual(x.RequestDigests, y.RequestDigests) {
			t.Fatalf("%s: results differ", label)
		}
		if !reflect.DeepEqual(sortedCopy(a.physical), sortedCopy(b.physical)) {
			t.Fatalf("%s: consumed files differ: %d sequential, %d prefetched", label, len(a.physical), len(b.physical))
		}
	}
	seqPlain, seqObs := run(false)
	prePlain, preObs := run(true)
	if seqPlain.err != "<nil>" || seqPlain.res.Mode != journal.ModeFull || seqPlain.res.StructuralConsistency != "CONSISTENT" {
		t.Fatalf("sequential sealed audit: %s", seqPlain.err)
	}
	same("plain", seqPlain, prePlain)
	same("observed", seqObs, preObs)

	// A flipped byte in a sealed segment.
	m, err := boundManifest(repo, head)
	if err != nil {
		t.Fatal(err)
	}
	e := m.Segments[1]
	segFile := segmentPath(repo, snapshot.SegmentName(e.First, e.Last))
	raw, err := os.ReadFile(segFile)
	if err != nil {
		t.Fatal(err)
	}
	tampered := append([]byte(nil), raw...)
	tampered[len(tampered)/2] ^= 1
	if err := os.WriteFile(segFile, tampered, 0o600); err != nil {
		t.Fatal(err)
	}
	a, _ := run(false)
	b, _ := run(true)
	if a.err == "<nil>" {
		t.Fatal("a tampered segment was accepted")
	}
	same("tampered segment", a, b)
	if err := os.WriteFile(segFile, raw, 0o600); err != nil {
		t.Fatal(err)
	}

	// A flipped byte in an active receipt.
	name, _ := snapshot.ReceiptName(head.LastSeq.Uint64() - 1)
	rp := filepath.Join(repo.StateDir, "receipts", name)
	rraw, err := os.ReadFile(rp)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(rp, append([]byte(" "), rraw...), 0o600); err != nil {
		t.Fatal(err)
	}
	a, _ = run(false)
	b, _ = run(true)
	if a.err == "<nil>" {
		t.Fatalf("a tampered active receipt was accepted: %s", a.err)
	}
	same("tampered active receipt", a, b)
}

func sortedCopy(in []string) []string {
	out := append([]string(nil), in...)
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j] < out[j-1]; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

// LGL-V0-814: a seal whose reads, unlinks and directory syncs run in
// parallel writes exactly the bytes the one-at-a-time seal wrote -- every
// segment, the manifest and head.json -- and the sealed store audits to the
// same verdicts. Both seals run on one store: an unseal between them
// restores the tree byte for byte (JSG-V0-013).
func TestLGLV0814_ParallelSealWritesTheSameBytes(t *testing.T) {
	ctx := context.Background()
	repo := historyStore(t, 3*snapshot.SegmentReceipts+30)
	sealWith := func(workers int) (map[string]string, *journal.Result) {
		t.Helper()
		old := sealWorkers.Swap(int64(workers))
		defer sealWorkers.Store(old)
		if _, err := SealJournal(ctx, repo); err != nil {
			t.Fatalf("seal with %d workers: %v", workers, err)
		}
		tree := map[string]string{}
		for p, raw := range stateTree(t, repo) {
			if p == "head.json" || strings.HasPrefix(p, snapshot.SegmentsDir+"/") || strings.HasPrefix(p, "receipts/") || strings.HasPrefix(p, "requests/") {
				tree[p] = raw
			}
		}
		res, err := segmentAudit(repo)
		if err != nil {
			t.Fatal(err)
		}
		return tree, res
	}
	serial, a := sealWith(1)
	if _, err := UnsealJournal(ctx, repo); err != nil {
		t.Fatal(err)
	}
	parallel, b := sealWith(32)
	if !reflect.DeepEqual(serial, parallel) {
		t.Fatalf("parallel seal wrote different bytes: %d files serial, %d parallel", len(serial), len(parallel))
	}
	// The inventory digest hashes modification times, which the unseal and
	// reseal rewrite, so the identity compared is the head and intent tree.
	if a.LastSeq != b.LastSeq || a.LastReceiptSha256 != b.LastReceiptSha256 || a.Identity.HeadSha256 != b.Identity.HeadSha256 || a.Identity.IntentTreeSha256 != b.Identity.IntentTreeSha256 || a.StructuralConsistency != b.StructuralConsistency || a.ProjectionAgreement != b.ProjectionAgreement {
		t.Fatalf("the two sealed stores audit differently: seq %s/%s identity %+v / %+v verdicts %s %s / %s %s", a.LastSeq, b.LastSeq, a.Identity, b.Identity, a.StructuralConsistency, a.ProjectionAgreement, b.StructuralConsistency, b.ProjectionAgreement)
	}
}
