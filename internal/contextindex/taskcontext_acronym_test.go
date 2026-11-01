package contextindex

import (
	"context"
	"fmt"
	"slices"
	"testing"
)

// TCP-V0-961 (V1-3710): an unquoted all-capitals acronym that many sources
// name does not take the definition slot. On the CEM canonical-binding probe
// the canonical binding file ranks above the package that happens to declare
// `type CEM`, while a backticked acronym and a rare one still name their
// definition.
func TestTaskContextCommonAcronymDoesNotTakeTheDefinitionSlot(t *testing.T) {
	files := map[string]string{
		"go.mod":                     "module example.test/acronym\n\ngo 1.27.0\n",
		"revieworder/order.go":       "package revieworder\n\n// CEM is the review order's map type.\ntype CEM struct{}\n",
		"cem/wire/canonical.go":      "package wire\n\n// CanonicalBinding computes the canonical binding of a CEM.\nfunc CanonicalBinding() string { return \"canonical binding\" }\n",
		"rare/xyz.go":                "package rare\n\n// XYZW is rare.\ntype XYZW struct{}\n",
		"docs/canonical.md":          "# Notes\n\nNothing here.\n",
		"cem/wire/canonical_test.go": "package wire\n\nfunc TestCanonicalBinding() { _ = CanonicalBinding() }\n",
	}
	for position := range 24 {
		files[fmt.Sprintf("notes/n%02d.go", position)] = fmt.Sprintf("package notes\n\n// Note%02d mentions a CEM.\nfunc Note%02d() {}\n", position, position)
	}
	index, err := Build(context.Background(), impactRepositoryWithFiles(t, files))
	if err != nil {
		t.Fatal(err)
	}
	order := func(task string) []string {
		t.Helper()
		packet, err := TaskContext(context.Background(), index, task, "", 10)
		if err != nil {
			t.Fatal(err)
		}
		paths := []string{}
		for _, row := range mapsFromAny(packet["results"]) {
			paths = append(paths, row["id"].(string))
		}
		return paths
	}
	probe := order("CEM canonical binding")
	canonical, declarer := slices.Index(probe, "cem/wire/canonical.go"), slices.Index(probe, "revieworder/order.go")
	if canonical < 0 || declarer >= 0 && declarer < canonical {
		t.Fatalf("probe order %v: canonical.go must rank above revieworder/order.go", probe)
	}
	if rows := contextRowsByKind(t, mustTaskContext(t, index, "CEM canonical binding")); len(rows["definition"]) != 0 {
		t.Fatalf("a common unquoted acronym took the definition slot: %v", rows["definition"])
	}
	if rows := contextRowsByKind(t, mustTaskContext(t, index, "the `CEM` type")); !slices.Contains(rows["definition"], "revieworder/order.go") {
		t.Fatalf("a backticked acronym lost its definition: %v", rows)
	}
	if rows := contextRowsByKind(t, mustTaskContext(t, index, "where is XYZW declared")); !slices.Contains(rows["definition"], "rare/xyz.go") {
		t.Fatalf("a rare acronym lost its definition: %v", rows)
	}
}

func mustTaskContext(t *testing.T, index *Index, task string) map[string]any {
	t.Helper()
	packet, err := TaskContext(context.Background(), index, task, "", 10)
	if err != nil {
		t.Fatal(err)
	}
	return packet
}
