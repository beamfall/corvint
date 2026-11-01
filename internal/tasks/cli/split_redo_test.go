//go:build darwin || linux

package cli_test

import (
	"errors"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/cli"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestTLFV0884_ResumeSettlesAPendingReceipt: a split interrupted after its
// CREATE receipt was linked but before head.json advanced leaves
// REDO_PENDING; the rerun settles it under the writer lock before its first
// read and finishes the split.
func TestTLFV0884_ResumeSettlesAPendingReceipt(t *testing.T) {
	id, root := splitParent(t)
	holdPending(t, root, func() {
		t.Helper()
		restore := cli.SetSplitAfterStep(func(step string) error {
			if step == "create" {
				return errors.New("interrupted for the test")
			}
			return nil
		})
		defer restore()
		if x := atm(t, root, nil, splitArgs(id, "--request-id", "split-h")...); x.res.Outcome == wire.OutcomeOK {
			t.Fatalf("interrupted split reported OK: %s", x.stdout)
		}
	})
	y := atm(t, root, nil, splitArgs(id, "--request-id", "split-h")...)
	if y.res.Outcome != wire.OutcomeOK {
		t.Fatalf("resume over a pending receipt: %+v\n%s", y.res, y.stdout)
	}
	parent, _ := showRecord(t, root, id)
	if got := strs(field(parent, "acceptanceCriteria")); len(got) != 1 {
		t.Fatalf("parent after resume: %v", got)
	}
	if n := len(atm(t, root, nil, "ticket", "search", "--kind", "FEATURE").res.Items); n != 2 {
		t.Fatalf("resume made %d tickets, want 2", n)
	}
	if !auditConsistent(t, root) {
		t.Fatal("receipt audit is not CONSISTENT")
	}
}
