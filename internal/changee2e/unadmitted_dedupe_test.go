package changee2e

import "testing"

// CCE-V0-138: the catalog lists each (kind, id, reason) unadmitted row once, in sorted order.
func TestCCEV0138UnadmittedRowsAreUnique(t *testing.T) {
	root, _, head := vocabularyRepo(t, nil)
	cat, err := BuildCatalog(bg, CatalogOptions{Root: root, Rev: head})
	if err != nil {
		t.Fatal(err)
	}
	if len(cat.Unadmitted) == 0 {
		t.Fatal("the vocabulary fixture lists no unadmitted rows")
	}
	seen := map[Unadmitted]bool{}
	for i, u := range cat.Unadmitted {
		if seen[u] {
			t.Fatalf("row %d repeats %+v", i, u)
		}
		seen[u] = true
	}
	edges := 0
	for _, u := range cat.Unadmitted {
		if u.Kind == KindPath && u.Reason == "edge-unproven" {
			edges++
		}
	}
	if edges == 0 {
		t.Fatal("the fixture lists no edge-unproven row, so the regression proves nothing")
	}
}
