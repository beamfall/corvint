package dispatch

import (
	"errors"
	"fmt"
	"sort"
	"strconv"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// StallState is one CAL-V0-185 ticket's count of finished sessions since its
// native status last changed. Status is the native status the count is
// against; a work-state touch or a candidate never resets it, because only
// a status transition does. Changed records that a tick observed a status
// other than Status while a session was running, so the session's finish
// restarts the count even when the status has since returned.
type StallState struct {
	Status   string `json:"status"`
	Sessions int    `json:"sessions"`
	Changed  bool   `json:"changed,omitempty"`
}

// maxStallTickets bounds the ledger's stall map; maxStallSessions saturates
// one count. A ticket past the map bound is simply not counted.
const maxStallTickets, maxStallSessions = 8192, 1 << 20

// stallStatuses is the closed native status set a count can be against.
// Terminal tickets are pruned, so only live statuses are recorded.
var stallStatuses = map[string]bool{"DRAFT": true, "OPEN": true, "HELD": true}

// seedStall records, at launch, the status the ticket's sessions count
// against when the ticket has no count yet.
func (d *Dispatcher) seedStall(obs *Observation, key string) {
	if t := observed(obs, key); t != nil && d.ledger.Stall[key] == nil {
		d.restartStall(t, key)
	}
}

// restartStall starts a zero count for a live ticket while the map has room.
func (d *Dispatcher) restartStall(t *Ticket, key string) {
	if !stallStatuses[t.Status] || len(d.ledger.Stall) >= maxStallTickets && d.ledger.Stall[key] == nil {
		return
	}
	if d.ledger.Stall == nil {
		d.ledger.Stall = map[string]*StallState{}
	}
	d.ledger.Stall[key] = &StallState{Status: t.Status}
}

// countStall accounts one finished session of a ticket-keyed worker against
// the post-session observation t (CAL-V0-185). It sets the finished event's
// sessionsSinceStatusChange detail and returns the advisory stalled event
// when the count just reached the configured threshold. A count the ledger
// cannot attest (no launch seed) is UNKNOWN and restarts from zero.
func (d *Dispatcher) countStall(t *Ticket, w *Worker, detail map[string]string) *Event {
	if w.Ticket == "" || t == nil {
		return nil
	}
	s := d.ledger.Stall[w.Key]
	switch {
	case s == nil:
		detail["sessionsSinceStatusChange"] = "UNKNOWN"
		d.restartStall(t, w.Key)
		return nil
	case s.Changed || s.Status != t.Status:
		// The session (or another writer during it) changed the status,
		// possibly and back again.
		detail["sessionsSinceStatusChange"] = "0"
		delete(d.ledger.Stall, w.Key)
		d.restartStall(t, w.Key)
		return nil
	case s.Sessions < maxStallSessions:
		s.Sessions++
	}
	detail["sessionsSinceStatusChange"] = strconv.Itoa(s.Sessions)
	n := d.Config.StalledAfterSessions
	if n == nil || s.Sessions != *n {
		return nil
	}
	return &Event{Kind: "stalled", Ticket: w.Ticket, Role: w.Role, Worker: w.ID, Message: fmt.Sprintf("%s finished %d session(s) with no native status change (still %s); advisory only, nothing is held", d.keyText(w.Key), s.Sessions, s.Status), Detail: map[string]string{"sessions": strconv.Itoa(s.Sessions), "status": s.Status, "threshold": strconv.Itoa(*n)}}
}

// pruneStall drops the count of a ticket no longer observed or no longer
// live, and restarts one whose status changed outside a session. A ticket
// with a running session is left to countStall at that session's finish, so
// a status change the session made is never absorbed by an earlier tick; a
// change observed meanwhile is kept as Changed, so a change and a return
// before the finish still restarts the count.
func (d *Dispatcher) pruneStall(obs *Observation) {
	if len(d.ledger.Stall) == 0 {
		return
	}
	byID := make(map[string]*Ticket, len(obs.Tickets))
	for i := range obs.Tickets {
		byID[obs.Tickets[i].ID] = &obs.Tickets[i]
	}
	running := make(map[string]bool, len(d.ledger.Workers))
	for _, w := range d.ledger.Workers {
		running[w.Key] = true
	}
	for key, s := range d.ledger.Stall {
		t := byID[key]
		switch {
		case running[key]:
			if t == nil || t.Status != s.Status {
				s.Changed = true
			}
		case t == nil || !stallStatuses[t.Status]:
			delete(d.ledger.Stall, key)
		case t.Status != s.Status:
			s.Status, s.Sessions = t.Status, 0
		}
	}
	if len(d.ledger.Stall) == 0 {
		d.ledger.Stall = nil
	}
}

func (l *Ledger) validateStall() error {
	if l.Stall != nil && len(l.Stall) == 0 || len(l.Stall) > maxStallTickets {
		return errors.New("invalid stall counts")
	}
	for key, s := range l.Stall {
		if _, err := wire.ParseTicketID("stall key", key); err != nil || s == nil || !stallStatuses[s.Status] || s.Sessions < 0 || s.Sessions > maxStallSessions {
			return errors.New("invalid stall count")
		}
	}
	return nil
}

// StallView is one `dispatch status` stall row.
type StallView struct {
	Ticket   string
	Status   string
	Sessions int
}

// maxStallView caps the rows `dispatch status` renders.
const maxStallView = 64

// StallRows lists counted tickets with at least one session, most sessions
// first, then by ticket, capped at maxStallView; the second result is the
// number of rows left out.
func (l *Ledger) StallRows() ([]StallView, int) {
	rows := []StallView{}
	for key, s := range l.Stall {
		if s.Sessions > 0 {
			rows = append(rows, StallView{Ticket: key, Status: s.Status, Sessions: s.Sessions})
		}
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Sessions != rows[j].Sessions {
			return rows[i].Sessions > rows[j].Sessions
		}
		return rows[i].Ticket < rows[j].Ticket
	})
	if len(rows) > maxStallView {
		return rows[:maxStallView], len(rows) - maxStallView
	}
	return rows, 0
}
