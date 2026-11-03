// SPDX-License-Identifier: Apache-2.0

//go:build linux

package intent_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/intent"
)

// TCX-V0-977: bytes read from B must never be retained under A's stat keys,
// even when B stays bound throughout phase 2 and A is restored before reuse.
func TestTCXV0977_CaptureDirectorySwapDoesNotPoisonRetention(t *testing.T) {
	root := loadTreeRoot(t, 9)
	dir := filepath.Join(root, intent.Dir, intent.TicketsDir)
	replacement := filepath.Join(root, "tickets-B")
	saved := filepath.Join(root, "tickets-A")
	if err := os.Mkdir(replacement, 0700); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		path := filepath.Join(dir, entry.Name())
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(replacement, entry.Name()), raw, 0600); err != nil {
			t.Fatal(err)
		}
		// A is divergent; B contains the original bytes at the same length.
		raw[len(raw)-1] = ' '
		if err := os.WriteFile(path, raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
	time.Sleep(2100 * time.Millisecond)
	var cache intent.TreeCapture
	restore := intent.SetBeforeTreeCaptureForTest(func() {
		if err := os.Rename(dir, saved); err != nil {
			t.Fatal(err)
		}
		if err := os.Rename(replacement, dir); err != nil {
			t.Fatal(err)
		}
	})
	defer restore()
	fromB, err := cache.Capture(root)
	restore()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(dir, replacement); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(saved, dir); err != nil {
		t.Fatal(err)
	}
	want, err := intent.TreeDigest(root)
	if err != nil {
		t.Fatal(err)
	}
	if fromB.Sha256 == want.Sha256 {
		t.Fatal("directory swap did not capture different bytes")
	}
	for round := 0; round < 2; round++ {
		got, err := cache.Capture(root)
		if err != nil || got.Sha256 != want.Sha256 {
			t.Fatalf("capture %d associated B's bytes with A's metadata: got %s, want %s, err %v", round, got.Sha256, want.Sha256, err)
		}
	}
}

// TCX-V0-977: compare the tree work formerly used at binding with a fresh
// content read. This is a local microbenchmark, not batch-close qualification.
func BenchmarkTCXV0977_BindingTree(b *testing.B) {
	for _, count := range []int{4000, 8000} {
		b.Run(fmt.Sprint(count), func(b *testing.B) {
			root := loadTreeRoot(b, count)
			time.Sleep(2100 * time.Millisecond)
			var cache intent.TreeCapture
			if _, err := cache.Capture(root); err != nil {
				b.Fatal(err)
			}
			for _, cached := range []bool{true, false} {
				b.Run(map[bool]string{true: "stat-cache", false: "live-bytes"}[cached], func(b *testing.B) {
					b.ReportAllocs()
					for i := 0; i < b.N; i++ {
						var err error
						if cached {
							_, err = cache.Capture(root)
						} else {
							_, err = intent.TreeDigest(root)
						}
						if err != nil {
							b.Fatal(err)
						}
					}
				})
			}
		})
	}
}
