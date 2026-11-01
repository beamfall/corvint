package store

import (
	"path/filepath"
	"sync"
	"testing"
)

// observeDerived registers fn for the derived checkpoint writes of the store
// at stateDir until stop is called or the test ends, whichever is first
// (V1-3665). It replaces the package-global derivedWritten hook, so tests on
// separate stores may run in parallel.
func observeDerived(t *testing.T, stateDir string, fn func(path string, n int)) (stop func()) {
	t.Helper()
	if _, loaded := derivedObservers.LoadOrStore(stateDir, fn); loaded {
		t.Fatalf("derived writes of %s are already observed", stateDir)
	}
	derivedObserved.Add(1)
	var once sync.Once
	stop = func() {
		once.Do(func() {
			derivedObservers.Delete(stateDir)
			derivedObserved.Add(-1)
		})
	}
	t.Cleanup(stop)
	return stop
}

// TestDerivedObserversSeeOnlyTheirStore (V1-3665): two observers registered
// at once each see only the writes in or beside their own state directory,
// including one whose name extends the other's, and nothing reaches a store
// nobody observes.
func TestDerivedObserversSeeOnlyTheirStore(t *testing.T) {
	a := filepath.Join(t.TempDir(), "state")
	var mu sync.Mutex
	seen := map[string][]string{}
	record := func(name string) func(string, int) {
		return func(path string, _ int) {
			mu.Lock()
			defer mu.Unlock()
			seen[name] = append(seen[name], path)
		}
	}
	observeDerived(t, a, record("a"))
	observeDerived(t, a+"2", record("a2"))
	var wg sync.WaitGroup
	for _, path := range []string{a + ".checkpoint.json", a + ".writer-checkpoint.base", a + "2.writer-checkpoint", filepath.Join(filepath.Dir(a), "other.checkpoint.json")} {
		wg.Add(1)
		go func() { defer wg.Done(); noteDerived(path, 1) }()
	}
	wg.Wait()
	if len(seen["a"]) != 2 || len(seen["a2"]) != 1 || seen["a2"][0] != a+"2.writer-checkpoint" {
		t.Fatalf("observers saw %v", seen)
	}
}
