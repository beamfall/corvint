//go:build darwin || linux

package contextindex

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// IDX-SNAP-V0-032: inject a directory swap after the old preflight check.
// In-root targets matter: os.Root follows these despite O_NOFOLLOW.
func TestSnapshotRefreshLockDirectorySwap(t *testing.T) {
	for _, prefix := range []string{".corvint", "corvint"} {
		for _, component := range []string{prefix, filepath.Join(prefix, "index")} {
			t.Run(component, func(t *testing.T) {
				root := t.TempDir()
				store := snapshotStore{base: root, directory: filepath.Join(root, prefix, "index")}
				if err := os.MkdirAll(store.directory, 0o700); err != nil {
					t.Fatal(err)
				}
				if err := refuseLinkedSnapshotDirectory(root, store.directory); err != nil {
					t.Fatal(err)
				}
				original := filepath.Join(root, component)
				moved := filepath.Join(root, "moved")
				if err := os.Rename(original, moved); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(moved, original); err != nil {
					t.Fatal(err)
				}
				lock, err := openSnapshotRefreshLock(store, true)
				if lock != nil {
					lock.Close()
				}
				if err == nil {
					t.Fatal("directory swap accepted")
				}
				target := moved
				if component == prefix {
					target = filepath.Join(moved, "index")
				}
				if _, err := os.Lstat(filepath.Join(target, "refresh.lock")); !os.IsNotExist(err) {
					t.Fatalf("lock created outside store: %v", err)
				}
			})
		}
	}
	// Exercise the public validation boundary on a fallback store. The inherited
	// inode remains locked, but its old pathname now resolves through a symlink.
	for _, component := range []string{".corvint", ".corvint/index"} {
		t.Run("validate/"+component, func(t *testing.T) {
			root := t.TempDir()
			lock, err := AcquireSnapshotRefreshLock(root)
			if err != nil || lock == nil {
				t.Fatalf("fixture lock: %v", err)
			}
			defer lock.Close()
			if err := ValidateSnapshotRefreshLock(root, lock); err != nil {
				t.Fatal(err)
			}
			if _, err := lock.WriteString("untouched"); err != nil {
				t.Fatal(err)
			}
			original := filepath.Join(root, component)
			moved := filepath.Join(root, "moved")
			if err := os.Rename(original, moved); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(moved, original); err != nil {
				t.Fatal(err)
			}
			if err := ValidateSnapshotRefreshLock(root, lock); err == nil {
				t.Fatal("redirected inherited descriptor accepted")
			}
			data := make([]byte, len("untouched"))
			if _, err := lock.ReadAt(data, 0); err != nil || string(data) != "untouched" {
				t.Fatalf("metadata changed: %q %v", data, err)
			}
		})
	}
}

// IDX-SNAP-V0-032: race first creation of the same lock. In particular, Darwin
// O_CREAT without O_EXCL can spuriously fail with ENOENT under concurrent opens.
func TestSnapshotRefreshLockConcurrentCreate(t *testing.T) {
	for round := 0; round < 100; round++ {
		root := t.TempDir()
		var wg sync.WaitGroup
		locks := make(chan *os.File, 12)
		for i := 0; i < cap(locks); i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				lock, err := AcquireSnapshotRefreshLock(root)
				if err != nil {
					t.Errorf("concurrent acquisition: %v", err)
				}
				if lock != nil {
					locks <- lock
				}
			}()
		}
		wg.Wait()
		close(locks)
		if len(locks) != 1 {
			t.Errorf("round %d admitted %d writers, want one", round, len(locks))
		}
		for lock := range locks {
			lock.Close()
		}
	}
}
