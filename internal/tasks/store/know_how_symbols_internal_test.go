package store

import (
	"testing"
)

// V1-3015: names that share one declaration carry that declaration's one
// digest, and names in other declarations carry their own. The test stays
// inside the Tasks boundary (decision 0397), so the extent text itself is
// pinned by contextindex's TestSymbolExtentsMatchTheJoinedReference.
func TestKnowHowSymbolTableDigestsSharedDeclarationsOnce(t *testing.T) {
	source := []byte("package p\n\nvar (\n\tA, B = 1,\n\t\t2\n\tC int\n)\n\nfunc F() {}\n")
	table := knowHowSymbolTable("p.go", source)
	if table.refusal != "" {
		t.Fatalf("refused: %q", table.refusal)
	}
	for _, name := range []string{"A", "B", "C", "F"} {
		if table.digests[name] == "" {
			t.Fatalf("%s has no digest: %+v", name, table.digests)
		}
	}
	if table.digests["A"] != table.digests["B"] || table.digests["A"] == table.digests["C"] || table.digests["C"] == table.digests["F"] {
		t.Fatalf("shared-declaration digests: %+v", table.digests)
	}
}
