// SPDX-License-Identifier: Apache-2.0
package affected

import (
	"context"
	"reflect"
	"strings"
	"testing"
)

// AFP-V0-021/549: preparing immutable tokens must preserve the complete plan,
// including unknowns and witnesses, across concurrent selections.
func TestPreparedPathTokensPreserveSelection(t *testing.T) {
	graph, err := Build(context.Background(), t.TempDir(), fake{name: "go", units: []Unit{
		{ID: "go:reader", Sources: []string{"reader/reader.go"}, Tests: []string{"reader/reader_test.go"}, PathTokens: []string{"../.corvint/change.cem.json", "docs/guide.md", "fixtures/", "guide.md"}},
		{ID: "go:other", Sources: []string{"other/other.go"}, Tests: []string{"other/other_test.go"}, PathTokens: []string{"docs/guide.md", "other.md"}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	for _, dirty := range [][]string{{"docs/guide.md"}, {"reader/fixtures/a.md"}, {".corvint/change.cem.json"}, {"other/other.go", "docs/guide.md"}, {"unowned.md"}, nil} {
		// Exercise the original per-selection preparation on the same graph.
		uncached := *graph
		uncached.pathTokenCache = nil
		want := Select(&uncached, dirty)
		t.Run(strings.Join(dirty, ","), func(t *testing.T) {
			t.Parallel()
			if got := Select(graph, dirty); !reflect.DeepEqual(got, want) {
				t.Fatalf("prepared-token plan differs for %v:\ngot %+v\nwant %+v", dirty, got, want)
			}
		})
	}
}
