package journal

import (
	"math"
	"testing"
)

// TestWriterBaseReadBoundClampsToInt pins V1-3581: the writer base bound is
// an int64 so 32-bit targets compile, and the bound a read applies is that
// value clamped to the platform int (unchanged on the 64-bit targets).
func TestWriterBaseReadBoundClampsToInt(t *testing.T) {
	want := MaxWriterBaseBytes
	if want > math.MaxInt {
		want = math.MaxInt
	}
	if int64(MaxWriterBaseReadBytes) != want {
		t.Fatalf("MaxWriterBaseReadBytes = %d, want %d", MaxWriterBaseReadBytes, want)
	}
	if math.MaxInt == math.MaxInt64 && int64(MaxWriterBaseReadBytes) != MaxWriterBaseBytes {
		t.Fatalf("64-bit read bound %d differs from MaxWriterBaseBytes %d", MaxWriterBaseReadBytes, MaxWriterBaseBytes)
	}
}
