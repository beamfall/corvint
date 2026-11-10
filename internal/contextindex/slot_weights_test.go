package contextindex

import (
	"reflect"
	"strings"
	"testing"
)

func TestLTAV0011SlotWeightOrderIsIdentityByDefaultAndStable(t *testing.T) {
	rows := []contextRow{{kind: "definition", path: "a"}, {kind: "test", path: "b"}, {kind: "lexical", path: "c"}, {kind: "test", path: "d"}}
	if got := orderBySlotWeight(append([]contextRow{}, rows...), nil); !reflect.DeepEqual(got, rows) {
		t.Fatalf("default order changed: %v", got)
	}
	got := orderBySlotWeight(append([]contextRow{}, rows...), SlotWeights{"test": 1, "lexical": -1})
	paths := []string{}
	for _, row := range got {
		paths = append(paths, row.path)
	}
	if strings.Join(paths, ",") != "b,d,a,c" {
		t.Fatalf("weighted order = %v", paths)
	}
}
