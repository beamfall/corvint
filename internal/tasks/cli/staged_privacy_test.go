// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// ASD-V0-2421: automatic snapshot hints must not follow staging links
// into protected home folders. Use real symlinks on the host filesystem.
func TestASDV02421StagedHintsAvoidProtectedFolders(t *testing.T) {
	for _, kind := range []string{"base", "directory", "file", "indirect"} {
		t.Run(kind, func(t *testing.T) {
			home, err := filepath.EvalSymlinks(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			t.Setenv("HOME", home)
			queue := fixture.QueueBytes()
			raw, err := json.Marshal(snapshotBundle{Profile: snapshotBundleSchema, Files: []snapshotBundleFile{{Path: "queue.json", Sha256: string(wire.Sum(queue)), Content: string(queue)}}})
			if err != nil {
				t.Fatal(err)
			}
			media := filepath.Join(home, "Music")
			if err := os.MkdirAll(media, 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(media, "queue-snapshot.json"), raw, 0600); err != nil {
				t.Fatal(err)
			}
			base := filepath.Join(home, "lane-inputs")
			// Prove the ordinary candidate path works before redirecting it.
			if err := os.MkdirAll(base, 0700); err != nil {
				t.Fatal(err)
			}
			control := filepath.Join(base, "queue-snapshot.json")
			if err := os.WriteFile(control, raw, 0600); err != nil {
				t.Fatal(err)
			}
			if got, _ := stagedSnapshots(); len(got) != 1 || got[0].path != control {
				t.Fatalf("fixture control failed: %+v", got)
			}
			if got := stagedSnapshotQueue(control); got != fixture.QueueID {
				t.Fatalf("encoded queue control failed: %s", got)
			}
			if err := os.Remove(control); err != nil {
				t.Fatal(err)
			}
			if kind == "base" {
				if err := os.Remove(base); err != nil {
					t.Fatal(err)
				}
			}
			link := func(target, path string) {
				t.Helper()
				if err := os.Symlink(target, path); err != nil {
					t.Fatal(err)
				}
			}
			if kind == "base" {
				link(media, base)
			} else {
				if err := os.MkdirAll(base, 0700); err != nil {
					t.Fatal(err)
				}
				switch kind {
				case "directory":
					link(media, filepath.Join(base, "ticket-bodies-media"))
				case "file":
					link(filepath.Join(media, "queue-snapshot.json"), filepath.Join(base, "queue-snapshot.json"))
				case "indirect":
					link(media, filepath.Join(home, "redirect"))
					link(filepath.Join(home, "redirect"), filepath.Join(base, "ticket-bodies-media"))
				}
			}
			candidate := filepath.Join(base, "queue-snapshot.json")
			if kind == "directory" || kind == "indirect" {
				candidate = filepath.Join(base, "ticket-bodies-media", "queue-snapshot.json")
			}
			if got, err := os.ReadFile(candidate); err != nil || !bytes.Equal(got, raw) {
				t.Fatalf("injected target not reachable: %q %v", got, err)
			}
			if got := stagedSnapshotQueue(candidate); got != "" {
				t.Fatalf("protected snapshot streamed: %s", got)
			}
			if got, truncated := stagedSnapshots(); len(got) != 0 || truncated {
				t.Fatalf("protected folder probed: %+v truncated=%v", got, truncated)
			}
		})
	}
}
