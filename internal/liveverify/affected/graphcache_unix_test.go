//go:build darwin || linux

package affected_test

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// stageOnce runs action the first time the cache reaches stage.
func stageOnce(t *testing.T, stage string, action func()) {
	t.Helper()
	var once sync.Once
	t.Cleanup(affected.SetGraphCacheHook(func(reached string) {
		if reached == stage {
			once.Do(action)
		}
	}))
}

// swapForSymlink renames dir aside and puts a symbolic link to target in its
// place, as another process could after the placement check.
func swapForSymlink(t *testing.T, dir, target string) string {
	t.Helper()
	moved := dir + ".moved"
	if err := os.Rename(dir, moved); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, dir); err != nil {
		t.Fatal(err)
	}
	return moved
}

func assertOnlyKeep(t *testing.T, root string) {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(root, ".corvint"))
	if err != nil || len(entries) != 1 || entries[0].Name() != "keep" {
		t.Fatalf("the cache wrote into the repository's .corvint: %v %v", entries, err)
	}
}

// AFP-V0-390: a cache directory replaced by a symbolic link into the
// repository after the caller's placement check is refused when it is opened,
// and a replacement after the open cannot redirect the write.
func TestAFPV0390CacheDirectoryReplacementCannotRedirectWrites(t *testing.T) {
	t.Run("replaced before the open", func(t *testing.T) {
		pinnable(t)
		root := graphCacheFixture(t)
		dir := filepath.Join(t.TempDir(), "cache")
		if err := os.Mkdir(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		stageOnce(t, "open", func() { swapForSymlink(t, dir, filepath.Join(root, ".corvint")) })
		graph, outcome := buildCached(t, root, dir, graphCacheIdentity)
		const refusal = "cache directory is inside the repository or its Git directory"
		if outcome.Hit || outcome.Stored || outcome.Reason != refusal || outcome.NotStored != refusal {
			t.Fatalf("a directory replaced by a link into the repository must be refused: %+v", outcome)
		}
		assertOnlyKeep(t, root)
		assertSameGraph(t, root, graph)
	})
	t.Run("replaced after the open", func(t *testing.T) {
		pinnable(t)
		root := graphCacheFixture(t)
		dir := filepath.Join(t.TempDir(), "cache")
		moved := ""
		stageOnce(t, "write", func() { moved = swapForSymlink(t, dir, filepath.Join(root, ".corvint")) })
		_, outcome := buildCached(t, root, dir, graphCacheIdentity)
		if !outcome.Stored {
			t.Fatalf("the write through the open handle must succeed: %+v", outcome)
		}
		assertOnlyKeep(t, root)
		if entries, err := os.ReadDir(moved); err != nil || len(entries) != 1 {
			t.Fatalf("the entry must land in the directory that was opened: %v %v", entries, err)
		}
	})
	t.Run("separate Git directory", func(t *testing.T) {
		pinnable(t)
		root := graphCacheFixture(t)
		gitDirectory := t.TempDir()
		for _, dir := range []string{gitDirectory, filepath.Join(gitDirectory, "cache")} {
			graph, outcome, err := affected.BuildCachedExcluding(context.Background(), root, dir, []string{gitDirectory}, graphCacheIdentity, allLanguages()...)
			if err != nil || outcome.Stored || outcome.NotStored != "cache directory is inside the repository or its Git directory" {
				t.Fatalf("%s: a cache in the Git directory must not be written: %+v %v", dir, outcome, err)
			}
			assertSameGraph(t, root, graph)
		}
		if entries, _ := os.ReadDir(gitDirectory); len(entries) != 0 {
			t.Fatalf("the Git directory was written: %v", entries)
		}
	})
}

// AFP-V0-392: an entry replaced by a FIFO between any check and the open
// cannot hang the call; it misses and the graph is built fresh.
func TestAFPV0392FIFOEntryCannotBlock(t *testing.T) {
	pinnable(t)
	root := graphCacheFixture(t)
	dir := privateCacheDir(t)
	if _, outcome := buildCached(t, root, dir, graphCacheIdentity); !outcome.Stored {
		t.Fatalf("seed entry not stored: %+v", outcome)
	}
	entry := affected.GraphCacheEntryPath(dir, root, allLanguages())
	stageOnce(t, "entry", func() {
		if err := os.Remove(entry); err != nil {
			t.Error(err)
		}
		if err := syscall.Mkfifo(entry, 0o600); err != nil {
			t.Error(err)
		}
	})
	type result struct {
		graph   *affected.Graph
		outcome affected.CacheOutcome
		err     error
	}
	done := make(chan result, 1)
	go func() {
		graph, outcome, err := affected.BuildCached(context.Background(), root, dir, graphCacheIdentity, allLanguages()...)
		done <- result{graph, outcome, err}
	}()
	select {
	case got := <-done:
		if got.err != nil || got.outcome.Hit || got.outcome.Reason != "entry is not a readable regular file" {
			t.Fatalf("a FIFO entry must miss: %+v %v", got.outcome, got.err)
		}
		assertSameGraph(t, root, got.graph)
	case <-time.After(30 * time.Second):
		t.Fatal("opening a FIFO in place of the entry blocked the call")
	}
}
