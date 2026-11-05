// SPDX-License-Identifier: Apache-2.0
package store

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/journal"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
)

// ATR-V0-7203: logical reads charged to the audit observation are not native
// live projections. Keep a requested sealed replay and the current state, but
// do not rehash unrelated sealed request/receipt history under the lock.
func TestATRV07203_SealedHistoryIsNotDecisionContent(t *testing.T) {
	repo := historyStore(t, 1100)
	if err := sealSegmentsLocked(context.Background(), repo); err != nil {
		t.Fatal(err)
	}
	head, err := readHead(repo)
	if err != nil || head.Segments == nil {
		t.Fatalf("sealed head: %+v %v", head, err)
	}
	proof, obs, err := journalReader(repo, head).AuditForWriteObserved()
	if err != nil || obs.Cleanup != nil || proof.Mode != journal.ModeFull {
		t.Fatalf("audit: %+v %v / %v", proof, err, obs.Cleanup)
	}
	sealed, _ := snapshot.RequestPath("history-100")
	unrelated, _ := snapshot.RequestPath("history-101")
	tail, _ := snapshot.RequestPath("history-1100")
	if _, err := os.Lstat(filepath.Join(repo.StateDir, sealed)); !os.IsNotExist(err) {
		t.Fatalf("sealed request still native: %v", err)
	}
	if _, ok := obs.Files[unrelated]; !ok {
		t.Fatal("fixture did not charge a logical sealed request")
	}
	inv, err := inventory(repo)
	if err != nil {
		t.Fatal(err)
	}
	binding, err := leaseDecisionContent(repo, inv, head, "history-100")
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{sealed, tail, "intent/policy.json", "intent/queue.json", "intent/tickets/AT-0002.json", "receipts/000000001100.json"} {
		if _, ok := binding[path]; !ok {
			t.Errorf("decision input omitted: %s", path)
		}
	}
	for _, path := range []string{unrelated, "receipts/000000000100.json"} {
		if _, ok := binding[path]; ok {
			t.Errorf("historical body retained: %s", path)
		}
	}
	if len(binding) > 800 {
		t.Fatalf("binding grew with history: %d entries", len(binding))
	}
	if err := verifyLeaseDecisionContent(repo, binding); err != nil {
		t.Fatal(err)
	}
}
