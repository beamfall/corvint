// Package worktrail is the lane commit-trailer grammar of TAW-V0-002: a lane
// reports a ticket transition as Git trailers on one commit, and the store
// host reads them back from an integration ref. It parses and formats only;
// it runs no Git and touches no store.
package worktrail

import (
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// Trailer keys (TAW-V0-002).
const (
	KeyTicket = "Corvint-Ticket"
	KeyState  = "Corvint-Ticket-State"
	KeyReason = "Corvint-Ticket-Reason"
	KeyLane   = "Corvint-Ticket-Lane"
)

// States a lane may report. One commit carries one state.
const (
	StateStarted   = "started"
	StateDelivered = "delivered"
	StateBlocked   = "blocked"
)

// MaxReasonBytes bounds Corvint-Ticket-Reason.
const MaxReasonBytes = 512

// Refusal tags carried in the message of a closed wire code (TAW-V0-003).
const (
	TagTicketID       = "TAW_TICKET_ID"
	TagState          = "TAW_STATE"
	TagReasonRequired = "TAW_REASON_REQUIRED"
	TagReason         = "TAW_REASON"
	TagLane           = "TAW_LANE"
	TagMalformed      = "TAW_TRAILER_MALFORMED"
)

// Transition is one commit's trailer block.
type Transition struct {
	Tickets []string
	State   string
	Reason  string
	Lane    string
}

// ValidState reports whether s is a lane state.
func ValidState(s string) bool {
	return s == StateStarted || s == StateDelivered || s == StateBlocked
}

// ValidTicket checks a local ticket ID token.
func ValidTicket(id string) error {
	if _, err := wire.ParseToken("ticket", id, wire.MaxLocalTokenBytes); err != nil {
		return wire.Errorf(wire.CodeMalformed, "ticket", "%s: %q is not a ticket local ID", TagTicketID, id)
	}
	return nil
}

func validLine(where, tag, s string) error {
	if len(s) > MaxReasonBytes {
		return wire.Errorf(wire.CodeMalformed, where, "%s: %s is %d bytes; the limit is %d", tag, where, len(s), MaxReasonBytes)
	}
	for _, r := range s {
		if r < 0x20 || r == 0x7f {
			return wire.Errorf(wire.CodeMalformed, where, "%s: %s must be one line with no control characters", tag, where)
		}
	}
	return nil
}

// Validate checks a transition against TAW-V0-002.
func (t Transition) Validate() error {
	if len(t.Tickets) == 0 {
		return wire.Errorf(wire.CodeMalformed, "ticket", "%s: at least one %s is required", TagTicketID, KeyTicket)
	}
	seen := map[string]bool{}
	for _, id := range t.Tickets {
		if err := ValidTicket(id); err != nil {
			return err
		}
		if seen[id] {
			return wire.Errorf(wire.CodeMalformed, "ticket", "%s: %s is named twice", TagTicketID, id)
		}
		seen[id] = true
	}
	if !ValidState(t.State) {
		return wire.Errorf(wire.CodeMalformed, "state", "%s: state %q is not one of started, delivered, blocked", TagState, t.State)
	}
	if t.State == StateBlocked && strings.TrimSpace(t.Reason) == "" {
		return wire.Errorf(wire.CodeMalformed, "reason", "%s: a blocked transition needs %s", TagReasonRequired, KeyReason)
	}
	if t.Reason != strings.TrimSpace(t.Reason) {
		return wire.Errorf(wire.CodeMalformed, "reason", "%s: reason has leading or trailing space", TagReason)
	}
	if err := validLine("reason", TagReason, t.Reason); err != nil {
		return err
	}
	if t.Lane != "" {
		if _, err := wire.ParseLabel("lane", t.Lane); err != nil {
			return wire.Errorf(wire.CodeMalformed, "lane", "%s: lane %q is not a label", TagLane, t.Lane)
		}
	}
	return nil
}

// Lines formats the trailer lines in the fixed order: tickets, state,
// reason, lane.
func (t Transition) Lines() []string {
	var out []string
	for _, id := range t.Tickets {
		out = append(out, KeyTicket+": "+id)
	}
	out = append(out, KeyState+": "+t.State)
	if t.Reason != "" {
		out = append(out, KeyReason+": "+t.Reason)
	}
	if t.Lane != "" {
		out = append(out, KeyLane+": "+t.Lane)
	}
	return out
}

// GitArgs are the `git commit` arguments that add the trailers.
func (t Transition) GitArgs() []string {
	var out []string
	for _, l := range t.Lines() {
		out = append(out, "--trailer", l)
	}
	return out
}

// Parse reads the transition from trailer lines as `git log
// --format=%(trailers:only,unfold)` prints them. Lines with other keys are
// ignored. ok is false when no Corvint-Ticket key is present: the commit
// reports no transition. A block that names tickets but is otherwise
// malformed returns an error.
func Parse(lines []string) (t Transition, ok bool, err error) {
	states := 0
	reasons := 0
	lanes := 0
	for _, raw := range lines {
		line := strings.TrimRight(raw, "\r")
		key, val, found := strings.Cut(line, ":")
		if !found {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		switch {
		case strings.EqualFold(key, KeyTicket):
			ok = true
			t.Tickets = append(t.Tickets, val)
		case strings.EqualFold(key, KeyState):
			states++
			t.State = val
		case strings.EqualFold(key, KeyReason):
			reasons++
			t.Reason = val
		case strings.EqualFold(key, KeyLane):
			lanes++
			t.Lane = val
		}
	}
	if !ok {
		if states+reasons+lanes > 0 {
			return t, true, wire.Errorf(wire.CodeMalformed, "trailers", "%s: %s without %s", TagMalformed, KeyState, KeyTicket)
		}
		return t, false, nil
	}
	if states != 1 {
		return t, true, wire.Errorf(wire.CodeMalformed, "trailers", "%s: a commit carries exactly one %s, found %d", TagMalformed, KeyState, states)
	}
	if reasons > 1 || lanes > 1 {
		return t, true, wire.Errorf(wire.CodeMalformed, "trailers", "%s: %s and %s appear at most once", TagMalformed, KeyReason, KeyLane)
	}
	if err := t.Validate(); err != nil {
		return t, true, err
	}
	return t, true, nil
}

// String is the trailer block, one line per trailer.
func (t Transition) String() string { return strings.Join(t.Lines(), "\n") }
