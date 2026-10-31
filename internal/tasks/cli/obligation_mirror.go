package cli

// Obligation mirror reads (TOL-V0-121..124, GitHub #1214): owner ledgers are
// folded once per read, and a dispatcher carries the folds across ticks
// keyed by ticket and ledger head, so an unchanged ledger is never refolded.
import (
	"sync"

	"github.com/Beamfall/corvint/internal/tasks/obligation"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

type ledgerFold struct {
	head wire.Digest
	l    *ticket.ObligationLedger
	err  error
}

// ledgerFoldCache is a dispatcher's fold carry, one entry per ticket.
type ledgerFoldCache struct {
	mu sync.Mutex
	m  map[string]ledgerFold
}

// mirrorFolds folds the ledgers of one inventory snapshot on demand.
type mirrorFolds struct {
	inv   *ticket.Inventory
	read  func(wire.Digest) ([]byte, error)
	cache *ledgerFoldCache
	local map[string]ledgerFold
}

func newMirrorFolds(rc *readCtx, inv *ticket.Inventory, cache *ledgerFoldCache) *mirrorFolds {
	return &mirrorFolds{inv: inv, read: store.ObligationEventReader(rc.repo), cache: cache, local: map[string]ledgerFold{}}
}

// fold returns rec's folded ledger; rec must carry a reference.
func (f *mirrorFolds) fold(rec *ticket.Record) (*ticket.ObligationLedger, error) {
	id, head := rec.TicketID.Raw, rec.ObligationsRef.Head
	if x, ok := f.local[id]; ok {
		return x.l, x.err
	}
	if f.cache != nil {
		f.cache.mu.Lock()
		x, ok := f.cache.m[id]
		f.cache.mu.Unlock()
		if ok && x.head == head && x.err == nil {
			f.local[id] = x
			return x.l, nil
		}
	}
	l, err := ticket.FoldObligationChain(rec.TicketID, *rec.ObligationsRef, f.read)
	x := ledgerFold{head: head, l: l, err: err}
	f.local[id] = x
	if f.cache != nil {
		f.cache.mu.Lock()
		if f.cache.m == nil {
			f.cache.m = map[string]ledgerFold{}
		}
		f.cache.m[id] = x
		f.cache.mu.Unlock()
	}
	return l, err
}

// owner is the obligation.MirrorLookup of this snapshot.
func (f *mirrorFolds) owner(id string) obligation.MirrorOwner {
	rec, ok := f.inv.Get(id)
	if !ok || rec == nil {
		return obligation.MirrorOwner{Missing: obligation.MirrorOwnerTicketMissing}
	}
	if rec.ObligationsRef == nil {
		return obligation.MirrorOwner{Missing: obligation.MirrorOwnerLedgerMissing}
	}
	l, err := f.fold(rec)
	if err != nil {
		return obligation.MirrorOwner{Missing: obligation.MirrorOwnerLedgerUnknown}
	}
	return obligation.MirrorOwner{Ledger: l}
}

func (f *mirrorFolds) resolve(l *ticket.ObligationLedger) []obligation.MirrorResolution {
	return obligation.ResolveMirrors(l, f.owner)
}

// activeLedger reports an OPEN or HELD ticket with a ledger reference.
func activeLedger(rec *ticket.Record) bool {
	return rec != nil && rec.ObligationsRef != nil && (rec.Status == ticket.StatusOpen || rec.Status == ticket.StatusHeld)
}

// mirrorFindingsOf lists the TOL-V0-122 findings of every OPEN or HELD
// ledger ticket in ids order; a ledger that cannot be folded has none here
// (the caller reports it as unknown).
func (f *mirrorFolds) findings(ids []string) []wire.Value {
	var out []wire.Value
	for _, id := range ids {
		if closedTicket(f.inv, id) {
			continue
		}
		rec, _ := f.inv.Get(id)
		if !activeLedger(rec) {
			continue
		}
		l, err := f.fold(rec)
		if err != nil {
			continue
		}
		rs := f.resolve(l)
		out = append(out, obligation.MirrorFindings(id, rs, obligation.MirrorsCompletable(l, rs))...)
	}
	return out
}

// mirrorsValue is obligations show's mirrors member: every mirror entry of
// l resolved (TOL-V0-121); ok is false when l has none.
func mirrorsValue(f *mirrorFolds, l *ticket.ObligationLedger) (wire.Value, bool) {
	rs := f.resolve(l)
	if len(rs) == 0 {
		return wire.Value{}, false
	}
	vs := make([]wire.Value, 0, len(rs))
	for _, r := range rs {
		vs = append(vs, obligation.MirrorValue(r))
	}
	return wire.ObjectValue(wire.NewObject().Set("completable", wire.Bool(obligation.MirrorsCompletable(l, rs))).Set("items", wire.Array(vs...))), true
}

// dispatchLedgerView is one OPEN or HELD ledger ticket as a dispatcher
// observation folds it (TOL-V0-123, TOL-V0-124).
type dispatchLedgerView struct {
	unblocks int64
	unknown  bool
}

// observeLedgers folds every OPEN or HELD ledger ticket of in and sets
// in.MirrorsDecided to the tickets whose pending items are all decided
// mirrors, so the observation's plan defers them.
func observeLedgers(rc *readCtx, in *transaction.PlanInput, cache *ledgerFoldCache) map[string]dispatchLedgerView {
	out := map[string]dispatchLedgerView{}
	if in.Tickets == nil {
		return out
	}
	f := newMirrorFolds(rc, in.Tickets, cache)
	for _, id := range in.Tickets.IDs() {
		if closedTicket(in.Tickets, id) {
			continue
		}
		rec, _ := in.Tickets.Get(id)
		if !activeLedger(rec) {
			continue
		}
		l, err := f.fold(rec)
		if err != nil {
			out[id] = dispatchLedgerView{unknown: true}
			continue
		}
		v := dispatchLedgerView{unblocks: l.Unblocks}
		if rs := f.resolve(l); obligation.MirrorsCompletable(l, rs) {
			if in.MirrorsDecided == nil {
				in.MirrorsDecided = map[string]bool{}
			}
			in.MirrorsDecided[id] = true
		}
		out[id] = v
	}
	return out
}
