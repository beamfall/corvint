package store

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/authority"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/journal"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// CAL-V0-026: reuse is real and private corruption cannot borrow a valid key.
func TestCALV0026_VerifiedAuditReuse(t *testing.T) {
	r := fixture.TempRepo(t)
	fixture.Write(t, filepath.Join(r.IntentDir, "queue.json"), fixture.QueueBytes())
	fixture.Write(t, filepath.Join(r.IntentDir, "policy.json"), fixture.PolicyBytes())
	repo, err := intent.Resolve(r.Root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = Init(context.Background(), repo, mutation.Binding{ID: "tester", Role: "OWNER"}, "cache-init", wire.Timestamp("2026-09-27T00:00:00Z")); err != nil {
		t.Fatal(err)
	}
	read := func() (*journal.Result, error) {
		g, err := authority.WatchChanges(repo)
		if err != nil {
			return nil, err
		}
		defer g.Close()
		inv, err := inventory(repo)
		if err != nil {
			return nil, err
		}
		head, _, _, err := journalBytes(repo)
		if err != nil {
			return nil, err
		}
		return leaseAudit(repo, g, inv, head)
	}
	a, err := read()
	if err != nil {
		t.Fatal(err)
	}
	b, err := read()
	if err != nil || a != b {
		t.Fatalf("same-head cache was not reused: %v", err)
	}
	request, _ := filepath.Glob(filepath.Join(repo.StateDir, "requests", "*", "*"))
	if len(request) != 1 {
		t.Fatalf("requests %v", request)
	}
	if err := os.WriteFile(request[0], []byte("{}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := read(); err == nil {
		t.Fatal("private corruption reused cached proof")
	}
}
