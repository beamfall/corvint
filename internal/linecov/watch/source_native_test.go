//go:build linux || darwin

// SPDX-License-Identifier: Apache-2.0
package watch

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// LPCV-V0-7607, LPCV-V0-1086: both an atomic editor replacement and an
// in-place write of Go source are saves on every native platform.
func TestNativeSaveNotification(t *testing.T) {
	root := module(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	events, err := Native(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	tmp := filepath.Join(root, "p", "save.tmp")
	if err := os.WriteFile(tmp, []byte("package p\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(tmp, filepath.Join(root, "p", "p.go")); err != nil {
		t.Fatal(err)
	}
	awaitSave(t, ctx, events)
	quietSaves(t, ctx, events)
	// In place: the replaced file's own descriptor must have been rewatched.
	f, err := os.OpenFile(filepath.Join(root, "p", "p.go"), os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("package p\n\nfunc F() int { return 2 }\n"); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	awaitSave(t, ctx, events)
}

// LPCV-V0-7607: watch existing untracked, newly created and moved directories,
// including saves after their initial parent event has been consumed.
func TestNativeNewDirectorySaves(t *testing.T) {
	for _, mode := range []string{"existing-untracked", "created", "moved"} {
		t.Run(mode, func(t *testing.T) {
			root := module(t)
			dir := filepath.Join(root, "new", "nested")
			if mode == "existing-untracked" {
				if err := os.MkdirAll(dir, 0700); err != nil {
					t.Fatal(err)
				}
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			events, err := Native(ctx, root)
			if err != nil {
				t.Fatal(err)
			}
			if mode == "created" {
				if err := os.MkdirAll(dir, 0700); err != nil {
					t.Fatal(err)
				}
			} else if mode == "moved" {
				source := filepath.Join(t.TempDir(), "source")
				if err := os.MkdirAll(filepath.Join(source, "nested"), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.Rename(source, filepath.Dir(dir)); err != nil {
					t.Fatal(err)
				}
			}
			if mode != "existing-untracked" {
				awaitSave(t, ctx, events)
				quietSaves(t, ctx, events)
			}
			for i := 0; i < 3; i++ {
				if err := os.WriteFile(filepath.Join(dir, "new.go"), []byte("package new\n"), 0600); err != nil {
					t.Fatal(err)
				}
				awaitSave(t, ctx, events)
				quietSaves(t, ctx, events)
			}
		})
	}
}

// Greptile PR #97: a directory created after start is watched, a tracked
// directory deleted before start is tolerated, and Git-ignored output alone
// schedules no run.
func TestNativeSaveNotificationDirectoriesAndIgnored(t *testing.T) {
	root := module(t)
	write(t, root, ".gitignore", "output.json\nhidden.go\n")
	write(t, root, "gone/g.go", "package gone\n")
	commit(t, root, "dirs")
	if err := os.RemoveAll(filepath.Join(root, "gone")); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	events, err := Native(ctx, root)
	if err != nil {
		t.Fatalf("deleted tracked directory aborted the watcher: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "p", "output.json"), []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	select {
	case <-events.C:
		t.Fatal("ignored output scheduled a run")
	case <-time.After(500 * time.Millisecond):
	}
	// Greptile PR #97: ignored Go source still compiles into go test.
	if err := os.WriteFile(filepath.Join(root, "p", "hidden.go"), []byte("package p\n"), 0600); err != nil {
		t.Fatal(err)
	}
	awaitSave(t, ctx, events)
	quietSaves(t, ctx, events)
	fresh := filepath.Join(root, "fresh")
	if err := os.Mkdir(fresh, 0700); err != nil {
		t.Fatal(err)
	}
	awaitSave(t, ctx, events) // the mkdir itself
	quietSaves(t, ctx, events)
	if err := os.WriteFile(filepath.Join(fresh, "f.go"), []byte("package fresh\n"), 0600); err != nil {
		t.Fatal(err)
	}
	awaitSave(t, ctx, events)
}

// LPCV-V0-1087: a root that disappears ends the source with an error, which
// Err reports after C closes; a context end is a deliberate stop with no error.
func TestNativeStopIsExplicit(t *testing.T) {
	t.Run("root-removed", func(t *testing.T) {
		root := module(t)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		events, err := Native(ctx, root)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.RemoveAll(root); err != nil {
			t.Fatal(err)
		}
		for {
			select {
			case _, ok := <-events.C:
				if ok {
					continue
				}
				if events.Err() == nil {
					t.Fatal("a removed root stopped the source silently")
				}
				return
			case <-ctx.Done():
				t.Fatal("source kept running after its root was removed")
			}
		}
	})
	t.Run("context", func(t *testing.T) {
		root := module(t)
		ctx, cancel := context.WithCancel(context.Background())
		events, err := Native(ctx, root)
		if err != nil {
			t.Fatal(err)
		}
		if events.Err() != nil {
			t.Fatal("Err before the source stopped")
		}
		cancel()
		deadline := time.After(10 * time.Second)
		for {
			select {
			case _, ok := <-events.C:
				if ok {
					continue
				}
				if err := events.Err(); err != nil {
					t.Fatalf("a context end reported %v", err)
				}
				return
			case <-deadline:
				t.Fatal("source did not stop with its context")
			}
		}
	})
}
