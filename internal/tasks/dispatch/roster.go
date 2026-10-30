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

// ProgressSignalHandoffEvidence is the native policy's ATR-V0-080
// HANDOFF_EVIDENCE progress signal as the dispatcher observes it.
const ProgressSignalHandoffEvidence = "HANDOFF_EVIDENCE"

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
	// RequiresPool is the pool a claim of the ticket must name, or empty.
	RequiresPool                  string
	ProgressToken, ProgressDigest string
	// Gates is the native ERG-V0-009 external review state by gate ID. A
	// workState program cannot supply it; absent means no record.
	Gates map[string]GateView
	// GatesObserved is set only by a native observation that read every
	// gate of the ticket. While false every gate is UNKNOWN, so no gate
	// predicate (NONE included) matches an unobserved ticket.
	GatesObserved bool
	// EscalationPending is the native ESC-V0-006 derived hold: the sorted
	// request IDs of current OPEN decision, scope or blocked questions. A
	// workState program cannot supply it. A held ticket is never rostered.
	EscalationPending []string
	// Loop is the native CAL-V0-102 LOOP_DETECTED hold, nil when none. A
	// workState program cannot supply it. A held ticket is never rostered.
	Loop *LoopHold
	// AcceptanceRevision is the native acceptance revision that scopes an
	// ESC-V0-007 infrastructure retry episode.
	AcceptanceRevision string
	// Infrastructure lists the holders (dispatcher worker IDs) of current
	// OPEN or ANSWERED infrastructure requests at AcceptanceRevision, so an
	// ended session is classified by its own typed request (ESC-V0-008).
	Infrastructure []string
	// OpenRequests lists the ticket's current OPEN escalation requests of
	// every kind, with the original OPEN time, sorted by request ID, so
	// status can show kinds and ages (ESC-V0-009). A workState program
	// cannot supply it.
	OpenRequests []OpenRequest
	// EscalationUnknown is set when the ticket's escalation material could
	// not be validated: its revision binding and infrastructure observation
	// are UNKNOWN, never progress. A workState program cannot clear it.
	EscalationUnknown bool
	// NextStage is the advisory CAL-V0-084 recorded hand-off target:
	// implement, review, integrate, STALE, or NONE when none is observed.
	// It comes from the native queue, never from a workState program.
	NextStage string
	// OperatorNote is the ticket's ON-V0-011 note as read at observation,
	// nil when the ticket was never noted. Only a native observation sets
	// it; a workState program cannot supply it.
	OperatorNote *NoteView
	// Obligations is the ticket's TOL-V0-017 obligation ledger view as
	// read at observation, nil when the ticket carries no ledger. Only a
	// native observation sets it; a workState program cannot supply it.
	Obligations *ObligationsView
}

// PlanReasonPoolStalled is the plan deferral of a ticket whose pool has
// every member QUARANTINED with a stalled recovery (PSR-V0-267); it equals
// wire.CodePoolMemberRecoveryStalled.
const PlanReasonPoolStalled = "POOL_MEMBER_RECOVERY_STALLED"

// PlanReasonMirrorsDecided is the plan deferral of a ticket whose pending
// obligation items are all mirrors decided on their owner tickets
// (TOL-V0-123); it equals transaction.PlanReasonMirrorsDecided.
const PlanReasonMirrorsDecided = "MIRRORS_DECIDED_ELSEWHERE"

// PlanReasonWorkStateHeld equals transaction.PlanReasonWorkStateHeld
// (CAL-V0-105).
const PlanReasonWorkStateHeld = "WORK_STATE_HELD"

// PlanReasonNoRoleMatch is the DSG-V0-001 deferral of a ticket whose known
// work state no enabled role admits and no hold declares. It is derived
// dispatcher output only, never a wire code, a stored value or a refusal.
const PlanReasonNoRoleMatch = "NO_ROLE_MATCH"

// ObligationsView is the TOL-V0-017 summary of a ticket's obligation
// ledger reference: its counts and its monotone high-water mark.
type ObligationsView struct {
	Witnessed, Total, CoreWitnessed, CoreTotal int64
	// HighWaterRevision and HighWater are the reference's highWater
	// acceptance revision and witnessed count.
	HighWaterRevision string
	HighWater         int64
	// Unblocks is the folded count of entries that left BLOCKED
	// (TOL-V0-124); Unknown marks a ledger the observation could not fold,
	// which leaves the ticket's work state UNKNOWN for progress and parking.
	Unblocks int64
	Unknown  bool
}

// LoopHold is a CAL-V0-102 hold as the dispatcher records it: the signal,
// the acceptance revision it is bound to and the counted generations,
// oldest first. The ticket, signal, acceptance revision and newest
// generation name the episode.
type LoopHold struct {
	Signal             string   `json:"signal"`
	AcceptanceRevision string   `json:"acceptanceRevision"`
	Generations        []string `json:"generations"`
	// Pending marks, in the ledger only, an episode whose needs-owner event
	// is not yet appended to the event log, so the next tick and a restart
	// retry it (CAL-V0-103).
	Pending bool `json:"pending,omitempty"`
}

// Attempt is the dispatcher's view of one attempt.
type Attempt struct {
	ID, Ticket, Phase, Stage, Generation, Holder, Candidate string
	// Cause is a terminal attempt's native cause (LEASE_EXPIRED after a
	// reap); empty otherwise.
	Cause          string
	LeaseExpires   time.Time
	Live           bool
	Gates, Reviews int

	// Pool and Member name the attempt's pool allocation; empty without one.
	Pool, Member string
	// HandoffEvidence is the attempt's latest recorded hand-off --evidence
	// reference, carried across generations that record none, the
	// HANDOFF_EVIDENCE progress fingerprint (ATR-V0-084); empty without one.
	HandoffEvidence string

	// Branch and Worktree are the attempt's recorded branch and supervised
	// worktree path, empty without them; a salvaged note names the commit
	// they hold (ERG-V0-028).
	Branch, Worktree string
}

// Member is one native pool member.
type Member struct {
	Pool, Member, State, Holder, Attempt string
	Queue, Allocation, Definition        string
	SafeReuse, Owned                     bool
	// Changed is the native receipt sequence of the member's last state
	// change; with State it identifies one state episode (CAL-V0-129).
	Changed string
	// Age is how long the dispatcher has observed this state episode; it
	// is zero for an episode first observed now.
	Age time.Duration
}

// Observation is one authoritative read of the native store.
type Observation struct {
	Tickets  []Ticket
	Attempts []Attempt
	Members  []Member
	// ProgressSignal is the policy's declared ATR-V0-080 progress signal,
	// empty when none is declared.
	ProgressSignal string
	// Replan, when set, replans the same snapshot with the tickets the
	// work state holds (CAL-V0-105) and those a budget holds (CAL-V0-155)
	// outside the selection window, and returns each planned ticket's plan
	// state and reason by ticket ID.
	Replan func(held, budgetHeld map[string]bool) map[string]PlanView
	// TicketWorktree, when set, returns the existing worktree the policy's
	// `worktrees` branch of a ticket local id has checked out, or "". It
	// never creates one (ATR-V0-341).
	TicketWorktree func(local string) string
	// PoolFree is the count of FREE members of each declared pool (no
	// state entry occupies them), or nil when the observation did not read
	// pools; a nil map applies no PSR-V0-140 pool-room gate.
	PoolFree map[string]int
	// PoolFreeMembers names, sorted, the FREE members PoolFree counts; the
	// ATR-V0-150 queue observation reports them.
	PoolFreeMembers map[string][]string
	// PoolStalled marks each declared pool whose every member is
	// QUARANTINED with a stalled recovery (PSR-V0-265); its held launches
	// report POOL_MEMBER_RECOVERY_STALLED instead of POOL_ROOM.
	PoolStalled map[string]bool
}

// PlanView is one ticket's plan state and reason.
type PlanView struct{ State, Reason string }

// workStateHeld names the tickets whose observed work state is a hold
// (CAL-V0-105): with a work-state reader configured, a ticket whose state is
// a known value (not NONE or UNKNOWN) that no ticket role's state predicate
// admits. Nothing is held without a reader or without a ticket role, and an
// UNKNOWN or NONE state is never held, so those tickets keep today's window.
//
// unmatched is the DSG-V0-001 subset whose state is no deliberate hold: not
// a declared workState holdStates value and not in an enabled role's
// excludeStates. Those tickets still leave the window, as NO_ROLE_MATCH.
func workStateHeld(c *Config, ts []Ticket) (held, unmatched map[string]bool) {
	if c.WorkState == nil {
		return nil, nil
	}
	for _, t := range ts {
		if t.State == "" || t.State == StateNone || t.State == StateUnknown {
			continue
		}
		admitted, roles, excluded := false, 0, false
		for _, r := range c.Roles {
			if r.Match == nil || r.Cap == 0 {
				continue // a disabled role admits nothing (CAL-V0-128)
			}
			roles++
			if stateMatches(r.Match, t.State) {
				admitted = true
				break
			}
			excluded = excluded || contains(r.Match.ExcludeStates, t.State)
		}
		if roles > 0 && !admitted {
			if held == nil {
				held = map[string]bool{}
			}
			held[t.ID] = true
			if !excluded && !contains(c.WorkState.HoldStates, t.State) {
				if unmatched == nil {
					unmatched = map[string]bool{}
				}
				unmatched[t.ID] = true
			}
		}
	}
	return held, unmatched
}

// stateMatches is a role's work-state predicate alone (CAL-V0-054).
func stateMatches(m *Match, state string) bool {
	if len(m.States)+len(m.ExcludeStates) > 0 && state == StateUnknown {
		return false
	}
	if len(m.States) > 0 && !contains(m.States, state) {
		return false
	}
	return !contains(m.ExcludeStates, state)
}

// Queue is the native store boundary. Release and Reap go through the
// existing fenced lease transactions; the dispatcher never writes the store
// any other way.
type Queue interface {
	Observe(ctx context.Context) (*Observation, error)
	Release(ctx context.Context, a Attempt, evidence, requestID string) error
	Reap(ctx context.Context, a Attempt, requestID string) error
	// ReapExpired reaps a while its lease still expires at a.LeaseExpires
	// (CAL-V0-191). reaped is true only when this request moved the attempt,
	// now or in the replayed original; an attempt that is no longer live
	// gives false and no error.
	ReapExpired(ctx context.Context, a Attempt, requestID string) (reaped bool, err error)
}

// Assignment is one roster decision: a role slot bound to one work key at
// one CAL-V0-057 escalation tier (0 is the base model).
type Assignment struct {
	Role, Key, Ticket, Local, State, Pool, Member string
	NextStage                                     string
	Slot, Tier                                    int
	// OperatorNote is the ticket's observed note, rendered only through the
	// {operatorNote} role-prompt placeholder (ON-V0-011).
	OperatorNote *NoteView
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
	out, _ := roster(c, obs, busy, skip, nil, nil, nil, nil, nil, nil)
	return out
}

// RosterTiers is Roster with each candidate's escalation tier from the pure
// tierOf (nil means every tier is 0). A candidate whose tier is at its tier
// cap waits; it never falls back to a lower tier.
func RosterTiers(c *Config, obs *Observation, busy []Busy, skip map[string]bool, tierOf func(role, key string) int) []Assignment {
	out, _ := roster(c, obs, busy, skip, tierOf, nil, nil, nil, nil, nil)
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
	return roster(c, obs, busy, skip, nil, budget, nil, nil, nil, nil)
}

// roster is the shared pure roster behind Roster, RosterTiers and
// RosterWithPressure. The CAL-V0-057 tier cap and the CAL-V0-155 budgets are
// static fences, so they are checked before the CAL-V0-068 pressure budget
// is charged.
//
// prefer names, by ticket, the role a CAL-V0-149 relaunch must use: while
// that role is enabled, a ticket role and matches the ticket, no other role
// is a candidate for it.
//
// room, when not nil, is the PSR-V0-140 pool-room gate: a static fence that
// holds a ticket whose pool has no room and charges each launch.
//
// adm, when not nil, carries the ATR-V0-152 calm-pressure input that lets a
// capMax role's effective cap rise, and receives the ATR-V0-150 queue
// observation: each ticket role's effective cap and waiting tickets, and the
// first admission or focus fence that kept each run-only candidate out. A
// role's run-only bypass (ATR-V0-151) applies with or without adm.
func roster(c *Config, obs *Observation, busy []Busy, skip map[string]bool, tierOf func(role, key string) int, budget *PressureBudget, prefer map[string]string, spend *spendGate, room *poolRoom, adm *admission) (out, held []Assignment) {
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
	preferred := map[string]bool{}
	for _, t := range obs.Tickets {
		if name, ok := prefer[t.ID]; ok {
			preferred[t.ID] = c.relaunchRole(name, t) != nil
		}
	}
	var cands []candidate
	for ri, r := range c.Roles {
		if r.Cap == 0 {
			continue // CAL-V0-128: a disabled role plans nothing
		}
		if r.Lane != nil {
			states := r.Lane.States
			if len(states) == 0 {
				states = []string{"QUARANTINED"}
			}
			for _, m := range obs.Members {
				if m.Pool == r.Lane.Pool && contains(states, m.State) && m.Age >= time.Duration(r.Lane.MinAgeSeconds)*time.Second {
					cands = append(cands, candidate{a: Assignment{Role: r.Name, Key: laneKey(m.Pool, m.Member), Pool: m.Pool, Member: m.Member}, pin: len(c.Pinned), role: r.Priority, prio: len(priorityRank), ord: ri})
				}
			}
			continue
		}
		for _, t := range obs.Tickets {
			if _, held := live[t.ID]; held {
				continue // a lease, an expired lease awaiting reap, or another supervisor holds it
			}
			if len(t.EscalationPending) > 0 {
				continue // ESC-V0-006: a session cannot claim it until the questions are answered
			}
			if t.Loop != nil {
				continue // CAL-V0-102: a session cannot claim it until the owner reopens it
			}
			if t.PlanReason == PlanReasonMirrorsDecided {
				continue // TOL-V0-123: its open items are decided elsewhere; the owner completes it
			}
			if preferred[t.ID] && prefer[t.ID] != r.Name {
				continue
			}
			if !matches(r.Match, t) {
				// PSR-V0-267: a ticket the plan defers only because every
				// member of its pool is stalled reaches the pool room as a
				// hold, so its pool-wait names the stall; it never launches.
				if t.PlanReason == PlanReasonPoolStalled && outsideFocusOnly(r.Match, t) {
					room.hold(Assignment{Role: r.Name, Key: t.ID, Ticket: t.ID, Local: t.Local, State: t.State, Pool: t.RequiresPool})
					continue
				}
				// ATR-V0-151: a run-only ticket the plan window alone keeps out
				// is a candidate while its pool has a free member and the role
				// opts in; otherwise the plan window is its recorded fence.
				if !r.RunOnly || !outsideFocusOnly(r.Match, t) {
					continue
				}
				if !r.BypassFocus || obs.PoolFree[r.Match.Pool] < 1 {
					adm.note(c, Assignment{Role: r.Name, Key: t.ID, Ticket: t.ID, Local: t.Local, State: t.State, Pool: t.RequiresPool}, FencePlanWindow, t.PlanReason)
					continue
				}
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
			cands = append(cands, candidate{a: Assignment{Role: r.Name, Key: t.ID, Ticket: t.ID, Local: t.Local, State: t.State, NextStage: t.NextStage, Pool: t.RequiresPool, OperatorNote: t.OperatorNote}, pin: pin, role: r.Priority, prio: prio, ord: ri, order: t.Order})
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
	pending := make([]Assignment, len(cands))
	for i, cd := range cands {
		pending[i] = cd.a
	}
	depth := adm.depths(busy, pending, skip)
	for _, r := range c.Roles {
		caps[r.Name] = adm.effectiveCap(r, depth[r.Name])
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
		a := cd.a
		if taken[a.Key] || skip[a.Key] {
			continue
		}
		if total >= c.GlobalCap {
			// Every later candidate meets the same fence; the loop goes on
			// only to record it (ATR-V0-150).
			adm.note(c, a, FenceGlobalCap, "")
			continue
		}
		if count[a.Role] >= caps[a.Role] {
			adm.note(c, a, FenceRoleCap, "")
			continue
		}
		if tierOf != nil {
			a.Tier = tierOf(a.Role, a.Key)
		}
		tk := tierKey(a.Role, a.Tier)
		if n, ok := tierCaps[tk]; ok && tierCount[tk] >= n {
			adm.note(c, a, FenceTierCap, "")
			continue
		}
		// CAL-V0-155: a budget is a static fence too; its hold is reported
		// as a budget event, never as pressure.
		if spend != nil && !spend.admits(a) {
			adm.note(c, a, FenceBudget, "")
			continue
		}
		// PSR-V0-140: no session launches for a pool with no room.
		if !room.admits(a) {
			continue
		}
		if budget != nil && !budget.Accept(a) {
			adm.note(c, a, FencePressure, "")
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
		if spend != nil {
			spend.charge(a)
		}
		room.charge(a)
		out = append(out, a)
	}
	adm.settle(pending, taken, skip, caps)
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

// relaunchRole is the role named for t's pending CAL-V0-149 relaunch when
// that role would serve t, else nil. A budget deferral (CAL-V0-155) keeps
// the restriction: such a ticket is matched as if it were selected.
func (c *Config) relaunchRole(name string, t Ticket) *Role {
	if t.PlanReason == PlanReasonBudgetHeld {
		t.Plan = "SELECTED"
	}
	r := c.roleNamed(name)
	if r == nil || r.Cap == 0 || r.Lane != nil || r.Match == nil {
		return nil
	}
	// ATR-V0-151: a bypassing run-only role serves an unselected ticket too,
	// so the relaunch keeps its role there as well.
	if !matches(r.Match, t) && (!r.BypassFocus || !outsideFocusOnly(r.Match, t)) {
		return nil
	}
	return r
}

// outsideFocusOnly reports a ticket that m would match but for its
// planSelected focus: the native plan has not selected it.
func outsideFocusOnly(m *Match, t Ticket) bool {
	if !m.PlanSelected || t.Plan == "SELECTED" {
		return false
	}
	unfocused := *m
	unfocused.PlanSelected = false
	return matches(&unfocused, t)
}

func matches(m *Match, t Ticket) bool {
	if t.RequiresPool != m.Pool {
		return false // a worker claiming without the ticket's pool is refused (CAL-V0-097)
	}
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
	if !stateMatches(m, t.State) {
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
		if t.ID == key && t.Obligations != nil {
			// TOL-V0-017: ledger, note and attachment writes move the
			// revision, so a ledger ticket fingerprints its acceptance
			// revision and its monotone high-water mark instead.
			fmt.Fprintf(h, "%s|%s|%s\n", t.Status, t.AcceptanceRevision, t.State)
			fmt.Fprintf(h, "obligations|%s|%d\n", t.Obligations.HighWaterRevision, t.Obligations.HighWater)
			// TOL-V0-124: an entry leaving BLOCKED is a state change, so a
			// ticket parked while its items were blocked is released on the
			// next observation. A ledger that never unblocked hashes as before.
			if n := t.Obligations.Unblocks; n > 0 {
				fmt.Fprintf(h, "obligations-unblocked|%d\n", n)
			}
			gateFingerprint(h, t)
		} else if t.ID == key {
			fmt.Fprintf(h, "%s|%s|%s\n", t.Status, t.Revision, t.State)
			gateFingerprint(h, t)
		}
	}
	var rows []string
	for _, a := range obs.Attempts {
		if a.Ticket == key && (a.Candidate != "" || a.Gates > 0 || a.Reviews > 0 || durablePhases[a.Phase]) {
			rows = append(rows, fmt.Sprintf("%s|%s|%s|%d|%d", a.ID, a.Phase, a.Candidate, a.Gates, a.Reviews))
		}
		// ATR-V0-084: under a declared HANDOFF_EVIDENCE signal the attempt's
		// latest recorded hand-off reference is progress identity, so only a
		// hand-off with a changed reference is progress; the same one, or a
		// generation that records none, is not. A policy without the signal
		// hashes exactly as before.
		if a.Ticket == key && a.HandoffEvidence != "" && obs.ProgressSignal == ProgressSignalHandoffEvidence {
			rows = append(rows, "handoffEvidence|"+a.ID+"|"+a.HandoffEvidence)
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
	// Candidate, Subject, Trust and EvidenceSha256 complete the ERG-V0-009
	// field set. Each is read from the head event, so the progress
	// fingerprint, which already carries Head, is unchanged. EvidenceSha256
	// is the head event's digest; all four are empty without a head.
	Candidate      GateCandidate
	Subject        GateSubject
	Trust          GateTrust
	EvidenceSha256 string
}

// GateCandidate is the reviewed candidate: Kind TREE with TreeOID, or Kind
// EVIDENCE with Sha256 and Bytes.
type GateCandidate struct{ Kind, TreeOID, Sha256, Bytes string }

// GateSubject is the author's reviewed submission.
type GateSubject struct {
	AttemptID, Generation, ReceiptSeq, ReceiptSha256, AttemptSha256 string
}

// GateTrust is the head event's trust: Source LEASE_BOUND or
// OPERATOR_ATTESTED; actor authentication and independence are always
// NOT_OBSERVED (ERG-V0-001).
type GateTrust struct{ ActorAuthentication, Independence, Source string }

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
	case !t.GatesObserved:
		return StateUnknown
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
