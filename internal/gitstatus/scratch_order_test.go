//go:build darwin || linux

package gitstatus

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"syscall"
	"testing"
)

// userCacheAt points os.UserCacheDir at a fresh directory under dir and returns it.
func userCacheAt(t *testing.T, dir string) string {
	t.Helper()
	cache := filepath.Join(dir, "cache")
	if runtime.GOOS == "darwin" {
		t.Setenv("HOME", dir)
		cache = filepath.Join(dir, "Library", "Caches")
	} else {
		t.Setenv("XDG_CACHE_HOME", cache)
	}
	if err := os.MkdirAll(cache, 0o700); err != nil {
		t.Fatal(err)
	}
	if got, err := os.UserCacheDir(); err != nil || got != cache {
		t.Fatalf("user cache dir=%q error=%v, want %q", got, err, cache)
	}
	return cache
}

// GPK-V0-882: with TMPDIR unset the user cache directory precedes the shared
// /tmp, and the first entry is not the operator's; a set TMPDIR stays first.
func TestGPKV0882UnsetTMPDIRPrefersUserCacheOverTmp(t *testing.T) {
	cache := userCacheAt(t, t.TempDir())
	t.Setenv("TMPDIR", "")
	parents, explicit := ambientScratchParents()
	if explicit || !slices.Equal(parents, []string{cache, "/tmp"}) {
		t.Fatalf("TMPDIR unset: parents=%q explicit=%v, want [%s /tmp] false", parents, explicit, cache)
	}
	operator := t.TempDir()
	t.Setenv("TMPDIR", operator)
	parents, explicit = ambientScratchParents()
	if !explicit || !slices.Equal(parents, []string{operator, cache, "/tmp"}) {
		t.Fatalf("TMPDIR set: parents=%q explicit=%v", parents, explicit)
	}
}

// GPK-V0-882: with TMPDIR unset the scratch is created under the user cache
// directory, Status still reads, and a user cache inside the repository is
// skipped (not refused as an operator TMPDIR would be).
func TestGPKV0882StatusUsesUserCacheWhenTMPDIRUnset(t *testing.T) {
	root := fixture(t)
	cache := userCacheAt(t, t.TempDir())
	t.Setenv("TMPDIR", "")
	resolved, err := filepath.EvalSymlinks(cache)
	if err != nil {
		t.Fatal(err)
	}
	parents, explicit := ambientScratchParents()
	temp, err := scratchUnder(context.Background(), parents, explicit, root)
	if err != nil {
		t.Fatal(err)
	}
	releaseScratch(temp)
	if !strings.HasPrefix(temp, resolved+string(filepath.Separator)) {
		t.Fatalf("scratch %s is not under the user cache directory %s", temp, resolved)
	}
	args := []string{"status", "--porcelain=v1", "-z"}
	want := gitTest(t, root, args...)
	if got, err := Status(context.Background(), root, metadataLimit, testRun, args...); err != nil || string(got) != string(want) {
		t.Fatalf("status=%q want=%q error=%v", got, want, err)
	}
	inside := filepath.Join(root, "cache")
	if err := os.Mkdir(inside, 0o700); err != nil {
		t.Fatal(err)
	}
	if temp, err := scratchUnder(context.Background(), []string{inside, t.TempDir()}, false, root); err != nil || strings.HasPrefix(temp, inside) {
		t.Fatalf("user cache inside the repository: scratch=%s error=%v, want it skipped", temp, err)
	} else {
		releaseScratch(temp)
	}
}

// GPK-V0-882: a full scratch filesystem is a typed scratch-dir refusal with a
// fixed reason and remedy, never a bare ENOSPC.
func TestGPKV0882FullScratchRefusalIsTyped(t *testing.T) {
	full := copyWriteRefusal("index", &fs.PathError{Op: "write", Path: "/tmp/x/metadata/index", Err: syscall.ENOSPC})
	if RefusalClass(full) != string(classScratchDir) {
		t.Fatalf("class=%s", RefusalClass(full))
	}
	want := "Git status cannot safely observe repository metadata: private copy of metadata file index cannot be written: the scratch filesystem is full; set TMPDIR to a writable directory with free space outside the repository"
	if RefusalMessage(full) != want || strings.Contains(RefusalMessage(full), "no space left") {
		t.Fatalf("refusal=%q want %q", RefusalMessage(full), want)
	}
	other := copyWriteRefusal("index", &fs.PathError{Op: "write", Path: "x", Err: syscall.EACCES})
	if reason, _ := RefusalReason(other); reason != "private copy of metadata file index cannot be written" || errors.Is(other, syscall.ENOSPC) {
		t.Fatalf("non-full refusal reason=%q", reason)
	}
}
