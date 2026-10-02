package cli

import (
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

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
	"pool cleanup":      transaction.LeasePoolCleanup, "pool recover": transaction.LeasePoolRecover, "health": transaction.LeasePoolPrepare,
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
	values   map[string]string
	scope    []string
	excluded []string
	whole    bool
	next     bool
	pos      []string
}

var leaseValueFlags = map[string]bool{
	"--pool": true, "--stage": true, "--member": true, "--allocation": true, "--evidence": true,
	"--request-id": true, "--role": true, "--holder": true, "--lease-minutes": true, "--branch": true,
	"--base": true, "--attempt": true, "--generation": true, "--reason": true,
	"--tree": true, "--gate": true, "--commit": true, "--worktree": true,
}

func parseLeaseArgs(args []string) (leaseArgs, error) {
	out := leaseArgs{values: map[string]string{}}
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--whole-repository":
			out.whole = true
		case a == "--next":
			out.next = true
		case a == "--exclude-member":
			if i+1 >= len(args) || args[i+1] == "" || strings.HasPrefix(args[i+1], "--") {
				return out, wire.Errorf(wire.CodeMalformed, "argv", "--exclude-member needs one nonempty member")
			}
			i++
			out.excluded = append(out.excluded, args[i])
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
	req := transaction.LeaseRequest{Pool: a.values["--pool"], Stage: a.values["--stage"], Member: a.values["--member"], Allocation: a.values["--allocation"], Evidence: a.values["--evidence"], Verb: verb, Holder: a.values["--holder"], Branch: a.values["--branch"], Base: a.values["--base"], Scope: scopePaths(a.scope), ExcludeMembers: scopePaths(a.excluded), WholeRepository: a.whole, AttemptID: a.values["--attempt"], Generation: wire.Size(a.values["--generation"]), Reason: a.values["--reason"], LeaseMinutes: wire.Size(a.values["--lease-minutes"]), Tree: a.values["--tree"], Gate: a.values["--gate"], Commit: a.values["--commit"]}
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
func leaseCommand(env Env, name string, args []string) *wire.Result {
	cmd := strings.Fields(name)
	parsed, err := parseLeaseArgs(args)
	if err != nil {
		return errorResult(cmd, err)
	}
	if evidence, supplied := parsed.values["--evidence"]; name == "release" && supplied && evidence == "" {
		return errorResult(cmd, wire.Errorf(wire.CodeMalformed, "evidence", "handoff reference must be a nonempty Identifier"))
	}
	worktree, hasWorktree := parsed.values["--worktree"]
	if hasWorktree && name != "gate run" {
		return errorResult(cmd, wire.Errorf(wire.CodeMalformed, "argv", "--worktree belongs to gate run"))
	}
	role := parsed.values["--role"]
	if role == "" {
		role = "OPERATOR"
	}
	requestID := parsed.values["--request-id"]
	if _, err = mutation.ParseRequestID("requestId", requestID); err != nil {
		return errorResult(cmd, err)
	}
	actor, err := initActor(role)
	if err != nil {
		return errorResult(cmd, err)
	}
	repo, err := intent.Resolve(env.Cwd)
	if err != nil {
		return errorResult(cmd, err)
	}
	observed, err := snapshot.Probe(repo.StateDir)
	if err != nil {
		return errorResult(cmd, err)
	}
	queueID := observed.Head.QueueID.Raw
	lease, err := parsed.request(leaseVerbs[name], queueID)
	if err != nil {
		return errorResult(cmd, err)
	}
	now, err := wire.ParseTimestamp("recordedAt", time.Now().UTC().Format("2006-01-02T15:04:05Z"))
	if err != nil {
		return errorResult(cmd, err)
	}
	choice := store.LeaseChoice{QueueID: queueID, RequestID: requestID, Root: env.Cwd, Lease: lease, Derive: env.ScopeDeriver}
	var report *store.Report
	if name == "health" || name == "pool cleanup" {
		ctx, stop := signal.NotifyContext(writerContext(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		kind := "health"
		if name == "pool cleanup" {
			kind = "cleanup"
		}
		report, err = store.PoolCommand(ctx, repo, actor, choice, kind)
	} else if name == "gate run" {
		// An interrupt kills the gate's process group and records nothing.
		ctx, stop := signal.NotifyContext(writerContext(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		report, err = store.GateRun(ctx, repo, actor, choice, gateWorktree(env.Cwd, worktree), time.Now)
	} else {
		ctx, stop := signal.NotifyContext(writerContext(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		report, err = store.Lease(ctx, repo, actor, choice, now)
	}
	if err != nil {
		return errorResult(cmd, err)
	}
	return leaseResult(cmd, report)
}

func leaseResult(cmd []string, report *store.Report) *wire.Result {
	res := mutateResult(cmd, report)
	o := res.Items[0].Obj
	if report.PoolAllocation != nil {
		o.Set("poolAllocation", snapshot.PoolAllocationValue(report.PoolAllocation))
	}
	o.Set("attemptId", stringOrNull(report.AttemptID))
	o.Set("generation", stringOrNull(string(report.Generation)))
	o.Set("expired", expiredValue(report.Expired))
	o.Set("reaped", expiredValue(report.Reaped))
	if len(report.ReapReceipts) > 0 {
		o.Set("reapReceipts", reapReceiptsValue(report.ReapReceipts))
	}
	return res
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
		proof, err := auditState(rc, path)
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
			addHolderObservation(item.Obj, a, observedAt)
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
	reader := journal.Reader{Source: journal.Native{StateDir: rc.repo.StateDir, PrimaryWorktree: rc.repo.PrimaryWorktree}, QueueID: rc.snap.Head.QueueID, PrimaryWorktree: rc.repo.PrimaryWorktree, SelectState: true, Checkpoint: readCheckpoint(rc.repo)}
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
