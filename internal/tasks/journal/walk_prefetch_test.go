package journal

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
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

// sameAudit fails unless both audits succeeded with the same verdict, records
// and canonical latest.
func sameAudit(t *testing.T, seq, pre *Result, seqErr, preErr error) {
	t.Helper()
	if seqErr != nil || preErr != nil {
		t.Fatalf("audits: sequential %v, prefetched %v", seqErr, preErr)
	}
	if seq.LastSeq != pre.LastSeq || seq.LastReceiptSha256 != pre.LastReceiptSha256 || seq.StructuralConsistency != pre.StructuralConsistency || seq.SemanticCoverage != pre.SemanticCoverage || seq.ProjectionAgreement != pre.ProjectionAgreement {
		t.Fatalf("verdicts differ:\nsequential %+v\nprefetched %+v", seq, pre)
	}
	if !reflect.DeepEqual(seq.Records, pre.Records) || !reflect.DeepEqual(seq.chain.canonical, pre.chain.canonical) {
		t.Fatal("records or canonical latest differ")
	}
}

// V1-1701 (JRT-V0-016): a receipt read ahead but never reached settles
// nothing, so a later receipt's close failure cannot replace the refusal of
// an earlier chain fault; a reached receipt's close failure is the
// sequential walk's refusal.
func TestV11701_ReadAheadCloseFailureSettlesInSequence(t *testing.T) {
	for _, c := range []struct {
		name string
		fork uint64
		code string
	}{
		{"earlier chain fault", 200, wire.CodeJournalForked},
		{"reached close failure", 0, wire.CodeUnsupportedFilesystem},
	} {
		t.Run(c.name, func(t *testing.T) {
			repo, r := prefetchHistory(t, 400)
			if c.fork != 0 {
				name, _ := snapshot.ReceiptName(c.fork)
				path := filepath.Join(repo.StateDir, "receipts", name)
				v, err := wire.Parse(read(t, path))
				if err != nil {
					t.Fatal(err)
				}
				v.Obj.Set("prev", wire.String(string(wire.Sum([]byte("not the previous receipt")))))
				fixture.Write(t, path, wire.EncodeFile(v))
			}
			failing, _ := snapshot.ReceiptName(250)
			old := closeReadFile
			t.Cleanup(func() { closeReadFile = old })
			closeReadFile = func(f *os.File) error {
				err := f.Close()
				if filepath.Base(f.Name()) == failing {
					return errors.Join(err, errors.New("injected close failure"))
				}
				return err
			}
			_, _, seqErr, preErr, _, _ := auditBoth(t, r)
			if seqErr == nil || preErr == nil || seqErr.Error() != preErr.Error() {
				t.Fatalf("refusals differ: sequential %v, prefetched %v", seqErr, preErr)
			}
			if wire.CodeOf(seqErr) != c.code {
				t.Fatalf("refusal %v, want %s", seqErr, c.code)
			}
		})
	}
}

// V1-1701 (JRT-V0-016): every read-ahead is bounded by the window's remaining
// byte budget before it buffers, so a window never holds more than the
// budget; a receipt larger than what is left begins the next window, and one
// larger than the whole budget is read once, by step, under its own bound.
func TestV11701_ReadAheadHoldsAtMostTheBudget(t *testing.T) {
	repo, r := prefetchHistory(t, 40)
	tk := fixture.Ticket("A")
	body := strings.Repeat("x", 6*wire.KiB)
	tk.Body = &body
	appendReceipt(t, repo, "MUTATION", map[string][]byte{"intent/tickets/A.json": tk.Encode()}, "", true, true, false)
	prefetchHistory2(t, repo, 40)
	old := prefetchBytes
	t.Cleanup(func() { prefetchBytes = old })
	prefetchBytes = 4 * wire.KiB
	seq, pre, seqErr, preErr, seqReads, preReads := auditBoth(t, r)
	sameAudit(t, seq, pre, seqErr, preErr)
	receipts := 82
	stats := pre.chain.prefetch.stats
	if seqReads != receipts || preReads != receipts {
		t.Fatalf("receipt reads: sequential %d, prefetched %d, want %d each", seqReads, preReads, receipts)
	}
	if stats.maxHeld > prefetchBytes || stats.maxHeld == 0 {
		t.Fatalf("a window held %d receipt bytes; budget %d", stats.maxHeld, prefetchBytes)
	}
	// A receipt that did not fit what was left ended its window and began
	// the next; only the one larger than the whole budget was left to step.
	if stats.routed == 0 || stats.read != receipts-1 || stats.windows < 3 {
		t.Fatalf("stats %+v: the budget must end windows and route receipts to step", stats)
	}
}

// prefetchHistory2 appends n more inline ticket receipts to repo.
func prefetchHistory2(t *testing.T, repo *fixture.Repo, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		local := []string{"A", "B", "C"}[i%3]
		tk := fixture.Ticket(local)
		body := fmt.Sprintf("later receipt %d", i)
		tk.Body = &body
		appendReceipt(t, repo, "MUTATION", map[string][]byte{"intent/tickets/" + local + ".json": tk.Encode()}, "", true, true, false)
	}
}

// V1-1701 (JRT-V0-015): a receipt with one inline and one blob-backed post is
// left whole to step before any post is checked, so each afterimage is
// validated exactly as often as in the sequential walk.
func TestV11701_MixedInlineAndBlobPostsValidateOnce(t *testing.T) {
	repo, r := setup(t)
	const n, mixed = 60, 12
	for i := 0; i < n; i++ {
		a, b := fixture.Ticket("A"), fixture.Ticket("B")
		body := fmt.Sprintf("receipt %d", i)
		a.Body, b.Body = &body, &body
		posts := map[string][]byte{"intent/tickets/A.json": a.Encode(), "intent/tickets/B.json": b.Encode()}
		if i%5 != 4 {
			appendReceipt(t, repo, "MUTATION", posts, "", true, true, false)
			continue
		}
		// Every fifth receipt: both posts blob-backed, then A's made inline.
		seq := appendReceipt(t, repo, "MUTATION", posts, "", true, false, true)
		name, _ := snapshot.ReceiptName(seq)
		path := filepath.Join(repo.StateDir, "receipts", name)
		v, err := wire.Parse(read(t, path))
		if err != nil {
			t.Fatal(err)
		}
		pv, _ := v.Obj.Get("post")
		post := pv.Arr
		first, _ := post[0].Obj.Get("path")
		if len(post) != 2 || first.Str != "intent/tickets/A.json" {
			t.Fatalf("post %v", post)
		}
		rec, err := wire.Parse(posts["intent/tickets/A.json"])
		if err != nil {
			t.Fatal(err)
		}
		post[0].Obj.Set("record", rec)
		post[0].Obj.Set("blobSha256", wire.Null())
		raw := wire.EncodeFile(v)
		fixture.Write(t, path, raw)
		hp := filepath.Join(repo.StateDir, "head.json")
		h, err := snapshot.DecodeHead(read(t, hp))
		if err != nil {
			t.Fatal(err)
		}
		fixture.Write(t, hp, wire.EncodeFile(fixture.HeadValue(repo.Root, seq, wire.Sum(raw), h.Generation.Uint64(), h.InitSha256)))
	}
	var calls atomic.Int64
	old := afterValidateRecord
	t.Cleanup(func() { afterValidateRecord = old })
	afterValidateRecord = func() { calls.Add(1) }
	oldPrefetch := walkPrefetch
	t.Cleanup(func() { walkPrefetch = oldPrefetch })
	walkPrefetch = false
	seq, seqErr := r.Audit()
	seqCalls := calls.Swap(0)
	walkPrefetch = true
	pre, preErr := r.Audit()
	preCalls := calls.Load()
	sameAudit(t, seq, pre, seqErr, preErr)
	if seqCalls == 0 || preCalls != seqCalls {
		t.Fatalf("validateRecord calls: sequential %d, prefetched %d", seqCalls, preCalls)
	}
	receipts := n + 1
	stats := pre.chain.prefetch.stats
	if stats.decoded != receipts || stats.validated != receipts-mixed-1 {
		t.Fatalf("prepared %d receipts and %d post sets, want %d and %d", stats.decoded, stats.validated, receipts, receipts-mixed-1)
	}
}
