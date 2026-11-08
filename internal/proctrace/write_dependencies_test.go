// SPDX-License-Identifier: Apache-2.0
package proctrace

import (
	"path/filepath"
	"testing"
)

// RSP-V0-007/021: mutation success is a dependency even without a later read.
// Only the closed creation-proof forms may suppress that dependency.
func TestRSPV0007WriteDependencyDefault(t *testing.T) {
	for _, tc := range []struct {
		kind    string
		proof   bool
		created bool
	}{
		{"overwrite", false, false}, {"append", false, false},
		{"truncate", false, false}, {"replace", false, false},
		{"create", false, false}, {"create", true, true},
		{"replace", true, true}, {"future-write-form", true, false},
	} {
		t.Run(tc.kind+"/"+map[bool]string{true: "proof", false: "no-proof"}[tc.proof], func(t *testing.T) {
			root := t.TempDir()
			name := filepath.Join(root, "fixture")
			r := emptyResult("test")
			c := collector{q: Request{Repositories: []Repository{{ID: "repo", Root: root}}, PathCap: 10}, r: &r, paths: map[Path]bool{}, created: map[string]bool{}}
			c.write(name, name, tc.kind, tc.proof)
			if len(r.Writes.Entries) != 1 || c.created[name] != tc.created {
				t.Fatalf("incorrect write provenance: %+v created=%v", r, c.created)
			}
			if got := len(r.Reads.Entries); (got == 0) != tc.created {
				t.Fatalf("dependency suppressed without understood creation proof: %+v", r)
			}
			if tc.created {
				c.write(name, name, "truncate", false)
				if len(r.Reads.Entries) != 0 {
					t.Fatalf("tree-owned path acquired prior dependency: %+v", r)
				}
			}
		})
	}
}
