// SPDX-License-Identifier: Apache-2.0

//go:build linux

package cli

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// ASD-V0-2421: an ancestor of HOME that grants traversal but no directory
// read, as / and the checkout's ancestors are under a package's declared
// read scope (test-confine, AFP-V0-023), must neither suppress a staged hint
// nor relax the protected-folder rule. An execute-only directory reproduces
// that boundary without Landlock.
func TestASDV02421StagedHintsNeedNoAncestorRead(t *testing.T) {
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	ancestor := filepath.Join(base, "ancestor")
	home := filepath.Join(ancestor, "home")
	lanes := filepath.Join(home, "lane-inputs")
	media := filepath.Join(home, "Music")
	elsewhere := filepath.Join(ancestor, "elsewhere")
	for _, dir := range []string{lanes, media, elsewhere} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	queue := fixture.QueueBytes()
	raw, err := json.Marshal(snapshotBundle{Profile: snapshotBundleSchema, Files: []snapshotBundleFile{{Path: "queue.json", Sha256: string(wire.Sum(queue)), Content: string(queue)}}})
	if err != nil {
		t.Fatal(err)
	}
	top := filepath.Join(lanes, "queue-snapshot.json")
	for _, file := range []string{top, filepath.Join(elsewhere, "queue-snapshot.json"), filepath.Join(media, "queue-snapshot.json")} {
		if err := os.WriteFile(file, raw, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	// A benign staging link whose target lies below the unreadable ancestor,
	// and one into a protected folder.
	if err := os.Symlink(elsewhere, filepath.Join(lanes, "ticket-bodies-linked")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(media, filepath.Join(lanes, "ticket-bodies-media")); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	if err := os.Chmod(ancestor, 0o111); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(ancestor, 0o755); err != nil {
			t.Error(err)
		}
	})
	if f, err := os.Open(ancestor); err == nil {
		_ = f.Close()
		t.Skip("runner can read an execute-only directory; the ancestor-read boundary is not observable")
	} else if !errors.Is(err, os.ErrPermission) {
		t.Fatal(err)
	}

	linked := filepath.Join(lanes, "ticket-bodies-linked", "queue-snapshot.json")
	got, truncated := stagedSnapshots()
	var paths []string
	for _, staged := range got {
		paths = append(paths, staged.path)
	}
	slices.Sort(paths)
	if want := []string{top, linked}; truncated || !slices.Equal(paths, want) {
		t.Fatalf("staged snapshots below an unreadable ancestor = %q (truncated %v), want %q", paths, truncated, want)
	}
	for _, path := range paths {
		if queueID := stagedSnapshotQueue(path); queueID != fixture.QueueID {
			t.Fatalf("queue of %s = %q, want %q", path, queueID, fixture.QueueID)
		}
	}
	if f, err := openStagedPath(filepath.Join(lanes, "ticket-bodies-media", "queue-snapshot.json")); err == nil {
		_ = f.Close()
		t.Fatal("a staging link into a protected folder was opened")
	}
}
