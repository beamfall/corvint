package dispatch

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Work states that are not program-defined.
const (
	StateNone    = "NONE"
	StateUnknown = "UNKNOWN"
)

// Ticket is the dispatcher's read-only view of one native ticket.
type Ticket struct {
	ID, Local, Status, Priority, Kind, Revision string
	Order                                       uint64
	Labels                                      []string
	Plan, PlanReason                            string
	State                                       string
	ProgressToken, ProgressDigest               string
	// Gates is the native ERG-V0-009 external review state by gate ID. A
	// workState program cannot supply it; absent means no record.
	Gates map[string]GateView
}

// Attempt is the dispatcher's view of one attempt.
type Attempt struct {
	ID, Ticket, Phase, Stage, Generation, Holder, Candidate string
	LeaseExpires                                            time.Time
	Live                                                    bool
	Gates, Reviews                                          int
}

// Member is one native pool member.
type Member struct {
	Pool, Member, State, Holder, Attempt string
	Queue, Allocation, Definition        string
	SafeReuse, Owned                     bool
}

// Observation is one authoritative read of the native store.
type Observation struct {
	Tickets  []Ticket
	Attempts []Attempt
	Members  []Member
}

// Queue is the native store boundary. Release and Reap go through the
// existing fenced lease transactions; the dispatcher never writes the store
// any other way.
type Queue interface {
	Observe(ctx context.Context) (*Observation, error)
	Release(ctx context.Context, a Attempt, evidence, requestID string) error
	Reap(ctx context.Context, a Attempt, requestID string) error
}

// Assignment is one roster decision: a role slot bound to one work key at
// one CAL-V0-057 escalation tier (0 is the base model).
type Assignment struct {
	Role, Key, Ticket, Local, State, Pool, Member string
	Slot, Tier                                    int
}

// Busy is a running worker's claim on a role slot, a work key and a tier.
type Busy struct {
	Role, Key  string
	Slot, Tier int
}

func laneKey(pool, member string) string { return "lane:" + pool + "/" + member }

var priorityRank = map[string]int{"P0": 0, "P1": 1, "P2": 2, "P3": 3}

// liveAttempt returns the newest live attempt of each ticket.
func liveAttempts(obs *Observation) map[string]Attempt {
	out := map[string]Attempt{}
	for _, a := range obs.Attempts {
		if a.Live {
			if old, ok := out[a.Ticket]; !ok || a.ID > old.ID {
				out[a.Ticket] = a
			}
		}
	}
	return out
}

// Roster is the CAL-V0-054 pure roster: the same configuration,
// observation, running set and skip set always produce the same assignments.
func Roster(c *Config, obs *Observation, busy []Busy, skip map[string]bool) []Assignment {
	out, _ := roster(c, obs, busy, skip, nil, nil)
	return out
}

// RosterTiers is Roster with each candidate's escalation tier from the pure
// tierOf (nil means every tier is 0). A candidate whose tier is at its tier
// cap waits; it never falls back to a lower tier.
func RosterTiers(c *Config, obs *Observation, busy []Busy, skip map[string]bool, tierOf func(role, key string) int) []Assignment {
	out, _ := roster(c, obs, busy, skip, tierOf, nil)
	return out
}

// RosterWithPressure is Roster with an optional CAL-V0-068 pressure budget.
// The budget is consulted only after every static fence admits a candidate
// and before the candidate consumes a slot or reserves its key, so a held
// candidate reserves nothing. held lists each held work key once, in roster
// order, while the role, tier and global caps (charged with launches and
// earlier holds) would have admitted it, unless a later exempt candidate launched the
// same key.
func RosterWithPressure(c *Config, obs *Observation, busy []Busy, skip map[string]bool, budget *PressureBudget) (out, held []Assignment) {
	return roster(c, obs, busy, skip, nil, budget)
}

// roster is the shared pure roster behind Roster, RosterTiers and
// RosterWithPressure. The CAL-V0-057 tier cap is a static fence, so it is
// checked before the CAL-V0-068 pressure budget is charged.
func roster(c *Config, obs *Observation, busy []Busy, skip map[string]bool, tierOf func(role, key string) int, budget *PressureBudget) (out, held []Assignment) {
	type candidate struct {
		a                    Assignment
		pin, role, prio, ord int
		order                uint64
	}
	pinned := map[string]int{}
	for i, p := range c.Pinned {
		pinned[p] = i
	}
	live := liveAttempts(obs)
	var cands []candidate
	for ri, r := range c.Roles {
		if r.Lane != nil {
			states := r.Lane.States
			if len(states) == 0 {
				states = []string{"QUARANTINED"}
			}
			for _, m := range obs.Members {
				if m.Pool == r.Lane.Pool && contains(states, m.State) {
					cands = append(cands, candidate{a: Assignment{Role: r.Name, Key: laneKey(m.Pool, m.Member), Pool: m.Pool, Member: m.Member}, pin: len(c.Pinned), role: r.Priority, prio: len(priorityRank), ord: ri})
				}
			}
			continue
		}
		for _, t := range obs.Tickets {
			if _, held := live[t.ID]; held {
				continue // a lease, an expired lease awaiting reap, or another supervisor holds it
			}
			if !matches(r.Match, t) {
				continue
			}
			pin, ok := pinned[t.ID]
			if !ok {
				if pin, ok = pinned[t.Local]; !ok {
					pin = len(c.Pinned)
				}
			}
			prio, ok := priorityRank[t.Priority]
			if !ok {
				prio = len(priorityRank)
			}
			cands = append(cands, candidate{a: Assignment{Role: r.Name, Key: t.ID, Ticket: t.ID, Local: t.Local, State: t.State}, pin: pin, role: r.Priority, prio: prio, ord: ri, order: t.Order})
		}
	}
	sort.SliceStable(cands, func(i, j int) bool {
		x, y := cands[i], cands[j]
		switch {
		case x.pin != y.pin:
			return x.pin < y.pin
		case x.role != y.role:
			return x.role < y.role
		case x.prio != y.prio:
			return x.prio < y.prio
		case x.order != y.order:
			return x.order < y.order
		case x.a.Key != y.a.Key:
			return x.a.Key < y.a.Key
		}
		return x.ord < y.ord
	})
	caps, tierCaps := map[string]int{}, map[string]int{}
	tierKey := func(role string, tier int) string { return fmt.Sprintf("%s\x00%d", role, tier) }
	for _, r := range c.Roles {
		caps[r.Name] = r.Cap
		for i, t := range r.Escalate {
			if t.Cap > 0 {
				tierCaps[tierKey(r.Name, i+1)] = t.Cap
			}
		}
	}
	taken, slots, count, tierCount := map[string]bool{}, map[string]map[int]bool{}, map[string]int{}, map[string]int{}
	total := 0
	for _, b := range busy {
		taken[b.Key] = true
		if slots[b.Role] == nil {
			slots[b.Role] = map[int]bool{}
		}
		slots[b.Role][b.Slot] = true
		count[b.Role]++
		tierCount[tierKey(b.Role, b.Tier)]++
		total++
	}
	heldKeys, heldCount, heldTierCount, heldTotal := map[string]bool{}, map[string]int{}, map[string]int{}, 0
	for _, cd := range cands {
		if total >= c.GlobalCap {
			break
		}
		a := cd.a
		if taken[a.Key] || skip[a.Key] || count[a.Role] >= caps[a.Role] {
			continue
		}
		if tierOf != nil {
			a.Tier = tierOf(a.Role, a.Key)
		}
		tk := tierKey(a.Role, a.Tier)
		if n, ok := tierCaps[tk]; ok && tierCount[tk] >= n {
			continue
		}
		if budget != nil && !budget.Accept(a) {
			// Report a hold only while the static role, tier and global
			// caps, charged with earlier launches and holds, would still
			// admit the candidate. The shadow counters never affect
			// admission.
			n, capped := tierCaps[tk]
			if !heldKeys[a.Key] && count[a.Role]+heldCount[a.Role] < caps[a.Role] && total+heldTotal < c.GlobalCap && (!capped || tierCount[tk]+heldTierCount[tk] < n) {
				heldKeys[a.Key] = true
				heldCount[a.Role]++
				heldTierCount[tk]++
				heldTotal++
				held = append(held, a)
			}
			continue
		}
		if slots[a.Role] == nil {
			slots[a.Role] = map[int]bool{}
		}
		for a.Slot = 1; slots[a.Role][a.Slot]; a.Slot++ {
		}
		slots[a.Role][a.Slot] = true
		taken[a.Key] = true
		count[a.Role]++
		tierCount[tierKey(a.Role, a.Tier)]++
		total++
		out = append(out, a)
	}
	if len(held) > 0 {
		kept := held[:0]
		for _, h := range held {
			if !taken[h.Key] {
				kept = append(kept, h)
			}
		}
		held = kept
	}
	return out, held
}

func matches(m *Match, t Ticket) bool {
	statuses := m.Statuses
	if len(statuses) == 0 {
		statuses = []string{"OPEN"}
	}
	if !contains(statuses, t.Status) {
		return false
	}
	for _, l := range m.Labels {
		if !contains(t.Labels, l) {
			return false
		}
	}
	if len(m.Kinds) > 0 && !contains(m.Kinds, t.Kind) {
		return false
	}
	if m.IDGlob != "" {
		if ok, _ := filepath.Match(m.IDGlob, t.Local); !ok {
			return false
		}
	}
	if len(m.States)+len(m.ExcludeStates) > 0 && t.State == StateUnknown {
		return false
	}
	if len(m.States) > 0 && !contains(m.States, t.State) {
		return false
	}
	if contains(m.ExcludeStates, t.State) {
		return false
	}
	return (!m.PlanSelected || t.Plan == "SELECTED") && gatesMatch(m.Gates, t)
}

// durablePhases are attempt phases that record work beyond an empty claim.
var durablePhases = map[string]bool{"BUILT": true, "CHECKING": true, "REVIEWING": true, "REPAIRING": true, "READY_FOR_INTEGRATION": true, "COMPLETED": true}

// Fingerprint is the CAL-V0-057 progress identity of one work key: ticket
// status, revision and work state plus every attempt that carries durable
// work, with an optional CAL-V0-064 digest from checked progress admission.
// An empty claim followed by a handoff leaves it unchanged.
func Fingerprint(obs *Observation, key string) string {
	base := baseFingerprint(obs, key)
	for _, t := range obs.Tickets {
		if t.ID == key && t.ProgressDigest != "" {
			return progressFingerprint(base, t.ProgressDigest)
		}
	}
	return base
}

func progressFingerprint(base, digest string) string {
	sum := sha256.Sum256([]byte("dispatch-progress-v1\x00" + base + "\x00" + digest))
	return hex.EncodeToString(sum[:])
}

func baseFingerprint(obs *Observation, key string) string {
	h := sha256.New()
	if strings.HasPrefix(key, "lane:") {
		for _, m := range obs.Members {
			if laneKey(m.Pool, m.Member) == key {
				fmt.Fprintf(h, "%s|%s|%s\n", m.State, m.Holder, m.Attempt)
			}
		}
		return hex.EncodeToString(h.Sum(nil))
	}
	for _, t := range obs.Tickets {
		if t.ID == key {
			fmt.Fprintf(h, "%s|%s|%s\n", t.Status, t.Revision, t.State)
			gateFingerprint(h, t)
		}
	}
	var rows []string
	for _, a := range obs.Attempts {
		if a.Ticket == key && (a.Candidate != "" || a.Gates > 0 || a.Reviews > 0 || durablePhases[a.Phase]) {
			rows = append(rows, fmt.Sprintf("%s|%s|%s|%d|%d", a.ID, a.Phase, a.Candidate, a.Gates, a.Reviews))
		}
	}
	sort.Strings(rows)
	for _, r := range rows {
		fmt.Fprintln(h, r)
	}
	return hex.EncodeToString(h.Sum(nil))
}

// GateView is one gate of the native ERG-V0-009 external review state, read
// from typed queue events, never from a workState program. Status is CURRENT,
// STALE or UNKNOWN. Verdict is PASS, RETURN, or empty for a current
// resubmission awaiting review.
type GateView struct {
	Verdict, Status      string
	Generation, Revision string
	Resubmitted          bool
	Head                 string
}

// Gate predicate states. NONE is a gate with no record; the others need a
// CURRENT head, so a STALE or UNKNOWN gate matches no predicate.
const (
	GatePass        = "PASS"
	GateReturn      = "RETURN"
	GateResubmitted = "RESUBMITTED"
	GateNone        = "NONE"
)

// GateState is the routing state of one gate: NONE, PASS, RETURN or
// RESUBMITTED, or the non-CURRENT status (STALE or UNKNOWN) otherwise.
func (t Ticket) GateState(gate string) string {
	v, ok := t.Gates[gate]
	switch {
	case !ok:
		return GateNone
	case v.Status != "CURRENT":
		if v.Status == "STALE" {
			return v.Status
		}
		return StateUnknown
	case v.Verdict == GatePass || v.Verdict == GateReturn:
		return v.Verdict
	case v.Verdict == "" && v.Resubmitted:
		return GateResubmitted
	}
	return StateUnknown
}

func gatesMatch(gates []GateMatch, t Ticket) bool {
	for _, g := range gates {
		if !contains(g.States, t.GateState(g.Gate)) {
			return false
		}
	}
	return true
}

// gateFingerprint adds the native gate heads to a ticket's progress identity:
// a new verdict or resubmission is progress. A ticket without gates hashes
// exactly as before.
func gateFingerprint(h io.Writer, t Ticket) {
	ids := make([]string, 0, len(t.Gates))
	for id := range t.Gates {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		v := t.Gates[id]
		fmt.Fprintf(h, "gate|%s|%s|%s|%s|%s|%t|%s\n", id, v.Status, v.Verdict, v.Generation, v.Revision, v.Resubmitted, v.Head)
	}
}
