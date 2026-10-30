// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/dispatch"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestATRV0162_DispatchStatusRendersForNilConfig: a nil configuration renders
// the same status as an empty one rather than panicking (V1-1767).
func TestATRV0162_DispatchStatusRendersForNilConfig(t *testing.T) {
	dir := t.TempDir()
	l := &dispatch.Ledger{Program: "prog", Backoff: map[string]*dispatch.BackoffState{}}
	now := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	got := wire.Encode(dispatchStatusValue(nil, dir, l, nil, now))
	want := wire.Encode(dispatchStatusValue(&dispatch.Config{StateDir: dir}, dir, l, nil, now))
	if string(got) != string(want) {
		t.Fatalf("nil config status\n%s\nwant\n%s", got, want)
	}
}
