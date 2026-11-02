package cli

import (
	"context"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/authority"
	"github.com/Beamfall/corvint/internal/tasks/dispatch"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/journal"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// leaseVerbs maps each lease command to its transaction verb (CAL-V0-007 to
// CAL-V0-013, CAL-V0-025).
var leaseVerbs = map[string]string{
	"pool confirm-safe": transaction.LeasePoolSafe,
	// CAL-V0-198..202: a live attempt acquires or returns one member.
	"pool acquire": transaction.LeasePoolAcquire, "pool release": transaction.LeasePoolRelease,
	// PSR-V0-130: one ticket's bounded place on a pool's waitlist.
	"pool wait": transaction.LeasePoolWait,
	// PSR-V0-160..167: one declared typed member lifecycle operation.
	"pool operate": transaction.LeasePoolOperate,
	// PSR-V0-260: one failed readiness observation of a QUARANTINED member.
	"pool observe": transaction.LeasePoolReport,
	// PSR-V0-268..271: an operator hold of one FREE member.
	"pool hold": transaction.LeasePoolHold, "pool hold renew": transaction.LeasePoolHoldRenew, "pool hold release": transaction.LeasePoolHoldRelease,
	// PSR-V0-032..037: shared-capacity slots of a declared resource.
	"slots acquire": transaction.LeaseSlotAcquire, "slots release": transaction.LeaseSlotRelease,
	"pool cleanup": transaction.LeasePoolCleanup, "pool recover": transaction.LeasePoolRecover, "health": transaction.LeasePoolPrepare,
	"claim":             transaction.LeaseClaim,
	"renew":             transaction.LeaseRenew,
	"attempt heartbeat": transaction.LeaseHeartbeat,
	"release":           transaction.LeaseRelease,
	"reap":              transaction.LeaseReap,
	"widen":             transaction.LeaseWiden,
	// S5 (CAL-V0-015..017).
	"submit":   transaction.LeaseSubmit,
	"gate run": transaction.LeaseGateRun,
	"complete": transaction.LeaseComplete,
}

// leaseArgs is the parsed argv of one lease command. --scope takes every
// following argument up to the next flag, and may repeat.
type leaseArgs struct {
	laneUntouched bool
	values        map[string]string
	scope         []string
	excluded      []string
	authors       string
	whole         bool
	exclusive     bool
	next          bool
	timing        bool
	admit         bool
	repos         []string
	pos           []string
}

var leaseValueFlags = map[string]bool{
	"--pool": true, "--stage": true, "--member": true, "--allocation": true, "--evidence": true,
	"--request-id": true, "--role": true, "--holder": true, "--lease-minutes": true, "--branch": true,
	"--base": true, "--attempt": true, "--generation": true, "--reason": true,
	"--handoff-to": true, "--handoff-reason": true, "--share-allocation": true,
	"--tree": true, "--gate": true, "--commit": true, "--worktree": true, "--ci-run": true,
	"--lock-wait": true, "--lease-expires-at": true,
	"--slots": true, "--count": true, "--class": true, "--source": true,
	"--ticket": true, "--ttl-seconds": true, "--operation": true,
	"--kind": true, "--result": true, "--detail-file": true,
	"--scope-from-diff": true,
}

// lockWaitVerbs are the lease commands that take the CAL-V0-111 --lock-wait:
// release and attempt heartbeat, and (CAL-V0-216) the other
// preparation-admitted agent writes.
var lockWaitVerbs = map[string]bool{
	"release": true, "attempt heartbeat": true,
	"claim": true, "renew": true, "submit": true,
	"pool acquire": true, "pool confirm-safe": true, "pool release": true,
	"slots acquire": true, "slots release": true,
	"pool wait": true,
}

// lockWaitRefusal names every lockWaitVerbs command for a misplaced --lock-wait.
const lockWaitRefusal = "--lock-wait belongs to release, attempt heartbeat, claim, renew, submit, pool acquire, pool confirm-safe, pool release, pool wait, slots acquire and slots release"

// parseLockWait reads one CAL-V0-111 --lock-wait value: whole seconds in
// canonical decimal, from 1 to authority.MaxCallerLockWait.
func parseLockWait(value string) (time.Duration, error) {
	limit := int64(authority.MaxCallerLockWait / time.Second)
	n := int64(0)
	ok := value != "" && value[0] != '0' && len(value) <= 3
	for i := 0; ok && i < len(value); i++ {
		ok = value[i] >= '0' && value[i] <= '9'
		n = n*10 + int64(value[i]-'0')
	}
	if !ok || n > limit {
		return 0, wire.Errorf(wire.CodeMalformed, "--lock-wait", "takes whole seconds from 1 to %d", limit)
	}
	return time.Duration(n) * time.Second, nil
}

func parseLeaseArgs(args []string) (leaseArgs, error) {
	out := leaseArgs{values: map[string]string{}}
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--lane-untouched":
			if out.laneUntouched {
				return out, wire.Errorf(wire.CodeMalformed, "argv", "repeated --lane-untouched")
			}
			out.laneUntouched = true
		case a == "--whole-repository":
			out.whole = true
		case a == "--exclusive":
			out.exclusive = true
		case a == "--next":
			out.next = true
		case a == "--timing":
			out.timing = true
		case a == "--admit-under-barrier":
			out.admit = true
		case a == "--exclude-member":
			if i+1 >= len(args) || args[i+1] == "" || strings.HasPrefix(args[i+1], "--") {
				return out, wire.Errorf(wire.CodeMalformed, "argv", "--exclude-member needs one nonempty member")
			}
			i++
			out.excluded = append(out.excluded, args[i])
		case a == "--exclude-authors" || strings.HasPrefix(a, "--exclude-authors="):
			mode, err := excludeAuthorsMode(a, out.authors)
			if err != nil {
				return out, err
			}
			out.authors = mode
		case a == "--repo":
			if i+1 >= len(args) {
				return out, wire.Errorf(wire.CodeMalformed, "argv", "flag --repo has no value")
			}
			i++
			out.repos = append(out.repos, args[i])
		case a == "--scope":
			n := scopeRun(args[i+1:])
			if n == 0 {
				return out, wire.Errorf(wire.CodeMalformed, "argv", "--scope needs at least one path")
			}
			out.scope = append(out.scope, args[i+1:i+1+n]...)
			i += n
		case leaseValueFlags[a]:
			if _, dup := out.values[a]; dup || i+1 >= len(args) {
				return out, wire.Errorf(wire.CodeMalformed, "argv", "flag %s is repeated or has no value", a)
			}
			i++
			out.values[a] = args[i]
		case strings.HasPrefix(a, "--"):
			return out, wire.Errorf(wire.CodeMalformed, "argv", "unknown flag %s", prose(a))
		default:
			out.pos = append(out.pos, a)
		}
	}
	return out, nil
}

// excludeAuthorsMode parses one CAL-V0-098 flag: bare for the most recent
// implement generation, =all for every recorded one; it never repeats.
func excludeAuthorsMode(flag, prior string) (string, error) {
	if prior != "" {
		return "", wire.Errorf(wire.CodeMalformed, "argv", "repeated --exclude-authors")
	}
	switch flag {
	case "--exclude-authors":
		return transaction.ExcludeAuthorsLatest, nil
	case "--exclude-authors=all":
		return transaction.ExcludeAuthorsAll, nil
	}
	return "", wire.Errorf(wire.CodeMalformed, "argv", "--exclude-authors takes no value or =all")
}

func scopeRun(rest []string) int {
	n := 0
	for n < len(rest) && !strings.HasPrefix(rest[n], "--") {
		n++
	}
	return n
}

// scopePaths sorts and deduplicates the requested paths; nil when none.
func scopePaths(paths []string) []string {
	if len(paths) == 0 {
		return nil
	}
	sorted := append([]string{}, paths...)
	sort.Strings(sorted)
	out := sorted[:1]
	for _, p := range sorted[1:] {
		if p != out[len(out)-1] {
			out = append(out, p)
		}
	}
	return out
}

func (a leaseArgs) request(verb, queueID string) (transaction.LeaseRequest, error) {
	if a.next && verb != transaction.LeaseClaim {
		return transaction.LeaseRequest{}, wire.Errorf(wire.CodeMalformed, "argv", "--next belongs to claim")
	}
	if a.next {
		verb = transaction.LeaseClaimNext
	}
	req := transaction.LeaseRequest{LaneUntouched: a.laneUntouched, Pool: a.values["--pool"], Stage: a.values["--stage"], Member: a.values["--member"], Allocation: a.values["--allocation"], Evidence: a.values["--evidence"], Verb: verb, Holder: a.values["--holder"], Branch: a.values["--branch"], Base: a.values["--base"], Scope: scopePaths(a.scope), ExcludeMembers: scopePaths(a.excluded), ExcludeAuthors: a.authors, WholeRepository: a.whole, AttemptID: a.values["--attempt"], Generation: wire.Size(a.values["--generation"]), LeaseExpiresAt: wire.Timestamp(a.values["--lease-expires-at"]), Reason: a.values["--reason"], HandoffTo: a.values["--handoff-to"], HandoffReason: a.values["--handoff-reason"], LeaseMinutes: wire.Size(a.values["--lease-minutes"]), Tree: a.values["--tree"], Gate: a.values["--gate"], Commit: a.values["--commit"], ShareAllocation: a.values["--share-allocation"], Slots: a.values["--slots"], SlotCount: wire.Count(a.values["--count"]), SlotExclusive: a.exclusive, SlotClass: a.values["--class"], Source: a.values["--source"], WaitSeconds: wire.Count(a.values["--ttl-seconds"]), Operation: a.values["--operation"], AdmitUnderBarrier: a.admit}
	if t := a.values["--ticket"]; t != "" {
		if verb != transaction.LeasePoolWait {
			return req, wire.Errorf(wire.CodeMalformed, "argv", "--ticket is a pool wait flag")
		}
		req.TicketID = qualifyTicket(queueID, t)
	}
	if verb == transaction.LeaseClaim {
		if len(a.pos) != 1 {
			return req, wire.Errorf(wire.CodeMalformed, "argv", "claim takes exactly one ticket id or local token")
		}
		req.TicketID = qualifyTicket(queueID, a.pos[0])
	} else if len(a.pos) != 0 && a.next {
		return req, wire.Errorf(wire.CodeMalformed, "argv", "claim --next takes no ticket id")
	} else if len(a.pos) != 0 {
		return req, wire.Errorf(wire.CodeMalformed, "argv", "%s takes no positional argument", strings.ToLower(verb))
	}
	if req.LeaseMinutes == "" && (verb == transaction.LeaseClaim || verb == transaction.LeaseClaimNext || verb == transaction.LeaseRenew) {
		req.LeaseMinutes = wire.SizeOf(transaction.DefaultLeaseMinutes)
	}
	return req, nil
}

// leaseCommand runs one lease write. Each commits through the §5.2 writer
// under the store lock; a claim first reaps, one receipt each, the expired
// leases that would block it.
func leaseCommand(env Env, name string, args []string) (res *wire.Result) {
	if name == "claim" {
		defer func() { res = withDrainNotice(res) }()
	}
	started := time.Now()
	cmd := strings.Fields(name)
	parsed, err := parseLeaseArgs(args)
	if err != nil {
		return errorResult(cmd, err)
	}
	fail := func(err error) *wire.Result {
		return timedFailure(parsed.timing && timingVerbs[name], started, cmd, err)
	}
	if evidence, supplied := parsed.values["--evidence"]; name == "release" && supplied && evidence == "" {
		return fail(wire.Errorf(wire.CodeMalformed, "evidence", "handoff reference must be a nonempty Identifier"))
	}
	if value, supplied := parsed.values["--share-allocation"]; supplied && value == "" {
		return errorResult(cmd, wire.Errorf(wire.CodeMalformed, "argv", "--share-allocation needs an allocation id"))
	}
	// CAL-V0-191: an empty expiry must not read as an unfenced reap.
	if value, supplied := parsed.values["--lease-expires-at"]; supplied && (name != "reap" || value == "") {
		return errorResult(cmd, wire.Errorf(wire.CodeMalformed, "argv", "--lease-expires-at belongs to reap and needs a timestamp"))
	}
	for _, flag := range []string{"--handoff-to", "--handoff-reason"} {
		if value, supplied := parsed.values[flag]; supplied && (name != "release" || value == "") {
			return errorResult(cmd, wire.Errorf(wire.CodeMalformed, "argv", "%s belongs to release and needs a value", flag))
		}
	}
	worktree, hasWorktree := parsed.values["--worktree"]
	if hasWorktree && name != "gate run" {
		return fail(wire.Errorf(wire.CodeMalformed, "argv", "--worktree belongs to gate run"))
	}
	// TCD-V0-155: --ci-run records a retained green CI run instead of
	// executing the gate, so it never takes a worktree.
	ciRunFile, hasCIRun := parsed.values["--ci-run"]
	if hasCIRun && (name != "gate run" || hasWorktree || ciRunFile == "") {
		return fail(wire.Errorf(wire.CodeMalformed, "argv", "--ci-run belongs to gate run, needs a file and excludes --worktree"))
	}
	// PSR-V0-260: pool observe needs --kind, --result and --detail-file, and
	// derives its evidence digest from the detail document; no other verb
	// takes them.
	for _, flag := range []string{"--kind", "--result", "--detail-file"} {
		if value, supplied := parsed.values[flag]; (name == "pool observe") != supplied || (supplied && value == "") {
			return errorResult(cmd, wire.Errorf(wire.CodeMalformed, "argv", "pool observe needs --kind health|cleanup, --result FAILED and --detail-file FILE; no other command takes them"))
		}
	}
	if _, supplied := parsed.values["--evidence"]; supplied && name == "pool observe" {
		return errorResult(cmd, wire.Errorf(wire.CodeMalformed, "argv", "pool observe derives its evidence digest from --detail-file"))
	}
	if parsed.timing && !timingVerbs[name] {
		return errorResult(cmd, wire.Errorf(wire.CodeMalformed, "argv", "--timing belongs to claim, renew, attempt heartbeat and release"))
	}
	// KHN-V0-027: --repo maps know-how repository aliases for the claim's
	// delivery; it is parsed and checked before anything commits and never
	// enters the lease request, so a replay is unaffected.
	if len(parsed.repos) > 0 && name != "claim" {
		return errorResult(cmd, wire.Errorf(wire.CodeMalformed, "argv", "--repo belongs to claim"))
	}
	// TCD-V0-174: --scope-from-diff derives one named claim's scope from a
	// committed diff; it excludes every other scope source and never enters
	// the lease request.
	scopeFromDiff, fromDiff := parsed.values["--scope-from-diff"]
	if fromDiff && (name != "claim" || scopeFromDiff == "" || parsed.next || parsed.whole || len(parsed.scope) > 0) {
		return errorResult(cmd, wire.Errorf(wire.CodeMalformed, "argv", "--scope-from-diff belongs to claim of one named ticket, needs a revision and excludes --scope and --next"))
	}
	// TCL-V0-866: only widen takes --whole-repository; a claim reaches
	// WHOLE_REPOSITORY as its fallback scope, so the flag is refused here
	// before any store read rather than as a misfit lease request.
	if parsed.whole && name == "claim" {
		return errorResult(cmd, wire.Errorf(wire.CodeMalformed, "argv", "claim takes no --whole-repository: a claim without --scope of a ticket that declares no scope already claims WHOLE_REPOSITORY; to widen a live attempt to the whole repository run widen --attempt A --generation G --whole-repository (TCL-V0-866)"))
	}
	// TRM-V0-801: the barrier exemption names one ticket or attempt, so it
	// belongs to a named claim or a widen and never to claim --next.
	if parsed.admit && ((name != "claim" && name != "widen") || parsed.next) {
		return errorResult(cmd, wire.Errorf(wire.CodeMalformed, "argv", "--admit-under-barrier belongs to claim of one named ticket and to widen, never to claim --next (TRM-V0-801)"))
	}
	knowHowRepos, err := store.KnowHowRepositoryArgs(env.Cwd, parsed.repos)
	if err != nil {
		return fail(err)
	}
	var lockWait time.Duration
	if value, supplied := parsed.values["--lock-wait"]; supplied {
		if !lockWaitVerbs[name] {
			return errorResult(cmd, wire.Errorf(wire.CodeMalformed, "argv", lockWaitRefusal))
		}
		if lockWait, err = parseLockWait(value); err != nil {
			return fail(err)
		}
	}
	role := parsed.values["--role"]
	if role == "" {
		role = "OPERATOR"
	}
	requestID := parsed.values["--request-id"]
	if _, err = mutation.ParseRequestID("requestId", requestID); err != nil {
		return fail(err)
	}
	actor, err := initActor(role)
	if err != nil {
		return fail(err)
	}
	repo, err := intent.Resolve(env.Cwd)
	if err != nil {
		return fail(err)
	}
	// The probe only names the queue. A committed receipt head.json does not
	// name yet (REDO_PENDING) is not refused here: the lease writer settles
	// it under the store lock before modelling this request, so a receipt a
	// killed writer left cannot refuse every heartbeat while no other writer
	// runs (CAL-V0-222, issue 860).
	observed, _, err := writerProbe(env.leaseProbe, repo.StateDir)
	if err != nil {
		return fail(err)
	}
	queueID := observed.Head.QueueID.Raw
	lease, err := parsed.request(leaseVerbs[name], queueID)
	if err != nil {
		return fail(err)
	}
	now, err := wire.ParseTimestamp("recordedAt", time.Now().UTC().Format("2006-01-02T15:04:05Z"))
	if err != nil {
		return fail(err)
	}
	choice := store.LeaseChoice{QueueID: queueID, RequestID: requestID, Root: env.Cwd, Lease: lease, Derive: env.ScopeDeriver, KnowHowRepos: knowHowRepos, ScopeFromDiff: scopeFromDiff}
	if name == "pool observe" {
		raw, e := intent.ReadFile(gateWorktree(env.Cwd, parsed.values["--detail-file"]), transaction.MaxPoolReportInputBytes)
		if e != nil {
			return fail(e)
		}
		doc, e := transaction.NewPoolRecoveryDetail(parsed.values["--kind"], parsed.values["--result"], raw)
		if e != nil {
			return fail(e)
		}
		choice.Lease.Evidence, choice.PoolDetail = string(wire.Sum(doc)), doc
	}
	var report *store.Report
	if name == "health" || name == "pool cleanup" {
		ctx, stop := signal.NotifyContext(writerContext(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		kind := "health"
		if name == "pool cleanup" {
			kind = "cleanup"
		}
		report, err = store.PoolCommand(ctx, repo, actor, choice, kind)
	} else if name == "pool operate" {
		// PSR-V0-166: an interrupt kills the operation's process group and
		// records the INTERRUPTED observation.
		ctx, stop := signal.NotifyContext(writerContext(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		report, err = store.PoolOperate(ctx, repo, actor, choice)
	} else if name == "gate run" {
		// An interrupt or hangup kills the gate's process group; a run that
		// started is still recorded with its partial output (CAL-V0-319).
		ctx, stop := signal.NotifyContext(writerContext(), os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
		defer stop()
		if hasCIRun {
			var ciRaw []byte
			if ciRaw, err = intent.ReadFile(gateWorktree(env.Cwd, ciRunFile), snapshot.MaxCIRunBytes); err == nil {
				report, err = store.GateRunCI(ctx, repo, actor, choice, ciRaw, time.Now)
			}
		} else {
			report, err = store.GateRun(ctx, repo, actor, choice, gateWorktree(env.Cwd, worktree), time.Now)
		}
	} else {
		base := writerContext()
		if env.leaseInterrupted != nil {
			base = store.WithClock(env.leaseInterrupted, store.WallClock)
		}
		if !env.leaseDeadline.IsZero() {
			var cancel context.CancelFunc
			base, cancel = context.WithDeadline(base, env.leaseDeadline)
			defer cancel()
		}
		ctx, stop := signal.NotifyContext(base, os.Interrupt, syscall.SIGTERM)
		defer stop()
		if lockWait > 0 {
			ctx = store.WithLeaseLockWait(ctx, lockWait)
		}
		if env.leaseProgress != nil {
			ctx = store.WithLeaseProgress(store.WithLeaseTiming(ctx, new(store.LeaseTiming)), env.leaseProgress)
		}
		if parsed.timing {
			return timedLease(ctx, started, cmd, repo, actor, choice, now)
		}
		report, err = store.Lease(ctx, repo, actor, choice, now)
		// A deadline (the detached ceiling) or an interrupt is LOCK_TIMEOUT,
		// never MALFORMED (ATR-V0-053).
		err = wire.ContextEnded(err, "lease")
	}
	res = unretryableResult(cmd, report, err)
	if name == "pool hold" && err == nil {
		// PSR-V0-268: the held allocation renew and release name, derived
		// from the queue, request and member so a replay reports it too;
		// null unless the hold was recorded.
		held := wire.Null()
		if report.Outcome.Outcome == mutation.OutcomeCompleted {
			held = wire.String(string(transaction.PoolHoldAllocation(queueID, requestID, lease.Member)))
		}
		res.Items[0].Obj.Set("allocationId", held)
	}
	return res
}

// unretryableResult is the lease command's result. When the call already
// started a program or committed a step it could not finish (store
// Report.Unretryable), a retry would run the program again or replay the step
// unfinished, so the result is never retryable whatever its codes (CAL-V0-078).
func unretryableResult(cmd []string, report *store.Report, err error) *wire.Result {
	var res *wire.Result
	if err != nil {
		res = errorResult(cmd, err)
	} else {
		res = leaseResult(cmd, report)
	}
	if report != nil && report.Unretryable {
		res.NotRetryable = true
	}
	return res
}

func leaseResult(cmd []string, report *store.Report) *wire.Result {
	res := mutateResult(cmd, report)
	o := res.Items[0].Obj
	if report.PoolAllocation != nil {
		o.Set("poolAllocation", snapshot.PoolAllocationValue(report.PoolAllocation))
	} else if cmd[0] == "claim" || strings.Join(cmd, " ") == "pool acquire" {
		// CAL-V0-096: every claim result, a refusal included, names its
		// allocation; so does every acquire (CAL-V0-204).
		o.Set("poolAllocation", wire.Null())
	}
	if strings.Join(cmd, " ") == "pool release" {
		// CAL-V0-204: the returned allocation, null unless one was returned.
		released := wire.Null()
		if report.ReleasedPoolAllocation != nil {
			released = snapshot.ReleasedPoolAllocationValue(report.ReleasedPoolAllocation)
		}
		o.Set("releasedPoolAllocation", released)
	}
	if report.Slot != nil {
		o.Set("slot", slotObservationValue(report.Slot))
	} else if cmd[0] == "slots" {
		o.Set("slot", wire.Null())
	}
	if strings.Join(cmd, " ") == "pool operate" {
		// PSR-V0-162, PSR-V0-164: the recorded receipt, null when none is.
		recorded := wire.Null()
		if report.PoolOperation != nil {
			recorded = report.PoolOperation.Value()
		}
		o.Set("operation", recorded)
	}
	if cmd[0] == "claim" {
		// ATR-V0-211: every claim result names the effective reserved
		// scope, null when nothing was reserved.
		o.Set("scope", claimScopeValue(report.Scope))
		// ATR-V0-214: an OUT_OF_SCOPE claim names the declared and the
		// rejected paths and how to retry.
		if r := report.ScopeRefusal; r != nil {
			o.Set("scopeRefusal", wire.ObjectValue(wire.NewObject().Set("declared", wire.Strings(r.Declared)).Set("rejected", wire.Strings(r.Rejected)).Set("nextAction", wire.String(transaction.ScopeRetry))))
		}
		// TRM-V0-006: a resumed claim names the paths it restored and the
		// ended generation that held them.
		if r := report.RestoredScope; r != nil {
			o.Set("restoredScope", wire.ObjectValue(wire.NewObject().Set("fromGeneration", wire.String(string(r.FromGeneration))).Set("paths", wire.Strings(r.Paths))))
		}
	}
	if report.SharedAllocation != nil {
		o.Set("sharedAllocation", snapshot.SharedAllocationValue(report.SharedAllocation))
	}
	if strings.Join(cmd, " ") == "gate run" {
		// CAL-V0-240: the earlier result a reuse names, null when the gate ran.
		reused := wire.Null()
		if r := report.GateReusedFrom; r != nil && r.CIRunID != "" {
			reused = wire.ObjectValue(wire.NewObject().Set("result", wire.String(string(r.Result))).Set("ciRunId", wire.String(r.CIRunID)))
		} else if r != nil {
			reused = wire.ObjectValue(wire.NewObject().Set("result", wire.String(string(r.Result))).Set("attemptId", wire.String(r.AttemptID)))
		}
		o.Set("reusedFrom", reused)
		// TCD-V0-806: a descendant PASS that integration would let a rerun reuse.
		if p := report.GateReusePending; p != nil {
			res.Warnings = append(res.Warnings, "gate "+p.Gate+": attempt "+p.AttemptID+" passed it at tree "+p.Tree+" (result "+string(p.Result)+"), which is not yet a commit on the protected intent branch "+p.Branch+"; integrate the commit carrying that tree, which must descend from this attempt's candidate, then rerun gate run: it reuses that PASS without executing and complete records a verified completion (TCD-V0-806)")
		}
		gate, warning := gateResultValue(report.GateExecuted)
		o.Set("gateResult", gate)
		if warning != "" {
			res.Warnings = append(res.Warnings, warning)
		}
		if report.GateInterrupted {
			res.Warnings = append(res.Warnings, "gate run interrupted: the gate's process group was stopped and the run's result and captured output were recorded (CAL-V0-319)")
		}
	}
	if report.LaneUntouchedAttestation != nil {
		o.Set("laneUntouchedAttestation", snapshot.LaneUntouchedAttestationValue(report.LaneUntouchedAttestation))
	}
	o.Set("attemptId", stringOrNull(report.AttemptID))
	o.Set("generation", stringOrNull(string(report.Generation)))
	o.Set("expired", expiredValue(report.Expired))
	o.Set("reaped", expiredValue(report.Reaped))
	if len(report.ReapReceipts) > 0 {
		o.Set("reapReceipts", reapReceiptsValue(report.ReapReceipts))
	}
	if report.Delivery != nil {
		claimDeliveryResult(res, report.Delivery)
	}
	// CAL-V0-107: say when the author exclusion rests on the caller's
	// explicit members rather than on recorded ones.
	res.Warnings = append(res.Warnings, report.AuthorExclusion.Notes()...)
	res.Warnings = append(res.Warnings, report.ScopeWarnings...)
	res.Warnings = append(res.Warnings, report.GrantWarnings...)
	res.Warnings = append(res.Warnings, report.ReplayWarnings...)
	return res
}

// claimScopeValue renders a claimed attempt's scope as its source and
// sorted resources, or null.
func claimScopeValue(sc *snapshot.Scope) wire.Value {
	if sc == nil {
		return wire.Null()
	}
	res, err := snapshot.ResourcesValue("/scope/resources", sc.Resources)
	if err != nil {
		return wire.Null()
	}
	return wire.ObjectValue(wire.NewObject().Set("source", wire.String(sc.Source)).Set("resources", res))
}

func reapReceiptsValue(list []store.ReapReceipt) wire.Value {
	out := make([]wire.Value, 0, len(list))
	for _, x := range list {
		o := wire.NewObject()
		o.Set("attemptId", wire.String(x.AttemptID))
		o.Set("generation", wire.String(string(x.Generation)))
		o.Set("receipt", wire.String(x.Receipt))
		out = append(out, wire.ObjectValue(o))
	}
	return wire.Array(out...)
}

func stringOrNull(s string) wire.Value {
	if s == "" {
		return wire.Null()
	}
	return wire.String(s)
}

func expiredValue(list []transaction.ExpiredLease) wire.Value {
	out := make([]wire.Value, 0, len(list))
	for _, x := range list {
		o := wire.NewObject()
		o.Set("attemptId", wire.String(x.AttemptID))
		o.Set("generation", wire.String(string(x.Generation)))
		out = append(out, wire.ObjectValue(o))
	}
	return wire.Array(out...)
}

// attemptCommand runs `corvint-tasks attempt show <attemptId>` as a pure read
// of the audited attempt record (CAL-V0-013).
func attemptCommand(env Env, args []string) *wire.Result {
	if len(args) > 0 && args[0] == "heartbeat" {
		return leaseCommand(env, "attempt heartbeat", args[1:])
	}
	if len(args) > 0 && args[0] != "show" {
		return groupMiss([]string{"attempt"}, args, "attempt needs the verb show <attemptId>")
	}
	return readAttempt(env, args, true)
}

// readAttempt keeps derived observations out of canonical criterion captures.
func readAttempt(env Env, args []string, observations bool) *wire.Result {
	cmd := []string{"attempt", "show"}
	if len(args) != 2 || args[0] != "show" || strings.HasPrefix(args[1], "--") {
		return usage([]string{"attempt"}, "attempt needs the verb show <attemptId>")
	}
	var item wire.Value
	observedAt := time.Now().UTC()
	rc, err := withStore(env, func(rc *readCtx) error {
		if _, err := snapshot.AttemptQueue(args[1]); err != nil {
			return err
		}
		path := "attempts/" + args[1] + ".json"
		paths := []string{path}
		if observations {
			paths = append(paths, "pools.json")
		}
		proof, err := auditState(rc, paths...)
		if err != nil {
			return err
		}
		record, ok := proof.Records[path]
		if !ok || record.Sha256 == nil {
			return wire.Errorf(wire.CodeMalformed, "attemptId", "attempt does not exist")
		}
		a, e := snapshot.DecodeAttempt(record.Raw)
		if e != nil {
			return e
		}
		item, err = wire.Parse(record.Raw)
		if err == nil && observations {
			addHolderObservation(item.Obj, a, observedAt, rc.store.Policy.HeartbeatTTLSeconds())
			addHistoryObservation(item.Obj)
			// CAL-V0-322: the newest failed gate's retained output tail.
			item.Obj.Set("lastFailedGate", attemptLastFailedGate(rc.repo, rc.store.Policy, a))
			if a.PoolAllocation != nil {
				binding, e := poolBindingValue(proof.Records["pools.json"].Raw, a)
				if e != nil {
					return e
				}
				item.Obj.Set("poolBinding", binding)
			}
		}
		return err
	})
	if err != nil {
		return failure(cmd, rc, err)
	}
	res := success(cmd, rc)
	res.Items = []wire.Value{item}
	res.Untrusted = true
	return res
}

// auditState audits the named private-state paths and binds the audit to
// the outer snapshot. One audit retains the reservation set, the pool state
// and every attempt record, so a command that asks for several of them pays
// for the journal once (CAL-V0-061). The audit resumes from the
// writer-retained checkpoint when one binds; `receipt audit` never does.
func auditState(rc *readCtx, paths ...string) (*journal.Result, error) {
	shared := true
	for _, p := range paths {
		if p != "reservations.json" && p != "pools.json" && !strings.HasPrefix(p, "attempts/") {
			shared = false
		}
	}
	if shared && rc.proof != nil {
		return rc.proof, nil
	}
	reader := journal.Reader{Source: journal.Native{StateDir: rc.repo.StateDir, PrimaryWorktree: rc.repo.IntentRoot()}, QueueID: rc.snap.Head.QueueID, PrimaryWorktree: rc.repo.PrimaryWorktree, SelectState: true, Checkpoint: readCheckpoint(rc.repo), IntentTree: rc.tree, ExpectHeadSha256: rc.snap.HeadSha256}
	proof, err := reader.Audit(paths...)
	if err != nil {
		return nil, err
	}
	if proof.Identity.HeadSha256 != rc.snap.HeadSha256 || proof.Identity.IntentTreeSha256 != rc.snap.IntentTree {
		return nil, wire.Errorf(wire.CodeSnapshotMoved, "attempts", "journal and outer snapshot differ")
	}
	rc.proof = proof
	return proof, nil
}

// readCheckpoint returns the retained audit checkpoint, or nil when it is
// absent or unreadable. The journal reader rebinds whatever this returns.
func readCheckpoint(repo *intent.Repository) *journal.Checkpoint {
	raw, err := intent.ReadFile(journal.CheckpointPath(repo.StateDir), journal.MaxCheckpointBytes)
	if err != nil {
		return nil
	}
	cp, err := journal.DecodeCheckpoint(raw)
	if err != nil {
		return nil
	}
	return cp
}

// auditMode names how the command's journal audit ran: FULL walked every
// retained receipt, CHECKPOINT_PLUS_TAIL resumed from a writer's complete
// audit. NOT_OBSERVED means the command needed no journal audit.
func auditMode(rc *readCtx) string {
	if rc.proof == nil {
		return "NOT_OBSERVED"
	}
	return rc.proof.Mode
}

// attemptPaths lists every retained attempt record the audit proved.
func attemptPaths(proof *journal.Result) []string {
	var paths []string
	for p, record := range proof.Records {
		if strings.HasPrefix(p, "attempts/") && record.Sha256 != nil {
			paths = append(paths, p)
		}
	}
	sort.Strings(paths)
	return paths
}

// reservationOracle projects journal membership. The legacy LiveAttempt method
// does not attest process health or lease validity; expiry alone removes no entry.
type reservationOracle struct{ set *snapshot.ReservationSet }

func (o reservationOracle) LiveAttempt(id string) ticket.Observation {
	for _, entry := range o.set.Entries {
		if entry.TicketID.Raw == id {
			return ticket.Satisfied
		}
	}
	return ticket.Unsatisfied
}

func journalReservations(rc *readCtx) (*snapshot.ReservationSet, error) {
	proof, err := auditState(rc, "reservations.json")
	if err != nil {
		return nil, err
	}
	record, ok := proof.Records["reservations.json"]
	if !ok || record.Sha256 == nil {
		return nil, wire.Errorf(wire.CodeMalformed, "reservations.json", "reservation set does not exist")
	}
	set, err := snapshot.DecodeReservations(record.Raw)
	if err != nil {
		return nil, err
	}
	if set.QueueID != rc.snap.Head.QueueID {
		return nil, wire.Errorf(wire.CodeMalformed, "reservations.json/queueId", "reservation queue differs from outer snapshot")
	}
	return set, nil
}

func ticketContext(rc *readCtx) (ticket.Context, error) {
	ctx := rc.store.Context()
	if rc.journalAbsent {
		return ctx, nil
	}
	set, err := journalReservations(rc)
	if err != nil {
		return ctx, err
	}
	ctx.Attempts = reservationOracle{set: set}
	return ctx, nil
}

// liveAttempts reads every live attempt named by the reservation set. A head
// generation of zero means no attempt was ever admitted, so there is nothing
// to audit.
func liveAttempts(rc *readCtx) ([]*snapshot.Attempt, error) {
	if rc.snap.Head.Generation.Uint64() == 0 {
		return nil, nil
	}
	set, err := journalReservations(rc)
	if err != nil {
		return nil, err
	}
	paths := make([]string, 0, len(set.Entries))
	for _, en := range set.Entries {
		paths = append(paths, "attempts/"+en.AttemptID+".json")
	}
	if len(paths) == 0 {
		return nil, nil
	}
	proof, err := auditState(rc, paths...)
	if err != nil {
		return nil, err
	}
	out := make([]*snapshot.Attempt, 0, len(paths))
	for _, p := range paths {
		a, err := snapshot.DecodeAttempt(proof.Records[p].Raw)
		if err != nil {
			return nil, err
		}
		if a.Lease == nil || a.Scope == nil {
			return nil, wire.Errorf(wire.CodeMalformed, p, "live external-agent attempt without a lease or scope")
		}
		out = append(out, a)
	}
	return out, nil
}

func liveAttemptsValue(list []*snapshot.Attempt) wire.Value {
	out := make([]wire.Value, 0, len(list))
	for _, a := range list {
		o := wire.NewObject()
		o.Set("attemptId", wire.String(a.AttemptID))
		o.Set("ticketId", wire.String(a.TicketID.Raw))
		o.Set("generation", wire.String(string(a.Generation)))
		o.Set("phase", wire.String(a.Phase))
		o.Set("holder", wire.String(a.Lease.Holder))
		o.Set("expiresAt", wire.String(string(a.Lease.ExpiresAt)))
		o.Set("scopeSource", wire.String(a.Scope.Source))
		out = append(out, wire.ObjectValue(o))
	}
	return wire.Array(out...)
}

// gateWorktree resolves --worktree against the working directory, which it
// defaults to.
func gateWorktree(cwd, worktree string) string {
	if filepath.IsAbs(worktree) {
		return worktree
	}
	return filepath.Join(cwd, worktree)
}

// claimDeliveryResult adds what a claim or claim-next delivers from its
// admission beside the existing members (ON-V0-007); a later delivered field
// sits beside operatorNote. Delivered prose is untrusted data. An unresolvable
// note never fails the committed claim: the result keeps its attempt,
// generation and receipt, and an exact replay retries delivery.
func claimDeliveryResult(res *wire.Result, d *store.ClaimDelivery) {
	res.Items[0].Obj.Set("operatorNote", claimedNoteValue(d))
	res.Untrusted = true
	if d.OperatorNote.Err != nil {
		res.Warnings = append(res.Warnings, "operator note unavailable ("+noteErrCode(d.OperatorNote.Err)+"): the claim is committed; replay the exact claim request to retry delivery, never claim again")
	}
	res.Items[0].Obj.Set("escalationAnswers", claimedAnswersValue(d))
	if d.EscalationAnswers.Err != nil {
		res.Warnings = append(res.Warnings, "escalation answers unavailable ("+wire.CodeOf(escalationReadError(d.EscalationAnswers.Err))+"): the claim is committed; replay the exact claim request to retry delivery, never claim again")
	}
	res.Items[0].Obj.Set("knowHow", claimedKnowHowValue(d.KnowHow))
	res.Warnings = append(res.Warnings, d.KnowHow.Warnings...)
	if d.KnowHow.Err != nil {
		res.Warnings = append(res.Warnings, "know-how notes unavailable ("+wire.CodeOf(d.KnowHow.Err)+"): the claim is committed; read them with ticket know-how list")
	}
}

// claimedAnswersValue is the `escalationAnswers` member of a claim or
// claim-next result (ESC-V0-005): the answers pinned by the claim's own
// admission, separate from operatorNote. Unresolvable material is UNAVAILABLE
// with its code and the pinned references, never an empty answer list.
func claimedAnswersValue(d *store.ClaimDelivery) wire.Value {
	a := d.EscalationAnswers
	o := wire.NewObject().Set("sourceTicketRecordSha256", wire.String(string(d.TicketRecordSha256)))
	if a.Err != nil {
		o.Set("state", wire.String("UNAVAILABLE")).Set("code", wire.String(wire.CodeOf(escalationReadError(a.Err))))
		o.Set("references", snapshot.EscalationAnswersValue(a.Refs)).Set("answers", wire.Null())
		return wire.ObjectValue(o)
	}
	answers := a.Value
	if len(a.Refs) == 0 {
		answers = wire.Array()
	}
	o.Set("state", wire.String("CURRENT")).Set("answers", answers)
	return wire.ObjectValue(o)
}

// withDrainNotice adds the drain-pending notice to a claim result while the
// signal named by CORVINT_DISPATCH_DRAIN_FILE is present (ATR-V0-059). The
// claim itself is unchanged; a signal that does not read adds nothing.
func withDrainNotice(res *wire.Result) *wire.Result {
	path := os.Getenv("CORVINT_DISPATCH_DRAIN_FILE")
	if res == nil || path == "" {
		return res
	}
	s, err := dispatch.ReadDrainSignal(path)
	if err != nil {
		return res
	}
	res.Warnings = append(res.Warnings, "DRAIN_PENDING since "+s.Since+": "+s.Notice)
	return res
}
