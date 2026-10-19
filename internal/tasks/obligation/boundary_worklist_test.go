package obligation

import (
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestTOLV0083_BoundaryTestKind: the closed naming classifier counts the
// providers' test files and nothing else (TOL-V0-083).
func TestTOLV0083_BoundaryTestKind(t *testing.T) {
	for p, want := range map[string]string{
		"pkg/a_test.go": "go", "pkg/a.go": "", "tests/test_x.py": "python", "x_test.py": "python", "x.py": "",
		"e2e/login.spec.ts": "javascript", "a.test.jsx": "javascript", "a.ts": "", "spec/a_spec.rb": "ruby",
		"src/FooTest.java": "jvm", "src/FooTests.kt": "jvm", "tests/it.rs": "rust", "src/lib.rs": "",
		"FooTests.cs": "dotnet", "FooTests.swift": "swift", "FooTest.php": "php", "a_test.exs": "elixir",
		"deploy/main.tf": "", "README.md": "",
	} {
		if got := BoundaryTestKind(p); got != want {
			t.Errorf("BoundaryTestKind(%q) = %q, want %q", p, got, want)
		}
	}
}

// TestTOLV0083_BoundaryVerdictDirectoryCite: a directory cite is tracked
// through any file under it; an unlisted repository is UNKNOWN, never
// NO_TEST_BOUNDARY (TOL-V0-083).
func TestTOLV0083_BoundaryVerdictDirectoryCite(t *testing.T) {
	infra := "infra"
	b := NewBoundaryInventory(&infra, []string{"deploy", "deploy/", "gone.tf"})
	b.Visit("deploy/main.tf")
	inv := map[string]*BoundaryInventory{"infra": b}
	dir := ticket.ObligationEntry{Cites: []ticket.ObligationCite{{Repository: &infra, Path: "deploy"}}}
	if v, r := BoundaryVerdict(dir, inv); v != BoundaryNoTestBoundary || r != BoundaryNoRunnableTests {
		t.Fatalf("directory cite: %s/%s", v, r)
	}
	slash := ticket.ObligationEntry{Cites: []ticket.ObligationCite{{Repository: &infra, Path: "deploy/"}}}
	if v, r := BoundaryVerdict(slash, inv); v != BoundaryNoTestBoundary || r != BoundaryNoRunnableTests {
		t.Fatalf("directory cite with a trailing slash: %s/%s", v, r)
	}
	gone := ticket.ObligationEntry{Cites: []ticket.ObligationCite{{Repository: &infra, Path: "gone.tf"}}}
	if v, r := BoundaryVerdict(gone, inv); v != BoundaryNoTestBoundary || r != BoundaryNoTrackedCode {
		t.Fatalf("untracked cite: %s/%s", v, r)
	}
	other := "other"
	if v, r := BoundaryVerdict(ticket.ObligationEntry{Cites: []ticket.ObligationCite{{Repository: &other, Path: "x"}}}, inv); v != BoundaryUnknown || r != BoundaryRepositoryUnavailable {
		t.Fatalf("unlisted repository: %s/%s", v, r)
	}
	b.Visit("deploy/main_test.go")
	if v, _ := BoundaryVerdict(dir, inv); v != BoundaryTestable {
		t.Fatalf("after a test file: %s", v)
	}
}

func heldInput(ticketID, prio, id, path string, remaining int64, at string) WorklistInput {
	return WorklistInput{TicketID: ticketID, Priority: prio, Remaining: remaining, ReviewedAt: wire.Timestamp(at), Entry: ticket.ObligationEntry{
		ID: id, State: ticket.ObligationOpen, Review: &ticket.ObligationReviewRecord{Outcome: ticket.ObligationReviewHeld,
			Review: ticket.ObligationReview{Subject: ticket.ObligationReviewSubject{Path: path}, Verdicts: []ticket.ObligationVerdict{{Reviewer: "a", Verdict: ticket.VerdictPartial}}}}}}
}

func worklistOrder(items []WorklistInput) string {
	var out []string
	for _, in := range items {
		out = append(out, in.TicketID+"/"+in.Entry.ID)
	}
	return strings.Join(out, ",")
}

// TestTOLV0087_RankPriorityThenNearest: the default rank is priority, then
// fewest remaining core obligations; --next keeps only the head's test file
// (TOL-V0-087, TOL-V0-088).
func TestTOLV0087_RankPriorityThenNearest(t *testing.T) {
	items := []WorklistInput{
		heldInput("T-3", "P2", "AC-1", "a.spec.ts", 1, "2026-10-01T00:00:00Z"),
		heldInput("T-2", "P1", "AC-1", "b.spec.ts", 5, "2026-10-02T00:00:00Z"),
		heldInput("T-1", "P1", "AC-2", "a.spec.ts", 2, "2026-10-03T00:00:00Z"),
		heldInput("T-1", "P1", "AC-1", "a.spec.ts", 2, "2026-10-03T00:00:00Z"),
	}
	RankWorklist(items, WorklistDefaultRank)
	if got := worklistOrder(items); got != "T-1/AC-1,T-1/AC-2,T-2/AC-1,T-3/AC-1" {
		t.Fatalf("default rank: %s", got)
	}
	if got := worklistOrder(WorklistNext(items, 5)); got != "T-1/AC-1,T-1/AC-2,T-3/AC-1" {
		t.Fatalf("next batch: %s", got)
	}
	if got := worklistOrder(WorklistNext(items, 1)); got != "T-1/AC-1,T-1/AC-2" {
		t.Fatalf("next 1: %s", got)
	}
	if got := worklistOrder(WorklistNext(items, 0)); got != "T-1/AC-1" {
		t.Fatalf("next 0: %s", got)
	}
	RankWorklist(items, []string{WorklistRankAge})
	if got := worklistOrder(items); !strings.HasPrefix(got, "T-3/AC-1,T-2/AC-1") {
		t.Fatalf("age rank: %s", got)
	}
	if _, err := ParseWorklistRank("nearest,nearest"); err == nil {
		t.Fatal("repeated rank key admitted")
	}
}
