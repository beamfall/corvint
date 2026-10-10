package store_test

import (
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestCALV0024_ExactPathKeyNamesTheDirectoryKeyCause: a PATH key without a
// trailing "/" names one path, so submit refuses OUT_OF_SCOPE for a file
// beneath it and names that key and the directory-key rule, without writing;
// the same tree is within a directory key ending in "/" (CAL-V0-021,
// CAL-V0-024, V1-1090).
func TestCALV0024_ExactPathKeyNamesTheDirectoryKeyCause(t *testing.T) {
	s := newGateStore(t)
	one, two := s.ticket(t, "one"), s.ticket(t, "two")
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
	s.lease(t, "release-1", releaseOf(claim), 2, nil)
	dir := s.claim(t, "claim-2", two, 3, "docs/build-log/", "src/a.go")
	if r := s.lease(t, "submit-2", submitOf(dir, tree), 4, nil); r.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("directory key submit: %+v", r)
	}
	auditOK(t, s.repo)
}
