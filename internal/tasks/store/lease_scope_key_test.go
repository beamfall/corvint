package store_test

import (
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestCALV0024_ExactPathKeyNamesTheDirectoryKeyCause: a PATH key without a
// trailing "/" names one path, so submit refuses OUT_OF_SCOPE for a file
// beneath it and names that key and the directory-key rule, without writing;
// once the attempt widens to the directory key ending in "/", the same
// candidate against the same base is within scope (CAL-V0-021, CAL-V0-024,
// V1-1090).
func TestCALV0024_ExactPathKeyNamesTheDirectoryKeyCause(t *testing.T) {
	s := newGateStore(t)
	one := s.ticket(t, "one")
	claim := s.claim(t, "claim-1", one, 0, "docs/build-log", "src/a.go")
	gitRun(t, s.root, "checkout", "-q", "-b", "agent")
	_, tree := s.commit(t, "docs/build-log/entry.md", "src/a.go")
	before := storeDigest(t, s.repo)
	report := s.lease(t, "submit-1", submitOf(claim, tree), 1, nil)
	refusedWith(t, report, mutation.OutcomeBlocked, wire.CodeOutOfScope)
	want := `outside the attempt's scope: docs/build-log/entry.md; a PATH key without a trailing "/" names one path, not the paths beneath it: docs/build-log (a directory key ends in "/")`
	if !strings.Contains(report.Detail, want) {
		t.Fatalf("detail: %q", report.Detail)
	}
	if storeDigest(t, s.repo) != before {
		t.Fatal("refused submit wrote")
	}
	widen := transaction.LeaseRequest{Verb: transaction.LeaseWiden, AttemptID: claim.AttemptID, Generation: claim.Generation, Scope: []string{"docs/build-log/"}}
	if r := s.lease(t, "widen-1", widen, 2, nil); r.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("widen to the directory key: %+v", r)
	}
	if r := s.lease(t, "submit-2", submitOf(claim, tree), 3, nil); r.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("directory key submit of the same candidate: %+v", r)
	}
	auditOK(t, s.repo)
}
