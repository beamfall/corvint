package store

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/authority"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/journal"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

type leaseAuditKey struct {
	root                                        string
	head, receipt, intent, physical, membership wire.Digest
}

// Only a successful in-process audit can populate this bounded cache. Every
// reuse compares all physical bytes outside the lock under a fresh guard;
// head and intent alone cannot detect private projection corruption.
var leaseAudits struct {
	sync.Mutex
	key   leaseAuditKey
	proof *journal.Result
}

type preparedLease struct {
	guard              *authority.ChangeGuard
	proof              *journal.Result
	head               []byte
	branch             string
	result             transaction.Result
	pending            bool
	failure            error
	observationFailure error
	fatalCleanup       []inventoryCleanup
	// writer is the writer checkpoint the complete audit derived, retained
	// with the read checkpoint at commit (CAL-V0-116, proposed), and
	// writerToken the invalidation token read before that audit.
	writer      *journal.WriterCheckpoint
	writerToken writerToken
	// writerProof and writerPhys are that complete audit and the bytes it
	// read, which verify the retained checkpoint at commit (JSG-V0-003).
	writerProof *journal.Result
	writerPhys  map[string]journal.PhysicalFile
	// base and baseObs are a settled audit from receipt 1 that a later round
	// may resume from (PSR-V0-051, proposed); noResume is set when the
	// prefix walk declined, so the command keeps the complete audit.
	base     *journal.Result
	baseObs  journal.PhysicalObservation
	noResume bool
}

// leaseResume is a resume base and the invalidation token read before the
// audit that derived it (PSR-V0-052, proposed). It lives for one command.
type leaseResume struct {
	base  *journal.ResumeBase
	token writerToken
}

// resumeFrom returns the resume base of a round that ended SNAPSHOT_MOVED,
// and records in noResume that the command must keep the complete audit:
// the prefix walk declined, or no base could be derived from the audit.
func (p *preparedLease) resumeFrom(noResume *bool) *leaseResume {
	if p == nil {
		return nil
	}
	if p.noResume {
		*noResume = true
	}
	if *noResume || p.base == nil {
		return nil
	}
	base, err := p.base.ResumeBase(p.baseObs)
	if err != nil {
		*noResume = true
		return nil
	}
	return &leaseResume{base: base, token: p.writerToken}
}

func leaseKey(repo *intent.Repository, guard *authority.ChangeGuard, inv *transaction.Inventory, headRaw []byte) (leaseAuditKey, error) {
	head, err := snapshot.DecodeHead(headRaw)
	if err != nil {
		return leaseAuditKey{}, err
	}
	if head.LastReceiptSha256 == nil {
		return leaseAuditKey{}, wire.Errorf(wire.CodeJournalForked, "head", "missing receipt digest")
	}
	h := sha256.New()
	var intents []intent.File
	for _, file := range inv.Files() {
		fmt.Fprintf(h, "%s\x00%s\x00%s\n", file.Path, file.Sha256, file.Bytes)
		if strings.HasPrefix(file.Path, "intent/") {
			intents = append(intents, intent.File{Path: strings.TrimPrefix(file.Path, "intent/"), Sha256: file.Sha256, Bytes: int(file.Bytes.Uint64())})
		}
	}
	return leaseAuditKey{repo.StateDir, wire.Sum(headRaw), *head.LastReceiptSha256, intent.DigestOfFiles(intents), wire.Digest(fmt.Sprintf("%x", h.Sum(nil))), guard.Membership()}, nil
}

// A possible hit selects the conservative inventory-first path. This hint
// grants nothing: leaseAuditObserved still compares the complete physical key.
func possibleLeaseAudit(repo *intent.Repository, headRaw []byte) bool {
	leaseAudits.Lock()
	defer leaseAudits.Unlock()
	return leaseAudits.proof != nil && leaseAudits.key.root == repo.StateDir && leaseAudits.key.head == wire.Sum(headRaw)
}

func leaseAudit(repo *intent.Repository, guard *authority.ChangeGuard, inv *transaction.Inventory, headRaw []byte) (*journal.Result, error) {
	proof, err, _ := leaseAuditObserved(repo, guard, inv, headRaw, nil, func(r journal.Reader) (*journal.Result, journal.PhysicalObservation, error) {
		proof, err := r.AuditForWrite()
		return proof, journal.PhysicalObservation{}, err
	})
	return proof, err
}

// Cleanup is a separate return channel because a changed guard must never
// transform failed native closure into a retryable ordinary observation.
func leaseAuditObserved(repo *intent.Repository, guard *authority.ChangeGuard, inv *transaction.Inventory, headRaw []byte, supplied *journal.Result, audit func(journal.Reader) (*journal.Result, journal.PhysicalObservation, error)) (*journal.Result, error, error) {
	key, err := leaseKey(repo, guard, inv, headRaw)
	if err != nil {
		return nil, err, nil
	}
	proof := supplied
	if proof == nil {
		leaseAudits.Lock()
		cached := leaseAudits.proof
		hit := cached != nil && leaseAudits.key == key
		leaseAudits.Unlock()
		if hit {
			return cached, nil, nil
		}
		head, err := snapshot.DecodeHead(headRaw)
		if err != nil {
			return nil, err, nil
		}
		var observation journal.PhysicalObservation
		proof, observation, err = audit(journalReader(repo, head))
		if err != nil || observation.Cleanup != nil {
			return proof, err, observation.Cleanup
		}
	}
	if proof.Identity.HeadSha256 != key.head || proof.Identity.IntentTreeSha256 != key.intent {
		return nil, wire.Errorf(wire.CodeSnapshotMoved, "audit cache", "inventory differs from audit"), nil
	}
	if err := guard.Check(); err != nil {
		return nil, err, nil
	}
	if !proof.StagingPresent && !proof.Pending {
		leaseAudits.Lock()
		leaseAudits.key, leaseAudits.proof = key, proof
		leaseAudits.Unlock()
	}
	return proof, nil, nil
}

// prepareLease audits the journal outside the writer lock and plans request.
// resumable selects the bounded preparation audit (PSR-V0-050, proposed):
// one complete attempt and, when the journal moved under it, the prefix walk
// to the head it captured. A complete or prefix result is kept as the round's
// resume base. A moved journal ends the round as SNAPSHOT_MOVED without the
// second complete audit; a declined prefix walk and every other result keep
// the complete audit below. A change guard overtaken while it registered
// cannot serve the complete route, but a resumable round still runs the
// bounded audit for its resume base (PSR-V0-054, proposed): the audit checks
// its own captures and the resumed write needs no change guard, so a journal
// that moves faster than the guard registers still makes progress.
func prepareLease(ctx context.Context, repo *intent.Repository, request transaction.Request, now wire.Timestamp, facts claimObserver, resumable bool) (prepared *preparedLease, err error) {
	hooks := hooksForInventory(ctx)
	g, err := hooks.watch(repo)
	guardMoved := false
	if err != nil {
		if !resumable || !journal.ResumableAudits() || wire.CodeOf(err) != wire.CodeSnapshotMoved {
			return nil, err
		}
		g, guardMoved = nil, true
	}
	p := &preparedLease{guard: g}
	timing, start, readEnd := leaseTimingOf(ctx), time.Now(), time.Time{}
	defer func() { timing.prepared(start, readEnd) }()
	defer func() {
		if err != nil || len(p.fatalCleanup) > 0 {
			p.failure = err
			p.observationFailure = g.Check()
			// Keep ordinary failures for the writer-locked recheck. Cleanup ownership
			// is independent: a dirty observation cannot make a failed close retryable.
			prepared, err = p, nil
		}
	}()
	head, barrier, reservations, err := journalBytes(repo)
	if err != nil {
		return nil, err
	}
	p.head = head
	var inv *transaction.Inventory
	var supplied *journal.Result
	if guardMoved && possibleLeaseAudit(repo, head) {
		return p, wire.Errorf(wire.CodeSnapshotMoved, "change guard", "store changed while the guard registered")
	}
	decoded, err := snapshot.DecodeHead(head)
	if err != nil {
		return nil, err
	}
	// A journal at or above the checkpoint floor gets the bounded complete
	// audit (PSR-V0-150): one attempt per round that stops once head.json
	// moves. Every audit's walks are charged to the timing split (PSR-V0-151).
	bounded := journal.ResumableAudits() && decoded.LastSeq.Uint64() >= writerCheckpointMin(repo)
	observe := func(r journal.Reader) (*journal.Result, journal.PhysicalObservation, error) {
		r.StopOnMove = bounded && !r.PrefixFallback
		proof, observation, auditErr := hooks.audit(r)
		timing.walked(observation.Walks)
		return proof, observation, auditErr
	}
	if !possibleLeaseAudit(repo, head) {
		// The writer checkpoint this audit derives is bound to the
		// invalidation token read before it (CAL-V0-117, proposed).
		p.writerToken = writerInvalidation(repo)
		reader := journalReader(repo, decoded)
		fallback := resumable && bounded
		if guardMoved && !fallback {
			return p, wire.Errorf(wire.CodeSnapshotMoved, "change guard", "store changed while the guard registered")
		}
		reader.PrefixFallback = fallback
		proof, observation, auditErr := observe(reader)
		if observation.Cleanup != nil {
			p.recordCleanup("journal audit close", observation.Cleanup)
			return p, auditErr
		}
		if !fallback && wire.CodeOf(auditErr) == wire.CodeSnapshotMoved {
			// The journal moved under the complete audit: the next round
			// samples it again (PSR-V0-150). The guarded audit below would
			// only walk it once more against a head this round already lost.
			readEnd = time.Now()
			return p, auditErr
		}
		if fallback {
			switch {
			case errors.Is(auditErr, journal.ErrPrefixDeclined):
				// The complete audit below derives the refusal itself.
				p.noResume = true
				if guardMoved {
					readEnd = time.Now()
					return p, auditErr
				}
			case auditErr == nil && proof != nil && proof.Mode == journal.ModePrefix:
				p.base, p.baseObs = proof, observation
				readEnd = time.Now()
				return p, wire.Errorf(wire.CodeSnapshotMoved, "receipts", "journal moved during the complete attempt; resuming from the audited prefix")
			case wire.CodeOf(auditErr) == wire.CodeSnapshotMoved:
				readEnd = time.Now()
				return p, auditErr
			case guardMoved:
				// Only the guarded complete route below decides a refusal.
				if auditErr == nil && proof != nil && proof.Mode == journal.ModeFull && !proof.Pending && !proof.StagingPresent && proof.IntentError == nil {
					p.base, p.baseObs = proof, observation
				}
				readEnd = time.Now()
				return p, wire.Errorf(wire.CodeSnapshotMoved, "change guard", "store changed while the guard registered; resuming from the audit")
			}
		}
		if auditErr == nil && proof != nil && proof.Mode == journal.ModeFull && !proof.Pending && !proof.StagingPresent && proof.IntentError == nil && observation.Files != nil {
			if fallback {
				p.base, p.baseObs = proof, observation
			}
			inv, err, p.fatalCleanup = guardedLeaseInventory(repo, g, hooks, observation.Files)
			if err != nil || len(p.fatalCleanup) > 0 {
				return p, err
			}
			supplied = proof
			p.writer, _ = proof.WriterCheckpoint(observation.Files)
			p.writerProof, p.writerPhys = proof, observation.Files
		}
		// Partial, divergent or failed audits supply no inventory metadata.
		// Preserve the ordinary inventory-first replay/recovery path below.
	}
	if inv == nil {
		inv, err, p.fatalCleanup = guardedLeaseInventory(repo, g, hooks)
		if err != nil || len(p.fatalCleanup) > 0 {
			return p, err
		}
	}
	var cleanup error
	p.proof, err, cleanup = leaseAuditObserved(repo, g, inv, head, supplied, observe)
	readEnd = time.Now()
	if cleanup != nil {
		p.recordCleanup("journal audit close", cleanup)
		return p, err
	}
	if wire.CodeOf(err) == wire.CodeRedoPending && p.proof != nil && p.proof.Pending {
		p.pending = true
		return p, g.Check()
	}
	if err != nil {
		return nil, err
	}
	if p.proof.StagingPresent {
		return nil, wire.Errorf(wire.CodeUnsupported, "staging", "active staging requires settlement")
	}
	path, err := snapshot.RequestPath(request.RequestID)
	if err != nil {
		return nil, err
	}
	if p.proof.RetiredRequest(path) {
		return nil, wire.Errorf(wire.CodeUnsupported, path, "request was retired with the prefix; its receipt is in the retained archive")
	}
	if digest, found := p.proof.RequestDigests[path]; found {
		bound, err := snapshot.PostBound(path)
		if err != nil {
			return nil, err
		}
		raw, err := readLogical(repo, path, bound)
		if err != nil {
			return nil, err
		}
		if wire.Sum(raw) != digest {
			return nil, wire.Errorf(wire.CodeSnapshotMoved, path, "audited request changed")
		}
		rc, err := snapshot.DecodeRequest(raw)
		if err != nil {
			return nil, err
		}
		p.result = replayResult(request, rc.Entry)
		return p, g.Check()
	}
	if p.proof.IntentError != nil {
		return nil, p.proof.IntentError
	}
	p.branch, err = primaryBranch(repo)
	if err != nil {
		return nil, err
	}
	headRc, err := headReceipt(repo, head)
	if err != nil {
		return nil, err
	}
	// The plan is prepared before the lock; a head that moves afterwards
	// fails the commit as SNAPSHOT_MOVED and the next round samples again.
	now = recordedAt(ctx, now)
	input := transaction.Input{Inventory: inv, Head: head, HeadReceipt: headRc, Queue: p.proof.Records["intent/queue.json"].Raw, Policy: p.proof.Records["intent/policy.json"].Raw, Barrier: barrier, Reservations: reservations, Pools: p.proof.Records["pools.json"].Raw, Programs: p.proof.Records["programs.json"].Raw, Premise: transaction.LocalOperator, Branch: p.branch, Replay: transaction.ReplayObservation{State: "ABSENT"}, RecordedAt: now}
	for path, record := range p.proof.Records {
		if record.Raw == nil {
			continue
		}
		switch {
		case strings.HasPrefix(path, "intent/tickets/"):
			input.CanonicalTickets = append(input.CanonicalTickets, record.Raw)
			input.CanonicalTicketPaths = append(input.CanonicalTicketPaths, path)
		case strings.HasPrefix(path, "intent/releases/"):
			input.CanonicalReleases = append(input.CanonicalReleases, record.Raw)
		case strings.HasPrefix(path, "attempts/"):
			input.Attempts = append(input.Attempts, record.Raw)
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := leaseInput(p.proof, input.Attempts, facts, &input); err != nil {
		return nil, err
	}
	p.result = transaction.Model(request, input)
	if a := transaction.HandoffPolicyCandidate(request, input, p.result); a != nil {
		selector := journal.HandoffPolicySelector{AttemptID: a.AttemptID, Generation: a.Generation, OriginalPolicySha256: a.PolicySha256}
		if a.PoolAllocation != nil {
			selector.PoolID, selector.MemberID = a.PoolAllocation.PoolID, a.PoolAllocation.MemberID
		}
		// This fresh full scan is outside the lock, under the same guard. Never
		// mutate/cache the first Result or retain the interval's other blobs.
		history, err := journalReader(repo, p.proof.Head).AuditForHandoff(selector)
		if err != nil {
			return nil, err
		}
		if history.IntentError != nil {
			return nil, history.IntentError
		}
		if history.Pending || history.StagingPresent || history.Mode != journal.ModeFull || history.Identity != p.proof.Identity || history.LastSeq != p.proof.LastSeq || history.LastReceiptSha256 != p.proof.LastReceiptSha256 || history.Records["intent/policy.json"].Sha256 == nil || *history.Records["intent/policy.json"].Sha256 != wire.Sum(input.Policy) {
			return nil, wire.Errorf(wire.CodeSnapshotMoved, "handoff history", "additional audit differs from prepared snapshot")
		}
		if err := g.Check(); err != nil {
			return nil, err
		}
		h := history.HandoffPolicy
		if h == nil || h.OriginalPolicy.Sha256 == nil {
			// Missing provenance keeps the original STALE_POLICY refusal.
			return p, g.Check()
		}
		input.HandoffPolicy = &transaction.HandoffPolicyObservation{AttemptID: selector.AttemptID, Generation: selector.Generation, PoolID: selector.PoolID, MemberID: selector.MemberID, HeadSha256: history.Identity.HeadSha256, LastSeq: history.LastSeq, LastReceiptSha256: history.LastReceiptSha256, FinalPolicySha256: h.FinalPolicySha256, OriginalPath: "intent/policy.json", OriginalSeq: h.OriginalPolicy.Seq, OriginalSha256: *h.OriginalPolicy.Sha256, OriginalReceiptSha256: h.OriginalReceiptSha256, OriginalRaw: h.OriginalPolicy.Raw, FirstAttemptPath: "attempts/" + selector.AttemptID + ".json", FirstAttemptSeq: h.FirstAttemptSeq, FirstAttemptSha256: h.FirstAttemptSha256, FirstAttemptReceiptSha256: h.FirstAttemptReceiptSha256, FirstPolicySha256: h.FirstPolicySha256, FirstConfigSha256: h.FirstConfigSha256, FirstCapabilitySha256: h.FirstCapabilitySha256, FirstAllocationSha256: h.FirstAllocationSha256, Compatible: h.Compatible}
		// A long history scan must not hide expiry behind the earlier sample.
		input.RecordedAt = recordedAt(ctx, input.RecordedAt)
		p.result = transaction.Model(request, input)
	}
	return p, g.Check()
}

// leaseWrite carries immutable audit and model results to a bounded critical
// section. Contention invalidates the entire preparation, including refusals
// and replays. No fallback scans the store while holding the lock or the
// preparation gate (CAL-V0-263).
func leaseWrite(ctx context.Context, repo *intent.Repository, request transaction.Request, now wire.Timestamp, beforeCommit func() error, facts claimObserver) (report *Report, proof *journal.Result, err error) {
	report = &Report{}
	if repo == nil {
		return report, nil, wire.Errorf(wire.CodeMalformed, "repository", "missing repository")
	}
	if request.Actor.Role != "OWNER" && request.Actor.Role != "OPERATOR" {
		setLeaseReport(report, transaction.Model(request, transaction.Input{}))
		return report, nil, nil
	}
	if _, err := transaction.Digest(request); err != nil {
		return report, nil, err
	}
	if _, err := authority.Qualify(repo.CommonDir); err != nil {
		return report, nil, err
	}
	// The preparation gate serializes cooperating lease writers' bounded
	// critical sections only: the writer-checkpoint route and the locked
	// commit of a planned complete-route write (CAL-V0-263). The complete
	// audit, the model and every refusal it decides run before the gate, and
	// commitLease's locked recheck binds them, so a commit that moved the
	// head fails as SNAPSHOT_MOVED and the next round samples again. The gate
	// is not writer authority: noncooperating writers and settlement still
	// require every existing guarded observation and locked rebind below.
	timing := leaseTimingOf(ctx)
	timing.Transactions++
	// The scheduled checkpoint refresh runs after the preparation gate and
	// the writer lock are released (CAL-V0-117).
	refresh := false
	defer func() {
		if refresh {
			refreshWriterCheckpoint(ctx, repo)
		}
	}()
	hooks := hooksForInventory(ctx)
	progress := leaseProgressOf(ctx)
	// A round that ended SNAPSHOT_MOVED after a settled audit from receipt 1
	// resumes from it in the next round (PSR-V0-052, proposed). A declined
	// resume falls through to that round's preparation; a declined prefix
	// walk keeps the complete audit for the rest of the command.
	var resume *leaseResume
	noResume := !journal.ResumableAudits()
	// writerRoute runs leaseWriter under the preparation gate. done is set
	// when the route served or failed the write.
	writerRoute := func(from *leaseResume) (done bool, rep *Report, fast *journal.Result, err error) {
		var handled, due bool
		var routeErr error
		ran, gateErr := underPreparation(ctx, repo, hooks, timing, func() {
			handled, due, fast, routeErr = leaseWriter(ctx, repo, request, now, beforeCommit, facts, report, from)
		})
		if !ran {
			return true, report, nil, gateErr
		}
		if handled {
			if routeErr != nil {
				report, _, routeErr = guardFailureAudit(report, request.RequestID, routeErr)
				return true, report, nil, errors.Join(routeErr, gateErr)
			}
			if gateErr != nil {
				return true, report, nil, gateErr
			}
			refresh = due
			if from != nil {
				timing.Resumed++
			}
			return true, report, fast, nil
		}
		if gateErr != nil {
			return true, report, nil, gateErr
		}
		if from != nil {
			timing.ResumeDeclines++
		}
		return false, report, nil, nil
	}
	for round := 0; round < 16; round++ {
		if err := ctx.Err(); err != nil {
			return report, nil, err
		}
		timing.Rounds++
		guards := time.Now()
		// Guard-before-recovery is preserved; clean stores take no extra lock.
		head, err := writerGuards(repo, guardOperation(request))
		if err != nil {
			timing.Guards += time.Since(guards)
			return guardFailureAudit(report, request.RequestID, err)
		}
		if head.QueueID.Raw != request.QueueID {
			timing.Guards += time.Since(guards)
			return report, nil, wire.Errorf(wire.CodeOutOfScope, "queueId", "request queue differs")
		}
		err = clearLeaseOrphans(ctx, repo, guardOperation(request))
		timing.Guards += time.Since(guards)
		if err != nil {
			return guardFailureAudit(report, request.RequestID, err)
		}
		// CAL-V0-116: a retained writer checkpoint serves the write under one
		// lock; every decline prepares the complete route below, outside the
		// gate (CAL-V0-263).
		if writerRouteReady(repo, request) {
			if done, rep, fast, err := writerRoute(nil); done {
				return rep, fast, err
			}
		}
		if resume != nil {
			from := resume
			resume = nil
			if done, rep, fast, err := writerRoute(from); done {
				return rep, fast, err
			}
			// A declined resume committed nothing; this round's
			// preparation audit derives a fresh base or decides.
		}
		p, err := prepareLease(ctx, repo, request, now, facts, !noResume)
		progress(*timing)
		if p != nil && len(p.fatalCleanup) > 0 {
			p.recordCleanup("change guard close", hooks.closeGuard(p.guard))
			return report, nil, p.fatalError()
		}
		if err != nil {
			if wire.CodeOf(err) == wire.CodeSnapshotMoved || os.IsNotExist(err) {
				continue
			}
			return guardFailureAudit(report, request.RequestID, err)
		}
		// The sweep re-reads over-budget files outside the lock and the gate;
		// the locked checks in commitLease only poll events, so guard work
		// under either does not grow with the store.
		hooks.commitStage("sweep")
		p.guard.Sweep()
		var closeErr error
		commit := func() {
			err = commitLease(ctx, repo, request, p, report, beforeCommit)
			monitor := time.Now()
			closeErr = hooks.closeGuard(p.guard) // thousands of descriptors, always unlocked
			timing.MonitorClose += time.Since(monitor)
		}
		var gateErr error
		if p.pending || (p.result.Kind == "Transaction" && p.result.Plan != nil) {
			var ran bool
			if ran, gateErr = underPreparation(ctx, repo, hooks, timing, commit); !ran {
				p.recordCleanup("change guard close", hooks.closeGuard(p.guard))
				if len(p.fatalCleanup) > 0 {
					return report, nil, p.fatalError()
				}
				return report, nil, gateErr
			}
		} else {
			// A replay or refusal writes nothing: decided before the gate, it
			// is reported under the writer lock without entering it.
			commit()
		}
		if len(p.fatalCleanup) > 0 {
			p.recordCleanup("change guard close", closeErr)
			p.recordCleanup("preparation gate close", gateErr)
			return report, nil, p.fatalError()
		}
		if closeErr != nil || gateErr != nil {
			return report, nil, errors.Join(err, closeErr, gateErr)
		}
		if wire.CodeOf(err) == wire.CodeSnapshotMoved {
			declined := noResume
			resume = p.resumeFrom(&noResume)
			if noResume && !declined {
				timing.ResumeDeclines++
			}
			continue
		}
		if err != nil {
			return guardFailureAudit(report, request.RequestID, err)
		}
		if p.pending {
			continue
		}
		return report, p.proof, nil
	}
	return report, nil, wire.Errorf(wire.CodeSnapshotMoved, "lease", "store changed during all preparation attempts")
}

func setLeaseReport(report *Report, result transaction.Result) {
	report.Outcome, report.Coverage, report.Detail, report.Kind = result.Outcome, result.Coverage, result.Detail, result.Kind
	report.AttemptID, report.Generation, report.Expired = result.AttemptID, result.Generation, result.Expired
	report.Escalation = result.Escalation
	report.AuthorExclusion = result.AuthorExclusion
}

func commitLease(ctx context.Context, repo *intent.Repository, request transaction.Request, p *preparedLease, report *Report, beforeCommit func() error) (err error) {
	if len(p.fatalCleanup) > 0 {
		return p.fatalError()
	}
	// The caller swept the guard before the gate (CAL-V0-263).
	hooksForInventory(ctx).commitStage("lock")
	timing, wait := leaseTimingOf(ctx), time.Now()
	lock, err := authority.AcquireLock(ctx, repo, authority.LockOptions{CallerWait: leaseLockWaitOf(ctx)})
	timing.LockWait += time.Since(wait)
	if err != nil {
		return err
	}
	held := time.Now()
	defer func() {
		cleanup := lock.Close()
		timing.LockHold += time.Since(held)
		if cleanup != nil {
			err = errors.Join(err, cleanup)
		}
	}()
	head, err := writerGuards(repo, guardOperation(request))
	if err != nil {
		return err
	}
	if p.branch != "" {
		branch, err := primaryBranch(repo)
		if err != nil {
			return err
		}
		if branch != p.branch {
			return wire.Errorf(wire.CodeSnapshotMoved, "branch", "prepared branch changed")
		}
	}
	if err := p.guard.CheckEvents(); err != nil {
		return err
	}
	if p.failure != nil {
		return p.failure
	}
	if head.QueueID.Raw != request.QueueID {
		return wire.Errorf(wire.CodeOutOfScope, "queueId", "request queue differs")
	}
	current, err := intent.ReadFile(filepath.Join(repo.StateDir, "head.json"), wire.MaxJournalHeadBytes)
	if err != nil {
		return err
	}
	if wire.Sum(current) != wire.Sum(p.head) {
		return wire.Errorf(wire.CodeSnapshotMoved, "head", "prepared head changed")
	}
	// The lock is held and the head is the audited one. Read verbs share
	// leaseAudit, so the checkpoint is retained here and never there.
	retainCheckpoint(repo, p.proof)
	if p.writerProof != nil {
		p.writerToken = verifyWriterCheckpoint(ctx, repo, p.writerToken, func(old *journal.WriterCheckpoint) (bool, error) {
			return p.writerProof.VerifyWriterCheckpoint(old, p.writerPhys)
		})
	}
	retainWriterCheckpoint(repo, p.writer, p.writerToken)
	if !p.pending && (p.result.Kind != "Transaction" || p.result.Plan == nil) {
		setLeaseReport(report, p.result)
		return nil
	}
	write := time.Now()
	session, err := authority.NewSession(repo, lock)
	if err != nil {
		timing.wrote(write, 0)
		return err
	}
	defer func() {
		cleanup := session.Close()
		timing.wrote(write, session.SyncDuration())
		if cleanup != nil {
			err = errors.Join(err, cleanup)
		}
	}()
	if p.pending {
		if err := redoLeaseProof(repo, session, p.proof); err != nil {
			return err
		}
		report.Redone = true
		return nil
	}
	if err := p.guard.CheckEvents(); err != nil {
		return err
	}
	setLeaseReport(report, p.result)
	report.Receipt, err = applyBeforeCommit(repo, session, p.result.Plan, beforeCommit)
	return err
}

// underPreparation runs f holding the preparation gate (CAL-V0-263). ran is
// false, with f not run, when admission failed; err is then the admission
// error, and otherwise the gate close error. Only bounded work runs in f: the
// writer-checkpoint route, or the locked commit of a prepared write and its
// monitor teardown, so the gate is released after the monitor as before.
func underPreparation(ctx context.Context, repo *intent.Repository, hooks inventoryHooks, timing *LeaseTiming, f func()) (ran bool, err error) {
	admission := time.Now()
	gate, err := authority.AcquirePreparation(ctx, repo, authority.LockOptions{CallerWait: leaseLockWaitOf(ctx)})
	timing.AdmissionWait += time.Since(admission)
	if err != nil {
		return false, err
	}
	f()
	return true, hooks.closePreparation(gate)
}
