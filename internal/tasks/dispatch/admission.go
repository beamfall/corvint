package dispatch

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// The admission and focus fences the ATR-V0-150 queue observation names for
// a run-only candidate that did not launch. A backoff, a live attempt and
// the PSR-V0-140 pool room are not admission fences and are never named.
const (
	FencePlanWindow = "PLAN_WINDOW"
	FenceGlobalCap  = "GLOBAL_CAP"
	FenceRoleCap    = "ROLE_CAP"
	FenceTierCap    = "TIER_CAP"
	FenceBudget     = "BUDGET"
	FencePressure   = "PRESSURE"
)

// Pressure labels of the queue observation (ATR-V0-150).
const (
	AdmissionPressureCalm          = "CALM"
	AdmissionPressureRaised        = "RAISED"
	AdmissionPressureUnknown       = "UNKNOWN"
	AdmissionPressureNotConfigured = "NOT_CONFIGURED"
)

// AdmissionProfile is the ATR-V0-150 queue observation the dispatcher
// rewrites at each tick as admission.json in its program directory. It is
// derived and advisory: no dispatcher decision reads it back, and the
// doctor reads it only to report.
const AdmissionProfile = "taskman-dispatch-admission/0"

// AdmissionFile is the observation's name in the program directory.
const AdmissionFile = "admission.json"

// Bounds of the queue observation.
const (
	MaxAdmissionBytes      = 1 << 20
	maxAdmissionHeld       = 256
	maxAdmissionFreeNames  = 64
	maxAdmissionDetailText = 200
	maxAdmissionUnmatched  = 64
	maxAdmissionDeferrals  = 32
)

// AdmissionRecord is one tick's ATR-V0-150 queue observation.
type AdmissionRecord struct {
	Profile     string    `json:"profile"`
	Program     string    `json:"program"`
	At          time.Time `json:"at"`
	TickSeconds int       `json:"tickSeconds"`
	// Pressure is CALM, RAISED, UNKNOWN or NOT_CONFIGURED at this tick.
	Pressure string `json:"pressure"`
	// LaunchesHeld is set when a CAL-V0-273 work-state hold or an
	// unavailable pressure budget launched nothing this tick: the roster
	// did not run, so every streak restarted.
	LaunchesHeld bool `json:"launchesHeld"`
	GlobalCap    int  `json:"globalCap"`
	// Running counts the sessions running after this tick's launches.
	Running int          `json:"running"`
	Roles   []RoleQueue  `json:"roles"`
	Held    []HeldRunner `json:"held"`
	// HeldTruncated is set when more run-only holds existed than the
	// record keeps; the kept ones are the longest held.
	HeldTruncated bool `json:"heldTruncated"`
	// UnmatchedStates (DSG-V0-002) counts, by state, the tickets this tick
	// deferred NO_ROLE_MATCH, in state order; absent when none.
	UnmatchedStates []UnmatchedState `json:"unmatchedStates,omitempty"`
	// IdleTicks (DSG-V0-003) counts the consecutive ticks, ending at this
	// one, that launched nothing while running stayed under the global cap
	// and an open ticket was planned but not running; IdleSince is the
	// first and IdleDeferred counts those tickets by plan reason.
	IdleTicks    int             `json:"idleTicks,omitempty"`
	IdleSince    *time.Time      `json:"idleSince,omitempty"`
	IdleDeferred []DeferralCount `json:"idleDeferred,omitempty"`
	// IdleAlertSeconds is the DSG-V0-004 threshold in force for the
	// streak, 0 when the alert is off; doctor reports against it.
	IdleAlertSeconds int `json:"idleAlertSeconds,omitempty"`
}

// UnmatchedState is one DSG-V0-002 work state no role admits and no hold
// declares: its ticket count and the least ticket ID as an example.
type UnmatchedState struct {
	State   string `json:"state"`
	Tickets int    `json:"tickets"`
	Example string `json:"example"`
}

// DeferralCount is one DSG-V0-003 plan reason and its idle ticket count.
type DeferralCount struct {
	Reason  string `json:"reason"`
	Tickets int    `json:"tickets"`
}

// RoleQueue is one enabled ticket role's queue at the tick.
type RoleQueue struct {
	Role         string `json:"role"`
	Cap          int    `json:"cap"`
	CapMax       int    `json:"capMax,omitempty"`
	EffectiveCap int    `json:"effectiveCap"`
	Running      int    `json:"running"`
	// Waiting counts the tickets the role matches that are not running,
	// not launched this tick and not in a backoff; a ticket several roles
	// match counts in each.
	Waiting int `json:"waiting"`
	// DeepTicks counts the consecutive ticks, ending at this one, at which
	// pressure was CALM and running plus waiting exceeded the effective
	// cap; DeepSince is the first of them.
	DeepTicks int        `json:"deepTicks"`
	DeepSince *time.Time `json:"deepSince,omitempty"`
	// OldestWaitTicket waits since OldestWaitSince, the first tick of this
	// dispatcher run that observed it waiting (a lower bound).
	OldestWaitTicket string     `json:"oldestWaitTicket,omitempty"`
	OldestWaitSince  *time.Time `json:"oldestWaitSince,omitempty"`
}

// HeldRunner is one run-only ticket kept out by an admission or focus
// fence while its pool had a free member.
type HeldRunner struct {
	Role   string `json:"role"`
	Ticket string `json:"ticket"`
	Pool   string `json:"pool"`
	Fence  string `json:"fence"`
	// Detail is the plan reason of a PLAN_WINDOW fence, else empty.
	Detail      string   `json:"detail,omitempty"`
	Free        int      `json:"free"`
	FreeMembers []string `json:"freeMembers"`
	// Ticks counts the consecutive ticks, ending at this one, that held
	// the ticket while the pool had a free member; Since is the first.
	Ticks int       `json:"ticks"`
	Since time.Time `json:"since"`
}

// admission is roster's per-tick ATR-V0-150..152 exchange: calm is an
// input, the rest roster fills.
type admission struct {
	calm    bool
	holds   map[string]admissionHold
	eff     map[string]int
	waiting map[string][]string
	// planned is this tick's roster launches; the observation counts only
	// those the launch loop then recorded as workers.
	planned []Assignment
}

type admissionHold struct {
	a             Assignment
	fence, detail string
}

// note records the first fence that kept a run-only role's ticket out.
func (adm *admission) note(c *Config, a Assignment, fence, detail string) {
	if adm == nil || a.Ticket == "" {
		return
	}
	if r := c.roleNamed(a.Role); r == nil || !r.RunOnly {
		return
	}
	if adm.holds == nil {
		adm.holds = map[string]admissionHold{}
	}
	k := a.Role + "\x00" + a.Key
	if _, ok := adm.holds[k]; !ok {
		adm.holds[k] = admissionHold{a: a, fence: fence, detail: detail}
	}
}

// depths is each role's running sessions plus its waiting candidates
// before this tick's launches; nil without adm.
func (adm *admission) depths(busy []Busy, pending []Assignment, skip map[string]bool) map[string]int {
	if adm == nil {
		return nil
	}
	out, running := map[string]int{}, map[string]bool{}
	for _, b := range busy {
		out[b.Role]++
		running[b.Key] = true
	}
	for _, a := range pending {
		if a.Ticket != "" && !running[a.Key] && !skip[a.Key] {
			out[a.Role]++
		}
	}
	return out
}

// effectiveCap is the ATR-V0-152 cap: between cap and capMax, at most the
// role's depth, while pressure is calm and the queue is deeper than cap;
// cap otherwise, and always without adm.
func (adm *admission) effectiveCap(r Role, depth int) int {
	if adm == nil || !adm.calm || r.CapMax == 0 || r.Cap == 0 || r.Lane != nil || depth <= r.Cap {
		return r.Cap
	}
	return min(r.CapMax, depth)
}

// settle records the queues after the roster: the effective caps, the
// planned launches, the waiting tickets, and only the holds of tickets
// that neither run nor launched nor wait out a backoff.
func (adm *admission) settle(pending []Assignment, taken, skip map[string]bool, caps map[string]int) {
	if adm == nil {
		return
	}
	adm.eff, adm.waiting = map[string]int{}, map[string][]string{}
	for k, v := range caps {
		adm.eff[k] = v
	}
	for _, a := range pending {
		if a.Ticket == "" || skip[a.Key] {
			continue
		}
		if taken[a.Key] {
			adm.planned = append(adm.planned, a)
		} else {
			adm.waiting[a.Role] = append(adm.waiting[a.Role], a.Ticket)
		}
	}
	for k, h := range adm.holds {
		if taken[h.a.Key] || skip[h.a.Key] {
			delete(adm.holds, k)
		}
	}
}

// streak is a run of consecutive ticks.
type streak struct {
	since time.Time
	ticks int
}

func (s streak) next(now time.Time) streak {
	if s.ticks == 0 {
		return streak{since: now, ticks: 1}
	}
	return streak{since: s.since, ticks: s.ticks + 1}
}

// admissionTracker holds the streaks the observation reports, in memory
// only: a restarted dispatcher starts every streak again.
type admissionTracker struct {
	deep  map[string]streak
	waits map[string]map[string]time.Time
	held  map[string]streak
}

// pressureLabel is the ATR-V0-150 pressure label of the dispatcher's
// current pressure state; only CALM lets capMax scale (ATR-V0-152).
func (d *Dispatcher) pressureLabel() string {
	if d.Config.Pressure == nil || d.ledger.Pressure == nil {
		return AdmissionPressureNotConfigured
	}
	st := d.ledger.Pressure.State
	switch {
	case st.Unknown:
		return AdmissionPressureUnknown
	case st.Level == 0 && st.PendingLevel == 0:
		return AdmissionPressureCalm
	}
	return AdmissionPressureRaised
}

// recordAdmission updates the streaks from this tick's roster exchange
// (nil when the roster did not run) and rewrites the observation. A write
// failure alerts once until a write succeeds again; it never stops a tick.
func (d *Dispatcher) recordAdmission(obs *Observation, adm *admission, pressure string, now time.Time) {
	now = now.UTC()
	t := &d.admission
	rec := AdmissionRecord{Profile: AdmissionProfile, Program: d.Program, At: now, TickSeconds: d.Config.TickSeconds, Pressure: pressure, GlobalCap: d.Config.GlobalCap, Roles: []RoleQueue{}, Held: []HeldRunner{}}
	if adm == nil {
		*t = admissionTracker{}
		rec.LaunchesHeld = true
		rec.Running = len(d.ledger.Workers)
		d.idleRun = streak{}
	} else {
		d.trackAdmission(obs, adm, pressure, now, &rec)
		d.trackIdle(obs, adm, now, &rec)
	}
	rec.UnmatchedStates = unmatchedStates(obs, d.unmatched)
	raw, err := json.Marshal(rec)
	// The bound holds for any input: drop the shortest held streaks until
	// the observation fits, and say so.
	for err == nil && len(raw)+1 > MaxAdmissionBytes && len(rec.Held) > 0 {
		rec.Held, rec.HeldTruncated = rec.Held[:len(rec.Held)/2], true
		raw, err = json.Marshal(rec)
	}
	if err == nil && len(raw)+1 > MaxAdmissionBytes {
		err = fmt.Errorf("queue observation exceeds %d bytes", MaxAdmissionBytes)
	}
	if err == nil {
		err = writeAtomic(filepath.Join(d.dir, AdmissionFile), append(raw, '\n'))
	}
	if err != nil && !d.admissionFailed {
		d.emit(Event{Kind: "alert", Message: "queue observation not written: " + err.Error()})
	}
	d.admissionFailed = err != nil
}

func (d *Dispatcher) trackAdmission(obs *Observation, adm *admission, pressure string, now time.Time, rec *AdmissionRecord) {
	t := &d.admission
	deep, waits, held := map[string]streak{}, map[string]map[string]time.Time{}, map[string]streak{}
	// After the launch loop: running is the recorded workers, and a planned
	// launch that control admission, a pool claim or the spawn refused is
	// still waiting.
	running, live := map[string]int{}, map[string]bool{}
	for _, w := range d.ledger.Workers {
		running[w.Role]++
		live[w.Key] = true
	}
	rec.Running = len(d.ledger.Workers)
	waiting := map[string][]string{}
	for r, ids := range adm.waiting {
		waiting[r] = append(waiting[r], ids...)
	}
	for _, a := range adm.planned {
		if !live[a.Key] {
			waiting[a.Role] = append(waiting[a.Role], a.Ticket)
		}
	}
	for _, r := range d.Config.Roles {
		if r.Match == nil || r.Cap == 0 {
			continue
		}
		q := RoleQueue{Role: r.Name, Cap: r.Cap, CapMax: r.CapMax, EffectiveCap: adm.eff[r.Name], Running: running[r.Name], Waiting: len(waiting[r.Name])}
		if pressure == AdmissionPressureCalm && q.Running+q.Waiting > q.EffectiveCap {
			deep[r.Name] = t.deep[r.Name].next(now)
			since := deep[r.Name].since
			q.DeepTicks, q.DeepSince = deep[r.Name].ticks, &since
		}
		waits[r.Name] = map[string]time.Time{}
		for _, id := range waiting[r.Name] {
			since, ok := t.waits[r.Name][id]
			if !ok {
				since = now
			}
			waits[r.Name][id] = since
			if q.OldestWaitSince == nil || since.Before(*q.OldestWaitSince) || since.Equal(*q.OldestWaitSince) && id < q.OldestWaitTicket {
				s := since
				q.OldestWaitTicket, q.OldestWaitSince = id, &s
			}
		}
		rec.Roles = append(rec.Roles, q)
	}
	for k, h := range adm.holds {
		free := obs.PoolFree[h.a.Pool]
		if free < 1 {
			continue // ATR-V0-150: only a hold beside a free member counts
		}
		held[k] = t.held[k].next(now)
		names := obs.PoolFreeMembers[h.a.Pool]
		if len(names) > maxAdmissionFreeNames {
			names = names[:maxAdmissionFreeNames]
		}
		detail := h.detail
		if len(detail) > maxAdmissionDetailText {
			detail = detail[:maxAdmissionDetailText]
		}
		rec.Held = append(rec.Held, HeldRunner{Role: h.a.Role, Ticket: h.a.Ticket, Pool: h.a.Pool, Fence: h.fence, Detail: detail, Free: free, FreeMembers: append([]string{}, names...), Ticks: held[k].ticks, Since: held[k].since})
	}
	sort.Slice(rec.Held, func(i, j int) bool {
		x, y := rec.Held[i], rec.Held[j]
		if x.Ticks != y.Ticks {
			return x.Ticks > y.Ticks
		}
		if x.Ticket != y.Ticket {
			return x.Ticket < y.Ticket
		}
		return x.Role < y.Role
	})
	if len(rec.Held) > maxAdmissionHeld {
		rec.Held, rec.HeldTruncated = rec.Held[:maxAdmissionHeld], true
	}
	*t = admissionTracker{deep: deep, waits: waits, held: held}
}

// unmatchedStates is the DSG-V0-002 per-state count of this tick's
// NO_ROLE_MATCH tickets, bounded to the first states in order.
func unmatchedStates(obs *Observation, unmatched map[string]bool) []UnmatchedState {
	if obs == nil || len(unmatched) == 0 {
		return nil
	}
	by := map[string]*UnmatchedState{}
	for _, t := range obs.Tickets {
		if !unmatched[t.ID] || t.PlanReason != PlanReasonNoRoleMatch {
			continue
		}
		u := by[t.State]
		if u == nil {
			u = &UnmatchedState{State: t.State, Example: t.ID}
			by[t.State] = u
		}
		u.Tickets++
		if t.ID < u.Example {
			u.Example = t.ID
		}
	}
	out := make([]UnmatchedState, 0, len(by))
	for _, u := range by {
		out = append(out, *u)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].State < out[j].State })
	if len(out) > maxAdmissionUnmatched {
		out = out[:maxAdmissionUnmatched]
	}
	return out
}

// trackIdle updates the DSG-V0-003 idle-with-capacity streak after the
// launch loop and raises the DSG-V0-004 alert once per threshold of a
// streak.
func (d *Dispatcher) trackIdle(obs *Observation, adm *admission, now time.Time, rec *AdmissionRecord) {
	// A launch is a worker the previous tick's ledger did not hold, so a
	// worker that keeps running is not one and a lane launch is.
	running, seen := map[string]bool{}, make(map[string]bool, len(d.ledger.Workers))
	launched := false
	for _, w := range d.ledger.Workers {
		running[w.Ticket] = true
		seen[w.ID] = true
		launched = launched || !d.idleWorkers[w.ID]
	}
	d.idleWorkers = seen
	reasons := map[string]int{}
	for _, t := range obs.Tickets {
		if running[t.ID] || t.Plan != "SELECTED" && t.Plan != "DEFERRED" {
			continue
		}
		reason := t.PlanReason
		if reason == "" {
			reason = t.Plan
		}
		reasons[reason]++
	}
	if launched || rec.Running >= d.Config.GlobalCap || len(reasons) == 0 {
		d.idleRun = streak{}
		return
	}
	d.idleRun = d.idleRun.next(now)
	since := d.idleRun.since
	rec.IdleTicks, rec.IdleSince = d.idleRun.ticks, &since
	for r, n := range reasons {
		rec.IdleDeferred = append(rec.IdleDeferred, DeferralCount{Reason: r, Tickets: n})
	}
	sort.Slice(rec.IdleDeferred, func(i, j int) bool {
		x, y := rec.IdleDeferred[i], rec.IdleDeferred[j]
		if x.Tickets != y.Tickets {
			return x.Tickets > y.Tickets
		}
		return x.Reason < y.Reason
	})
	if len(rec.IdleDeferred) > maxAdmissionDeferrals {
		rec.IdleDeferred = rec.IdleDeferred[:maxAdmissionDeferrals]
	}
	after := d.Config.IdleAlertAfter()
	rec.IdleAlertSeconds = int(after / time.Second)
	if after <= 0 || now.Sub(since) < after || !d.idleAlerted.Before(since) && now.Sub(d.idleAlerted) < after {
		return
	}
	d.idleAlerted = now
	parts := make([]string, 0, len(rec.IdleDeferred))
	for _, c := range rec.IdleDeferred {
		parts = append(parts, c.Reason+"="+strconv.Itoa(c.Tickets))
	}
	d.emit(Event{Kind: "alert", Message: fmt.Sprintf("IDLE_WITH_CAPACITY: no launch for %s with %d of %d sessions running; deferred tickets: %s", now.Sub(since).Round(time.Second), rec.Running, d.Config.GlobalCap, strings.Join(parts, ", ")),
		Detail: map[string]string{"code": "IDLE_WITH_CAPACITY", "idleSince": since.Format(time.RFC3339), "idleTicks": strconv.Itoa(d.idleRun.ticks), "running": strconv.Itoa(rec.Running), "globalCap": strconv.Itoa(d.Config.GlobalCap)}})
}

// DecodeAdmission strictly reads one queue observation of program: another
// profile is UNSUPPORTED_VERSION, and an unknown member, another program,
// a value outside its bounds or trailing data is MALFORMED.
func DecodeAdmission(raw []byte, program string) (*AdmissionRecord, error) {
	if len(raw) > MaxAdmissionBytes {
		return nil, wire.Errorf(wire.CodeMalformed, "", "queue observation exceeds %d bytes", MaxAdmissionBytes)
	}
	var head struct {
		Profile string `json:"profile"`
	}
	if err := json.Unmarshal(raw, &head); err != nil {
		return nil, wire.Errorf(wire.CodeMalformed, "", "queue observation is not a JSON object")
	}
	if head.Profile != AdmissionProfile {
		return nil, wire.Errorf(wire.CodeUnsupportedVersion, "/profile", "queue observation profile %q is not %s", head.Profile, AdmissionProfile)
	}
	var rec AdmissionRecord
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&rec); err != nil {
		return nil, wire.Errorf(wire.CodeMalformed, "", "queue observation: %v", err)
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, wire.Errorf(wire.CodeMalformed, "", "queue observation has trailing data")
	}
	if err := rec.validate(program); err != nil {
		return nil, wire.Errorf(wire.CodeMalformed, "", "queue observation: %v", err)
	}
	return &rec, nil
}

func (r *AdmissionRecord) validate(program string) error {
	switch {
	case r.Program != program:
		return fmt.Errorf("program %q is not %q", r.Program, program)
	case r.At.IsZero() || r.TickSeconds < 1 || r.TickSeconds > 3600 || r.GlobalCap < 1 || r.GlobalCap > 64 || r.Running < 0:
		return fmt.Errorf("time, tick, global cap or running out of bounds")
	case r.Pressure != AdmissionPressureCalm && r.Pressure != AdmissionPressureRaised && r.Pressure != AdmissionPressureUnknown && r.Pressure != AdmissionPressureNotConfigured:
		return fmt.Errorf("unknown pressure %q", r.Pressure)
	case len(r.Roles) > 32 || len(r.Held) > maxAdmissionHeld:
		return fmt.Errorf("too many roles or holds")
	case len(r.UnmatchedStates) > maxAdmissionUnmatched || len(r.IdleDeferred) > maxAdmissionDeferrals || r.IdleTicks < 0 || (r.IdleTicks > 0) != (r.IdleSince != nil) || r.IdleAlertSeconds < 0 || r.IdleAlertSeconds > maxIdleAlertMinutes*60 || r.IdleAlertSeconds > 0 && r.IdleTicks == 0:
		return fmt.Errorf("unmatched states or idle streak out of bounds")
	}
	for _, u := range r.UnmatchedStates {
		if u.State == "" || len(u.State) > 512 || u.Tickets < 1 || u.Example == "" || len(u.Example) > 512 {
			return fmt.Errorf("invalid unmatched state %q", u.State)
		}
	}
	for _, c := range r.IdleDeferred {
		if c.Reason == "" || len(c.Reason) > 128 || c.Tickets < 1 {
			return fmt.Errorf("invalid idle deferral %q", c.Reason)
		}
	}
	for _, q := range r.Roles {
		if !ValidName(q.Role) || q.Cap < 1 || q.Cap > 64 || q.CapMax < 0 || q.CapMax > 64 || q.EffectiveCap < 0 || q.EffectiveCap > 64 || q.Running < 0 || q.Waiting < 0 || q.DeepTicks < 0 || (q.DeepTicks > 0) != (q.DeepSince != nil) || (q.OldestWaitTicket != "") != (q.OldestWaitSince != nil) || len(q.OldestWaitTicket) > 512 {
			return fmt.Errorf("invalid role queue %q", q.Role)
		}
	}
	for _, h := range r.Held {
		if !ValidName(h.Role) || h.Ticket == "" || len(h.Ticket) > 512 || h.Pool == "" || len(h.Pool) > 512 || len(h.Detail) > maxAdmissionDetailText || h.Free < 1 || len(h.FreeMembers) > maxAdmissionFreeNames || h.Ticks < 1 || h.Since.IsZero() {
			return fmt.Errorf("invalid run-only hold of %q", h.Ticket)
		}
		switch h.Fence {
		case FencePlanWindow, FenceGlobalCap, FenceRoleCap, FenceTierCap, FenceBudget, FencePressure:
		default:
			return fmt.Errorf("unknown fence %q", h.Fence)
		}
	}
	return nil
}
