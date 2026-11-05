// SPDX-License-Identifier: Apache-2.0
package v8

import "sort"

// PaintUnits returns -1 for uncovered UTF-16 units, 0 for unexecuted units and
// 1 for executed units. Narrower ranges override wider ranges; equal spans
// retain input order. Ranges are clipped to the source length. The caller's
// ranges remain unchanged. Node and browser coverage share this painter.
func PaintUnits(size int, ranges []Range) []int8 {
	units := make([]int8, size)
	for i := range units {
		units[i] = -1
	}
	all := append([]Range(nil), ranges...)
	sort.SliceStable(all, func(i, j int) bool { return all[i].End-all[i].Start > all[j].End-all[j].Start })
	for _, r := range all {
		state := int8(0)
		if r.Count > 0 {
			state = 1
		}
		for off := max(r.Start, 0); off < min(r.End, size); off++ {
			units[off] = state
		}
	}
	return units
}
