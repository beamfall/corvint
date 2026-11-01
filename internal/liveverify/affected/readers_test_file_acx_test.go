package affected

import (
	"context"
	"slices"
	"testing"
)

// ACX-V0-852 (V1-3578): a _test.go-only edit selects its own package and the
// Go units whose tokens name the file, not a Go unit whose literal names only
// an ancestor directory; a non-Go unit and a non-test edit keep their rules.
func TestTestFileEditSelectsOnlyFileNamingReaders_ACX_V0_852(t *testing.T) {
	golang := fake{name: "go", units: []Unit{
		{ID: "go:a/store", Sources: []string{"a/store/store.go"}, Tests: []string{"a/store/claim_test.go"}},
		{ID: "go:a/cli", Sources: []string{"a/cli/cli.go"}, Tests: []string{"a/cli/cli_test.go"}, PathTokens: []string{"/store"}},
		{ID: "go:a/walk", Sources: []string{"a/walk/walk.go"}, Tests: []string{"a/walk/walk_test.go"}, PathTokens: []string{"../../a/"}},
		{ID: "go:a/named", Sources: []string{"a/named/named.go"}, Tests: []string{"a/named/named_test.go"}, PathTokens: []string{"a/store/claim_test.go"}},
		{ID: "go:a/fragment", Sources: []string{"a/fragment/fragment.go"}, Tests: []string{"a/fragment/fragment_test.go"}, PathTokens: []string{"store/claim_"}},
	}}
	shell := fake{name: "s", units: []Unit{
		{ID: "s:runner", Sources: []string{"runner.s"}, Tests: []string{"runner_test.s"}, PathTokens: []string{"./a/store"}},
	}}
	graph, err := Build(context.Background(), t.TempDir(), golang, shell)
	if err != nil {
		t.Fatal(err)
	}
	selected := func(dirty string) []string {
		ids := []string{}
		for _, selection := range Select(graph, []string{dirty}).Selected {
			ids = append(ids, selection.UnitID)
		}
		slices.Sort(ids)
		return ids
	}
	if got, want := selected("a/store/claim_test.go"), []string{"go:a/fragment", "go:a/named", "go:a/store", "s:runner"}; !slices.Equal(got, want) {
		t.Fatalf("test-only edit selected %v, want %v", got, want)
	}
	if got := selected("a/store/store.go"); !slices.Contains(got, "go:a/cli") || !slices.Contains(got, "go:a/walk") {
		t.Fatalf("source edit selected %v, want the directory readers kept", got)
	}
}
