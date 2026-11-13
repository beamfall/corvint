// SPDX-License-Identifier: Apache-2.0

package appflows

import (
	"testing"

	"github.com/Beamfall/corvint/internal/runevidence"
)

// RVD-V1-007, AFU-V1-161: a clean PASS at HEAD stored first and a failing run of the same test
// against the dirty worktree stored after it give the newer FAIL, marked stale with its commit and
// tree, never the older PASS. Stored the other way round, the newer clean PASS wins and is fresh.
// An older run on another commit followed by none at HEAD stays the newest and stale.
func TestRVDV1007NewestStoredResultWins(t *testing.T) {
	at := head{commit: "c1", tree: "t1"}
	stored := func(id, outcome string, clean bool) []TestRunEvidence {
		t.Helper()
		records, err := storedRecords(runevidence.Record{
			Entry: runevidence.Entry{ID: id, Profile: runevidence.ProfileReceipt},
			Rows:  []runevidence.Row{{TestID: "go:a", Outcome: outcome, Commit: "c1", Tree: "t1", Clean: clean}},
		})
		if err != nil || len(records) != 1 {
			t.Fatalf("stored records %+v %v", records, err)
		}
		return records
	}
	cleanPass, dirtyFail := stored("clean-pass", "passed", true), stored("dirty-fail", "failed", false)

	got := lastResult(append(append([]TestRunEvidence{}, cleanPass...), dirtyFail...), "go:a", "", at)
	if got.Result != "failed" || !got.Stale || got.RunID != "dirty-fail" || got.SourceCommit != "c1" || got.SourceTree != "t1" {
		t.Fatalf("clean PASS then dirty FAIL: %+v", got)
	}
	got = lastResult(append(append([]TestRunEvidence{}, dirtyFail...), cleanPass...), "go:a", "", at)
	if got.Result != "passed" || got.Stale || got.RunID != "clean-pass" {
		t.Fatalf("dirty FAIL then clean PASS: %+v", got)
	}
	other := cleanPass[0]
	other.Source, other.RunID = RunSource{Commit: "c0", Tree: "t0", Clean: true}, "older-commit"
	got = lastResult([]TestRunEvidence{other}, "go:a", "", at)
	if got.Result != "passed" || !got.Stale || got.SourceCommit != "c0" {
		t.Fatalf("another commit only: %+v", got)
	}
}
