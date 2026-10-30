package corpusindex

import (
	"context"
	"runtime"
	"testing"

	"github.com/Beamfall/corvint/internal/doccorpus"
)

// TestDCPV1085OpenAllocationIgnoresEncoderPool brackets Open with TotalAlloc on a cold json/v2
// encoder pool (emptied by two GC cycles) and on the following, possibly warm, calls. Open's
// re-encodes once grew pooled json.Marshal buffers to several times the artifact (V1-1380).
func TestDCPV1085OpenAllocationIgnoresEncoderPool(t *testing.T) {
	t.Run("DCP-V1-085 Open allocation follows input size", func(t *testing.T) {
		root, corpus := fixture(t, 4000, 800)
		raw, err := Build(context.Background(), root, corpus)
		if err != nil {
			t.Fatal(err)
		}
		digest := doccorpus.Digest(raw)
		measure := func(cold bool) uint64 {
			if cold {
				runtime.GC()
				runtime.GC()
			}
			var before, after runtime.MemStats
			runtime.ReadMemStats(&before)
			if _, err := Open(context.Background(), raw, digest); err != nil {
				t.Fatal(err)
			}
			runtime.ReadMemStats(&after)
			return after.TotalAlloc - before.TotalAlloc
		}
		samples := []uint64{measure(true), measure(false), measure(false), measure(true)}
		low, high := samples[0], samples[0]
		for _, s := range samples {
			low, high = min(low, s), max(high, s)
		}
		t.Logf("open_total_alloc=%v input_bytes=%d race=%v", samples, len(raw), raceEnabled)
		if float64(high-low) > 0.10*float64(low) {
			t.Fatalf("Open allocation varies with encoder pool state: %v", samples)
		}
		if !raceEnabled && high > 6*uint64(len(raw)) {
			t.Fatalf("Open allocated %d bytes for %d input bytes", high, len(raw))
		}
	})
}
