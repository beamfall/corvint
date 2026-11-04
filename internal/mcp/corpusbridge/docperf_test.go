// SPDX-License-Identifier: Apache-2.0

package corpusbridge

import (
	"context"
	"fmt"
	"os"
	"slices"
	"testing"
	"time"
)

func BenchmarkDocperfStampChunks(b *testing.B) {
	savedWindow, savedChunk := stampRacyWindow, stampStatChunk
	b.Cleanup(func() { stampRacyWindow, stampStatChunk = savedWindow, savedChunk })
	stampRacyWindow = 0
	root, extra := multiRepositoryFixture(b, [3]int{4000, 12000, 1000})
	dirs := append([]string{root}, extra...)
	for _, chunk := range []int{64, 128, 256, 512, 1024, 2048} {
		b.Run(fmt.Sprint(chunk), func(b *testing.B) {
			stampStatChunk = chunk
			b.ReportAllocs()
			for b.Loop() {
				if checkoutStamp(context.Background(), dirs) == "" {
					b.Fatal("no stamp")
				}
			}
		})
	}
}

// DCR-V1-7802: assembling a chunk must allocate in proportion to the chunks,
// rather than repeatedly grow a record buffer for its individual entries.
func TestDCRV17802StampChunkAllocation(t *testing.T) {
	supportedStampPlatform(t)
	info, err := os.Stat("stamp.go")
	if err != nil {
		t.Fatal(err)
	}
	entries := make([]os.DirEntry, 1024)
	for i := range entries {
		entries[i] = stampTestEntry{info}
	}
	s := &stamper{ctx: context.Background(), horizon: 1<<63 - 1}
	allocs := testing.AllocsPerRun(20, func() {
		if c := s.describe(entries); c.err != nil {
			t.Fatal(c.err)
		}
	})
	if allocs > 4 {
		t.Fatalf("chunk assembly allocated %.0f times, want at most 4", allocs)
	}
}

type stampTestEntry struct{ os.FileInfo }

func (e stampTestEntry) Type() os.FileMode          { return e.Mode().Type() }
func (e stampTestEntry) Info() (os.FileInfo, error) { return e.FileInfo, nil }

// TestDocperfStampQualification is opt-in because it creates the complete
// DCP-V1-317 fixture. It reports individual capture latency, not fixture setup.
func TestDocperfStampQualification(t *testing.T) {
	if os.Getenv("CORVINT_DOCPERF_STAMP") == "" {
		t.Skip("set CORVINT_DOCPERF_STAMP=1")
	}
	saved := stampRacyWindow
	t.Cleanup(func() { stampRacyWindow = saved })
	stampRacyWindow = 0
	root, extra := multiRepositoryFixture(t, [3]int{4000, 12000, 1000})
	dirs := append([]string{root}, extra...)
	var timings []time.Duration
	before, _ := os.ReadFile("/proc/loadavg")
	for range 20 {
		start := time.Now()
		if checkoutStamp(context.Background(), dirs) == "" {
			t.Fatal("no stamp")
		}
		timings = append(timings, time.Since(start))
	}
	slices.Sort(timings)
	after, _ := os.ReadFile("/proc/loadavg")
	t.Logf("DCR-V1-7802 n=20 p50=%v p95=%v min=%v max=%v load_before=%s load_after=%s", timings[9], timings[18], timings[0], timings[19], before, after)
}
