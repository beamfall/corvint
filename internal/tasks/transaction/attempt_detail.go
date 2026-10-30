package transaction

import (
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// HolderStatus classifies an attempt's recorded heartbeat at now against the
// effective policy TTL (CAL-V0-120). It describes a recorded signal, never
// physical quiescence, and is derived on every read and never written.
func HolderStatus(a *snapshot.Attempt, now time.Time, ttlSeconds int64) string {
	if a.LastHeartbeatAt == nil {
		return "NOT_OBSERVED"
	}
	at, _ := time.Parse(time.RFC3339, string(*a.LastHeartbeatAt))
	switch {
	case !a.Live():
		return "TERMINAL"
	case a.Lease != nil && now.UTC().Format(time.RFC3339) >= string(a.Lease.ExpiresAt):
		return "LEASE_EXPIRED"
	case now.Before(at):
		return "CLOCK_BEFORE_HEARTBEAT"
	case now.Sub(at) >= time.Duration(ttlSeconds)*time.Second:
		return "STALE_HOLDER"
	default:
		return "FRESH_HOLDER"
	}
}

// AttemptSummary is the ATR-V0-190 description of one live attempt holding a
// reservation entry: who holds it, how its holder reads now, and the one
// command that frees it.
type AttemptSummary struct {
	AttemptID, TicketID, Generation, Phase string
	// Holder and LeaseExpiresAt are empty when the attempt has no work lease
	// (a supervised attempt); HolderStatus and Recovery are always set.
	Holder, LeaseExpiresAt, HolderStatus, Recovery string
}

// SummarizeAttempt describes the live attempt of a reservation entry at now.
// A nil attempt (its record was not loaded) keeps the entry's identity and
// reports the rest NOT_OBSERVED rather than guessing.
func SummarizeAttempt(en snapshot.ReservationEntry, a *snapshot.Attempt, now wire.Timestamp, ttlSeconds int64) AttemptSummary {
	s := AttemptSummary{AttemptID: en.AttemptID, TicketID: en.TicketID.Raw, Generation: string(en.Generation), Phase: "NOT_OBSERVED", HolderStatus: "NOT_OBSERVED"}
	if a == nil {
		s.Recovery = "this writer did not read its record; `corvint-tasks ticket show " + s.TicketID + "` names its holder and recovery"
		return s
	}
	at, _ := time.Parse(timestampLayout, string(now))
	s.Phase, s.HolderStatus = a.Phase, HolderStatus(a, at, ttlSeconds)
	if a.Lease != nil {
		s.Holder, s.LeaseExpiresAt = a.Lease.Holder, string(a.Lease.ExpiresAt)
	}
	release := "`corvint-tasks release --attempt " + en.AttemptID + " --generation " + s.Generation + " --request-id NEW_ID`"
	switch {
	case expired(a, now):
		s.Recovery = "the lease expired; reap it: `corvint-tasks reap --attempt " + en.AttemptID + " --generation " + s.Generation + " --lease-expires-at " + s.LeaseExpiresAt + " --request-id NEW_ID`"
	case a.Supervision != nil:
		// A supervised attempt's liveness is its program's, not a work
		// lease's, so nothing reaps it (ATR-V0-281, CAL-V0-121).
		s.Recovery = "it is supervised by program " + a.Supervision.ProgramID + ", whose liveness replaces a work lease, so nothing reaps it; it ends with its program, or its holder releases it: " + release
	case a.Lease == nil:
		s.Recovery = "it has no work lease to reap; its holder releases it: " + release
	default:
		s.Recovery = "the lease is live until " + s.LeaseExpiresAt + "; wait for it, or its holder releases it: " + release
	}
	if s.HolderStatus == "STALE_HOLDER" {
		// STALE_HOLDER is advisory and never release authority (CAL-V0-121,
		// ATR-V0-280): the refusal says so and names the same command.
		s.Recovery = "its heartbeat is stale, which is advisory and frees nothing; " + s.Recovery
	}
	return s
}

// Detail renders the summary for a refusal detail.
func (s AttemptSummary) Detail() string {
	holder, lease := s.Holder, s.LeaseExpiresAt
	if holder == "" {
		holder = "none"
	}
	if lease == "" {
		lease = "null"
	}
	return "attempt " + s.AttemptID + " (ticket " + s.TicketID + ", generation " + s.Generation + ", phase " + s.Phase + ", holder " + holder + ", leaseExpiresAt " + lease + ", holderStatus " + s.HolderStatus + "); recovery: " + s.Recovery
}

// Value renders the summary as `ticket show` liveAttempt (ATR-V0-191).
func (s AttemptSummary) Value() wire.Value {
	str := func(v string) wire.Value {
		if v == "" {
			return wire.Null()
		}
		return wire.String(v)
	}
	o := wire.NewObject().Set("attemptId", wire.String(s.AttemptID)).Set("generation", wire.String(s.Generation)).Set("phase", wire.String(s.Phase))
	o.Set("holder", str(s.Holder)).Set("leaseExpiresAt", str(s.LeaseExpiresAt)).Set("holderStatus", wire.String(s.HolderStatus)).Set("recovery", wire.String(s.Recovery))
	return wire.ObjectValue(o)
}

// CurrentAttempt summarizes the ticket's live attempt from a plan input, or
// returns nil when the reservation set holds none for it.
func CurrentAttempt(in PlanInput, ticketID string) *AttemptSummary {
	if in.Reservations == nil {
		return nil
	}
	for _, en := range in.Reservations.Entries {
		if en.TicketID.Raw == ticketID {
			s := SummarizeAttempt(en, in.Attempts[en.AttemptID], in.Now, in.Policy.HeartbeatTTLSeconds())
			return &s
		}
	}
	return nil
}

// collisionDetail names the colliding attempt, its ticket, holder status and
// the colliding keys of rs (ATR-V0-192).
func (c leaseContext) collisionDetail(attemptID string, rs []ticket.Resource) string {
	for _, en := range c.st.reservations.Entries {
		if en.AttemptID != attemptID {
			continue
		}
		keys := []string{}
		for _, x := range rs {
			if ticket.Collide([]ticket.Resource{x}, en.Resources) {
				keys = append(keys, x.Class+":"+x.Key)
			}
		}
		s := SummarizeAttempt(en, c.st.attempts[en.AttemptID], c.in.RecordedAt, c.st.policy.HeartbeatTTLSeconds())
		return s.Detail() + "; colliding keys: " + strings.Join(keys, ",")
	}
	return "attempt " + attemptID
}

// describeLive is the mutation path's ATR-V0-190 describer: it decodes the
// attempt records only when a refusal needs them. A writer route that
// supplies no attempt records gets the entry's identity only; the CLI then
// adds the full summary from a read (cli mutate).
func describeLive(in Input, st inputState) func(snapshot.ReservationEntry) string {
	return func(en snapshot.ReservationEntry) string {
		attempts := st.attempts
		if attempts == nil && st.head != nil {
			attempts, _ = loadAttempts(in, st.reservations)
		}
		s := SummarizeAttempt(en, attempts[en.AttemptID], in.RecordedAt, st.policy.HeartbeatTTLSeconds())
		return s.Detail()
	}
}
