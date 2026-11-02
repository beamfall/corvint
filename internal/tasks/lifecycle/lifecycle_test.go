package lifecycle_test

import (
	"slices"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/lifecycle"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

func sp(s string) *string { return &s }
func bp(b bool) *bool     { return &b }

const head = "1111111111111111111111111111111111111111"

func progress(phase string) *ticket.Progress {
	return &ticket.Progress{ProgressMark: ticket.ProgressMark{Phase: phase, Branch: sp("claude/lane-x"), Head: sp(head)}}
}

// confirmed is the Git observation of a head reachable from its branch ref
// and, when integrated, from the integration ref.
func confirmed(integrated bool) *lifecycle.Git {
	return &lifecycle.Git{IntegrationRef: "refs/heads/main", IntegrationCommit: head, Ref: "refs/heads/claude/lane-x", RefCommit: head,
		HeadExists: bp(true), Integrated: bp(integrated), HeadOnRef: bp(true)}
}

// TestTLCV0001_Vocabulary pins the closed state set, in byte order.
func TestTLCV0001_Vocabulary(t *testing.T) {
	want := []string{"ARCHIVED", "AWAITING_CLOSEOUT", "AWAITING_INTEGRATION", "COMPLETED", "DRAFT", "HELD", "IN_PROGRESS", "IN_REVIEW", "OPEN", "SUBMITTED", "UNKNOWN"}
	if !slices.Equal(lifecycle.States, want) || !slices.IsSorted(lifecycle.States) || !slices.Equal(wire.FacetLifecycles, want) {
		t.Fatalf("states %v, want %v", lifecycle.States, want)
	}
	for _, s := range want {
		if !lifecycle.IsState(s) {
			t.Fatalf("%s is not a state", s)
		}
	}
	for _, s := range []string{"", "open", "DELIVERED_ON_BRANCH", "BLOCKED"} {
		if lifecycle.IsState(s) {
			t.Fatalf("%q is a state", s)
		}
	}
}

// TestTLCV0002_Precedence pins the first-match-wins order.
func TestTLCV0002_Precedence(t *testing.T) {
	live := &lifecycle.Attempt{ID: "a1", Phase: "RUNNING", Stage: "implement"}
	tree := sp("2222222222222222222222222222222222222222")
	for _, c := range []struct {
		name, state, reason, from string
		in                        lifecycle.Input
	}{
		{"completed beats everything", "COMPLETED", "", "status", lifecycle.Input{Status: ticket.StatusCompleted, Attempt: live, AttemptsObserved: true, Progress: progress(ticket.PhaseInProgress)}},
		{"archived", "ARCHIVED", "", "status", lifecycle.Input{Status: ticket.StatusArchived}},
		{"draft", "DRAFT", "", "status", lifecycle.Input{Status: ticket.StatusDraft, AttemptsObserved: true, Attempt: live}},
		{"held beats attempt", "HELD", "", "hold", lifecycle.Input{Status: ticket.StatusHeld, HoldIDs: []string{"owner-wait"}, AttemptsObserved: true, Attempt: live}},
		{"ready for integration", "AWAITING_INTEGRATION", "", "attempt", lifecycle.Input{Status: ticket.StatusOpen, AttemptsObserved: true, Attempt: &lifecycle.Attempt{ID: "a", Phase: "READY_FOR_INTEGRATION", CandidateTreeOid: tree}}},
		{"reviewing", "IN_REVIEW", "", "attempt", lifecycle.Input{Status: ticket.StatusOpen, AttemptsObserved: true, Attempt: &lifecycle.Attempt{ID: "a", Phase: "REVIEWING"}}},
		{"review stage", "IN_REVIEW", "", "attempt", lifecycle.Input{Status: ticket.StatusOpen, AttemptsObserved: true, Attempt: &lifecycle.Attempt{ID: "a", Phase: "RUNNING", Stage: "review"}}},
		{"candidate tree", "SUBMITTED", "", "attempt", lifecycle.Input{Status: ticket.StatusOpen, AttemptsObserved: true, Attempt: &lifecycle.Attempt{ID: "a", Phase: "GATING", CandidateTreeOid: tree}}},
		{"attempt beats progress", "IN_PROGRESS", "", "attempt", lifecycle.Input{Status: ticket.StatusOpen, AttemptsObserved: true, Attempt: live, Progress: progress(ticket.PhaseDeliveredOnBranch), Git: confirmed(true)}},
		{"progress in progress", "IN_PROGRESS", "", "progress", lifecycle.Input{Status: ticket.StatusOpen, AttemptsObserved: true, Progress: progress(ticket.PhaseInProgress)}},
		{"progress in review", "IN_REVIEW", "", "progress", lifecycle.Input{Status: ticket.StatusOpen, AttemptsObserved: true, Progress: progress(ticket.PhaseInReview)}},
		{"progress merge queue", "IN_REVIEW", "", "progress", lifecycle.Input{Status: ticket.StatusOpen, AttemptsObserved: true, Progress: progress(ticket.PhaseInMergeQueue)}},
		{"progress blocked", "OPEN", "REPORTED_BLOCKED", "progress", lifecycle.Input{Status: ticket.StatusOpen, AttemptsObserved: true, Progress: progress(ticket.PhaseBlocked)}},
		{"delivered on branch", "AWAITING_INTEGRATION", "", "progress", lifecycle.Input{Status: ticket.StatusOpen, AttemptsObserved: true, Progress: progress(ticket.PhaseDeliveredOnBranch), Git: confirmed(false)}},
		{"delivered and integrated", "AWAITING_CLOSEOUT", "", "progress", lifecycle.Input{Status: ticket.StatusOpen, AttemptsObserved: true, Progress: progress(ticket.PhaseDeliveredOnBranch), Git: confirmed(true)}},
		{"merged awaiting closeout", "AWAITING_CLOSEOUT", "", "progress", lifecycle.Input{Status: ticket.StatusOpen, AttemptsObserved: true, Progress: progress(ticket.PhaseMergedAwaitingCloseout), Git: confirmed(true)}},
		{"cleared is open", "OPEN", "", "none", lifecycle.Input{Status: ticket.StatusOpen, AttemptsObserved: true, Progress: progress(ticket.PhaseCleared)}},
		{"nothing is open", "OPEN", "", "none", lifecycle.Input{Status: ticket.StatusOpen, AttemptsObserved: true}},
		{"unobserved attempts are unknown", "UNKNOWN", "ATTEMPTS_NOT_OBSERVED", "none", lifecycle.Input{Status: ticket.StatusOpen}},
		{"unobserved attempts hide reported progress", "UNKNOWN", "ATTEMPTS_NOT_OBSERVED", "none", lifecycle.Input{Status: ticket.StatusOpen, Progress: progress(ticket.PhaseInReview)}},
		{"git-verified progress stands without attempts", "AWAITING_CLOSEOUT", "", "progress", lifecycle.Input{Status: ticket.StatusOpen, Progress: progress(ticket.PhaseDeliveredOnBranch), Git: confirmed(true)}},
		{"unobserved attempts hide an unconfirmed delivery", "UNKNOWN", "ATTEMPTS_NOT_OBSERVED", "none", lifecycle.Input{Status: ticket.StatusOpen, Progress: progress(ticket.PhaseDeliveredOnBranch), Git: &lifecycle.Git{Failed: true}}},
	} {
		l := lifecycle.Derive(c.in)
		if l.State != c.state || l.Reason != c.reason || l.DerivedFrom != c.from {
			t.Errorf("%s: got %s/%q/%s, want %s/%q/%s", c.name, l.State, l.Reason, l.DerivedFrom, c.state, c.reason, c.from)
		}
	}
}

// TestTLCV0004_FallbackReasons pins each named fallback of an unconfirmed
// delivered phase.
func TestTLCV0004_FallbackReasons(t *testing.T) {
	git := func(f func(g *lifecycle.Git)) *lifecycle.Git {
		g := confirmed(false)
		f(g)
		return g
	}
	for _, c := range []struct {
		name, phase, state, reason string
		g                          *lifecycle.Git
	}{
		{"git failed", ticket.PhaseDeliveredOnBranch, "UNKNOWN", "GIT_NOT_OBSERVED", &lifecycle.Git{Failed: true}},
		{"no observation", ticket.PhaseDeliveredOnBranch, "UNKNOWN", "GIT_NOT_OBSERVED", nil},
		{"missing commit", ticket.PhaseDeliveredOnBranch, "OPEN", "COMMIT_MISSING", git(func(g *lifecycle.Git) { g.HeadExists = bp(false) })},
		{"head existence unknown", ticket.PhaseDeliveredOnBranch, "UNKNOWN", "REACHABILITY_NOT_OBSERVED", git(func(g *lifecycle.Git) { g.HeadExists = nil })},
		{"no integration ref", ticket.PhaseDeliveredOnBranch, "UNKNOWN", "INTEGRATION_REF_MISSING", git(func(g *lifecycle.Git) { g.IntegrationRef, g.IntegrationCommit = "", "" })},
		{"integration walk over budget", ticket.PhaseDeliveredOnBranch, "UNKNOWN", "REACHABILITY_NOT_OBSERVED", git(func(g *lifecycle.Git) { g.Integrated = nil })},
		{"merged but not integrated", ticket.PhaseMergedAwaitingCloseout, "OPEN", "NOT_ON_INTEGRATION_REF", confirmed(false)},
		{"branch deleted", ticket.PhaseDeliveredOnBranch, "OPEN", "REF_DELETED", git(func(g *lifecycle.Git) { g.Ref, g.RefCommit, g.HeadOnRef = "", "", nil })},
		{"branch walk over budget", ticket.PhaseDeliveredOnBranch, "UNKNOWN", "REACHABILITY_NOT_OBSERVED", git(func(g *lifecycle.Git) { g.HeadOnRef = nil })},
		{"branch rewritten", ticket.PhaseDeliveredOnBranch, "OPEN", "HEAD_NOT_ON_REF", git(func(g *lifecycle.Git) { g.HeadOnRef = bp(false) })},
	} {
		l := lifecycle.Derive(lifecycle.Input{Status: ticket.StatusOpen, AttemptsObserved: true, Progress: progress(c.phase), Git: c.g})
		if l.State != c.state || l.Reason != c.reason {
			t.Errorf("%s: got %s/%s, want %s/%s", c.name, l.State, l.Reason, c.state, c.reason)
		}
	}
}

// TestTLCV0005_EvidenceValue pins the closed evidence object.
func TestTLCV0005_EvidenceValue(t *testing.T) {
	keys := func(v wire.Value) []string {
		return v.Obj.SortedKeys()
	}
	get := func(v wire.Value, k string) wire.Value {
		x, _ := v.Obj.Get(k)
		return x
	}
	top := []string{"attempt", "derivedFrom", "git", "holdIds", "progress", "reason", "state", "status"}

	v := lifecycle.Derive(lifecycle.Input{Status: ticket.StatusOpen, AttemptsObserved: true, Progress: progress(ticket.PhaseDeliveredOnBranch), Git: confirmed(false)}).Value()
	if got := keys(v); !slices.Equal(got, top) {
		t.Fatalf("keys %v, want %v", got, top)
	}
	if get(v, "state").Str != "AWAITING_INTEGRATION" || get(v, "reason").Kind != wire.KindNull || get(v, "attempt").Kind != wire.KindNull || get(v, "progress").Kind != wire.KindObject {
		t.Fatalf("value %+v", v)
	}
	g := get(v, "git")
	if got := keys(g); !slices.Equal(got, []string{"headExists", "headOnRef", "integrated", "integrationCommit", "integrationRef", "ref", "refCommit"}) {
		t.Fatalf("git keys %v", got)
	}
	if get(g, "ref").Str != "refs/heads/claude/lane-x" || !get(g, "headOnRef").Bool || get(g, "integrated").Bool {
		t.Fatalf("git %+v", g)
	}

	held := lifecycle.Derive(lifecycle.Input{Status: ticket.StatusHeld, HoldIDs: []string{"z", "a"}}).Value()
	if get(held, "attempt").Str != "NOT_OBSERVED" || get(held, "git").Kind != wire.KindNull || get(held, "progress").Kind != wire.KindNull {
		t.Fatalf("held %+v", held)
	}
	if ids := get(held, "holdIds").Arr; len(ids) != 2 || ids[0].Str != "a" || ids[1].Str != "z" {
		t.Fatalf("hold ids %+v, want sorted", ids)
	}

	attempt := get(lifecycle.Derive(lifecycle.Input{Status: ticket.StatusOpen, AttemptsObserved: true, Attempt: &lifecycle.Attempt{ID: "a1", Phase: "RUNNING"}}).Value(), "attempt")
	if got := keys(attempt); !slices.Equal(got, []string{"attemptId", "candidateTreeOid", "phase", "stage"}) || get(attempt, "stage").Kind != wire.KindNull {
		t.Fatalf("attempt %v %+v", got, attempt)
	}
	unknown := lifecycle.Derive(lifecycle.Input{Status: ticket.StatusOpen, Progress: progress(ticket.PhaseDeliveredOnBranch), Git: &lifecycle.Git{Failed: true}}).Value()
	if gk := keys(get(unknown, "git")); len(gk) != 7 || get(get(unknown, "git"), "headExists").Kind != wire.KindNull {
		t.Fatalf("failed git %+v", unknown)
	}
}
