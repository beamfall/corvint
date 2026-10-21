package journal

import (
	"runtime"
	"sync"

	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// A complete walk spends most of its time decoding and validating receipts
// and their inline afterimages, not opening files (V1-1701). receiptPrefetch
// lets the walk read one abort window of receipts ahead, in sequence and on
// the walking goroutine, and decode and validate them in parallel. step then
// folds every receipt in order exactly as before: a prepared receipt only
// replaces work whose result is a pure function of its bytes, and anything
// the preparation could not finish without error (an unreadable receipt, an
// elided receipt, a blob-backed post, any decode or validation failure) is
// left to step's own sequential code, so every refusal is the one, and at
// the place, the sequential walk reports.
type receiptPrefetch struct {
	// first is the sequence number of ready[0].
	first uint64
	ready []*preparedReceipt
	// stats counts reads and preparations for the maintained tests.
	stats prefetchStats
}

type prefetchStats struct {
	windows, read, decoded, validated int
}

// preparedReceipt is one receipt read ahead: its bytes or read error and,
// when they decoded as a whole receipt, the receipt and (only when every post
// was prepared without error) each post's checked afterimage.
type preparedReceipt struct {
	raw     []byte
	readErr error
	rc      *snapshot.Receipt
	posts   []checkedPost
}

// checkedPost is one post's afterimage bytes (nil for a deletion) with the
// validateRecord result for them.
type checkedPost struct {
	raw        []byte
	coverage   bool
	descriptor *snapshot.Init
	decoded    decodedPost
}

// walkPrefetch is false only in the tests that compare the prefetched walk
// with the sequential one.
var walkPrefetch = true

func newReceiptPrefetch() *receiptPrefetch {
	if !walkPrefetch {
		return nil
	}
	return &receiptPrefetch{}
}

// prefetchBytes bounds the receipt bytes one window reads ahead, so the
// walk still streams history in bounded memory (TMV0002 AS10): a window holds
// at most this much plus one receipt of at most wire.MaxReceiptFileBytes.
const prefetchBytes = 512 * wire.KiB

// prefetchWorkers bounds the decoding goroutines of one window.
func prefetchWorkers() int {
	return min(runtime.GOMAXPROCS(0), 8)
}

// prefetchFrom fills the next window when receipt index i (0-based) is not
// already prepared. A window runs from i up to, but excluding, the next index
// whose step is preceded by the head-moved check, so no read crosses a check
// the sequential walk makes first, and ends early once it holds
// prefetchBytes. Reading stops at the first read error, which is kept for
// step to return at that receipt.
func (r Reader) prefetchFrom(pf *receiptPrefetch, names []string, i int) {
	seq := uint64(i + 1)
	if pf == nil || (seq >= pf.first && seq < pf.first+uint64(len(pf.ready))) {
		return
	}
	end := min(i+1+(abortEvery-1-(i+1)%abortEvery), len(names))
	pf.first, pf.ready = seq, pf.ready[:0]
	pf.stats.windows++
	held := 0
	for k := i; k < end && held < prefetchBytes; k++ {
		raw, err := requiredRead(r.Source, "receipts/"+names[k], wire.MaxReceiptFileBytes)
		pf.stats.read++
		held += len(raw)
		pf.ready = append(pf.ready, &preparedReceipt{raw: raw, readErr: err})
		if err != nil {
			break
		}
	}
	work := pf.ready
	if n := len(work); n > 0 && work[n-1].readErr != nil {
		work = work[:n-1]
	}
	var next sync.Mutex
	at := 0
	var wg sync.WaitGroup
	for w := 0; w < prefetchWorkers(); w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				next.Lock()
				k := at
				at++
				next.Unlock()
				if k >= len(work) {
					return
				}
				r.prepare(work[k])
			}
		}()
	}
	wg.Wait()
	for _, p := range work {
		if p.rc != nil {
			pf.stats.decoded++
		}
		if p.posts != nil {
			pf.stats.validated++
		}
	}
}

// prepare decodes one read-ahead receipt and checks its inline posts. It
// reads nothing: a blob-backed post leaves the posts to step.
func (r Reader) prepare(p *preparedReceipt) {
	rc, err := snapshot.WholeReceipt(p.raw)
	if err != nil || rc == nil {
		return
	}
	p.rc = rc
	posts := make([]checkedPost, len(rc.Post))
	for j, e := range rc.Post {
		if e.Sha256 != nil && e.Record == nil {
			return
		}
		raw, err := r.postBytes(e)
		if err != nil {
			return
		}
		posts[j].raw = raw
		if raw == nil {
			continue
		}
		c := &posts[j]
		if c.coverage, c.descriptor, c.decoded, err = r.validateRecord(e.Path, raw, rc); err != nil {
			return
		}
	}
	p.posts = posts
}

// take hands step the prepared receipt for seq, or nil.
func (pf *receiptPrefetch) take(seq uint64) *preparedReceipt {
	if pf == nil || seq < pf.first || seq >= pf.first+uint64(len(pf.ready)) {
		return nil
	}
	p := pf.ready[seq-pf.first]
	pf.ready[seq-pf.first] = nil
	return p
}

// receipt is step's read and decode of one receipt: the prepared result when
// there is one, otherwise the sequential read and chain decode.
func (p *preparedReceipt) receipt(r Reader, st *chain, name string) (*snapshot.Receipt, []byte, error) {
	if p == nil {
		raw, err := requiredRead(r.Source, "receipts/"+name, wire.MaxReceiptFileBytes)
		if err != nil {
			return nil, nil, err
		}
		return st.decode(name, raw)
	}
	if p.readErr != nil {
		return nil, nil, p.readErr
	}
	if p.rc == nil || st.restore == nil {
		return st.decode(name, p.raw)
	}
	st.restore.Accept(p.rc)
	return p.rc, p.raw, nil
}

// post is post j's checked afterimage when every post was prepared.
func (p *preparedReceipt) post(j int) *checkedPost {
	if p == nil || p.posts == nil {
		return nil
	}
	return &p.posts[j]
}
