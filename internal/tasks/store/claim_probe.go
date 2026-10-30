package store

import (
	"context"
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/journal"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// claimProbe is the claim observer the writer route calls under the writer
// lock (V1-2138). It is the PSR-V0-168 deferral applied to a claim's Git
// observations and scope derivation: the locked observer runs no Git command
// and no deriver. It serves only memoized results; on a miss it records the
// audited state and declines with errClaimDeferred. settle then reruns the
// observer for that state with no lock or preparation gate held, and the
// retried route serves the memoized results. The complete route keeps the
// direct observer, which already runs before the preparation gate.
//
// A query naming an exact object (rev-parse or ls-tree of a full OID) is
// immutable and memoized for its argument list. Every other Git query (a
// symbolic revision, worktree status) is mutable: its result is bound to the
// head.json and intent tree digests read before it ran, and the locked
// observer serves it only while its audited state still carries that
// snapshot. Drift is a miss, so the route reprobes once and then falls back
// to the complete route. The one exception is the root's HEAD commit
// (PSR-V0-224): when the HEAD and ref files resolve to the commit Git
// reported, that result is bound to them instead, and the locked observer
// serves it only while those files, read without Git, still name it. A
// journal write alone (a heartbeat) then no longer misses the base.
type claimProbe struct {
	outputs  map[string]gitMemo
	scopes   map[string]derivedScope
	layouts  map[string]*headLayout
	at       *journal.Identity
	deferred bool
	pending  *claimPending
}

// gitMemo is one memoized Git output. A nil bound and empty head mark an
// immutable query; head is the commit a HEAD query is bound to.
type gitMemo struct {
	out   []byte
	bound *journal.Identity
	head  string
}

type derivedScope struct {
	paths  []string
	digest wire.Digest
}

// claimPending is the audited state a locked claim observer could not serve.
type claimPending struct {
	proof *journal.Result
	input transaction.Input
}

// errClaimDeferred is a writer-locked claim observer's decline.
var errClaimDeferred = errWriterRoute("claim", "claim Git observation deferred until the writer lock is released")

// newClaimProbe observes what a claim needs from Git independently of the
// audited state, before any lock: the base commit, its tree when a deriver
// may run, the TCD-V0-174 --scope-from-diff diff, and a prepared pool
// observation's command source, bound to the snapshot read first. Without that snapshot, or on a failed observation,
// nothing mutable is memoized; the locked observer then defers it.
func newClaimProbe(ctx context.Context, repo *intent.Repository, choice LeaseChoice) *claimProbe {
	p := &claimProbe{outputs: map[string]gitMemo{}, scopes: map[string]derivedScope{}, layouts: map[string]*headLayout{}}
	if repo == nil {
		return p
	}
	at, err := observedSnapshot(repo)
	if err != nil {
		return p
	}
	p.at = &at
	git := p.git(false)
	root := leaseRoot(repo, choice)
	rev := choice.Lease.Base
	if rev == "" {
		rev = "HEAD"
	}
	if base, err := resolveObjectWith(git, root, rev, "commit"); err == nil {
		switch {
		case choice.ScopeFromDiff != "":
			_, _, _ = diffScopeWith(git, root, base, choice.ScopeFromDiff, choice.Lease.ScopeFromDiff)
		case choice.Derive != nil:
			_, _ = resolveObjectWith(git, root, base, "tree")
		}
	}
	if len(choice.pool.Observation) > 0 {
		_, _, _ = poolSourceWith(git, root)
	}
	return p
}

// immutableQuery reports whether a Git query names only an exact object, so
// its output cannot change while the object exists.
func immutableQuery(args []string) bool {
	full := func(s string) bool { _, err := wire.ParseOID("oid", s); return err == nil }
	switch {
	case len(args) == 4 && args[0] == "rev-parse" && args[1] == "--verify" && args[2] == "--end-of-options":
		oid, kind, ok := strings.Cut(args[3], "^{")
		return ok && full(oid) && (kind == "commit}" || kind == "tree}")
	case len(args) == 5 && args[0] == "ls-tree" && args[1] == "-z" && args[3] == "--":
		return full(args[2])
	case len(args) == 8 && strings.Join(args[:6], " ") == "diff-tree -r -z --no-renames --name-only --end-of-options":
		return full(args[6]) && full(args[7])
	}
	return false
}

// git serves a memoized output while it is immutable, its HEAD commit is
// still named by the ref files, or it is bound to p.at. The locked runner
// defers any other query; the unlocked one runs it and binds a HEAD query to
// the ref files when they name the commit Git reported, and any other
// mutable output to p.at, or memoizes nothing without one.
func (p *claimProbe) git(locked bool) gitRunner {
	return func(root string, args ...string) ([]byte, error) {
		key := strings.Join(append([]string{root}, args...), "\x00")
		if m, ok := p.outputs[key]; ok && p.fresh(root, m) {
			return m.out, nil
		}
		if locked {
			p.deferred = true
			return nil, errClaimDeferred
		}
		out, err := gitOutput(root, args...)
		head := ""
		if err == nil && headQuery(args) {
			if oid, ok := p.head(root, false); ok && oid == strings.TrimSpace(string(out)) {
				head = oid
			}
		}
		switch {
		case err != nil:
		case immutableQuery(args):
			p.outputs[key] = gitMemo{out: out}
		case head != "":
			p.outputs[key] = gitMemo{out: out, head: head}
		case p.at != nil:
			at := *p.at
			p.outputs[key] = gitMemo{out: out, bound: &at}
		}
		return out, err
	}
}

// fresh reports whether a memoized output still holds: an immutable one
// always, a HEAD-bound one while the ref files name the same commit, any
// other while the audited snapshot is the one it was bound to.
func (p *claimProbe) fresh(root string, m gitMemo) bool {
	switch {
	case m.head != "":
		oid, ok := p.head(root, true)
		return ok && oid == m.head
	case m.bound == nil:
		return true
	default:
		return p.at != nil && sameSnapshot(*m.bound, *p.at)
	}
}

// head resolves root's HEAD from its ref files. Only an unlocked caller may
// observe the layout with Git; a locked one uses the layout observed before.
func (p *claimProbe) head(root string, locked bool) (string, bool) {
	l, seen := p.layouts[root]
	if !seen {
		if locked {
			return "", false
		}
		if found, ok := observeHeadLayout(root); ok {
			l = &found
		}
		p.layouts[root] = l
	}
	if l == nil {
		return "", false
	}
	return l.head()
}

func (p *claimProbe) scope(locked bool, deriver ScopeDeriver) scopeRunner {
	return func(ctx context.Context, root, tree string, rec *ticket.Record) ([]string, wire.Digest, error) {
		body := ""
		if rec.Body != nil {
			body = *rec.Body
		}
		key := strings.Join([]string{root, tree, rec.Title, body}, "\x00")
		if d, ok := p.scopes[key]; ok {
			return d.paths, d.digest, nil
		}
		if locked {
			p.deferred = true
			return nil, "", errClaimDeferred
		}
		paths, digest := derive(ctx, deriver, root, tree, rec)
		p.scopes[key] = derivedScope{paths: paths, digest: digest}
		return paths, digest, nil
	}
}

// observer is the writer-locked claim observer. A served observation clears
// the memo, so a later round observes Git afresh.
func (p *claimProbe) observer(ctx context.Context, repo *intent.Repository, choice LeaseChoice) claimObserver {
	locked := claimFactsWith(ctx, repo, choice, p.git(true), p.scope(true, choice.Derive))
	return func(proof *journal.Result, input *transaction.Input) (transaction.LeaseFacts, error) {
		p.deferred = false
		p.at = &proof.Identity
		facts, err := locked(proof, input)
		if p.deferred {
			p.pending = &claimPending{proof: proof, input: *input}
			return transaction.LeaseFacts{}, errClaimDeferred
		}
		p.outputs = map[string]gitMemo{}
		p.scopes = map[string]derivedScope{}
		return facts, err
	}
}

// settle runs the observation a locked observer deferred, against the state
// it recorded, and reports whether it completed. Its caller holds no lock
// and no preparation gate.
func (p *claimProbe) settle(ctx context.Context, repo *intent.Repository, choice LeaseChoice) bool {
	pending := p.pending
	p.pending = nil
	if pending == nil {
		return false
	}
	input := pending.input
	// The pending state was audited before this observation runs, so a
	// mutable output bound to it is stale exactly when the retried route
	// audits a moved snapshot.
	p.at = &pending.proof.Identity
	_, err := claimFactsWith(ctx, repo, choice, p.git(false), p.scope(false, choice.Derive))(pending.proof, &input)
	return err == nil
}
