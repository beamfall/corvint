//go:build darwin || linux

// SPDX-License-Identifier: Apache-2.0
package store

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/journal"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
)

// ATR-V0-7203 keeps the locked recheck without rereading audited files while
// building the preparation inventory. Unobserved files still get a fresh read.
func TestATRV07203_PreparationReusesSuccessfulAudit(t *testing.T) {
	repo, choice := inventoryClaimRepo(t)
	forgetLeaseAudits()
	hooks := hooksForInventory(context.Background())
	audit, read := hooks.audit, hooks.read
	borrowed := map[string]bool{}
	hooks.audit = func(r journal.Reader) (*journal.Result, journal.PhysicalObservation, error) {
		proof, obs, err := audit(r)
		if err == nil && proof != nil && proof.Mode == journal.ModeFull && obs.Cleanup == nil {
			for path := range obs.Files {
				full := filepath.Join(repo.StateDir, path)
				if strings.HasPrefix(path, "intent/") {
					full = filepath.Join(repo.IntentRoot(), intent.Dir, strings.TrimPrefix(path, "intent/"))
				}
				borrowed[full] = true
			}
		}
		return proof, obs, err
	}
	rereads := []string{}
	hooks.read = func(root *os.Root, path, name string, max int) ([]byte, error) {
		if borrowed[path] {
			rereads = append(rereads, path)
		}
		return read(root, path, name, max)
	}
	ctx := context.WithValue(context.Background(), inventoryHooksKey{}, hooks)
	rep, err := Lease(ctx, repo, mutation.Binding{ID: "tester", Role: "OWNER"}, choice, fixture.Timestamp)
	if err != nil || rep.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("claim: %+v %v", rep, err)
	}
	if !borrowed[filepath.Join(repo.IntentRoot(), intent.Dir, "policy.json")] {
		t.Fatal("no successful policy observation")
	}
	if len(rereads) > 0 {
		t.Fatalf("audited files reread during preparation: %v", rereads)
	}
}
