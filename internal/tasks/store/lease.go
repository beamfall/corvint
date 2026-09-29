package store

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/journal"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// ScopeDeriver is the CAL-V0-022 derivation: the paths a ticket's title and
// body touch at baseTree, the SHA-256 of the derivation's inputs, and
// ok=false when the index is absent, stale or abstains. It runs in process
// before the store lock and must not write outside the task store.
type ScopeDeriver func(ctx context.Context, root, baseTree, title, body string) (paths []string, inputSha256 string, ok bool)

// NoScopeDeriver always abstains, so an undeclared claim is WHOLE_REPOSITORY.
func NoScopeDeriver(context.Context, string, string, string, string) ([]string, string, bool) {
	return nil, "", false
}

// LeaseChoice is one lease command. Root is the checkout whose HEAD a claim
// without --base starts from; Derive is nil for NoScopeDeriver.
type LeaseChoice struct {
	QueueID, RequestID, Root string
	Lease                    transaction.LeaseRequest
	Derive                   ScopeDeriver
	gate                     *gateRun
	pool                     transaction.PoolFacts
}

// claimObserver computes claim facts from the guarded audit before locking.
type claimObserver func(*journal.Result, *transaction.Input) (transaction.LeaseFacts, error)

// Lease commits one lease command. A claim or reap survey the model answers
// with expired leases reaps each one in its own transaction and receipt,
// then a claim is retried (CAL-V0-011).
func Lease(ctx context.Context, repo *intent.Repository, actor mutation.Binding, choice LeaseChoice, now wire.Timestamp) (*Report, error) {
	reaped := []transaction.ExpiredLease{}
	for round := 0; round <= wire.MaxActiveAttempts; round++ {
		report, err := leaseOnce(ctx, repo, actor, choice, now)
		if err == nil && (choice.Lease.Verb == transaction.LeaseClaim || choice.Lease.Verb == transaction.LeaseClaimNext) && choice.Lease.Pool != "" && report.Outcome.HasCode(wire.CodeQuiescenceUnproved) {
			report, err = healthClaim(ctx, repo, actor, choice, report)
		}
		if err != nil || len(report.Expired) == 0 {
			report.Reaped = reaped
			return report, claimedTicket(repo, choice.Lease.Verb, report, err)
		}
		for _, x := range report.Expired {
			done, err := reapOne(ctx, repo, actor, choice.QueueID, x, now)
			if err != nil {
				return report, err
			}
			if done {
				reaped = append(reaped, x)
			}
		}
		if choice.Lease.Verb == transaction.LeaseReap {
			report.Reaped = reaped
			return report, nil
		}
	}
	return &Report{Reaped: reaped}, wire.Errorf(wire.CodeLimitExceeded, "claim", "expired leases kept blocking the claim")
}

func leaseOnce(ctx context.Context, repo *intent.Repository, actor mutation.Binding, choice LeaseChoice, now wire.Timestamp) (*Report, error) {
	lease := choice.Lease
	request := transaction.Request{Operation: transaction.Lease, QueueID: choice.QueueID, RequestID: choice.RequestID, Actor: actor, Lease: &lease}
	facts := gateFacts(repo, choice)
	if strings.HasPrefix(lease.Verb, "POOL_") {
		facts = poolFacts(repo, choice)
	}
	if lease.Verb == transaction.LeaseClaim || lease.Verb == transaction.LeaseClaimNext {
		facts = claimFacts(ctx, repo, choice)
	}
	report, _, err := administrativeWriteWith(ctx, repo, request, now, nil, facts)
	if err != nil || report.Kind != "Replay" || report.Outcome.ReceiptSeq == nil {
		return report, err
	}
	return report, replayedAttempt(repo, report)
}

// claimedTicket names the ticket a completed claim holds, which `claim
// --next` did not name. An attempt's ticket never changes, so reading its
// record after the commit is enough.
func claimedTicket(repo *intent.Repository, verb string, report *Report, err error) error {
	if err != nil || report.AttemptID == "" || (verb != transaction.LeaseClaim && verb != transaction.LeaseClaimNext) {
		return err
	}
	if report.Outcome.ReceiptSeq == nil {
		return wire.Errorf(wire.CodeMissingEvidence, "claim", "receipt binding absent")
	}
	rc, err := readReceipt(repo, report.Outcome.ReceiptSeq.Uint64())
	if err != nil {
		return err
	}
	var raw []byte
	for _, post := range rc.Post {
		if post.Path != "attempts/"+report.AttemptID+".json" {
			continue
		}
		if post.Record != nil {
			raw = wire.EncodeFile(*post.Record)
		} else if post.BlobSha256 != nil {
			raw, err = intent.ReadFile(filepath.Join(repo.StateDir, "evidence", string(*post.BlobSha256)), wire.MaxAttemptRecordBytes)
			if err != nil {
				return err
			}
		}
		if post.Sha256 == nil || wire.Sum(raw) != *post.Sha256 {
			return wire.Errorf(wire.CodeJournalForked, "claim", "receipt attempt digest differs")
		}
	}
	a, err := snapshot.DecodeAttempt(raw)
	if err != nil {
		return err
	}
	if a.AttemptID != report.AttemptID || a.Generation != report.Generation {
		return wire.Errorf(wire.CodeFenced, "claim", "receipt generation differs")
	}
	report.Ticket = a.TicketID.Raw
	report.PoolAllocation = a.PoolAllocation
	return nil
}

// reapID is the deterministic request id of one reap, so a repeated reap of
// the same generation replays.
func reapID(x transaction.ExpiredLease) string {
	sum := sha256.Sum256([]byte(x.AttemptID + ":" + string(x.Generation)))
	return "reap-" + hex.EncodeToString(sum[:16])
}

func reapOne(ctx context.Context, repo *intent.Repository, actor mutation.Binding, queueID string, x transaction.ExpiredLease, now wire.Timestamp) (bool, error) {
	choice := LeaseChoice{QueueID: queueID, RequestID: reapID(x), Lease: transaction.LeaseRequest{Verb: transaction.LeaseReap, AttemptID: x.AttemptID, Generation: x.Generation}}
	report, err := leaseOnce(ctx, repo, actor, choice, now)
	if err != nil {
		return false, err
	}
	return report.Kind == "Transaction" && report.Outcome.Outcome == mutation.OutcomeCompleted, nil
}

// replayedAttempt reads the attempt a replayed lease command named from its
// original receipt.
func replayedAttempt(repo *intent.Repository, report *Report) error {
	rc, err := readReceipt(repo, report.Outcome.ReceiptSeq.Uint64())
	if err != nil {
		return err
	}
	if rc.AttemptID != nil && rc.Generation != nil {
		report.AttemptID, report.Generation = *rc.AttemptID, *rc.Generation
	}
	return nil
}

func readReceiptBytes(repo *intent.Repository, seq uint64) ([]byte, error) {
	name, err := snapshot.ReceiptName(seq)
	if err != nil {
		return nil, err
	}
	return intent.ReadFile(filepath.Join(repo.StateDir, "receipts", name), wire.MaxReceiptFileBytes)
}

func readReceipt(repo *intent.Repository, seq uint64) (*snapshot.Receipt, error) {
	raw, err := readReceiptBytes(repo, seq)
	if err != nil {
		return nil, err
	}
	return snapshot.DecodeReceipt(raw)
}

// leaseInput adds the audited attempts and the claim facts to a Lease model
// input.
func leaseInput(proof *journal.Result, attempts [][]byte, facts claimObserver, input *transaction.Input) error {
	input.Attempts = attempts
	if facts == nil {
		return nil
	}
	var err error
	input.LeaseFacts, err = facts(proof, input)
	return err
}

func mintAttemptID(queueID string) (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return "attempt:" + strings.TrimPrefix(queueID, "queue:") + ":" + hex.EncodeToString(b[:]), nil
}

// resolveObject names the full object id of rev^{kind} in root.
func resolveObject(root, rev, kind string) (string, error) {
	out, err := gitOutput(root, "rev-parse", "--verify", "--end-of-options", rev+"^{"+kind+"}")
	if err != nil {
		return "", err
	}
	return wire.ParseOID("base", strings.TrimSpace(string(out)))
}

func claimedRecord(proof *journal.Result, ticketID string) *ticket.Record {
	id, err := wire.ParseTicketID("ticketId", ticketID)
	if err != nil {
		return nil
	}
	rec, err := ticket.Decode(proof.Records["intent/tickets/"+id.Local+".json"].Raw)
	if err != nil {
		return nil
	}
	return rec
}

// leaseRoot is the checkout a lease command reads Git from: the caller's,
// else the primary worktree.
func leaseRoot(repo *intent.Repository, choice LeaseChoice) string {
	if choice.Root == "" {
		return repo.PrimaryWorktree
	}
	return choice.Root
}

func claimFacts(ctx context.Context, repo *intent.Repository, choice LeaseChoice) claimObserver {
	return func(proof *journal.Result, input *transaction.Input) (transaction.LeaseFacts, error) {
		root := leaseRoot(repo, choice)
		rev := choice.Lease.Base
		if rev == "" {
			rev = "HEAD"
		}
		id, err := mintAttemptID(choice.QueueID)
		if err != nil {
			return transaction.LeaseFacts{}, err
		}
		base, err := resolveObject(root, rev, "commit")
		if err != nil {
			return transaction.LeaseFacts{}, err
		}
		facts := transaction.LeaseFacts{AttemptID: id, BaseCommit: base, Pool: choice.pool}
		if len(choice.pool.Observation) > 0 {
			o, e := snapshot.DecodePoolObservation(choice.pool.Observation)
			if e != nil {
				return facts, e
			}
			rev, tree, e := poolSource(root)
			if e != nil {
				return facts, e
			}
			if rev != o.Revision || tree != o.Tree {
				return facts, wire.Errorf(wire.CodeStaleTree, "pool claim", "command source changed before admission")
			}
		}
		policy, e := intent.DecodePolicy(proof.Records["intent/policy.json"].Raw)
		if e != nil {
			return facts, e
		}
		if pool := policy.Pool(choice.Lease.Pool); pool != nil {
			for _, member := range pool.Members {
				if e := poolConfig(root, pool.MemberConfig[member].ConfigRef); e != nil {
					return facts, e
				}
			}
		}
		rec := claimedRecord(proof, choice.Lease.TicketID)
		if choice.Lease.Verb == transaction.LeaseClaimNext {
			candidateInput := *input
			candidateInput.LeaseFacts = facts
			rec, err = transaction.NextClaimTicket(choice.QueueID, candidateInput, choice.Lease)
			if err != nil {
				return facts, err
			}
		}
		if rec == nil || choice.Lease.Scope != nil || len(transaction.Declared(rec)) > 0 {
			return facts, nil
		}
		tree, err := resolveObject(root, base, "tree")
		if err != nil {
			return transaction.LeaseFacts{}, err
		}
		facts.DerivedPaths, facts.DerivationSha256 = derive(ctx, choice.Derive, root, tree, rec)
		facts.DerivedTicketID = rec.TicketID.Raw
		return facts, nil
	}
}

// derive runs the deriver and keeps only a bounded, valid result; anything
// else is an abstention (CAL-V0-022).
func derive(ctx context.Context, deriver ScopeDeriver, root, tree string, rec *ticket.Record) ([]string, wire.Digest) {
	if deriver == nil {
		deriver = NoScopeDeriver
	}
	body := ""
	if rec.Body != nil {
		body = *rec.Body
	}
	paths, digest, ok := deriver(ctx, root, tree, rec.Title, body)
	if !ok || len(paths) == 0 || len(paths) > wire.MaxTouchPaths {
		return nil, ""
	}
	d, err := wire.ParseDigest("derivationSha256", digest)
	if err != nil {
		return nil, ""
	}
	sorted := uniqueSorted(paths)
	for _, p := range sorted {
		if !validScopePath(p) {
			return nil, ""
		}
	}
	return sorted, d
}

func validScopePath(p string) bool {
	_, pathErr := wire.ParsePath("scope", p)
	_, idErr := wire.ParseIdentifier("scope", p)
	return pathErr == nil && idErr == nil
}

func uniqueSorted(paths []string) []string {
	set := map[string]bool{}
	for _, p := range paths {
		set[p] = true
	}
	out := make([]string, 0, len(set))
	for p := range set {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}
