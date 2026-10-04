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

func prepareLease(ctx context.Context, repo *intent.Repository, request transaction.Request, now wire.Timestamp, facts claimObserver) (prepared *preparedLease, err error) {
	g, err := authority.WatchChanges(repo)
	if err != nil {
		return nil, err
	}
	p := &preparedLease{guard: g}
	defer func() {
		if err != nil || len(p.fatalCleanup) > 0 {
			p.failure = err
			p.observationFailure = g.Check()
			// Keep ordinary failures for the writer-locked recheck. Cleanup ownership
			// is independent: a dirty observation cannot make a failed close retryable.
			prepared, err = p, nil
		}
	}()
	hooks := hooksForInventory(ctx)
	head, barrier, reservations, err := journalBytes(repo)
	if err != nil {
		return nil, err
	}
	p.head = head
	var inv *transaction.Inventory
	var supplied *journal.Result
	if !possibleLeaseAudit(repo, head) {
		decoded, decodeErr := snapshot.DecodeHead(head)
		if decodeErr != nil {
			return nil, decodeErr
		}
		proof, observation, auditErr := hooks.audit(journalReader(repo, decoded))
		if observation.Cleanup != nil {
			p.recordCleanup("journal audit close", observation.Cleanup)
			return p, auditErr
		}
		if auditErr == nil && proof != nil && proof.Mode == journal.ModeFull && !proof.Pending && !proof.StagingPresent && proof.IntentError == nil && observation.Files != nil {
			inv, err, p.fatalCleanup = guardedLeaseInventory(repo, g, hooks, observation.Files)
			if err != nil || len(p.fatalCleanup) > 0 {
				return p, err
			}
			supplied = proof
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
	p.proof, err, cleanup = leaseAuditObserved(repo, g, inv, head, supplied, hooks.audit)
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
	if digest, found := p.proof.RequestDigests[path]; found {
		bound, err := snapshot.PostBound(path)
		if err != nil {
			return nil, err
		}
		raw, err := intent.ReadFile(filepath.Join(repo.StateDir, path), bound)
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
// and replays. No fallback scans the store while holding the lock.
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
	// Serializing cooperating preparations prevents each successful lease commit
	// from discarding other lease writers' full inventory/audit work. This gate is
	// not writer authority: noncooperating writers and settlement still require
	// every existing guarded observation and locked rebind below.
	preparation, err := authority.AcquirePreparation(ctx, repo, authority.LockOptions{})
	if err != nil {
		return report, nil, err
	}
	hooks := hooksForInventory(ctx)
	var terminal *preparedLease
	defer func() {
		closeErr := hooks.closePreparation(preparation)
		if terminal != nil {
			terminal.recordCleanup("preparation gate close", closeErr)
			proof, err = nil, terminal.fatalError()
		} else if closeErr != nil {
			err = errors.Join(err, closeErr)
		}
	}()
	for round := 0; round < 16; round++ {
		if err := ctx.Err(); err != nil {
			return report, nil, err
		}
		// Guard-before-recovery is preserved; clean stores take no extra lock.
		head, err := writerGuards(repo, guardOperation(request))
		if err != nil {
			return guardFailureAudit(report, request.RequestID, err)
		}
		if head.QueueID.Raw != request.QueueID {
			return report, nil, wire.Errorf(wire.CodeOutOfScope, "queueId", "request queue differs")
		}
		if err := clearLeaseOrphans(ctx, repo, guardOperation(request)); err != nil {
			return guardFailureAudit(report, request.RequestID, err)
		}
		p, err := prepareLease(ctx, repo, request, now, facts)
		if p != nil && len(p.fatalCleanup) > 0 {
			terminal = p
			p.recordCleanup("change guard close", hooks.closeGuard(p.guard))
			return report, nil, p.fatalError()
		}
		if err != nil {
			if wire.CodeOf(err) == wire.CodeSnapshotMoved || os.IsNotExist(err) {
				continue
			}
			return guardFailureAudit(report, request.RequestID, err)
		}
		err = commitLease(ctx, repo, request, p, report, beforeCommit)
		closeErr := hooks.closeGuard(p.guard) // thousands of descriptors, always unlocked
		if len(p.fatalCleanup) > 0 {
			terminal = p
			p.recordCleanup("change guard close", closeErr)
			return report, nil, p.fatalError()
		}
		if closeErr != nil {
			return report, nil, errors.Join(err, closeErr)
		}
		if wire.CodeOf(err) == wire.CodeSnapshotMoved {
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
}

func commitLease(ctx context.Context, repo *intent.Repository, request transaction.Request, p *preparedLease, report *Report, beforeCommit func() error) (err error) {
	if len(p.fatalCleanup) > 0 {
		return p.fatalError()
	}
	lock, err := authority.AcquireLock(ctx, repo, authority.LockOptions{})
	if err != nil {
		return err
	}
	defer func() {
		if cleanup := lock.Close(); cleanup != nil {
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
	if err := p.guard.Check(); err != nil {
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
	if !p.pending && (p.result.Kind != "Transaction" || p.result.Plan == nil) {
		setLeaseReport(report, p.result)
		return nil
	}
	session, err := authority.NewSession(repo, lock)
	if err != nil {
		return err
	}
	defer func() {
		if cleanup := session.Close(); cleanup != nil {
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
	if err := p.guard.Check(); err != nil {
		return err
	}
	setLeaseReport(report, p.result)
	report.Receipt, err = applyBeforeCommit(repo, session, p.result.Plan, beforeCommit)
	return err
}
