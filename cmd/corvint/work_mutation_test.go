package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// WQO-V0-017: byte/type/name/mode evidence includes ignored and metadata stores.
func TestWorkMutationManifestCoverage(t *testing.T) {
	t.Parallel()
	t.Run("WQO-V0-017", func(t *testing.T) {
		cases := []struct {
			name   string
			mutate func(*testing.T, string)
		}{
			{"ignored-queue", func(t *testing.T, p string) { cemWrite(t, p, "queue/ignored", "changed") }},
			{"git-index", func(t *testing.T, p string) { cemWrite(t, p, "git/index", "changed") }},
			{"common-dir", func(t *testing.T, p string) { cemWrite(t, p, "common/refs/main", "changed") }},
			{"trace", func(t *testing.T, p string) { cemWrite(t, p, ".corvint/trace", "changed") }},
			{"external-checkpoint", func(t *testing.T, p string) { cemWrite(t, p, "external/checkpoint", "changed") }},
			{"external-config", func(t *testing.T, p string) { cemWrite(t, p, "external/config", "changed") }},
			{"addition", func(t *testing.T, p string) { cemWrite(t, p, "added", "new") }},
			{"removal", func(t *testing.T, p string) {
				if err := os.Remove(filepath.Join(p, "file")); err != nil {
					t.Fatal(err)
				}
			}},
			{"mode", func(t *testing.T, p string) {
				if err := os.Chmod(filepath.Join(p, "file"), 0755); err != nil {
					t.Fatal(err)
				}
			}},
			{"link", func(t *testing.T, p string) {
				if err := os.Remove(filepath.Join(p, "link")); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink("queue/ignored", filepath.Join(p, "link")); err != nil {
					t.Fatal(err)
				}
			}},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				root := t.TempDir()
				for _, p := range []string{"file", "queue/ignored", "git/index", "common/refs/main", ".corvint/trace", "external/checkpoint", "external/config"} {
					cemWrite(t, root, p, "before")
				}
				if err := os.Symlink("file", filepath.Join(root, "link")); err != nil {
					t.Fatal(err)
				}
				opening := workMutationManifest(context.Background(), []string{root})
				if !opening.monitoredComplete || opening.complete {
					t.Fatalf("coverage %#v", opening)
				}
				tc.mutate(t, root)
				closing := workCloseManifest(opening, workMutationManifest(context.Background(), []string{root}))
				if !closing.monitoredComplete || !closing.changed || closing.complete {
					t.Fatalf("missed mutation %#v", closing)
				}
			})
		}
	})
}

// WQO-V0-017: incomplete external scope and restored writes never prove no writes.
func TestWorkMutationManifestUnknownAndRestoredWrite(t *testing.T) {
	t.Parallel()
	t.Run("WQO-V0-017", func(t *testing.T) {
		root := t.TempDir()
		cemWrite(t, root, "file", "before")
		opening := workMutationManifest(context.Background(), []string{root})
		unchanged := workCloseManifest(opening, workMutationManifest(context.Background(), []string{root}))
		if unchanged.changed || unchanged.complete || !unchanged.monitoredComplete {
			t.Fatalf("coverage %#v", unchanged)
		}
		cemWrite(t, root, "file", "changed")
		cemWrite(t, root, "file", "before")
		restored := workCloseManifest(opening, workMutationManifest(context.Background(), []string{root}))
		if restored.changed || restored.complete {
			t.Fatalf("detect-only cannot detect restored bytes: %#v", restored)
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		incomplete := workCloseManifest(opening, workMutationManifest(ctx, []string{root}))
		if incomplete.monitoredComplete || incomplete.complete || incomplete.changed {
			t.Fatalf("incomplete %#v", incomplete)
		}
		incomplete = workCloseManifest(workManifest{changed: true}, incomplete)
		if !incomplete.changed {
			t.Fatal("positive evidence erased by incomplete closure")
		}
	})
}

// WQO-V0-017: unavailable scope cannot discard bytes observed in readable scope.
func TestWorkMutationRetainsPartialEvidence(t *testing.T) {
	t.Parallel()
	t.Run("WQO-V0-017", func(t *testing.T) {
		root := t.TempDir()
		cemWrite(t, root, "file", "before")
		other := t.TempDir()
		cemWrite(t, other, "file", "other")
		roots := []string{root, other}
		opening := workMutationManifest(context.Background(), roots)
		cemWrite(t, root, "file", "changed")
		if err := os.RemoveAll(other); err != nil {
			t.Fatal(err)
		}
		closing := workCloseManifest(opening, workMutationManifest(context.Background(), roots))
		if !closing.changed || closing.monitoredComplete || closing.complete {
			t.Fatalf("partial mutation erased %#v", closing)
		}
	})
}
