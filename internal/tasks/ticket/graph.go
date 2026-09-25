package ticket

import (
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// Inventory is the set of current ticket records of one queue with the
// TM-V0-005 structural checks: unique IDs under ASCII case folding, one
// queue, and the dependency graph (missing edges, cycles including via
// GATE_PASSED and self edges).
type Inventory struct {
	QueueID wire.QueueID
	byID    map[string]*Record
	ids     []string
	inCycle map[string][]string // ticketId → sorted members of its strongly connected component
}

// NewInventory builds an inventory. It fails on DUPLICATE_ID (exact or
// case-folded local token), on a record outside the queue, and on more than
// the §1 ticket bound. Missing dependencies and cycles are not failures of
// the inventory: they are reported per ticket by Problems, because a queue
// with one bad edge must still be listable.
func NewInventory(queue wire.QueueID, records []*Record) (*Inventory, error) {
	if len(records) > wire.MaxTicketsPerQueue {
		return nil, wire.Errorf(wire.CodeLimitExceeded, "/", "more than %d tickets in queue", wire.MaxTicketsPerQueue)
	}
	inv := &Inventory{QueueID: queue, byID: map[string]*Record{}, inCycle: map[string][]string{}}
	folded := map[string]string{}
	for _, r := range records {
		if r.TicketID.QueueID() != queue.Raw {
			return nil, wire.Errorf(wire.CodeMalformed, "/ticketId", "ticket %s is outside queue %s", r.TicketID.Raw, queue.Raw)
		}
		if _, dup := inv.byID[r.TicketID.Raw]; dup {
			return nil, wire.Errorf(wire.CodeDuplicateID, "/ticketId", "duplicate ticket %s", r.TicketID.Raw)
		}
		f := wire.FoldToken(r.TicketID.Local)
		if other, dup := folded[f]; dup {
			return nil, wire.Errorf(wire.CodeDuplicateID, "/ticketId", "local tokens %q and %q collide under ASCII case folding", other, r.TicketID.Local)
		}
		folded[f] = r.TicketID.Local
		inv.byID[r.TicketID.Raw] = r
		inv.ids = append(inv.ids, r.TicketID.Raw)
	}
	sort.Strings(inv.ids)
	inv.findCycles()
	return inv, nil
}

// Get returns a record by ID.
func (inv *Inventory) Get(id string) (*Record, bool) {
	r, ok := inv.byID[id]
	return r, ok
}

// IDs returns every ticket ID in byte order.
func (inv *Inventory) IDs() []string {
	out := make([]string, len(inv.ids))
	copy(out, inv.ids)
	return out
}

// Len returns the number of records.
func (inv *Inventory) Len() int { return len(inv.ids) }

// findCycles runs Tarjan's algorithm over every dependency edge (both
// obligations count as edges) and records the members of each non-trivial
// strongly connected component. Self edges are rejected at record validation
// but are handled here too for robustness.
func (inv *Inventory) findCycles() {
	index := 0
	indices := map[string]int{}
	low := map[string]int{}
	onStack := map[string]bool{}
	var stack []string
	var visit func(v string)
	visit = func(v string) {
		indices[v] = index
		low[v] = index
		index++
		stack = append(stack, v)
		onStack[v] = true
		self := false
		for _, d := range inv.byID[v].Dependencies {
			w := d.TicketID.Raw
			if w == v {
				self = true
			}
			if _, ok := inv.byID[w]; !ok {
				continue
			}
			if _, seen := indices[w]; !seen {
				visit(w)
				if low[w] < low[v] {
					low[v] = low[w]
				}
			} else if onStack[w] && indices[w] < low[v] {
				low[v] = indices[w]
			}
		}
		if low[v] == indices[v] {
			var comp []string
			for {
				w := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				onStack[w] = false
				comp = append(comp, w)
				if w == v {
					break
				}
			}
			if len(comp) > 1 || self {
				sort.Strings(comp)
				for _, m := range comp {
					inv.inCycle[m] = comp
				}
			}
		}
	}
	for _, id := range inv.ids {
		if _, seen := indices[id]; !seen {
			visit(id)
		}
	}
}

// Problem is one structural dependency problem of a ticket.
type Problem struct {
	Code     string // DEPENDENCY_MISSING or CYCLE
	TicketID string // the missing dependency, or "" for a cycle
	Detail   string
}

// Problems returns the TM-V0-005 structural problems of one ticket: missing
// dependencies and cycle membership.
func (inv *Inventory) Problems(id string) []Problem {
	r, ok := inv.byID[id]
	if !ok {
		return nil
	}
	var out []Problem
	for _, d := range r.Dependencies {
		if _, ok := inv.byID[d.TicketID.Raw]; !ok {
			out = append(out, Problem{Code: wire.CodeDependencyMissing, TicketID: d.TicketID.Raw, Detail: "dependency " + d.TicketID.Raw + " does not exist in the queue"})
		}
	}
	if comp, ok := inv.inCycle[id]; ok {
		out = append(out, Problem{Code: wire.CodeCycle, Detail: "dependency cycle through " + strings.Join(comp, ", ")})
	}
	return out
}

// CheckDependencies validates a proposed record's dependency edges against
// the inventory as TM-V0-005 requires of a mutation: every dependency must
// exist (or be the record itself being created, which is a self cycle), and
// adding the record must not create a cycle (direct, transitive, or through a
// GATE_PASSED edge). The inventory is not modified. This is the pure check
// TCP-02b wires to SET_DEPENDENCIES and CREATE.
func (inv *Inventory) CheckDependencies(proposed *Record) error {
	for i, d := range proposed.Dependencies {
		where := "/dependencies/" + idx(i)
		if d.TicketID.Raw == proposed.TicketID.Raw {
			return wire.Errorf(wire.CodeCycle, where, "ticket depends on itself")
		}
		if _, ok := inv.byID[d.TicketID.Raw]; !ok {
			return wire.Errorf(wire.CodeDependencyMissing, where, "dependency %s does not exist in the queue", d.TicketID.Raw)
		}
	}
	// DFS from each dependency looking for a path back to the proposed ticket
	// over the existing graph (the proposed record overlays its own entry).
	target := proposed.TicketID.Raw
	visited := map[string]bool{}
	var stack []string
	for _, d := range proposed.Dependencies {
		stack = append(stack, d.TicketID.Raw)
	}
	for len(stack) > 0 {
		v := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if v == target {
			return wire.Errorf(wire.CodeCycle, "/dependencies", "adding these dependencies creates a cycle through %s", target)
		}
		if visited[v] {
			continue
		}
		visited[v] = true
		rec, ok := inv.byID[v]
		if !ok {
			continue
		}
		for _, d := range rec.Dependencies {
			stack = append(stack, d.TicketID.Raw)
		}
	}
	return nil
}

// CheckLocalToken reports DUPLICATE_ID when a local token collides, exactly
// or under ASCII case folding, with any live or tombstoned ticket (§2).
func (inv *Inventory) CheckLocalToken(local string) error {
	f := wire.FoldToken(local)
	for _, id := range inv.ids {
		if wire.FoldToken(inv.byID[id].TicketID.Local) == f {
			return wire.Errorf(wire.CodeDuplicateID, "/ticketId", "local token %q collides with %s under ASCII case folding", local, id)
		}
	}
	return nil
}

// Sorted returns the ticket IDs in the §4.3 planning order:
// (priority asc, order asc, ticketId bytes asc).
func (inv *Inventory) Sorted() []string {
	ids := inv.IDs()
	sort.SliceStable(ids, func(i, j int) bool {
		a, b := inv.byID[ids[i]], inv.byID[ids[j]]
		if a.Priority != b.Priority {
			return a.Priority < b.Priority
		}
		if a.Order.Int() != b.Order.Int() {
			return a.Order.Int() < b.Order.Int()
		}
		return ids[i] < ids[j]
	})
	return ids
}
