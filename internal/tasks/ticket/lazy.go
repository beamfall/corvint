// SPDX-License-Identifier: Apache-2.0

package ticket

import (
	"sort"
	"sync"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// Summary is the part of a record the inventory's structural checks, its
// planning order and the closed-set reads consult (TCX-V0-980). A lazy
// inventory holds only this for a COMPLETED or ARCHIVED ticket until a
// caller asks for the whole record.
type Summary struct {
	TicketID           wire.TicketID
	Status             string
	ArchivedFrom       *string
	Priority           string
	Order              wire.Count
	AcceptanceRevision wire.Count
	Kind               string
	Owner              *string
	Milestone          *string
	Labels             []string
	Dependencies       []Dependency
	UpdatedAt          wire.Timestamp
	CompletedAt        *wire.Timestamp
}

// SummaryOf is r's summary.
func SummaryOf(r *Record) *Summary {
	s := &Summary{TicketID: r.TicketID, Status: r.Status, ArchivedFrom: r.ArchivedFrom, Priority: r.Priority, Order: r.Order,
		AcceptanceRevision: r.AcceptanceRevision, Kind: r.Kind, Owner: r.Owner, Milestone: r.Milestone, Labels: r.Labels,
		Dependencies: r.Dependencies, UpdatedAt: r.UpdatedAt}
	if r.Completion != nil {
		at := r.Completion.RecordedAt
		s.CompletedAt = &at
	}
	return s
}

// Closed reports a COMPLETED or ARCHIVED status, the only ones a lazy
// inventory may hold as a summary.
func Closed(status string) bool { return status == StatusCompleted || status == StatusArchived }

// MaterializeError is the panic value Get raises when a lazy record cannot
// be loaded. Readers recover it at their snapshot boundary and report Err
// (SNAPSHOT_MOVED when the file no longer has the indexed bytes).
type MaterializeError struct{ Err error }

// Loader decodes the whole record of a lazily held ticket.
type Loader func(s *Summary) (*Record, error)

type lazyRecord struct {
	once sync.Once
	rec  *Record
	err  error
}

// NewLazyInventory is NewInventory over full records plus summaries of
// closed tickets whose records load decodes on first Get (TCX-V0-980). The
// structural checks, cycles and planning order are computed from the
// summaries and are identical to NewInventory over the full records.
func NewLazyInventory(queue wire.QueueID, records []*Record, closed []*Summary, load Loader) (*Inventory, error) {
	if len(records)+len(closed) > wire.MaxTicketsPerQueue {
		return nil, wire.Errorf(wire.CodeLimitExceeded, "/", "more than %d tickets in queue", wire.MaxTicketsPerQueue)
	}
	inv := &Inventory{QueueID: queue, byID: map[string]*Record{}, sum: map[string]*Summary{}, inCycle: map[string][]string{}}
	if len(closed) > 0 {
		inv.lazy, inv.load = map[string]*lazyRecord{}, load
	}
	folded := map[string]string{}
	add := func(s *Summary) error {
		if s.TicketID.QueueID() != queue.Raw {
			return wire.Errorf(wire.CodeMalformed, "/ticketId", "ticket %s is outside queue %s", s.TicketID.Raw, queue.Raw)
		}
		if _, dup := inv.sum[s.TicketID.Raw]; dup {
			return wire.Errorf(wire.CodeDuplicateID, "/ticketId", "duplicate ticket %s", s.TicketID.Raw)
		}
		f := wire.FoldToken(s.TicketID.Local)
		if other, dup := folded[f]; dup {
			return wire.Errorf(wire.CodeDuplicateID, "/ticketId", "local tokens %q and %q collide under ASCII case folding", other, s.TicketID.Local)
		}
		folded[f] = s.TicketID.Local
		inv.sum[s.TicketID.Raw] = s
		inv.ids = append(inv.ids, s.TicketID.Raw)
		return nil
	}
	for _, r := range records {
		if err := add(SummaryOf(r)); err != nil {
			return nil, err
		}
		inv.byID[r.TicketID.Raw] = r
	}
	for _, s := range closed {
		if err := add(s); err != nil {
			return nil, err
		}
		inv.lazy[s.TicketID.Raw] = &lazyRecord{}
	}
	sort.Strings(inv.ids)
	inv.findCycles()
	return inv, nil
}

// Summary returns id's summary without loading a lazily held record.
func (inv *Inventory) Summary(id string) (*Summary, bool) {
	s, ok := inv.sum[id]
	return s, ok
}

// materialize loads a lazily held record once; a failure panics with
// MaterializeError.
func (inv *Inventory) materialize(id string) (*Record, bool) {
	l, ok := inv.lazy[id]
	if !ok {
		return nil, false
	}
	l.once.Do(func() { l.rec, l.err = inv.load(inv.sum[id]) })
	if l.err != nil {
		panic(MaterializeError{Err: l.err})
	}
	return l.rec, true
}
