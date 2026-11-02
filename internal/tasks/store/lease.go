package store

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"path/filepath"
	"sort"
	"strings"
	"sync"

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
	// KnowHowRepos maps the repository aliases a claim resolves know-how
	// freshness against (KHN-V0-027); it never enters the lease request.
	KnowHowRepos map[string]string
	// PoolDetail is the PSR-V0-260 recovery detail document a `pool
	// observe` carries; its digest is Lease.Evidence.
	PoolDetail []byte
	// ScopeFromDiff is the revision whose tree, diffed against the claim's
	// base, a claim derives its PATH scope from (TCD-V0-174). Lease resolves
	// it to REV's full tree OID, which enters the lease request as
	// Lease.ScopeFromDiff (TCD-V0-181).
	ScopeFromDiff string
	gate          *gateRun
	pool          transaction.PoolFacts
}

// claimObserver computes claim facts from the guarded audit before locking.
type claimObserver func(*journal.Result, *transaction.Input) (transaction.LeaseFacts, error)

// Lease commits one lease command. A claim or reap survey the model answers
// with expired leases reaps each one in its own transaction and receipt,
// then a claim is retried (CAL-V0-011).
func Lease(ctx context.Context, repo *intent.Repository, actor mutation.Binding, choice LeaseChoice, now wire.Timestamp) (out *Report, outErr error) {
	unretryable := false
	defer func() {
		out, outErr = intentFix(repo, out, outErr)
		if unretryable {
			if out == nil {
				out = &Report{}
			}
			out.Unretryable = true
		}
	}()
	if (choice.Lease.Verb == transaction.LeaseClaim || choice.Lease.Verb == transaction.LeaseClaimNext) && choice.Lease.Base != "" {
		// ATR-V0-808: --base resolves any revision naming one commit, as
		// --scope-from-diff does, and the request records its full OID.
		if _, e := wire.ParseOID("base", choice.Lease.Base); e != nil {
			base, err := resolveObjectWith(gitOutput, leaseRoot(repo, choice), choice.Lease.Base, "commit")
			if err != nil {
				return nil, wire.Errorf(wire.CodeMalformed, "base", "--base %s does not name a commit in %s: %v; pass a revision `git rev-parse --verify %s^{commit}` resolves, or its full OID (ATR-V0-808)", choice.Lease.Base, leaseRoot(repo, choice), err, choice.Lease.Base)
			}
			choice.Lease.Base = base
		}
	}
	if choice.ScopeFromDiff != "" && choice.Lease.ScopeFromDiff == "" {
		// TCD-V0-181: the request names the exact tree, so a replay under
		// the same request id that names another tree refuses.
		tree, err := resolveObjectWith(gitOutput, leaseRoot(repo, choice), choice.ScopeFromDiff, "tree")
		if err != nil {
			return nil, scopeFromDiffUnresolved(choice.ScopeFromDiff, leaseRoot(repo, choice), err)
		}
		choice.Lease.ScopeFromDiff = tree
	}
	reaped := []transaction.ExpiredLease{}
	reapReceipts := []ReapReceipt{}
	for round := 0; round <= wire.MaxActiveAttempts; round++ {
		report, err := leaseOnce(ctx, repo, actor, choice, now)
		unretryable = unretryable || report != nil && report.Unretryable
		if err == nil && (choice.Lease.Verb == transaction.LeaseClaim || choice.Lease.Verb == transaction.LeaseClaimNext || choice.Lease.Verb == transaction.LeasePoolAcquire) && choice.Lease.Pool != "" && report.Outcome.HasCode(wire.CodeQuiescenceUnproved) {
			report, err = healthClaim(ctx, repo, actor, choice, report)
			// Reaping can replace this report in the next round (CAL-V0-215).
			unretryable = unretryable || report != nil && report.Unretryable
		}
		if err != nil || len(report.Expired) == 0 {
			report.Reaped = reaped
			report.ReapReceipts = reapReceipts
			if err == nil && (choice.Lease.Verb == transaction.LeaseSlotAcquire || choice.Lease.Verb == transaction.LeaseSlotRelease) {
				// PSR-V0-043: a replay reports no slot; the current state may
				// be a later request's grant.
				if report.Outcome.Replayed {
					return report, nil
				}
				return report, slotObservation(ctx, repo, choice.Lease, report)
			}
			if err == nil && (choice.Lease.Verb == transaction.LeasePoolAcquire || choice.Lease.Verb == transaction.LeasePoolRelease) {
				return report, pooledAttempt(ctx, repo, choice.Lease.AttemptID, report)
			}
			if err == nil && choice.Lease.Verb == transaction.LeaseWiden {
				return report, widenedScope(repo, choice, report)
			}
			return report, claimedTicket(repo, choice.Root, choice.KnowHowRepos, choice.Lease.Verb, report, err)
		}
		for _, x := range report.Expired {
			child, err := reapOne(ctx, repo, actor, choice.QueueID, x, now)
			unretryable = unretryable || child != nil && child.Unretryable
			if err != nil {
				return report, err
			}
			if child.Kind == "Transaction" && child.Outcome.Outcome == mutation.OutcomeCompleted {
				reaped = append(reaped, x)
				reapReceipts = append(reapReceipts, ReapReceipt{ExpiredLease: x, Receipt: child.Receipt})
			}
		}
		if choice.Lease.Verb == transaction.LeaseReap {
			report.Reaped = reaped
			report.ReapReceipts = reapReceipts
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
		probe := &identityProbe{}
		facts = poolFactsWith(ctx, repo, choice, probe, false)
		if lease.Verb == transaction.LeasePoolSafe && choice.pool.Identity == nil {
			// PSR-V0-168: the writer route observes under the writer lock,
			// so it gets an observer that never runs the identity command.
			settleCtx := ctx
			ctx = withWriterFacts(ctx, &writerFacts{
				observe: poolFactsWith(ctx, repo, choice, probe, true),
				settle:  func() bool { return probe.settle(settleCtx, repo, choice) },
			})
		}
	}
	if lease.Verb == transaction.LeaseClaim || lease.Verb == transaction.LeaseClaimNext {
		// V1-2138: the writer route observes a claim under the writer lock,
		// so it gets an observer that runs no Git command and no deriver.
		// The complete route observes Git afresh; a derivation depends only
		// on the exact tree, title and body, so it reuses the probe's.
		probe := newClaimProbe(ctx, repo, choice)
		facts = claimFactsWith(ctx, repo, choice, gitOutput, probe.scope(false, choice.Derive))
		settleCtx := ctx
		ctx = withWriterFacts(ctx, &writerFacts{
			observe: probe.observer(settleCtx, repo, choice),
			settle:  func() bool { return probe.settle(settleCtx, repo, choice) },
		})
	}
	report, proof, err := administrativeWriteWith(ctx, repo, request, now, nil, facts)
	if err == nil && lease.LaneUntouched && report.Outcome.Outcome == mutation.OutcomeCompleted {
		return report, releasedUntouched(repo, actor, choice, report)
	}
	if err != nil || report.Kind != "Replay" || report.Outcome.ReceiptSeq == nil {
		return report, err
	}
	if err = replayedAttempt(repo, report); err != nil {
		return report, err
	}
	report.ReplayWarnings = supersededReplayWarnings(proof, lease.Verb, report)
	return report, nil
}

// supersededReplayWarnings is the CAL-V0-236 advisory on a replayed completed
// claim, renew or heartbeat: when the audit the replay was served from shows
// the attempt at another generation, it names that generation. A completed
// result held the live generation and generations only advance, so a
// difference is a supersession. It is an observation for the operator, never
// authority: the replayed outcome is unchanged, and an unobserved attempt, a
// refused original or another verb adds nothing.
func supersededReplayWarnings(proof *journal.Result, verb string, report *Report) []string {
	switch verb {
	case transaction.LeaseClaim, transaction.LeaseClaimNext, transaction.LeaseRenew, transaction.LeaseHeartbeat:
	default:
		return nil
	}
	if proof == nil || report.Outcome.Outcome != mutation.OutcomeCompleted || report.AttemptID == "" || report.Generation == "" {
		return nil
	}
	a, ok := lockedAttempt(proof, report.AttemptID)
	if !ok || a.Generation == report.Generation {
		return nil
	}
	return []string{"advisory, not authoritative: this replays the original result of generation " + string(report.Generation) +
		"; attempt " + report.AttemptID + " is now at generation " + string(a.Generation) +
		", so the replayed result does not describe the live lease; use a fresh request ID to act on generation " + string(a.Generation)}
}

// claimedTicket names the ticket a completed claim holds, which `claim
// --next` did not name. An attempt's ticket never changes, so reading its
// record after the commit is enough. root is the claimant's checkout, whose
// HEAD the delivered know-how freshness is computed against (KHN-V0-006).
func claimedTicket(repo *intent.Repository, root string, repos map[string]string, verb string, report *Report, err error) error {
	if err != nil || report.AttemptID == "" {
		return err
	}
	if verb != transaction.LeaseClaim && verb != transaction.LeaseClaimNext {
		return nil
	}
	if report.Outcome.ReceiptSeq == nil {
		return wire.Errorf(wire.CodeMissingEvidence, "claim", "receipt binding absent")
	}
	a, err := receiptAttempt(repo, report)
	if err != nil {
		return err
	}
	report.Ticket = a.TicketID.Raw
	report.PoolAllocation, report.SharedAllocation = a.PoolAllocation, a.SharedAllocation
	report.Scope = a.Scope
	report.Delivery = claimDelivery(repo, a)
	report.Delivery.KnowHow = claimKnowHow(repo, root, repos, a.TicketID.Raw)
	if report.Kind == "Transaction" && report.Outcome.Outcome == mutation.OutcomeCompleted {
		if root == "" {
			root = repo.PrimaryWorktree
		}
		report.ScopeWarnings = scopeDirectoryKeyWarnings(root, a)
		if a.Scope != nil && a.Scope.Source == "WHOLE_REPOSITORY" {
			report.ScopeWarnings = append(report.ScopeWarnings, wholeRepositoryFallbackWarning(a))
		}
		report.GrantWarnings = claimGrantWarnings(repo, a)
	}
	return nil
}

// WarningWholeRepositoryFallback prefixes the ATR-V0-806 warning.
const WarningWholeRepositoryFallback = "WHOLE_REPOSITORY_FALLBACK"

func wholeRepositoryFallbackWarning(a *snapshot.Attempt) string {
	return WarningWholeRepositoryFallback + ": ticket " + a.TicketID.Raw + " declares no PATH scope and the claim gave no --scope or --scope-from-diff, so attempt " + a.AttemptID +
		" reserves the whole repository and every other claim collides with it until it ends; release it and claim again with --scope PATHS or --scope-from-diff REV, or declare the ticket's paths with ticket set-effects first (ATR-V0-806)"
}

// widenedScope adds the CAL-V0-211 warning for each requested PATH key a
// fresh widen committed that names a directory at the base (ATR-V0-807).
func widenedScope(repo *intent.Repository, choice LeaseChoice, report *Report) error {
	if report.Kind != "Transaction" || report.Outcome.Outcome != mutation.OutcomeCompleted || report.Outcome.ReceiptSeq == nil || report.AttemptID == "" || len(choice.Lease.Scope) == 0 {
		return nil
	}
	a, err := receiptAttempt(repo, report)
	if err != nil || a.Scope == nil {
		return err
	}
	held := map[string]bool{}
	for _, k := range pathKeys(a.Scope.Resources) {
		held[k] = true
	}
	var keys []string
	for _, k := range choice.Lease.Scope {
		if held[k] {
			keys = append(keys, k)
		}
	}
	report.ScopeWarnings = ExactDirectoryKeyWarnings(leaseRoot(repo, choice), a.BaseCommit, keys)
	return nil
}

// pooledAttempt reports the allocation a completed pool acquire or release
// bound in its receipt, fresh or replayed. A refusal or a recorded fence
// posts no attempt and reports none, but still names the attempt's ticket
// (CAL-V0-204).
func pooledAttempt(ctx context.Context, repo *intent.Repository, attemptID string, report *Report) error {
	if report.Outcome.Outcome != mutation.OutcomeCompleted || report.Outcome.ReceiptSeq == nil {
		proof, err := readLeaseProof(ctx, repo)
		if err != nil {
			return err
		}
		if a, ok := lockedAttempt(proof, attemptID); ok {
			report.Ticket = a.TicketID.Raw
		}
		return nil
	}
	a, err := receiptAttempt(repo, report)
	if err != nil {
		return err
	}
	report.Ticket = a.TicketID.Raw
	report.PoolAllocation, report.ReleasedPoolAllocation = a.PoolAllocation, a.ReleasedPoolAllocation
	return nil
}

// receiptAttempt decodes the attempt record report's receipt posted, bound
// to the receipt's digest, the attempt and its generation.
func receiptAttempt(repo *intent.Repository, report *Report) (*snapshot.Attempt, error) {
	rc, err := readReceipt(repo, report.Outcome.ReceiptSeq.Uint64())
	if err != nil {
		return nil, err
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
				return nil, err
			}
		}
		if post.Sha256 == nil || wire.Sum(raw) != *post.Sha256 {
			return nil, wire.Errorf(wire.CodeJournalForked, "claim", "receipt attempt digest differs")
		}
	}
	a, err := snapshot.DecodeAttempt(raw)
	if err != nil {
		return nil, err
	}
	if a.AttemptID != report.AttemptID || a.Generation != report.Generation {
		return nil, wire.Errorf(wire.CodeFenced, "claim", "receipt generation differs")
	}
	return a, nil
}

// reapID is the deterministic request id of one reap, so a repeated reap of
// the same generation replays.
func reapID(x transaction.ExpiredLease) string {
	sum := sha256.Sum256([]byte(x.AttemptID + ":" + string(x.Generation)))
	return "reap-" + hex.EncodeToString(sum[:16])
}

func reapOne(ctx context.Context, repo *intent.Repository, actor mutation.Binding, queueID string, x transaction.ExpiredLease, now wire.Timestamp) (*Report, error) {
	choice := LeaseChoice{QueueID: queueID, RequestID: reapID(x), Lease: transaction.LeaseRequest{Verb: transaction.LeaseReap, AttemptID: x.AttemptID, Generation: x.Generation}}
	report, err := leaseOnce(ctx, repo, actor, choice, now)
	if err != nil {
		return report, err
	}
	return report, nil
}

// replayedAttempt reads the attempt a replayed lease command named from its
// original receipt. A replayed heartbeat or renew may name an elided
// liveness receipt (CAL-V0-300), whose header is retained in full.
func replayedAttempt(repo *intent.Repository, report *Report) error {
	raw, err := readReceiptBytes(repo, report.Outcome.ReceiptSeq.Uint64())
	if err != nil {
		return err
	}
	rc, _, _, err := snapshot.DecodeRetainedReceipt(raw)
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
	return readLogical(repo, "receipts/"+name, wire.MaxReceiptFileBytes)
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

// attemptEntropy is the source of fresh attempt IDs. Tests that compare two
// routes byte for byte replace it with a fixed source.
var attemptEntropy io.Reader = rand.Reader

// attemptEntropies holds, per store state directory, the fixed source a
// package test set for that store's attempt IDs. Keying by store lets tests
// over different stores run in parallel (V1-0961); it is empty in production.
var attemptEntropies sync.Map // state directory -> io.Reader

func mintAttemptID(repo *intent.Repository, queueID string) (string, error) {
	source := attemptEntropy
	if repo != nil {
		if r, ok := attemptEntropies.Load(repo.StateDir); ok {
			source = r.(io.Reader)
		}
	}
	var b [16]byte
	if _, err := io.ReadFull(source, b[:]); err != nil {
		return "", err
	}
	return "attempt:" + strings.TrimPrefix(queueID, "queue:") + ":" + hex.EncodeToString(b[:]), nil
}

// resolveObject names the full object id of rev^{kind} in root.
func resolveObject(root, rev, kind string) (string, error) {
	return resolveObjectWith(gitOutput, root, rev, kind)
}

// gitRunner runs one Git observation in root; gitOutput is the direct one.
type gitRunner func(root string, args ...string) ([]byte, error)

func resolveObjectWith(git gitRunner, root, rev, kind string) (string, error) {
	out, err := git(root, "rev-parse", "--verify", "--end-of-options", rev+"^{"+kind+"}")
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

// scopeRunner is the claim observer's scope derivation; a writer-locked one
// declines instead of running the deriver (V1-2138).
type scopeRunner func(ctx context.Context, root, tree string, rec *ticket.Record) ([]string, wire.Digest, error)

func claimFactsWith(ctx context.Context, repo *intent.Repository, choice LeaseChoice, git gitRunner, scope scopeRunner) claimObserver {
	return func(proof *journal.Result, input *transaction.Input) (transaction.LeaseFacts, error) {
		root := leaseRoot(repo, choice)
		rev := choice.Lease.Base
		if rev == "" {
			rev = "HEAD"
		}
		id, err := mintAttemptID(repo, choice.QueueID)
		if err != nil {
			return transaction.LeaseFacts{}, err
		}
		base, err := resolveObjectWith(git, root, rev, "commit")
		if err != nil {
			return transaction.LeaseFacts{}, err
		}
		facts := transaction.LeaseFacts{AttemptID: id, BaseCommit: base, Pool: choice.pool}
		policy, e := intent.DecodePolicy(proof.Records["intent/policy.json"].Raw)
		if e != nil {
			return facts, e
		}
		if e := poolAdmissionSourceWith(git, root, choice.pool.Observation, policy, choice.Lease.Pool); e != nil {
			return facts, e
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
		if choice.ScopeFromDiff != "" {
			if rec == nil {
				return facts, nil
			}
			paths, digest, err := diffScopeWith(git, root, base, choice.ScopeFromDiff, choice.Lease.ScopeFromDiff)
			if err != nil {
				return transaction.LeaseFacts{}, err
			}
			// A declared scope stays the scope; the diff only has to fit
			// inside it (TCD-V0-175). Otherwise the diff is the DERIVED
			// scope, recorded with its digest (TCD-V0-174).
			facts.DiffPaths = paths
			if len(transaction.Declared(rec)) == 0 {
				facts.DerivedTicketID, facts.DerivedPaths, facts.DerivationSha256 = rec.TicketID.Raw, paths, digest
			}
			return facts, nil
		}
		if rec == nil || choice.Lease.Scope != nil || len(transaction.Declared(rec)) > 0 {
			return facts, nil
		}
		if choice.Derive == nil {
			// NoScopeDeriver abstains for every tree, so no Git runs.
			facts.DerivedTicketID = rec.TicketID.Raw
			return facts, nil
		}
		tree, err := resolveObjectWith(git, root, base, "tree")
		if err != nil {
			return transaction.LeaseFacts{}, err
		}
		if facts.DerivedPaths, facts.DerivationSha256, err = scope(ctx, root, tree, rec); err != nil {
			return transaction.LeaseFacts{}, err
		}
		facts.DerivedTicketID = rec.TicketID.Raw
		return facts, nil
	}
}

// DiffScopeProfile names the TCD-V0-174 diff-derived scope digest input.
const DiffScopeProfile = "taskman-diff-scope/0"

// DiffScopeDigest is the TCD-V0-174 derivation digest: the SHA-256 of the
// canonical JSON of the profile, the claim's base commit, the diffed tree
// and the sorted changed paths, so anyone holding the attempt's base and the
// submitted tree can recompute it.
func DiffScopeDigest(baseCommit, headTree string, paths []string) wire.Digest {
	raw, _ := json.Marshal(struct {
		Profile string   `json:"profile"`
		Base    string   `json:"base"`
		Tree    string   `json:"tree"`
		Paths   []string `json:"paths"`
	}{DiffScopeProfile, baseCommit, headTree, paths})
	sum := sha256.Sum256(raw)
	return wire.Digest(hex.EncodeToString(sum[:]))
}

// diffScopeWith derives a claim's PATH scope from the rename-free diff
// between the base commit's tree and rev's tree, so a rename reserves both
// sides (TCD-V0-174). It fails closed: an unresolvable rev, an empty diff or
// one over wire.MaxTouchPaths refuses rather than falling back to another
// scope.
func diffScopeWith(git gitRunner, root, base, rev, tree string) ([]string, wire.Digest, error) {
	baseTree, err := resolveObjectWith(git, root, base, "tree")
	if err != nil {
		return nil, "", err
	}
	// tree is REV's tree as the lease request names it (TCD-V0-181); the
	// diff uses exactly that tree.
	headTree := tree
	if headTree == "" {
		if headTree, err = resolveObjectWith(git, root, rev, "tree"); err != nil {
			return nil, "", scopeFromDiffUnresolved(rev, root, err)
		}
	}
	out, err := git(root, "diff-tree", "-r", "-z", "--no-renames", "--name-only", "--end-of-options", baseTree, headTree)
	if err != nil {
		return nil, "", err
	}
	paths := []string{}
	for _, p := range strings.Split(string(out), "\x00") {
		if p != "" {
			paths = append(paths, p)
		}
	}
	paths = uniqueSorted(paths)
	switch {
	case len(paths) == 0:
		return nil, "", wire.Errorf(wire.CodeMalformed, "scopeFromDiff", "--scope-from-diff %s changes no path against base %s, so it proves no scope; commit the change first or claim with an explicit --scope (TCD-V0-174)", rev, base)
	case len(paths) > wire.MaxTouchPaths:
		return nil, "", wire.Errorf(wire.CodeLimitExceeded, "scopeFromDiff", "--scope-from-diff %s changes %d paths against base %s, over the %d-path scope limit; claim with directory keys in --scope or --whole-repository (TCD-V0-174)", rev, len(paths), base, wire.MaxTouchPaths)
	}
	return paths, DiffScopeDigest(base, headTree, paths), nil
}

func scopeFromDiffUnresolved(rev, root string, err error) error {
	return wire.Errorf(wire.CodeMalformed, "scopeFromDiff", "--scope-from-diff %s does not name a tree-ish in %s: %v", rev, root, err)
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

// releasedUntouched reconstructs both fresh and replay payloads exclusively
// from the original receipt. Current successors and policy are irrelevant.
func releasedUntouched(repo *intent.Repository, actor mutation.Binding, choice LeaseChoice, report *Report) error {
	fail := func() error {
		return wire.Errorf(wire.CodeMissingEvidence, "lane-untouched", "original release payload or receipt binding missing/damaged")
	}
	if report.Outcome.ReceiptSeq == nil {
		return fail()
	}
	rc, err := readReceipt(repo, report.Outcome.ReceiptSeq.Uint64())
	if err != nil {
		return err
	}
	l := choice.Lease
	if rc.Seq != *report.Outcome.ReceiptSeq || rc.Kind != "TRANSITION" || rc.Outcome != mutation.OutcomeCompleted || rc.RequestID == nil || *rc.RequestID != choice.RequestID || rc.AttemptID == nil || *rc.AttemptID != l.AttemptID || rc.Generation == nil || *rc.Generation != l.Generation || rc.ActorID != actor.ID || rc.ActorRole != actor.Role {
		return fail()
	}
	var raw []byte
	found := false
	for _, post := range rc.Post {
		if post.Path != "attempts/"+l.AttemptID+".json" {
			continue
		}
		if found {
			return fail()
		}
		found = true
		if post.Record != nil {
			raw = wire.EncodeFile(*post.Record)
		} else if post.BlobSha256 != nil {
			raw, err = intent.ReadFile(filepath.Join(repo.StateDir, "evidence", string(*post.BlobSha256)), wire.MaxAttemptRecordBytes)
			if err != nil {
				return err
			}
			if wire.Sum(raw) != *post.BlobSha256 {
				return fail()
			}
		}
		if post.Sha256 == nil || wire.Sum(raw) != *post.Sha256 {
			return fail()
		}
	}
	a, err := snapshot.DecodeAttempt(raw)
	if err != nil {
		return err
	}
	canonical, err := a.Encode()
	if err != nil || !bytes.Equal(raw, canonical) {
		return fail()
	}
	x := a.LaneUntouchedAttestation
	if !found || x == nil || a.AttemptID != l.AttemptID || a.Generation != l.Generation || a.PhaseSinceSeq != rc.Seq || x.RecordedSeq != rc.Seq || x.RecordedAt != rc.RecordedAt || x.ActorID != rc.ActorID || x.ActorRole != rc.ActorRole || x.Evidence != l.Evidence || (l.Reason == "" && a.Cause != nil) || (l.Reason != "" && (a.Cause == nil || *a.Cause != l.Reason)) {
		return fail()
	}
	q, err := snapshot.AttemptQueue(a.AttemptID)
	if err != nil || q.Raw != choice.QueueID {
		return fail()
	}
	report.AttemptID, report.Generation = a.AttemptID, a.Generation
	report.PoolAllocation, report.LaneUntouchedAttestation = a.PoolAllocation, x
	return nil
}
