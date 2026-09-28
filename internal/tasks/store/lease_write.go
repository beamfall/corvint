package store

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
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
	guard   *authority.ChangeGuard
	proof   *journal.Result
	head    []byte
	branch  string
	result  transaction.Result
	pending bool
}

func leaseAudit(repo *intent.Repository, guard *authority.ChangeGuard, inv *transaction.Inventory, headRaw []byte) (*journal.Result, error) {
	head, err := snapshot.DecodeHead(headRaw)
	if err != nil {
		return nil, err
	}
	if head.LastReceiptSha256 == nil {
		return nil, wire.Errorf(wire.CodeJournalForked, "head", "missing receipt digest")
	}
	h := sha256.New()
	var intents []intent.File
	for _, file := range inv.Files() {
		fmt.Fprintf(h, "%s\x00%s\x00%s\n", file.Path, file.Sha256, file.Bytes)
		if strings.HasPrefix(file.Path, "intent/") {
			intents = append(intents, intent.File{Path: strings.TrimPrefix(file.Path, "intent/"), Sha256: file.Sha256, Bytes: int(file.Bytes.Uint64())})
		}
	}
	key := leaseAuditKey{repo.StateDir, wire.Sum(headRaw), *head.LastReceiptSha256, intent.DigestOfFiles(intents), wire.Digest(fmt.Sprintf("%x", h.Sum(nil))), guard.Membership()}
	leaseAudits.Lock()
	proof := leaseAudits.proof
	hit := proof != nil && leaseAudits.key == key
	leaseAudits.Unlock()
	if hit {
		return proof, nil
	}
	proof, err = journalReader(repo, head).AuditForWrite()
	if err != nil {
		return proof, err
	}
	if proof.Identity.HeadSha256 != key.head || proof.Identity.IntentTreeSha256 != key.intent {
		return nil, wire.Errorf(wire.CodeSnapshotMoved, "audit cache", "inventory differs from audit")
	}
	if err := guard.Check(); err != nil {
		return nil, err
	}
	if !proof.StagingPresent && !proof.Pending {
		leaseAudits.Lock()
		leaseAudits.key, leaseAudits.proof = key, proof
		leaseAudits.Unlock()
	}
	return proof, nil
}

func prepareLease(ctx context.Context, repo *intent.Repository, request transaction.Request, now wire.Timestamp, facts claimObserver) (_ *preparedLease, err error) {
	g, err := authority.WatchChanges(repo)
	if err != nil {
		return nil, err
	}
	p := &preparedLease{guard: g}
	defer func() {
		if err != nil {
			if moved := g.Check(); moved != nil {
				err = moved
			}
			if cleanup := g.Close(); cleanup != nil {
				err = errors.Join(err, cleanup)
			}
		}
	}()
	inv, err := inventory(repo)
	if err != nil {
		return nil, err
	}
	head, barrier, reservations, err := journalBytes(repo)
	if err != nil {
		return nil, err
	}
	p.head = head
	p.proof, err = leaseAudit(repo, g, inv, head)
	if wire.CodeOf(err) == wire.CodeRedoPending && p.proof != nil && p.proof.Pending {
		p.pending = true
		return p, g.Check()
	}
	if err != nil {
		return nil, err
	}
	if p.proof.StagingPresent {
		return nil, wire.Errorf(wire.CodeSnapshotMoved, "staging", "orphan staging requires settlement")
	}
	path, err := snapshot.RequestPath(request.RequestID)
	if err != nil {
		return nil, err
	}
	if raw := p.proof.Records[path].Raw; raw != nil {
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
	input := transaction.Input{Inventory: inv, Head: head, HeadReceipt: headRc, Queue: p.proof.Records["intent/queue.json"].Raw, Policy: p.proof.Records["intent/policy.json"].Raw, Barrier: barrier, Reservations: reservations, Premise: transaction.LocalOperator, Branch: p.branch, Replay: transaction.ReplayObservation{State: "ABSENT"}, RecordedAt: now}
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
	return p, g.Check()
}

// leaseWrite carries immutable audit and model results to a bounded critical
// section. Contention invalidates the entire preparation, including refusals
// and replays. No fallback scans the store while holding the lock.
func leaseWrite(ctx context.Context, repo *intent.Repository, request transaction.Request, now wire.Timestamp, beforeCommit func() error, facts claimObserver) (*Report, *journal.Result, error) {
	report := &Report{}
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
		if err != nil {
			if wire.CodeOf(err) == wire.CodeSnapshotMoved || os.IsNotExist(err) {
				continue
			}
			return guardFailureAudit(report, request.RequestID, err)
		}
		err = commitLease(ctx, repo, request, p, report, beforeCommit)
		closeErr := p.guard.Close() // thousands of descriptors, always unlocked
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
	if head.QueueID.Raw != request.QueueID {
		return wire.Errorf(wire.CodeOutOfScope, "queueId", "request queue differs")
	}
	if wire.Sum(wire.EncodeFile(head.Value())) != wire.Sum(p.head) {
		return wire.Errorf(wire.CodeSnapshotMoved, "head", "prepared head changed")
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
