// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"bytes"
	"fmt"
	"slices"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/intent"
)

// TestTCXV0847PreloadMatchesFullLoad: the mutation preload holds indexed
// tickets as summaries, and its inventory orders, diagnoses, views and
// materializes every ticket exactly as the full load does.
func TestTCXV0847PreloadMatchesFullLoad(t *testing.T) {
	r := tcxRepo(t, true, true)
	full, err := intent.Load(r.Root)
	if err != nil {
		t.Fatal(err)
	}
	held, err := intent.LoadSummarized(r.Root, intent.ReadClosedIndex(r.StateDir))
	if err != nil {
		t.Fatal(err)
	}
	if len(held.Tickets) != 0 || len(full.Tickets) != 6 {
		t.Fatalf("preload decoded %d tickets up front, want none of %d", len(held.Tickets), len(full.Tickets))
	}
	if held.Tree.Sha256 != full.Tree.Sha256 || !slices.Equal(held.Inventory.Sorted(), full.Inventory.Sorted()) {
		t.Fatal("preload tree or planning order differs from the full load")
	}
	for _, id := range full.Inventory.IDs() {
		if a, b := fmt.Sprint(held.Inventory.Problems(id)), fmt.Sprint(full.Inventory.Problems(id)); a != b {
			t.Errorf("%s problems %s, want %s", id, a, b)
		}
		va, _ := held.Inventory.View(id, held.Context())
		vb, _ := full.Inventory.View(id, full.Context())
		if !bytes.Equal(va.Record.Encode(), vb.Record.Encode()) {
			t.Errorf("%s view record differs", id)
		}
		va.Record, vb.Record = nil, nil
		if fmt.Sprint(va) != fmt.Sprint(vb) {
			t.Errorf("%s view %v, want %v", id, va, vb)
		}
		a, _ := held.Inventory.Get(id)
		b, _ := full.Inventory.Get(id)
		if !bytes.Equal(a.Encode(), b.Encode()) {
			t.Errorf("%s record differs", id)
		}
		if a, b := fmt.Sprint(unmetDependencyWarnings(held.Inventory, id)), fmt.Sprint(unmetDependencyWarnings(full.Inventory, id)); a != b {
			t.Errorf("%s warnings %s, want %s", id, a, b)
		}
	}
}
