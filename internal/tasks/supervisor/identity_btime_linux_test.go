//go:build linux

package supervisor

import (
	"os"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// TestPSRV0227_BtimeSampleWithinTolerance (V1-1091 AC1) samples /proc/stat
// btime, as another process recording an identity would read it, and logs
// the distribution (run with -v to record it). Every sampled value must be
// within the one-second tolerance of the others, so an identity any of these
// reads produced compares as the same process; a wider spread is a realtime
// clock step, reported as unknown, never as another process.
func TestPSRV0227_BtimeSampleWithinTolerance(t *testing.T) {
	const n = 3000
	counts := map[uint64]int{}
	for i := 0; i < n; i++ {
		raw, e := os.ReadFile("/proc/stat")
		if e != nil {
			t.Skip("no /proc/stat:", e)
		}
		for _, line := range strings.Split(string(raw), "\n") {
			if v, ok := strings.CutPrefix(line, "btime "); ok {
				b, e := strconv.ParseUint(strings.TrimSpace(v), 10, 64)
				if e != nil {
					t.Fatalf("btime %q: %v", v, e)
				}
				counts[b]++
			}
		}
	}
	values := make([]uint64, 0, len(counts))
	for b := range counts {
		values = append(values, b)
	}
	sort.Slice(values, func(i, j int) bool { return values[i] < values[j] })
	var dist []string
	for _, b := range values {
		dist = append(dist, strconv.FormatUint(b, 10)+"x"+strconv.Itoa(counts[b]))
	}
	t.Logf("btime n=%d distinct=%d spread=%ds distribution=%s", n, len(values), values[len(values)-1]-values[0], strings.Join(dist, ","))
	for _, a := range values {
		for _, b := range values {
			got := CompareIdentity("linux:"+strconv.FormatUint(a, 10)+":77", "linux:"+strconv.FormatUint(b, 10)+":77")
			if got == IdentityDifferent {
				t.Fatalf("btime %d and %d compare as different processes", a, b)
			}
			if values[len(values)-1]-values[0] <= 1 && got != IdentitySame {
				t.Fatalf("btime %d and %d within 1 s compare as %d", a, b, got)
			}
		}
	}
}
