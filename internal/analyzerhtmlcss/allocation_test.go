//go:build !race

package analyzerhtmlcss

import "testing"

// Race instrumentation deliberately changes allocation accounting; this is a normal-build ratchet.
func TestAnalyzeCandidateAllocationRatchet(t *testing.T) {
	result := testing.Benchmark(func(b *testing.B) {
		request := benchmarkRequest()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_ = Analyze(request)
		}
	})
	if result.AllocedBytesPerOp() > 105000 || result.AllocsPerOp() > 635 {
		t.Fatalf("allocation ratchet failed: %d B/op, %d allocs/op", result.AllocedBytesPerOp(), result.AllocsPerOp())
	}
}

func TestSortFactsWorstCaseAllocationRatchet(t *testing.T) {
	seed := descendingFacts(maxFacts)
	work := make([]fact, len(seed))
	if allocations := testing.AllocsPerRun(10, func() {
		copy(work, seed)
		sortFacts(work)
	}); allocations != 0 {
		t.Fatalf("typed O(n log n) sort allocation ratchet failed: %.2f allocs/op", allocations)
	}
}
