// SPDX-License-Identifier: Apache-2.0

package corpusbridge

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
)

// stampBatchRepository is a stamp repository with one directory of n files.
func stampBatchRepository(t *testing.T, n int) string {
	t.Helper()
	dir := stampRepository(t)
	wide := filepath.Join(dir, "wide")
	if err := os.MkdirAll(wide, 0o700); err != nil {
		t.Fatal(err)
	}
	for i := range n {
		if err := os.WriteFile(filepath.Join(wide, fmt.Sprintf("f%03d", i)), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// countStampReads replaces the batched directory reader with one that counts
// the entries it returns and runs after on each batch.
func countStampReads(t *testing.T, after func()) *atomic.Int64 {
	t.Helper()
	var read atomic.Int64
	saved := readStampDir
	t.Cleanup(func() { readStampDir = saved })
	readStampDir = func(f *os.File, n int) ([]os.DirEntry, error) {
		entries, err := saved(f, n)
		read.Add(int64(len(entries)))
		if after != nil {
			after()
		}
		return entries, err
	}
	return &read
}

func setStampBatch(t *testing.T, batch int) {
	t.Helper()
	saved := stampReadBatch
	stampReadBatch = batch
	t.Cleanup(func() { stampReadBatch = saved })
}

// TestDCPV1314StampBoundStopsBatchedReads (V1-3029): the entry bound is
// enforced on each batched directory read, so a directory larger than the
// bound makes the stamp unavailable after at most one batch beyond the bound
// instead of after every entry was read and sorted.
func TestDCPV1314StampBoundStopsBatchedReads(t *testing.T) {
	supportedStampPlatform(t)
	shortenStampWindow(t)
	dir := stampBatchRepository(t, 400)
	saved := stampEntryBound
	stampEntryBound = 40
	t.Cleanup(func() { stampEntryBound = saved })
	setStampBatch(t, 8)
	read := countStampReads(t, nil)
	if got := checkoutStamp(context.Background(), []string{dir}); got != "" {
		t.Fatal("a checkout over the entry bound was stamped")
	}
	// Parallel directory reads may each finish one batch past the bound.
	if got, limit := read.Load(), stampEntryBound+int64(4*stampReadBatch); got > limit {
		t.Fatalf("read %d entries with a bound of %d, want at most %d", got, stampEntryBound, limit)
	}
}

// TestDCPV1314StampDigestIndependentOfBatch (V1-3029): batching does not
// change the stamp of a checkout within the bound.
func TestDCPV1314StampDigestIndependentOfBatch(t *testing.T) {
	supportedStampPlatform(t)
	shortenStampWindow(t)
	dir := stampBatchRepository(t, 50)
	want := ""
	for _, batch := range []int{1, 3, 7, 1 << 20} {
		setStampBatch(t, batch)
		got := checkoutStamp(context.Background(), []string{dir})
		if got == "" {
			t.Fatalf("batch %d: no stamp", batch)
		}
		if want == "" {
			want = got
		} else if got != want {
			t.Fatalf("batch %d stamp %s, want %s", batch, got, want)
		}
	}
}

// TestDCPV1314StampStopsOnCancel (V1-3029): a cancelled context stops the
// traversal at the next batch and leaves no stamp.
func TestDCPV1314StampStopsOnCancel(t *testing.T) {
	supportedStampPlatform(t)
	shortenStampWindow(t)
	dir := stampBatchRepository(t, 400)
	setStampBatch(t, 8)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var batches atomic.Int64
	read := countStampReads(t, func() {
		if batches.Add(1) == 2 {
			cancel()
		}
	})
	if got := checkoutStamp(ctx, []string{dir}); got != "" {
		t.Fatal("a cancelled traversal returned a stamp")
	}
	// Parallel readers each finish at most the batch in hand after the cancel.
	if got := read.Load(); got >= 200 {
		t.Fatalf("read %d entries after cancel, want the traversal to stop", got)
	}
}

// TestDCPV1314CancelledStampReleasesLoad (V1-3029): a call cancelled while
// the reused artifact's checkout is stamped returns the cancellation and
// releases the load slot.
func TestDCPV1314CancelledStampReleasesLoad(t *testing.T) {
	supportedStampPlatform(t)
	shortenStampWindow(t)
	root, _ := lazyFixture(t)
	r, e := New(root, "corpus.json")
	if e != nil {
		t.Fatal(e)
	}
	if _, _, failure, err := r.Call(context.Background(), "corvint.docs_info", []byte(`{}`)); err != nil || failure != nil {
		t.Fatalf("first call: %v %v", failure, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	saved := readStampDir
	t.Cleanup(func() { readStampDir = saved })
	readStampDir = func(f *os.File, n int) ([]os.DirEntry, error) {
		cancel()
		return saved(f, n)
	}
	_, _, err := r.artifactFor(ctx, func() ([]byte, error) { return nil, errors.New("unexpected read") }, &Timing{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled stamp: %v, want context.Canceled", err)
	}
	if len(r.load) != 0 {
		t.Fatal("the load slot was not released")
	}
}
