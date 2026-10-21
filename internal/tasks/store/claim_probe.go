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
// and no deriver. It serves only results memoized for the identical argument
// list, or for the identical root, tree, title and body; on a miss it records
// the audited state and declines with errClaimDeferred. settle then reruns the
// observer for that state with no lock or preparation gate held, and the
// retried route serves the memoized results. The complete route keeps the
// direct observer, which already runs before the preparation gate.
type claimProbe struct {
	outputs  map[string][]byte
	scopes   map[string]derivedScope
	deferred bool
	pending  *claimPending
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
// may run, and a prepared pool observation's command source. A failed
// observation memoizes nothing; the locked observer then defers it.
func newClaimProbe(ctx context.Context, repo *intent.Repository, choice LeaseChoice) *claimProbe {
	p := &claimProbe{outputs: map[string][]byte{}, scopes: map[string]derivedScope{}}
	git := p.git(false)
	root := leaseRoot(repo, choice)
	rev := choice.Lease.Base
	if rev == "" {
		rev = "HEAD"
	}
	if base, err := resolveObjectWith(git, root, rev, "commit"); err == nil && choice.Derive != nil {
		_, _ = resolveObjectWith(git, root, base, "tree")
	}
	if len(choice.pool.Observation) > 0 {
		_, _, _ = poolSourceWith(git, root)
	}
	return p
}

func (p *claimProbe) git(locked bool) gitRunner {
	return func(root string, args ...string) ([]byte, error) {
		key := strings.Join(append([]string{root}, args...), "\x00")
		if out, ok := p.outputs[key]; ok {
			return out, nil
		}
		if locked {
			p.deferred = true
			return nil, errClaimDeferred
		}
		out, err := gitOutput(root, args...)
		if err == nil {
			p.outputs[key] = out
		}
		return out, err
	}
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
		facts, err := locked(proof, input)
		if p.deferred {
			p.pending = &claimPending{proof: proof, input: *input}
			return transaction.LeaseFacts{}, errClaimDeferred
		}
		p.outputs = map[string][]byte{}
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
	_, err := claimFactsWith(ctx, repo, choice, p.git(false), p.scope(false, choice.Derive))(pending.proof, &input)
	return err == nil
}
