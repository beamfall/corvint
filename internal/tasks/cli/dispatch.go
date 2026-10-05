package cli

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/dispatch"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// dispatchCommand routes `dispatch`, `dispatch status` and `dispatch unpark`
// (CAL-V0-052).
func dispatchCommand(env Env, args []string) *wire.Result {
	if len(args) > 0 && (args[0] == "status" || args[0] == "unpark") {
		return dispatchAux(env, args[0], args[1:])
	}
	cmd := []string{"dispatch"}
	values, err := dispatchFlags(args, map[string]bool{"--program": true, "--config": true, "--ticks": true}, map[string]bool{"--once": true})
	if err != "" {
		return usage(cmd, err)
	}
	ticks := 0
	if _, once := values["--once"]; once {
		if values["--ticks"] != "" {
			return usage(cmd, "--once and --ticks are exclusive")
		}
		ticks = 1
	} else if values["--ticks"] != "" {
		n, e := strconv.Atoi(values["--ticks"])
		if e != nil || n < 1 || n > 1000000 {
			return usage(cmd, "--ticks must be 1..1000000")
		}
		ticks = n
	}
	c, res := dispatchConfig(cmd, values)
	if res != nil {
		return res
	}
	repo, e := intent.Resolve(env.Cwd)
	if e != nil {
		return errorResult(cmd, e)
	}
	// Workers claim through the store at workRoot; heal and reap act on
	// this one. They must be the same store.
	if work, e := intent.Resolve(c.WorkRoot); e != nil || work.StateDir != repo.StateDir {
		return usage(cmd, "workRoot must resolve to the same task store as the dispatcher's working directory")
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	defer cancel()
	d, e := dispatch.Open(values["--program"], c, dispatchQueue{env: env, pools: c.TicketPools()}, env.Stderr)
	if e != nil {
		return dispatchReaderError(cmd, e)
	}
	runErr := d.Run(ctx, ticks)
	closeErr := d.Close()
	o := wire.NewObject()
	o.Set("profile", wire.String("taskman-dispatch-run/0"))
	o.Set("program", wire.String(values["--program"]))
	o.Set("stateDir", wire.String(dispatch.ProgramDir(c, values["--program"])))
	o.Set("workersRunning", wire.String(strconv.Itoa(d.Running())))
	o.Set("lastEventSeq", wire.String(strconv.FormatUint(d.LastEvent(), 10)))
	o.Set("interrupted", wire.Bool(ctx.Err() != nil))
	containment, quarantined, diagnostic := dispatch.ReaderContainment(dispatch.ProgramDir(c, values["--program"]), values["--program"])
	o.Set("readerContainment", wire.String(containment))
	o.Set("readerQuarantined", wire.Bool(quarantined))
	o.Set("readerDiagnostic", wire.String(diagnostic))
	out := &wire.Result{Command: cmd, Outcome: wire.OutcomeOK, Codes: []string{}, Items: []wire.Value{{Kind: wire.KindObject, Obj: o}}}
	for _, e := range []error{runErr, closeErr} {
		if e != nil {
			out.Outcome, out.Codes = wire.OutcomeError, []string{wire.CodeOf(e)}
			if errors.Is(e, dispatch.ErrReaderQuiescence) {
				out.Codes = []string{wire.CodeQuiescenceUnproved}
			}
			out.Warnings = append(out.Warnings, prose(e.Error()))
		}
	}
	return out
}

func dispatchReaderError(cmd []string, err error) *wire.Result {
	if errors.Is(err, dispatch.ErrReaderQuiescence) {
		return errorResult(cmd, wire.Errorf(wire.CodeQuiescenceUnproved, "reader", "%v", err))
	}
	return errorResult(cmd, err)
}

func dispatchFlags(args []string, valued, bare map[string]bool) (map[string]string, string) {
	values := map[string]string{}
	for i := 0; i < len(args); i++ {
		f := args[i]
		if _, dup := values[f]; dup {
			return nil, "repeated dispatch flag " + f
		}
		switch {
		case bare[f]:
			values[f] = ""
		case valued[f] && i+1 < len(args):
			values[f] = args[i+1]
			i++
		default:
			return nil, "unknown or incomplete dispatch flag " + f
		}
	}
	if values["--program"] == "" || values["--config"] == "" {
		return nil, "dispatch requires --program and --config"
	}
	if !dispatch.ValidName(values["--program"]) {
		return nil, "program name must match [a-z][a-z0-9-]{0,23}"
	}
	return values, ""
}

func dispatchConfig(cmd []string, values map[string]string) (*dispatch.Config, *wire.Result) {
	raw, e := intent.ReadFile(values["--config"], dispatch.MaxConfig)
	if e != nil {
		return nil, errorResult(cmd, e)
	}
	c, e := dispatch.DecodeConfig(raw)
	if e != nil {
		return nil, usage(cmd, e.Error())
	}
	return c, nil
}

// dispatchAux is `dispatch status` (a pure read of the dispatcher's own
// files; it never opens the native store) and `dispatch unpark` (a request
// file the running dispatcher consumes on its next tick).
func dispatchAux(env Env, verb string, args []string) *wire.Result {
	cmd := []string{"dispatch", verb}
	valued := map[string]bool{"--program": true, "--config": true, "--events": true}
	if verb == "unpark" {
		valued = map[string]bool{"--program": true, "--config": true, "--key": true}
	}
	values, err := dispatchFlags(args, valued, nil)
	if err != "" {
		return usage(cmd, err)
	}
	c, res := dispatchConfig(cmd, values)
	if res != nil {
		return res
	}
	dir := dispatch.ProgramDir(c, values["--program"])
	if verb == "unpark" {
		key := values["--key"]
		if key == "" || len(key) > 512 {
			return usage(cmd, "unpark requires --key (a ticket ID or lane:POOL/MEMBER)")
		}
		raw, _ := json.Marshal(dispatch.UnparkRequest{Unpark: key})
		sum := sha256.Sum256([]byte(key))
		path := filepath.Join(dir, "requests", hex.EncodeToString(sum[:8])+".json")
		if e := os.MkdirAll(filepath.Dir(path), 0o700); e != nil {
			return errorResult(cmd, e)
		}
		if e := os.WriteFile(path+".tmp", raw, 0o600); e != nil {
			return errorResult(cmd, e)
		}
		if e := os.Rename(path+".tmp", path); e != nil {
			return errorResult(cmd, e)
		}
		o := wire.NewObject()
		o.Set("profile", wire.String("taskman-dispatch-unpark/0"))
		o.Set("key", wire.String(key))
		o.Set("request", wire.String(path))
		return &wire.Result{Command: cmd, Outcome: wire.OutcomeOK, Codes: []string{}, Items: []wire.Value{{Kind: wire.KindObject, Obj: o}}}
	}
	n := 20
	if values["--events"] != "" {
		v, e := strconv.Atoi(values["--events"])
		if e != nil || v < 0 || v > 1000 {
			return usage(cmd, "--events must be 0..1000")
		}
		n = v
	}
	l, e := dispatch.LoadLedger(dir, values["--program"])
	if e != nil {
		return errorResult(cmd, e)
	}
	events, e := dispatch.ReadEvents(dir, n)
	if e != nil {
		return errorResult(cmd, e)
	}
	status := dispatchStatusValue(c, dir, l, events, time.Now())
	// SERVICE500-008: additive and present only when this program's
	// installed user service binds this dispatcher state root.
	if svc, ok := serviceHost().DispatchService(values["--program"], c.StateDir); ok {
		status.Obj.Set("service", wire.ObjectValue(svc))
	}
	return &wire.Result{Command: cmd, Outcome: wire.OutcomeOK, Codes: []string{}, Items: []wire.Value{status}}
}

func dispatchStatusValue(c *dispatch.Config, dir string, l *dispatch.Ledger, events []dispatch.Event, now time.Time) wire.Value {
	str := wire.String
	ts := func(t time.Time) wire.Value { return str(t.UTC().Format(time.RFC3339)) }
	sweeps := []wire.Value{}
	keysSweep := make([]string, 0, len(l.PoolSweeps))
	for key := range l.PoolSweeps {
		keysSweep = append(keysSweep, key)
	}
	sort.Strings(keysSweep)
	for _, key := range keysSweep {
		r := l.PoolSweeps[key]
		o := wire.NewObject().Set("requestId", str(r.RequestID)).Set("pool", str(r.Pool)).Set("member", str(r.Member)).Set("allocation", str(r.Allocation)).Set("phase", str(r.Phase)).Set("receiptSeq", str(r.Result.ReceiptSeq)).Set("evidence", str(r.Result.Evidence)).Set("reason", str(r.Reason))
		sweeps = append(sweeps, wire.ObjectValue(o))
	}
	workers := []wire.Value{}
	for _, w := range l.Workers {
		o := wire.NewObject()
		o.Set("worker", str(w.ID))
		o.Set("role", str(w.Role))
		o.Set("slot", str(strconv.Itoa(w.Slot)))
		o.Set("key", str(w.Key))
		o.Set("pid", str(strconv.Itoa(w.PID)))
		o.Set("processes", str(strconv.Itoa(len(w.Members))))
		o.Set("state", str(w.State))
		o.Set("started", ts(w.Started))
		o.Set("lastActive", ts(w.LastActive))
		if w.Model != "" {
			o.Set("tier", str(strconv.Itoa(w.Tier)))
			o.Set("model", str(w.Model))
		}
		workers = append(workers, wire.Value{Kind: wire.KindObject, Obj: o})
	}
	keys := make([]string, 0, len(l.Backoff))
	for k := range l.Backoff {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parked, cooling := []string{}, []wire.Value{}
	for _, k := range keys {
		b := l.Backoff[k]
		if b.Parked {
			parked = append(parked, k)
		} else if b.CooldownUntil.After(now) {
			o := wire.NewObject()
			o.Set("key", str(k))
			o.Set("until", ts(b.CooldownUntil))
			o.Set("noProgressRuns", str(strconv.Itoa(b.NoProgress)))
			cooling = append(cooling, wire.Value{Kind: wire.KindObject, Obj: o})
		}
	}
	evs := []wire.Value{}
	for _, e := range events {
		o := wire.NewObject()
		o.Set("seq", str(strconv.FormatUint(e.Seq, 10)))
		o.Set("at", str(e.At))
		o.Set("kind", str(e.Kind))
		o.Set("message", str(prose(e.Message)))
		evs = append(evs, wire.Value{Kind: wire.KindObject, Obj: o})
	}
	o := wire.NewObject()
	o.Set("profile", str("taskman-dispatch-status/0"))
	o.Set("program", str(l.Program))
	o.Set("stateDir", str(dir))
	o.Set("dispatcherRunning", str(dispatch.OwnerState(dir)))
	containment, quarantined, diagnostic := dispatch.ReaderContainment(dir, l.Program)
	o.Set("readerContainment", str(containment))
	o.Set("readerQuarantined", wire.Bool(quarantined))
	o.Set("readerDiagnostic", str(diagnostic))
	if len(sweeps) > 0 {
		o.Set("poolSweeps", wire.Array(sweeps...))
	}
	o.Set("workers", wire.Value{Kind: wire.KindArray, Arr: workers})
	o.Set("parked", wire.Strings(parked))
	o.Set("cooling", wire.Value{Kind: wire.KindArray, Arr: cooling})
	if held := dispatchEscalationPending(l); len(held) > 0 {
		o.Set("escalationPending", wire.Value{Kind: wire.KindArray, Arr: held})
	}
	if held := dispatchLoopDetected(l); len(held) > 0 {
		o.Set("loopDetected", wire.Value{Kind: wire.KindArray, Arr: held})
	}
	if retry := dispatchInfraRetry(c, l); len(retry) > 0 {
		o.Set("infrastructureRetry", wire.Value{Kind: wire.KindArray, Arr: retry})
	}
	if c.Escalates() {
		o.Set("escalation", dispatchEscalationValue(c, l))
	}
	if l.Pressure != nil {
		o.Set("pressure", dispatchPressureValue(l.Pressure, c.Pressure))
	}
	o.Set("lastEventSeq", str(strconv.FormatUint(l.EventSeq, 10)))
	o.Set("events", wire.Value{Kind: wire.KindArray, Arr: evs})
	return wire.Value{Kind: wire.KindObject, Obj: o}
}

// dispatchEscalationValue is the CAL-V0-057 ladder view: per ticket, the
// no-progress streak and the tier and model each laddered role launches next.
func dispatchEscalationValue(c *dispatch.Config, l *dispatch.Ledger) wire.Value {
	keys := make([]string, 0, len(l.Escalation))
	for k := range l.Escalation {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := []wire.Value{}
	for _, k := range keys {
		e := l.Escalation[k]
		roles := []wire.Value{}
		for i := range c.Roles {
			r := &c.Roles[i]
			if len(r.Escalate) == 0 {
				continue
			}
			tier := dispatch.LaunchTier(r, e)
			o := wire.NewObject()
			o.Set("role", wire.String(r.Name))
			o.Set("tier", wire.String(strconv.Itoa(tier)))
			o.Set("model", wire.String(r.ModelAt(tier)))
			roles = append(roles, wire.Value{Kind: wire.KindObject, Obj: o})
		}
		o := wire.NewObject()
		o.Set("key", wire.String(k))
		o.Set("streak", wire.String(strconv.Itoa(e.Streak)))
		o.Set("roles", wire.Value{Kind: wire.KindArray, Arr: roles})
		out = append(out, wire.Value{Kind: wire.KindObject, Obj: o})
	}
	return wire.Value{Kind: wire.KindArray, Arr: out}
}

// dispatchPressureValue is the CAL-V0-068 status view: the recorded level
// and dwell, the newest sample's observed inputs (UNKNOWN when unobserved),
// the active non-exempt cap under the given configuration, and held work.
func dispatchPressureValue(r *dispatch.PressureRecord, pc *dispatch.PressureConfig) wire.Value {
	str := wire.String
	num := func(x float64, ok bool) wire.Value {
		if !ok {
			return str(dispatch.StateUnknown)
		}
		return str(strconv.FormatFloat(x, 'f', 3, 64))
	}
	s := r.Sample
	o := wire.NewObject()
	o.Set("level", str(strconv.Itoa(r.State.Level)))
	o.Set("pendingLevel", str(strconv.Itoa(r.State.PendingLevel)))
	o.Set("pendingTicks", str(strconv.Itoa(r.State.PendingTicks)))
	sample := "OBSERVED"
	if r.State.Unknown {
		sample = dispatch.StateUnknown
	}
	o.Set("sample", str(sample))
	sampledAt := dispatch.StateUnknown
	if !s.SampledAt.IsZero() {
		sampledAt = s.SampledAt.UTC().Format(time.RFC3339)
	}
	o.Set("sampledAt", str(sampledAt))
	source := dispatch.StateUnknown
	if s.Source != "" {
		source = prose(s.Source)
	}
	o.Set("source", str(source))
	o.Set("loadAverage", num(s.LoadAverage, s.LoadKnown))
	cpus := dispatch.StateUnknown
	if s.CPUKnown && s.CPUs > 0 {
		cpus = strconv.Itoa(s.CPUs)
	}
	o.Set("cpus", str(cpus))
	o.Set("loadPerCpu", num(s.LoadPerCPU()))
	o.Set("swapFraction", num(s.SwapFraction()))
	swapUsed, swapTotal := dispatch.StateUnknown, dispatch.StateUnknown
	if s.SwapKnown {
		swapUsed, swapTotal = strconv.FormatUint(s.SwapUsedBytes, 10), strconv.FormatUint(s.SwapTotalBytes, 10)
	}
	o.Set("swapUsedBytes", str(swapUsed))
	o.Set("swapTotalBytes", str(swapTotal))
	problems := make([]string, 0, len(s.Problems))
	for _, p := range s.Problems {
		problems = append(problems, prose(p))
	}
	o.Set("problems", wire.Strings(problems))
	limit := "NONE"
	if r.State.Level > 0 {
		limit = dispatch.StateUnknown
		if pc != nil {
			limit = strconv.Itoa(pc.LevelCaps[strconv.Itoa(r.State.Level)])
		}
	}
	o.Set("cap", str(limit))
	held := []wire.Value{}
	for _, h := range r.Held {
		x := wire.NewObject()
		x.Set("role", str(h.Role))
		x.Set("key", str(h.Key))
		held = append(held, wire.Value{Kind: wire.KindObject, Obj: x})
	}
	o.Set("held", wire.Value{Kind: wire.KindArray, Arr: held})
	return wire.Value{Kind: wire.KindObject, Obj: o}
}

// dispatchQueue is the native store boundary of the dispatcher: one pure
// read per observation and the existing fenced release/reap transactions.
// dispatchQueue observes and heals the store for one dispatcher. pools is
// its config's TicketPools; nil plans as plan preview does.
type dispatchQueue struct {
	env   Env
	pools []string
}

func (q dispatchQueue) Observe(ctx context.Context) (*dispatch.Observation, error) {
	obs := &dispatch.Observation{}
	_, err := withStore(q.env, func(rc *readCtx) error {
		in, _, err := planInput(rc)
		if err != nil {
			return err
		}
		in.ClaimablePools = q.pools // pool tickets no role can claim never use the window (CAL-V0-097)
		obs.Tickets = dispatchTickets(in)
		for i := range obs.Tickets {
			r, _ := in.Tickets.Get(obs.Tickets[i].ID)
			observeEscalations(rc.repo, in.Queue.QueueID.Raw, r, &obs.Tickets[i])
		}
		// The review binding fold runs once, and only when a gate exists; a
		// fold that refuses leaves every gate unobserved (ERG-V0-009).
		var fold *transaction.ExternalReviewReceiptAudit
		folded := false
		for i := range obs.Tickets {
			r, _ := in.Tickets.Get(obs.Tickets[i].ID)
			// ERG-V0-009: a gate set that cannot be read stays unobserved
			// (every gate UNKNOWN) instead of failing the whole observation.
			if len(r.ExternalReviews) > 0 && !folded {
				folded = true
				fold, _ = store.FoldExternalReviews(rc.repo, rc.snap.Head.LastSeq.Uint64(), nil)
			}
			if len(r.ExternalReviews) == 0 || fold != nil {
				if gates, err := externalReviewGateViews(rc.repo, r, in.Policy, in.Attempts, fold); err == nil {
					obs.Tickets[i].Gates, obs.Tickets[i].GatesObserved = gates, true
				}
			}
		}
		for _, a := range in.Attempts {
			x := dispatch.Attempt{ID: a.AttemptID, Ticket: a.TicketID.Raw, Phase: a.Phase, Stage: a.Stage, Generation: string(a.Generation), Live: a.Live(), Gates: len(a.GateResults), Reviews: len(a.Reviews)}
			if a.CandidateTreeOid != nil {
				x.Candidate = *a.CandidateTreeOid
			}
			if a.PoolAllocation != nil {
				x.Pool, x.Member = a.PoolAllocation.PoolID, a.PoolAllocation.MemberID
			}
			if a.Lease != nil {
				x.Holder = a.Lease.Holder
				if t, e := time.Parse("2006-01-02T15:04:05Z", string(a.Lease.ExpiresAt)); e == nil {
					x.LeaseExpires = t
				}
			}
			obs.Attempts = append(obs.Attempts, x)
		}
		sort.Slice(obs.Attempts, func(i, j int) bool { return obs.Attempts[i].ID < obs.Attempts[j].ID })
		if in.Pools != nil {
			for _, m := range in.Pools.Entries {
				configured := false
				if pool := in.Policy.Pool(m.PoolID); pool != nil {
					configured = pool.MemberConfig[m.MemberID].SafeReuse != nil
				}
				obs.Members = append(obs.Members, dispatch.Member{Pool: m.PoolID, Member: m.MemberID, State: m.State, Holder: m.Holder, Attempt: m.AttemptID, Queue: in.Queue.QueueID.Raw, Allocation: string(m.AllocationID), Definition: string(m.DefinitionSha256), SafeReuse: configured, Owned: m.Sweep != nil})
			}
		}
		return nil
	})
	return obs, err
}

func (q dispatchQueue) Release(ctx context.Context, a dispatch.Attempt, evidence, requestID string) error {
	args := []string{"--attempt", a.ID, "--generation", a.Generation, "--reason", wire.CodeHandoff, "--request-id", requestID}
	if evidence != "" {
		args = append(args, "--evidence", evidence)
	}
	return leaseOutcome(leaseCommand(q.quiet(), "release", args))
}

func (q dispatchQueue) Reap(ctx context.Context, a dispatch.Attempt, requestID string) error {
	return leaseOutcome(leaseCommand(q.quiet(), "reap", []string{"--attempt", a.ID, "--generation", a.Generation, "--request-id", requestID}))
}

func (q dispatchQueue) quiet() Env {
	env := q.env
	env.Stdout, env.Stderr = io.Discard, io.Discard
	return env
}

func leaseOutcome(r *wire.Result) error {
	if r.Outcome == wire.OutcomeOK {
		return nil
	}
	return fmt.Errorf("%s %s: %s", r.Outcome, strings.Join(r.Codes, ","), strings.Join(r.Warnings, "; "))
}

// dispatchEscalationPending lists, by ticket, the request IDs of each
// ESC-V0-006 hold in the dispatcher's last native observation. The hold is
// kept apart from the plan reason, so a ticket first blocked by another
// reason still shows it; status reads no native store, and both the roster
// and the native claim path enforce the hold themselves.
func dispatchEscalationPending(l *dispatch.Ledger) []wire.Value {
	if l.Seen == nil {
		return nil
	}
	ids := make([]string, 0, len(l.Seen.Escalations))
	for id := range l.Seen.Escalations {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	held := []wire.Value{}
	for _, id := range ids {
		x := wire.NewObject()
		x.Set("ticket", wire.String(id))
		x.Set("requests", wire.Strings(l.Seen.Escalations[id]))
		held = append(held, wire.Value{Kind: wire.KindObject, Obj: x})
	}
	return held
}

// dispatchInfraRetry lists each ESC-V0-007 infrastructure retry episode,
// apart from parked keys and the ladder: state, charged count against the
// configured bound, next eligible time, reason and the native requests it
// charged, which stay OPEN whatever the local state.
func dispatchInfraRetry(c *dispatch.Config, l *dispatch.Ledger) []wire.Value {
	keys := make([]string, 0, len(l.InfraRetry))
	for k := range l.InfraRetry {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	maxRetries, _, _ := c.InfrastructureRetry.Limits()
	out := []wire.Value{}
	for _, k := range keys {
		e := l.InfraRetry[k]
		x := wire.NewObject()
		x.Set("ticket", wire.String(k)).Set("state", wire.String(e.State))
		x.Set("count", wire.String(strconv.Itoa(e.Count))).Set("maxRetries", wire.String(strconv.Itoa(maxRetries)))
		x.Set("acceptanceRevision", wire.String(e.AcceptanceRevision))
		x.Set("requests", wire.Strings(append([]string{}, e.Requests...)))
		if !e.NextEligible.IsZero() {
			x.Set("nextEligible", wire.String(e.NextEligible.UTC().Format(time.RFC3339)))
		}
		if e.Reason != "" {
			x.Set("reason", wire.String(e.Reason))
		}
		if e.Pending != nil {
			x.Set("reservedWorker", wire.String(e.Pending.Worker))
		}
		out = append(out, wire.Value{Kind: wire.KindObject, Obj: x})
	}
	return out
}

// dispatchLoopDetected lists, by ticket, each CAL-V0-102 LOOP_DETECTED hold
// in the dispatcher's last native observation, kept apart from the plan
// reason like the ESC-V0-006 hold.
func dispatchLoopDetected(l *dispatch.Ledger) []wire.Value {
	if l.Seen == nil {
		return nil
	}
	ids := make([]string, 0, len(l.Seen.Loops))
	for id := range l.Seen.Loops {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	held := []wire.Value{}
	for _, id := range ids {
		h := l.Seen.Loops[id]
		x := wire.NewObject()
		x.Set("ticket", wire.String(id))
		x.Set("signal", wire.String(h.Signal))
		x.Set("acceptanceRevision", wire.String(h.AcceptanceRevision))
		x.Set("generations", wire.Strings(h.Generations))
		held = append(held, wire.Value{Kind: wire.KindObject, Obj: x})
	}
	return held
}

// dispatchTickets is the ticket half of the native observation: each
// ticket's plan state and primary reason, plus its ESC-V0-006 hold derived
// apart from that reason, so a hold behind another blocker still reaches the
// roster and status.
// observeEscalations gives the dispatcher a ticket's typed requests
// (ESC-V0-007, ESC-V0-008): the current-acceptance OPEN and ANSWERED
// requests with the holder their own audited source recorded, and the
// effective work revision, so a typed write alone is never work progress.
// Material that cannot be read or validated marks the ticket
// EscalationUnknown instead of guessing.
func observeEscalations(repo *intent.Repository, queueID string, r *ticket.Record, t *dispatch.Ticket) {
	t.AcceptanceRevision = string(r.AcceptanceRevision)
	if r.Escalations == nil {
		return
	}
	m := loadEscalations(repo, queueID, r)
	if m.err != nil {
		t.EscalationUnknown = true
		return
	}
	work, err := transaction.EffectiveEscalationWorkRevision(transaction.EscalationSnapshot{QueueID: queueID, TicketID: r.TicketID.Raw, TicketRevision: r.Revision, AcceptanceRevision: r.AcceptanceRevision, Refs: r.Escalations, Blobs: m.blobs})
	if err != nil {
		t.EscalationUnknown = true
		return
	}
	t.Revision = string(work)
	for _, ref := range r.Escalations.Entries {
		if ref.AcceptanceRevision != r.AcceptanceRevision || (ref.State != "OPEN" && ref.State != "ANSWERED") {
			continue
		}
		origin, err := ticket.DecodeEscalationEvent(m.blobs[ref.OriginSha256])
		if err != nil {
			t.EscalationUnknown, t.Requests = true, nil
			return
		}
		x := dispatch.EscalationRequest{ID: ref.RequestID, Kind: ref.Kind, State: ref.State, Holder: origin.Source.Holder}
		if at, err := time.Parse("2006-01-02T15:04:05Z", string(origin.RecordedAt)); err == nil {
			x.Opened = at
		}
		t.Requests = append(t.Requests, x)
	}
}

func dispatchTickets(in transaction.PlanInput) []dispatch.Ticket {
	planned := map[string]transaction.PlanEntry{}
	for _, e := range transaction.PriorityFirst(in).Entries {
		planned[e.Ticket.TicketID.Raw] = e
	}
	var out []dispatch.Ticket
	for _, id := range in.Tickets.IDs() {
		r, _ := in.Tickets.Get(id)
		t := dispatch.Ticket{ID: r.TicketID.Raw, Local: r.TicketID.Local, Status: r.Status, Priority: r.Priority, Kind: r.Kind, Revision: string(r.Revision), Order: uint64(r.Order.Int()), Labels: r.Labels, RequiresPool: r.RequiresPool}
		if e, ok := planned[id]; ok {
			t.Plan, t.PlanReason = e.State, e.Reason
		}
		t.EscalationPending = r.EscalationPending()
		if h := transaction.LoopHoldOf(in.Attempts, r, in.Policy); h != nil {
			t.Loop = &dispatch.LoopHold{Signal: h.Signal, AcceptanceRevision: string(h.AcceptanceRevision), Generations: h.Generations}
		}
		t.NextStage = dispatch.StateNone
		if s := transaction.NextStage(in.Attempts, r); s.Kind == wire.KindString {
			t.NextStage = s.Str
		}
		out = append(out, t)
	}
	return out
}
