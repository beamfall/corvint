package doccorpus

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

// longClaimCorpus pads the adoption fixture to 100,000 records whose claims
// carry long text, the shape that made per-claim term sets dominate index
// derivation allocation (V1-2001).
func longClaimCorpus(t *testing.T) *Artifact {
	t.Helper()
	root, m := adoptionFixture(t, nil)
	a, err := Build(context.Background(), root, m)
	if err != nil {
		t.Fatal(err)
	}
	subjects := 20000 - len(a.Subjects)
	first := len(a.Subjects)
	padIndexCapacity(a, subjects, 0, 10000-len(a.Relations))
	word := func(i int) string { return indexCapacityWords[i%len(indexCapacityWords)] }
	for i := len(a.Claims); len(a.Subjects)+len(a.Claims)+len(a.Relations) < 100000; i++ {
		s := a.Subjects[first+(i*7)%subjects]
		text := make([]string, 0, 200)
		for j := 0; j < 200; j++ {
			text = append(text, word(i*13+j*5))
		}
		a.Claims = append(a.Claims, Claim{ID: fmt.Sprintf("pad:long:%06d", i), Subject: s.ID, Text: strings.Join(text, " ") + fmt.Sprintf(" Item%dValue", i), Provider: "pad", Evidence: s.Evidence})
	}
	return a
}

// Deriving the index for 100,000 records with long claims allocates well
// under 1 GiB (V1-2001).
func TestQueryIndexLongClaimAllocation(t *testing.T) {
	if testing.Short() {
		t.Skip("allocation measurement")
	}
	a := longClaimCorpus(t)
	result := testing.Benchmark(func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			if _, err := BuildQueryIndex(context.Background(), a); err != nil {
				b.Fatal(err)
			}
		}
	})
	t.Logf("records=%d claims=%d: %d B/op, %d allocs/op", len(a.Subjects)+len(a.Claims)+len(a.Relations), len(a.Claims), result.AllocedBytesPerOp(), result.AllocsPerOp())
	if result.AllocedBytesPerOp() >= 1<<30 {
		t.Fatalf("index derivation allocated %d bytes, want under 1 GiB", result.AllocedBytesPerOp())
	}
}
