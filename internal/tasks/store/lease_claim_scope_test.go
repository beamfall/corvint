package store_test

import (
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// declaredTicket creates a QUALIFIED ticket whose touchPaths are paths.
func (s *leaseStore) declaredTicket(t *testing.T, title string, paths ...string) string {
	t.Helper()
	payload := createPayload(title)
	payload.Obj.Set("effects", obj("coverage", str("QUALIFIED"), "externalUnbounded", wire.Bool(false), "resources", wire.Array(), "touchPaths", wire.Strings(paths)))
	report := mutate(t, s.repo, envelope("create-"+title, "CREATE", "", "", payload))
	if report.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("create %s: %+v", title, report)
	}
	s.t0 = now(t)
	return report.Ticket
}

// TestATRV0210_ClaimRefusesRequestedPathsOutsideDeclaredScope (V1-1321): a
// ticket with declared QUALIFIED touchPaths keeps the DECLARED scope
// (CAL-V0-021); a requested --scope path it covers is admitted, and one it
// does not cover refuses OUT_OF_SCOPE naming that path instead of being
// silently dropped. The refusal holds for a re-claim after release, which
// re-activates the same attempt, and writes no new generation.
func TestATRV0210_ClaimRefusesRequestedPathsOutsideDeclaredScope(t *testing.T) {
	t.Parallel()
	s := newLeaseStore(t)
	id := s.declaredTicket(t, "declared", "docs/", "src/a.go")
	first := s.claim(t, "claim-1", id, 0, "docs/x.md", "src/a.go")
	if first.Scope == nil || first.Scope.Source != "DECLARED" || len(first.Scope.Resources) != 2 {
		t.Fatalf("claim result scope: %+v", first.Scope)
	}
	if r := s.lease(t, "release-1", releaseOf(first), 1, nil); r.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("release: %+v", r)
	}
	before := s.attempt(t, first.AttemptID)
	refused := s.lease(t, "claim-2", claimOf(id, "cmd/x/testdata/g.json", "src/a.go", "src/b.go"), 2, nil)
	refusedWith(t, refused, mutation.OutcomeBlocked, wire.CodeOutOfScope)
	// ATR-V0-214: the rejected segment names exactly the unaccepted paths,
	// and the declared paths follow it.
	rejected, declared, _ := strings.Cut(refused.Detail, "; declared paths: ")
	if !strings.HasSuffix(rejected, ": cmd/x/testdata/g.json src/b.go") || !strings.HasPrefix(declared, "docs/ src/a.go;") {
		t.Fatalf("refusal must name exactly the unaccepted paths: %q", refused.Detail)
	}
	if sr := refused.ScopeRefusal; sr == nil || strings.Join(sr.Rejected, " ") != "cmd/x/testdata/g.json src/b.go" || strings.Join(sr.Declared, " ") != "docs/ src/a.go" {
		t.Fatalf("scope refusal %+v", refused.ScopeRefusal)
	}
	if refused.Scope != nil {
		t.Fatalf("a refused claim reserves no scope: %+v", refused.Scope)
	}
	if after := s.attempt(t, first.AttemptID); after.Generation != before.Generation || after.Phase != before.Phase {
		t.Fatalf("refused re-claim moved the attempt: %s/%s -> %s/%s", before.Generation, before.Phase, after.Generation, after.Phase)
	}
	second := s.claim(t, "claim-3", id, 3, "src/a.go")
	if second.AttemptID != first.AttemptID || second.Scope == nil || second.Scope.Source != "DECLARED" {
		t.Fatalf("covered re-claim: %+v", second)
	}
	auditOK(t, s.repo)
}

// TestATRV0211_RequestedScopeWithoutDeclaredPathsIsReported: a ticket that
// declares no path takes the REQUESTED scope, and the claim result lists it.
func TestATRV0211_RequestedScopeWithoutDeclaredPathsIsReported(t *testing.T) {
	t.Parallel()
	s := newLeaseStore(t)
	report := s.claim(t, "claim-1", s.ticket(t, "one"), 0, "docs/", "src/a.go")
	if report.Scope == nil || report.Scope.Source != "REQUESTED" || len(report.Scope.Resources) != 2 {
		t.Fatalf("claim result scope: %+v", report.Scope)
	}
}
