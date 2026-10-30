//go:build darwin || linux

package depsource

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// DSE-V0-004: a cache entry replaced between inventory and open by a symlink
// leading outside the cache, or by a FIFO, is refused promptly; its bytes never
// reach the hash or the captured excerpt.
func TestCacheEntrySwappedAfterInventoryIsRefused(t *testing.T) {
	outside := t.TempDir()
	writeFiles(t, outside, map[string]string{"dep.go": "package outside\n", "sub/inner.go": "package outside\n"})
	swaps := map[string]struct {
		rel  string
		swap func(string) error
	}{
		"symlink-outside":  {"dep.go", func(target string) error { return os.Symlink(filepath.Join(outside, "dep.go"), target) }},
		"fifo":             {"dep.go", func(target string) error { return syscall.Mkfifo(target, 0o644) }},
		"ancestor-symlink": {"sub/inner.go", func(target string) error { return os.Symlink(filepath.Join(outside, "sub"), target) }},
	}
	for name, swap := range swaps {
		t.Run(name, func(t *testing.T) {
			directory := t.TempDir()
			writeFiles(t, directory, fixtureFiles)
			target := filepath.Join(directory, filepath.FromSlash(swap.rel))
			if filepath.Base(swap.rel) != swap.rel {
				target = filepath.Dir(target)
			}
			afterCacheInventory = func() {
				if err := os.RemoveAll(target); err != nil {
					t.Error(err)
				}
				if err := swap.swap(target); err != nil {
					t.Error(err)
				}
			}
			defer func() { afterCacheInventory = func() {} }()
			type outcome struct {
				captured *fileCapture
				err      error
			}
			done := make(chan outcome, 1)
			go func() {
				_, _, captured, err := hashDirectory(context.Background(), directory, fixtureModule+"@"+fixtureVersion, swap.rel)
				done <- outcome{captured: captured, err: err}
			}()
			select {
			case got := <-done:
				if got.err == nil || got.captured != nil {
					t.Fatalf("swapped entry was hashed: captured=%v err=%v", got.captured, got.err)
				}
			case <-time.After(10 * time.Second):
				if writer, err := os.OpenFile(target, os.O_WRONLY|syscall.O_NONBLOCK, 0); err == nil {
					writer.Close()
				}
				t.Fatal("hashDirectory blocked on the swapped entry")
			}
		})
	}
}

// DSE-V0-004: a ziphash record that is a FIFO fails closed as a mismatch
// instead of blocking the open.
func TestZipHashFIFOIsRefused(t *testing.T) {
	hash := expectedHash1(t, fixtureFiles, fixtureModule+"@"+fixtureVersion)
	cache := newCache(t, fixtureFiles, hash)
	ziphashPath := filepath.Join(cache, "cache", "download", "example.com", "!dep", "@v", fixtureVersion+".ziphash")
	if err := os.Remove(ziphashPath); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(ziphashPath, 0o644); err != nil {
		t.Fatal(err)
	}
	root := fixtureRepository(t, hash)
	type outcome struct {
		result Result
		err    error
	}
	done := make(chan outcome, 1)
	go func() {
		result, err := Resolve(context.Background(), Options{Root: root, Module: fixtureModule, ModuleCache: cache})
		done <- outcome{result: result, err: err}
	}()
	select {
	case got := <-done:
		if got.err != nil || got.result.Verified || got.result.Reason != "ziphash-mismatch" {
			t.Fatalf("want ziphash-mismatch, got %+v, %v", got.result, got.err)
		}
	case <-time.After(10 * time.Second):
		if writer, err := os.OpenFile(ziphashPath, os.O_WRONLY|syscall.O_NONBLOCK, 0); err == nil {
			writer.Close()
		}
		t.Fatal("ziphash read blocked on a FIFO")
	}
}

// DSE-V0-004: a ziphash record whose presence cannot be determined because the
// cache root is unreadable is not an absent record; the verb must abstain
// rather than skip the check and report the module verified.
func TestZipHashBehindUnreadableCacheRootIsRefused(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory read permission")
	}
	hash := expectedHash1(t, fixtureFiles, fixtureModule+"@"+fixtureVersion)
	cache := newCache(t, fixtureFiles, "h1:"+strings.Repeat("A", 43)+"=")
	root := fixtureRepository(t, hash)
	if err := os.Chmod(cache, 0o311); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(cache, 0o755) })
	result, err := Resolve(context.Background(), Options{Root: root, Module: fixtureModule, ModuleCache: cache})
	if err != nil || result.Verified || result.Reason != "ziphash-mismatch" {
		t.Fatalf("want ziphash-mismatch, got %+v, %v", result, err)
	}
}

// DSE-V0-004: os.Root follows an in-root leaf symlink despite O_NOFOLLOW, so a
// ziphash record that is a symlink to a matching record inside the cache is
// still refused as a mismatch.
func TestZipHashInRootSymlinkIsRefused(t *testing.T) {
	hash := expectedHash1(t, fixtureFiles, fixtureModule+"@"+fixtureVersion)
	cache := newCache(t, fixtureFiles, hash)
	ziphashPath := filepath.Join(cache, "cache", "download", "example.com", "!dep", "@v", fixtureVersion+".ziphash")
	if err := os.Rename(ziphashPath, ziphashPath+".real"); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Base(ziphashPath)+".real", ziphashPath); err != nil {
		t.Fatal(err)
	}
	result, err := Resolve(context.Background(), Options{Root: fixtureRepository(t, hash), Module: fixtureModule, ModuleCache: cache})
	if err != nil || result.Verified || result.Reason != "ziphash-mismatch" {
		t.Fatalf("want ziphash-mismatch, got %+v, %v", result, err)
	}
}
