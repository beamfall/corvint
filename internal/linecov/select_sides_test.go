// SPDX-License-Identifier: Apache-2.0
package linecov

import (
	"testing"
)

// GLS-V0-012, LPCV-V0-953: old and new bodies must both be proved. Missing
// head evidence, signatures, helper declarations and boundary changes abstain.
func TestAnalyzeRequiresBothSides(t *testing.T) {
	src := []byte("package p\nvar State int\nfunc unused() {\n\n}\n")
	ix := &Index{Files: []File{{Path: "p/p.go", OID: "old"}}}
	ix.reindex()
	read := func(string) ([]byte, error) { return src, nil }
	for _, tc := range []struct {
		name   string
		head   []byte
		hunks  []Hunk
		mapped bool
	}{
		{"missing", nil, nil, false},
		{"body", []byte("package p\nvar State int\nfunc unused() {\nState = 1\n}\n"), []Hunk{{4, 4}}, true},
		{"init insertion", []byte("package p\nvar State int\nfunc unused() {\n}\nfunc init() { State=1\n}\n"), []Hunk{{4, 5}}, false},
		{"signature", []byte("package p\nvar State int\nfunc unused(x int) {\n\n}\n"), []Hunk{{3, 3}}, false},
		{"invalid head", []byte("package"), []Hunk{{4, 4}}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := FileDiff{Path: "p/p.go", OldOID: "old", NewOID: "new", Hunks: []Hunk{{4, 4}}, NewHunks: tc.hunks, NewSource: tc.head}
			results, _ := ix.AnalyzePaths([]FileDiff{d}, read)
			if results[0].Mapped() != tc.mapped {
				t.Fatalf("mapped=%v want %v: %+v", results[0].Mapped(), tc.mapped, results[0])
			}
		})
	}
}
