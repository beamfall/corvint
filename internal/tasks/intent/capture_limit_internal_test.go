// SPDX-License-Identifier: Apache-2.0

package intent

import (
	"errors"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestTCXV0_CachedEntriesCountTowardTreeBound: a file read during capture
// that grew since the stat pass, followed by indexed entries that are not
// read, still refuses a tree over the bound with LIMIT_EXCEEDED (TCX-V0-981).
func TestTCXV0_CachedEntriesCountTowardTreeBound(t *testing.T) {
	read := func(string, string, int) ([]byte, error) { return []byte("x"), nil }
	plan := []plannedFile{
		{path: "tickets/OPEN.json", max: wire.MaxIntentTreeBytes},
		{path: "tickets/CLOSED.json", known: &ClosedEntry{Size: int64(wire.MaxIntentTreeBytes)}},
	}
	_, err := captureTree("/store", plan, read)
	var we *wire.Error
	if !errors.As(err, &we) || we.Code != wire.CodeLimitExceeded {
		t.Fatalf("capture of %d bytes: %v, want LIMIT_EXCEEDED", wire.MaxIntentTreeBytes+1, err)
	}
	plan[1].known.Size--
	if tree, err := captureTree("/store", plan, read); err != nil || tree.TotalBytes != wire.MaxIntentTreeBytes {
		t.Fatalf("capture at the bound: %d bytes, %v", tree.TotalBytes, err)
	}
}
