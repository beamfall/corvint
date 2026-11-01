//go:build darwin || linux

package store

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/authority"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestTRLV0960_InterruptedAfterRenameLeavesTheWholeStore (TRL-V0-960, V1-3590) interrupts a
// restore after its rename, at the parent directory sync. The store is whole and opens with no
// RESTORE_INCOMPLETE marker; the rerun refuses UNSUPPORTED naming the published store; and a
// marker found in a state dir names a recovery that works instead of `archive restore`, which
// refuses an existing state dir.
func TestTRLV0960_InterruptedAfterRenameLeavesTheWholeStore(t *testing.T) {
	repo, path, _ := restoreFixture(t)
	parent, err := os.Lstat(filepath.Dir(repo.StateDir))
	if err != nil {
		t.Fatal(err)
	}
	interrupted := errors.New("interrupted after the rename")
	t.Cleanup(authority.SetDirectorySyncTestHook(func(dir os.FileInfo, next func() error) error {
		if _, err := os.Lstat(repo.StateDir); err == nil && os.SameFile(dir, parent) {
			return interrupted
		}
		return next()
	}))
	if _, err := RestoreArchive(context.Background(), repo, path); !errors.Is(err, interrupted) {
		t.Fatalf("restore = %v, want the injected interruption after the rename", err)
	}
	if _, err := snapshot.Probe(repo.StateDir); err != nil {
		t.Fatalf("the store published before the interruption does not open: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(repo.StateDir, "RESTORE_INCOMPLETE")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("restore left a RESTORE_INCOMPLETE marker: %v", err)
	}
	_, err = RestoreArchive(context.Background(), repo, path)
	if wire.CodeOf(err) != wire.CodeUnsupported || !strings.Contains(err.Error(), "already published the whole store") {
		t.Fatalf("rerun = %v, want UNSUPPORTED naming the published store", err)
	}
	if err := os.WriteFile(filepath.Join(repo.StateDir, "RESTORE_INCOMPLETE"), []byte("incomplete\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for name, err := range map[string]error{"probe": probeErr(repo.StateDir), "writer guard": guardErr(repo)} {
		if wire.CodeOf(err) != wire.CodeRestoreIncomplete || !strings.Contains(err.Error(), "move the state dir aside") || strings.Contains(err.Error(), "may continue") {
			t.Errorf("%s with a marker = %v, want RESTORE_INCOMPLETE naming the move-aside recovery", name, err)
		}
	}
}

func probeErr(stateDir string) error {
	_, err := snapshot.Probe(stateDir)
	return err
}

func guardErr(repo *intent.Repository) error {
	_, err := guardsWith(repo, "MUTATE", true)
	return err
}
