// SPDX-License-Identifier: Apache-2.0

package corpusbridge

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestDCRV1417ConcurrentColdCallsShareOneRead (V1-3028): concurrent cold calls that all find the
// artifact stamp unverified perform one artifact read and digest check and share its bytes, so
// at most one artifact copy is held for them, instead of one read per call before the load slot.
func TestDCRV1417ConcurrentColdCallsShareOneRead(t *testing.T) {
	root, _ := lazyFixture(t)
	r, e := New(root, "corpus.json")
	if e != nil {
		t.Fatal(e)
	}
	var reads, inFlight, peak atomic.Int32
	release := make(chan struct{})
	saved := readArtifactFile
	t.Cleanup(func() { readArtifactFile = saved })
	readArtifactFile = func(root, path string) ([]byte, error) {
		reads.Add(1)
		n := inFlight.Add(1)
		for p := peak.Load(); n > p && !peak.CompareAndSwap(p, n); p = peak.Load() {
		}
		<-release
		defer inFlight.Add(-1)
		return saved(root, path)
	}
	const calls = 16
	var wg sync.WaitGroup
	texts := make([]string, calls)
	for i := range texts {
		wg.Go(func() {
			_, texts[i], _, _ = r.Call(context.Background(), "corvint.docs_info", []byte(`{}`))
		})
	}
	// Let every call reach the read; before V1-3028 each blocked in its own read here.
	for deadline := time.Now().Add(2 * time.Second); reads.Load() < calls && time.Now().Before(deadline); {
		time.Sleep(10 * time.Millisecond)
	}
	close(release)
	wg.Wait()
	if got := reads.Load(); got != 1 {
		t.Fatalf("%d concurrent cold calls read the artifact %d times, want once", calls, got)
	}
	if got := peak.Load(); got != 1 {
		t.Fatalf("peak concurrent artifact reads %d, want 1", got)
	}
	for i, text := range texts {
		if text == "" || text != texts[0] {
			t.Fatalf("call %d answered %q", i, text)
		}
	}
}
