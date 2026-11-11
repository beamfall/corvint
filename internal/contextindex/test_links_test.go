package contextindex

import (
	"context"
	"reflect"
	"slices"
	"testing"
)

func compareStrings(left, right string) int {
	switch {
	case left < right:
		return -1
	case left > right:
		return 1
	}
	return 0
}

// TestTestLinkTableRefusals_IDXSNAPV0809: a table built for another symbol
// count is not used, a truncated or out-of-range table is refused when
// decoded, and a written snapshot loads with the table equal to a fresh build.
func TestTestLinkTableRefusals_IDXSNAPV0809(t *testing.T) {
	index := taskContextFixture(t)
	table := buildTestLinkTable(index.Symbols)
	stale := *table
	stale.symbols++
	if stale.usable(index) || (*testLinkTable)(nil).usable(index) || !table.usable(index) {
		t.Fatal("usable is wrong")
	}
	data, _ := table.MarshalBinary()
	for _, broken := range [][]byte{data[:len(data)-1], append(append([]byte{}, data...), 0), data[:3]} {
		if err := (&testLinkTable{}).UnmarshalBinary(broken); err == nil {
			t.Fatalf("a malformed table of %d bytes decoded", len(broken))
		}
	}
	if len(table.definerIDs) == 0 {
		t.Fatal("fixture has no definers")
	}
	forged := *table
	forged.definerIDs = slices.Clone(table.definerIDs)
	forged.definerIDs[0] = uint32(len(table.paths))
	bad, _ := forged.MarshalBinary()
	if err := (&testLinkTable{}).UnmarshalBinary(bad); err == nil {
		t.Fatal("a table naming a path outside its list decoded")
	}
	if _, err := WriteSnapshot(index); err != nil {
		t.Fatal(err)
	}
	loaded, hit, err := LoadSnapshot(context.Background(), index.Root)
	if err != nil || !hit || loaded.testLinks == nil || !reflect.DeepEqual(loaded.testLinks, buildTestLinkTable(loaded.Symbols)) {
		t.Fatalf("the loaded snapshot does not carry the table: hit=%v err=%v", hit, err)
	}
}
