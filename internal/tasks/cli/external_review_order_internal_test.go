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

// TestERGV0023_MalformedEvidenceNeverDroppedAsDuplicate: a malformed
// --evidence flag whose raw text is the canonical element bytes of a valid
// flag is kept in either input order, sorted first, so composition refuses
// it instead of the dedup silently discarding it (V1-1923 review).
func TestERGV0023_MalformedEvidenceNeverDroppedAsDuplicate(t *testing.T) {
	h := strings.Repeat("c", 64)
	valid := "report=" + h + ":1"
	malformed := `{"bytes":"1","label":"report","sha256":"` + h + `"}`
	if string(wire.Encode(wire.ObjectValue(wire.NewObject().Set("label", wire.String("report")).Set("sha256", wire.String(h)).Set("bytes", wire.String("1"))))) != malformed {
		t.Fatal("the fixture no longer spells the valid flag's element bytes")
	}
	want := []string{malformed, valid}
	for _, in := range [][]string{{valid, malformed}, {malformed, valid}, {valid, malformed, valid}} {
		if got := canonicalReviewList(in, evidenceKey); !reflect.DeepEqual(got, want) {
			t.Fatalf("%v composed %v; want %v", in, got, want)
		}
	}
	if got := canonicalReviewList([]string{"TEST", "TEST:x", "TEST"}, reasonKey); !reflect.DeepEqual(got, []string{"TEST", "TEST:x"}) {
		t.Fatalf("malformed reason: %v", got)
	}
}
