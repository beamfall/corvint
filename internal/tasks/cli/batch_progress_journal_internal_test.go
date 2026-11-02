// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestTLFV01102_BulkProgressWithoutJournalRefuses: in an intent-only
// checkout (no journal) a progress batch, with "current" or numeric
// revisions, refuses UNINITIALIZED instead of panicking on the absent
// journal snapshot (TLF-V0-1102).
func TestTLFV01102_BulkProgressWithoutJournalRefuses(t *testing.T) {
	t.Setenv("CORVINT_TASKS_ACTOR", "tester")
	r := fixture.TempRepo(t)
	fixture.WriteIntent(t, r, fixture.Ticket("A"))
	for _, rev := range []string{`"current"`, `"1"`} {
		payload := `[{"target":"A","expectedRevision":` + rev + `,"payload":{"phase":"CLEARED"}}]`
		got := projectionRead(t, Env{Cwd: r.Root, Args: []string{"ticket", "progress", "--batch", "--request-id", "b1", "--payload", payload}})
		if got.Outcome == wire.OutcomeOK || !wire.HasCode(got, wire.CodeUninitialized) {
			t.Fatalf("revision %s: %+v", rev, got)
		}
	}
}
