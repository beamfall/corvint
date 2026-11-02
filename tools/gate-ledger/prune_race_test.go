package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// TestPruneSurvivesConcurrentPrunes is GL-V0-1920 (V1-4262): concurrent prunes of one
// over-full ledger each glob the same names, one removes the overflow first, and the others
// stat fewer records than they globbed. Before the change the slice bound went negative and
// the tool panicked with "slice bounds out of range [:-N]".
func TestPruneSurvivesConcurrentPrunes(t *testing.T) {
	dir := t.TempDir()
	const extra = 64
	for i := 0; i < maxRecords+extra; i++ {
		if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("%064x.json", i)), []byte("{}\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	// Every writer records then prunes, as record() does, so a prune's glob goes stale while
	// other writers add and prune records: more of its globbed names vanish than its own
	// overflow, and it stats fewer than maxRecords.
	var wg sync.WaitGroup
	panics := make(chan any, 32)
	for g := 0; g < 32; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					panics <- r
				}
			}()
			l := &ledger{dir: dir}
			for i := 0; i < 24; i++ {
				key := fmt.Sprintf("%056x%08x", g+1, i)
				if err := l.record(record{Schema: schema, Step: "race", Key: key}); err != nil {
					t.Error(err)
					return
				}
			}
		}(g)
	}
	wg.Wait()
	close(panics)
	for r := range panics {
		t.Fatalf("prune panicked under concurrent prunes: %v", r)
	}
	matches, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) > maxRecords {
		t.Fatalf("concurrent writers left %d records, want at most %d", len(matches), maxRecords)
	}
}

// TestOverflowNeverSlicesBelowZero pins the bound itself: fewer survivors than keep prune
// nothing, and exactly the oldest beyond keep are returned otherwise (GL-V0-1920).
func TestOverflowNeverSlicesBelowZero(t *testing.T) {
	for _, tc := range []struct {
		n, keep, want int
	}{{0, 4, 0}, {3, 4, 0}, {4, 4, 0}, {5, 4, 1}, {9, 4, 5}} {
		records := make([]int, tc.n)
		for i := range records {
			records[i] = i
		}
		got := overflow(records, tc.keep)
		if len(got) != tc.want {
			t.Fatalf("overflow(%d records, keep %d) = %d, want %d", tc.n, tc.keep, len(got), tc.want)
		}
		for i, v := range got {
			if v != i {
				t.Fatalf("overflow(%d, %d) returned %v, want the oldest first", tc.n, tc.keep, got)
			}
		}
	}
}
