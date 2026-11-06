// SPDX-License-Identifier: Apache-2.0
package v8

import (
	"reflect"
	"testing"
)

// LPCV-V0-8300: independent expected states include nesting, bounds and holes.
func TestPaintUnits(t *testing.T) {
	ranges := []Range{{2, 8, 0}, {4, 6, 2}, {-2, 10, 1}, {7, 7, 1}, {20, 21, 1}}
	before := append([]Range(nil), ranges...)
	want := []int8{1, 1, 0, 0, 1, 1, 0, 0, 1, 1, -1, -1}
	if got := PaintUnits(12, ranges); !reflect.DeepEqual(got, want) {
		t.Fatalf("states=%v want=%v", got, want)
	}
	if !reflect.DeepEqual(ranges, before) {
		t.Fatal("modified caller ranges")
	}
	if got := PaintUnits(3, []Range{{0, 2, 1}, {0, 2, 0}}); !reflect.DeepEqual(got, []int8{0, 0, -1}) {
		t.Fatalf("equal-span order: %v", got)
	}
	if got := PaintUnits(0, ranges); len(got) != 0 {
		t.Fatalf("zero source: %v", got)
	}
}
