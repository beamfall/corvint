package store_test

import (
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
)

// TestCALV0211_ClaimWarnsOnExactDirectoryKey: a fresh claim whose scope holds
// a PATH key without a trailing "/" that names a directory at the base commit
// commits as before and reports one EXACT_KEY_NAMES_DIRECTORY warning naming
// the key and the directory key; a file key, a directory key ending in "/"
// and an absent path add none, and a replay adds none (V1-1118).
func TestCALV0211_ClaimWarnsOnExactDirectoryKey(t *testing.T) {
	t.Parallel()
	s := newGateStore(t)
	_, tree := s.commit(t, "src/a.go", "lib/b.go", "doc/c.md", "tool/d.go")
	one := s.ticket(t, "one")
	claim := s.claim(t, "claim-1", one, 0, "absent", "lib/", "src", "src/a.go")
	if len(claim.ScopeWarnings) != 1 {
		t.Fatalf("warnings = %q", claim.ScopeWarnings)
	}
	w := claim.ScopeWarnings[0]
	if !strings.HasPrefix(w, store.WarningExactDirectoryKey+": PATH key src names a directory in tree "+tree+" ") || !strings.HasSuffix(w, "use src/ to cover them") {
		t.Fatalf("warning = %q", w)
	}
	replay := s.lease(t, "claim-1", claimOf(one, "absent", "lib/", "src", "src/a.go"), 0, nil)
	if replay.Kind != "Replay" || replay.Outcome.Outcome != mutation.OutcomeCompleted || len(replay.ScopeWarnings) != 0 {
		t.Fatalf("replay: kind %s warnings %q", replay.Kind, replay.ScopeWarnings)
	}
	two := s.ticket(t, "two")
	if r := s.claim(t, "claim-2", two, 0, "doc/", "tool/d.go"); len(r.ScopeWarnings) != 0 {
		t.Fatalf("file and directory keys warned: %q", r.ScopeWarnings)
	}
	// The claim reads its base commit's tree, not HEAD's.
	base := gitOut(t, s.root, "rev-parse", "HEAD")
	s.commit(t, "late/e.go", "later/f.go")
	three := s.ticket(t, "three")
	l := claimOf(three, "late")
	l.Base = base
	if r := s.lease(t, "claim-3", l, 0, nil); r.Outcome.Outcome != mutation.OutcomeCompleted || len(r.ScopeWarnings) != 0 {
		t.Fatalf("a directory absent at the base warned: %+v", r)
	}
	four := s.ticket(t, "four")
	if r := s.claim(t, "claim-4", four, 0, "later"); len(r.ScopeWarnings) != 1 {
		t.Fatalf("a directory at the base did not warn: %q", r.ScopeWarnings)
	}
}

// TestCALV0211_ExactDirectoryKeyWarnings: only keys without a trailing "/"
// that name a tree warn, in key order; an unreadable revision warns nothing.
func TestCALV0211_ExactDirectoryKeyWarnings(t *testing.T) {
	s := newGateStore(t)
	s.commit(t, "src/a.go", "lib/deep/b.go")
	got := store.ExactDirectoryKeyWarnings(s.root, "HEAD", []string{"src/a.go", "lib/deep", "src", "src/", "lib/deep", "absent"})
	if len(got) != 2 || !strings.Contains(got[0], "PATH key lib/deep ") || !strings.Contains(got[1], "PATH key src ") {
		t.Fatalf("warnings = %q", got)
	}
	if got := store.ExactDirectoryKeyWarnings(s.root, "refs/heads/absent", []string{"src"}); got != nil {
		t.Fatalf("unreadable revision warned: %q", got)
	}
}

// TestTCBV0036_VerdictPathKeysWarnLikeTouchPaths: an exact verdictPaths key
// that names a directory gets the CAL-V0-211 warning a touchPaths key gets,
// once, since a covered directory key governs only that exact path.
func TestTCBV0036_VerdictPathKeysWarnLikeTouchPaths(t *testing.T) {
	s := newGateStore(t)
	s.commit(t, "src/deep/a.go")
	e := ticket.Effects{TouchPaths: []string{"src/"}, VerdictPaths: []string{"src/deep"}, Resources: []ticket.Resource{}}
	got := store.EffectsDirectoryKeyWarnings(s.root, "HEAD", e)
	if len(got) != 1 || !strings.Contains(got[0], "PATH key src/deep ") {
		t.Fatalf("warnings = %q", got)
	}
}
