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
// the preparation could not finish without error (a receipt whose read-ahead
// failed or did not fit the window's byte budget, an elided receipt, a
// receipt with a blob-backed post, any decode or validation failure) is left
// to step's own sequential code, so every refusal is the one, and at the
// place, the sequential walk reports. A read-ahead records nothing: its close
// error, physical witness and read hook are settled only when step reaches
// that receipt, so a receipt the walk never reaches cannot change its result.
type receiptPrefetch struct {
	// first is the sequence number of ready[0].
	first uint64
	ready []*preparedReceipt
	// stats counts reads and preparations for the maintained tests.
	stats prefetchStats
}

// read counts read-ahead receipts kept for step, routed the read-ahead
// attempts discarded so step reads that receipt itself, and maxHeld the most
// receipt bytes one window held.
type prefetchStats struct {
	windows, read, routed, decoded, validated, maxHeld int
}

// preparedReceipt is one receipt read ahead: its bytes, the file's close
// error held unsettled and, when the bytes decoded as a whole receipt, the
// receipt and (only when every post is inline and was prepared without
// error) each post's checked afterimage.
type preparedReceipt struct {
	source  *nativeRead
	path    string
	raw     []byte
	cleanup error
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

// prefetchBytes bounds the receipt bytes one window holds, so the walk still
// streams history in bounded memory (TMV0002 AS10). Each read-ahead is bounded
// by the budget left before it buffers anything, so a window never holds more
// than this many receipt bytes; a receipt that does not fit ends the window
// and begins the next one, and a receipt larger than the whole budget is read
// by step under its own bound. The decoded receipts and inline
// afterimages a window retains are in addition, in proportion to those bytes.
// It is a variable only so a test can narrow it.
var prefetchBytes = 512 * wire.KiB

// prefetchWorkers bounds the decoding goroutines of one window.
func prefetchWorkers() int {
	return min(runtime.GOMAXPROCS(0), 8)
}

// prefetchFrom fills the next window when receipt index i (0-based) is not
// already prepared. A window runs from i up to, but excluding, the next index
// whose step is preceded by the head-moved check, so no read crosses a check
// the sequential walk makes first, and ends at the first receipt that does
// not fit the bytes left in prefetchBytes. Only a native source reads ahead.
// A read-ahead that fails for any reason (including the budget's tighter
// bound) is discarded with its close error and ends the window; the receipt
// is read again when the walk reaches it, first ahead with the whole budget
// and otherwise by step, which reports exactly what the sequential walk
// reports.
func (r Reader) prefetchFrom(pf *receiptPrefetch, names []string, i int) {
	seq := uint64(i + 1)
	if pf == nil || (seq >= pf.first && seq < pf.first+uint64(len(pf.ready))) {
		return
	}
	pf.first, pf.ready = seq, pf.ready[:0]
	native, ok := r.Source.(*nativeRead)
	if !ok {
		return
	}
	end := min(i+1+(abortEvery-1-(i+1)%abortEvery), len(names))
	pf.stats.windows++
	held := 0
	for k := i; k < end && held < prefetchBytes; k++ {
		p := "receipts/" + names[k]
		raw, cleanup, err := native.readHeld(p, min(wire.MaxReceiptFileBytes, prefetchBytes-held))
		if err != nil {
			pf.stats.routed++
			break
		}
		pf.stats.read++
		held += len(raw)
		pf.ready = append(pf.ready, &preparedReceipt{source: native, path: p, raw: raw, cleanup: cleanup})
	}
	pf.stats.maxHeld = max(pf.stats.maxHeld, held)
	work := pf.ready
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
// reads nothing: a receipt with any blob-backed post leaves all its posts to
// step, before any is checked, so no post is validated twice (JRT-V0-015).
func (r Reader) prepare(p *preparedReceipt) {
	rc, err := snapshot.WholeReceipt(p.raw)
	if err != nil || rc == nil {
		return
	}
	p.rc = rc
	for _, e := range rc.Post {
		if e.Sha256 != nil && e.Record == nil {
			return
		}
	}
	posts := make([]checkedPost, len(rc.Post))
	for j, e := range rc.Post {
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
// there is one, settled now that the walk has reached it, otherwise the
// sequential read and chain decode.
func (p *preparedReceipt) receipt(r Reader, st *chain, name string) (*snapshot.Receipt, []byte, error) {
	if p == nil {
		raw, err := requiredRead(r.Source, "receipts/"+name, wire.MaxReceiptFileBytes)
		if err != nil {
			return nil, nil, err
		}
		return st.decode(name, raw)
	}
	if _, err := p.source.settle(p.path, p.raw, p.cleanup, nil); err != nil {
		return nil, nil, err
	}
	if p.raw == nil {
		p.raw = []byte{}
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
