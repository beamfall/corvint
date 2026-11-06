// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"fmt"
	"slices"
	"strings"
	"testing"
)

// TestTCDV01101_CollapseScope: each round collapses only the deepest entries
// to their parent directory key and drops entries a key covers, stopping at
// the first set within the limit; top-level overflow cannot collapse.
func TestTCDV01101_CollapseScope(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		paths []string
		limit int
		want  string
		ok    bool
	}{
		{"within limit", []string{"b.go", "a/x.go"}, 2, "a/x.go b.go", true},
		{"deepest only", []string{"a/b/c/1", "a/b/c/2", "a/b/3", "z"}, 3, "a/b/3 a/b/c/ z", true},
		{"covered dropped", []string{"a/b/1", "a/b/2", "a/c", "a/d"}, 2, "a/", true},
		{"two rounds", []string{"a/b/c/1", "a/b/c/2", "a/b/d/3", "a/e"}, 2, "a/b/ a/e", true},
		{"top-level overflow", []string{"a", "b", "c/d"}, 2, "a b c/", false},
	}
	for _, c := range cases {
		got, ok := collapseScope(c.paths, c.limit)
		if strings.Join(got, " ") != c.want || ok != c.ok {
			t.Errorf("%s: got %v %v, want %q %v", c.name, got, ok, c.want, c.ok)
		}
	}
	var big []string
	for d := 0; d < 9; d++ {
		for f := 0; f < 79; f++ {
			big = append(big, fmt.Sprintf("src/a/d%02d/f%02d.txt", d, f))
		}
	}
	big = append(big, "docs/b.md")
	got, ok := collapseScope(big, 256)
	if !ok || len(got) != 10 || !slices.IsSorted(got) || got[0] != "docs/b.md" || got[9] != "src/a/d08/" {
		t.Fatalf("712 paths: %v %v", got, ok)
	}
}
