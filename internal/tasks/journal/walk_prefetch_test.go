package journal

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// prefetchHistory appends n ticket receipts after genesis, every seventh
// with a blob-backed post, so the walk crosses an abort window and mixes
// prepared and sequentially read posts.
func prefetchHistory(t *testing.T, n int) (*fixture.Repo, Reader) {
	t.Helper()
	repo, r := setup(t)
	for i := 0; i < n; i++ {
		local := []string{"A", "B", "C"}[i%3]
		tk := fixture.Ticket(local)
		body := fmt.Sprintf("receipt %d", i)
		tk.Body = &body
		appendReceipt(t, repo, "MUTATION", map[string][]byte{"intent/tickets/" + local + ".json": tk.Encode()}, "", true, true, i%7 == 6)
	}
	return repo, r
}

// auditBoth runs the same complete audit with and without the prefetch and
// counts each run's receipt reads.
func auditBoth(t *testing.T, r Reader, selected ...string) (seq, pre *Result, seqErr, preErr error, seqReads, preReads int) {
	t.Helper()
	reads := 0
	old, oldPrefetch := afterNativeRead, walkPrefetch
	t.Cleanup(func() { afterNativeRead, walkPrefetch = old, oldPrefetch })
	afterNativeRead = func(p string) {
		if strings.HasPrefix(p, "receipts/") {
			reads++
		}
	}
	walkPrefetch = false
	seq, seqErr = r.Audit(selected...)
	seqReads, reads = reads, 0
	walkPrefetch = true
	pre, preErr = r.Audit(selected...)
	return seq, pre, seqErr, preErr, seqReads, reads
}

// V1-1701: the prefetched walk returns the sequential walk's result, reads
// every receipt exactly once, and prepares every receipt whose posts are all
// inline; blob-backed posts stay on step's sequential route.
func TestV11701_PrefetchedWalkMatchesSequentialWalk(t *testing.T) {
	const n = 600
	_, r := prefetchHistory(t, n)
	seq, pre, seqErr, preErr, seqReads, preReads := auditBoth(t, r, "intent/tickets/A.json", "intent/tickets/B.json")
	if seqErr != nil || preErr != nil {
		t.Fatalf("audits: sequential %v, prefetched %v", seqErr, preErr)
	}
	if seq.LastSeq != pre.LastSeq || seq.LastReceiptSha256 != pre.LastReceiptSha256 || seq.StructuralConsistency != pre.StructuralConsistency || seq.SemanticCoverage != pre.SemanticCoverage || seq.ProjectionAgreement != pre.ProjectionAgreement {
		t.Fatalf("verdicts differ:\nsequential %+v\nprefetched %+v", seq, pre)
	}
	if !reflect.DeepEqual(seq.Records, pre.Records) || !reflect.DeepEqual(seq.RequestDigests, pre.RequestDigests) {
		t.Fatal("selected records differ")
	}
	if !reflect.DeepEqual(seq.chain.canonical, pre.chain.canonical) {
		t.Fatal("canonical latest differs")
	}
	if seq.chain.prefetch != nil {
		t.Fatal("the sequential walk prefetched")
	}
	receipts := n + 1
	blobs := n/7 + 1 // and genesis, whose posts are blob-backed
	stats := pre.chain.prefetch.stats
	if seqReads != receipts || preReads != receipts || stats.read != receipts {
		t.Fatalf("receipt reads: sequential %d, prefetched %d (window reads %d), want %d each", seqReads, preReads, stats.read, receipts)
	}
	if stats.decoded != receipts || stats.validated != receipts-blobs {
		t.Fatalf("prepared %d receipts and %d post sets, want %d and %d", stats.decoded, stats.validated, receipts, receipts-blobs)
	}
	if stats.windows < 2 {
		t.Fatalf("%d windows; a %d-receipt walk crosses the %d-receipt abort window", stats.windows, receipts, abortEvery)
	}
}

// V1-1701: a refusal from a receipt inside a read-ahead window is the
// sequential walk's refusal, whether its bytes fail to decode (left to step)
// or decode and break the chain (prepared, refused by step's order checks).
func TestV11701_PrefetchedWalkRefusesLikeSequentialWalk(t *testing.T) {
	for _, c := range []struct {
		name   string
		seq    uint64
		mutate func(t *testing.T, raw []byte) []byte
		code   string
	}{
		{"undecodable", 300, func(t *testing.T, raw []byte) []byte { return append([]byte("{"), raw...) }, ""},
		{"forked prev", 200, func(t *testing.T, raw []byte) []byte {
			v, err := wire.Parse(raw)
			if err != nil {
				t.Fatal(err)
			}
			v.Obj.Set("prev", wire.String(string(wire.Sum([]byte("not the previous receipt")))))
			return wire.EncodeFile(v)
		}, wire.CodeJournalForked},
	} {
		t.Run(c.name, func(t *testing.T) {
			repo, r := prefetchHistory(t, 400)
			name, _ := snapshot.ReceiptName(c.seq)
			path := filepath.Join(repo.StateDir, "receipts", name)
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			fixture.Write(t, path, c.mutate(t, raw))
			_, _, seqErr, preErr, _, _ := auditBoth(t, r)
			if seqErr == nil || preErr == nil || seqErr.Error() != preErr.Error() {
				t.Fatalf("refusals differ: sequential %v, prefetched %v", seqErr, preErr)
			}
			if c.code != "" && !strings.Contains(seqErr.Error(), c.code) {
				t.Fatalf("refusal %v, want %s", seqErr, c.code)
			}
		})
	}
}
