package dispatch

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
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

// Assignment is one roster decision: a role slot bound to one work key.
type Assignment struct {
	Role, Key, Ticket, Local, State, Pool, Member string
	Slot                                          int
}

// Busy is a running worker's claim on a role slot and a work key.
type Busy struct {
	Role, Key string
	Slot      int
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
	caps := map[string]int{}
	for _, r := range c.Roles {
		caps[r.Name] = r.Cap
	}
	taken, slots, count := map[string]bool{}, map[string]map[int]bool{}, map[string]int{}
	total := 0
	for _, b := range busy {
		taken[b.Key] = true
		if slots[b.Role] == nil {
			slots[b.Role] = map[int]bool{}
		}
		slots[b.Role][b.Slot] = true
		count[b.Role]++
		total++
	}
	var out []Assignment
	for _, cd := range cands {
		if total >= c.GlobalCap {
			break
		}
		a := cd.a
		if taken[a.Key] || skip[a.Key] || count[a.Role] >= caps[a.Role] {
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
		total++
		out = append(out, a)
	}
	return out
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
	return !m.PlanSelected || t.Plan == "SELECTED"
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
