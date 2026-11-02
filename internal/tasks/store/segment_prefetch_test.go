package store

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
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
	t.Parallel()
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
