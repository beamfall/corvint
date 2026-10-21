package cli

import (
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestERGV0023_EvidenceFlagsComposeSortedSet: --evidence flags in any order
// compose an evidence array the request decode accepts as a canonical-byte
// sorted set, with an exact repeat dropped; a malformed flag survives for
// composition to refuse.
func TestERGV0023_EvidenceFlagsComposeSortedSet(t *testing.T) {
	a, b := strings.Repeat("a", 64), strings.Repeat("b", 64)
	got := canonicalReviewList([]string{"zed=" + a + ":9", "report=" + b + ":10", "report=" + a + ":10", "zed=" + a + ":9", "bad"}, evidenceKey)
	if len(got) != 4 {
		t.Fatalf("got %v; want the repeat dropped", got)
	}
	var vs []wire.Value
	for _, e := range got {
		if e == "bad" {
			continue
		}
		vs = append(vs, wire.ObjectValue(wire.NewObject().Set("label", wire.String(e[:strings.Index(e, "=")])).Set("sha256", wire.String(e[strings.Index(e, "=")+1:strings.LastIndex(e, ":")])).Set("bytes", wire.String(e[strings.LastIndex(e, ":")+1:]))))
	}
	if err := wire.CheckSortedUnique("evidence", vs); err != nil {
		t.Fatalf("composed evidence %v is not a sorted set: %v", got, err)
	}
	if !reflect.DeepEqual(canonicalReviewList([]string{"TESTS:x", "TEST:x"}, reasonKey), []string{"TEST:x", "TESTS:x"}) {
		t.Fatal("reasons sort by element bytes, so TEST:x precedes TESTS:x")
	}
}
