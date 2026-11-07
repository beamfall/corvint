// Package currentproof is the deliberate root-locator for the companionrelease
// proofs over the enclosing checkout's committed Git content (V1-0643,
// PUB-V0-670). It stays on the affected-plan UNBOUNDED_READER floor so that
// internal/companionrelease itself can declare a bounded read scope.
package currentproof

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Beamfall/corvint/internal/companionrelease"
)

const gitPath = "/usr/bin/git"

func checkoutRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	return root
}

// Explicit supplemental proof; final nine-component qualification remains a separate gate.
func TestCurrentExportedTasksAndVSIX(t *testing.T) {
	cache := os.Getenv("CORVINT_PROOF_NPM_CACHE")
	if cache == "" {
		t.Skip("requires populated npm cache proof input")
	}
	t.Run("CRB-V0-017 exported-current-tasks-and-vsix", func(t *testing.T) {
		log, err := companionrelease.ProveCurrentExportedTasksAndVSIX(context.Background(), gitPath, checkoutRoot(t), t.TempDir(), cache)
		t.Log(log)
		if err != nil {
			t.Fatal(err)
		}
	})
}

func TestCurrentExportedTasksSmokeGenerator(t *testing.T) {
	log, err := companionrelease.ProveCurrentTasksSmokeGenerator(context.Background(), gitPath, checkoutRoot(t), t.TempDir())
	t.Log(log)
	if err != nil {
		t.Fatal(err)
	}
}

// This archive-only regression runs without npm or a VSIX cache. It uses the
// shipped tar bytes, not the parallel materialization path.
func TestCurrentTasksSourceArchiveBuildsOffline(t *testing.T) {
	t.Run("PUB-V0-012 PUB-V0-013 actual Tasks archive dependency closure", func(t *testing.T) {
		log, err := companionrelease.ProveCurrentTasksSourceArchiveBuildsOffline(context.Background(), gitPath, checkoutRoot(t), t.TempDir())
		t.Log(log)
		if err != nil {
			t.Fatal(err)
		}
	})
}
