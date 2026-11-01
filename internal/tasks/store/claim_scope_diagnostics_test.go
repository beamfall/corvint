package store_test

import (
	"context"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

func hasWarning(ws []string, prefix string) bool {
	for _, w := range ws {
		if strings.HasPrefix(w, prefix) {
			return true
		}
	}
	return false
}

// TestATRV0806_WholeRepositoryFallbackWarns: a claim of a ticket that
// declares no PATH scope, given no --scope, commits WHOLE_REPOSITORY as
// before and warns with the narrowing routes (V1-3185); a replay and a
// requested scope warn nothing.
func TestATRV0806_WholeRepositoryFallbackWarns(t *testing.T) {
	t.Parallel()
	s := newGateStore(t)
	s.commit(t, "src/a.go")
	one := s.ticket(t, "one")
	claim := s.claim(t, "claim-1", one, 0)
	if claim.Scope == nil || claim.Scope.Source != "WHOLE_REPOSITORY" {
		t.Fatalf("scope = %+v", claim.Scope)
	}
	if len(claim.ScopeWarnings) != 1 {
		t.Fatalf("warnings = %q", claim.ScopeWarnings)
	}
	w := claim.ScopeWarnings[0]
	for _, want := range []string{store.WarningWholeRepositoryFallback + ": ticket " + one + " ", "attempt " + claim.AttemptID + " reserves the whole repository", "--scope PATHS", "--scope-from-diff REV", "ticket set-effects", "(ATR-V0-806)"} {
		if !strings.Contains(w, want) {
			t.Fatalf("warning lacks %q: %q", want, w)
		}
	}
	replay := s.lease(t, "claim-1", claimOf(one), 0, nil)
	if replay.Kind != "Replay" || len(replay.ScopeWarnings) != 0 {
		t.Fatalf("replay: kind %s warnings %q", replay.Kind, replay.ScopeWarnings)
	}
	s.lease(t, "release-1", transaction.LeaseRequest{Verb: transaction.LeaseRelease, AttemptID: claim.AttemptID, Generation: claim.Generation}, 0, nil)
	two := s.ticket(t, "two")
	if r := s.claim(t, "claim-2", two, 0, "src/a.go"); len(r.ScopeWarnings) != 0 {
		t.Fatalf("requested scope warned: %q", r.ScopeWarnings)
	}
}

// TestATRV0807_WidenWarnsOnExactDirectoryKey: a widen that commits a key
// without a trailing "/" naming a directory at the base warns as a claim
// does (V1-3540); a file key, a directory key and a replay warn nothing.
func TestATRV0807_WidenWarnsOnExactDirectoryKey(t *testing.T) {
	t.Parallel()
	s := newGateStore(t)
	_, tree := s.commit(t, "src/a.go", "ci/shards/x.json", "doc/c.md")
	one := s.ticket(t, "one")
	claim := s.claim(t, "claim-1", one, 0, "src/a.go")
	widen := transaction.LeaseRequest{Verb: transaction.LeaseWiden, AttemptID: claim.AttemptID, Generation: claim.Generation, Scope: []string{"ci/shards", "doc/", "doc/c.md"}}
	r := s.lease(t, "widen-1", widen, 0, nil)
	if r.Outcome.Outcome != mutation.OutcomeCompleted || len(r.ScopeWarnings) != 1 {
		t.Fatalf("widen: %+v warnings %q", r.Outcome, r.ScopeWarnings)
	}
	if w := r.ScopeWarnings[0]; !strings.HasPrefix(w, store.WarningExactDirectoryKey+": PATH key ci/shards names a directory in tree "+tree+" ") {
		t.Fatalf("warning = %q", w)
	}
	if replay := s.lease(t, "widen-1", widen, 0, nil); replay.Kind != "Replay" || len(replay.ScopeWarnings) != 0 {
		t.Fatalf("replay: kind %s warnings %q", replay.Kind, replay.ScopeWarnings)
	}
}

// TestATRV0808_ClaimBaseResolvesARevision: claim --base takes a short OID
// or a branch name and records the full commit OID (V1-3547); a revision
// naming no commit refuses MALFORMED naming git rev-parse.
func TestATRV0808_ClaimBaseResolvesARevision(t *testing.T) {
	t.Parallel()
	s := newGateStore(t)
	s.commit(t, "src/a.go")
	full := gitOut(t, s.root, "rev-parse", "HEAD")
	gitOut(t, s.root, "branch", "lane-base")
	s.commit(t, "src/b.go")
	for i, rev := range []string{full[:8], "lane-base", full} {
		id := s.ticket(t, "t"+string(rune('a'+i)))
		l := claimOf(id, "src/"+string(rune('a'+i))+".go")
		l.Base = rev
		r := s.lease(t, "claim-"+rev, l, 0, nil)
		if r.Outcome.Outcome != mutation.OutcomeCompleted {
			t.Fatalf("--base %s: %+v", rev, r.Outcome)
		}
		if got := s.attempt(t, r.AttemptID).BaseCommit; got != full {
			t.Fatalf("--base %s recorded %s, want %s", rev, got, full)
		}
	}
	l := claimOf(s.ticket(t, "bad"), "src/z.go")
	l.Base = "no-such-rev"
	choice := store.LeaseChoice{QueueID: fixture.QueueID, RequestID: "claim-bad", Root: s.root, Lease: l}
	_, err := store.Lease(context.Background(), s.repo, operator(), choice, s.at(t, 0))
	if wire.CodeOf(err) != wire.CodeMalformed || !strings.Contains(err.Error(), "git rev-parse --verify no-such-rev^{commit}") {
		t.Fatalf("unresolvable --base: %v", err)
	}
}
