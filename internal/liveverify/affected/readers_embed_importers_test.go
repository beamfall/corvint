package affected

import (
	"slices"
	"testing"
)

// GLS-V0-013: a package that embeds its own source exposes the changed bytes
// through its API, so an importer's test can observe an edit to a function no
// test executes. Readers carries the embedding package's read reach to its
// transitive importers and their test-only users; an unrelated package stays
// out.
func TestReadersCarriesEmbeddingPackageToImporters(t *testing.T) {
	t.Parallel()
	graph := mustBuild(t, fake{name: "go", units: []Unit{
		{ID: "go:a", Sources: []string{"a/a.go"}, Embeds: true, EmbedFiles: []string{"a/a.go"}},
		{ID: "go:b", Sources: []string{"b/b.go"}, Tests: []string{"b/b_test.go"}, Imports: []string{"go:a"}},
		{ID: "go:c", Sources: []string{"c/c.go"}, Imports: []string{"go:b"}},
		{ID: "go:t", Tests: []string{"t/t_test.go"}, TestImports: []string{"go:c"}},
		{ID: "go:z", Sources: []string{"z/z.go"}, Tests: []string{"z/z_test.go"}},
	}})
	readers := graph.Readers([]string{"a/a.go"})
	var got []string
	for id := range readers {
		got = append(got, id)
	}
	slices.Sort(got)
	if want := []string{"go:a", "go:b", "go:c", "go:t"}; !slices.Equal(got, want) {
		t.Fatalf("readers of an embedded source = %v, want %v", got, want)
	}
	// Without an embedding package the read reach does not widen to importers.
	plain := mustBuild(t, fake{name: "go", units: []Unit{
		{ID: "go:a", Sources: []string{"a/a.go"}},
		{ID: "go:b", Sources: []string{"b/b.go"}, Tests: []string{"b/b_test.go"}, Imports: []string{"go:a"}},
	}})
	if readers := plain.Readers([]string{"a/a.go"}); len(readers) != 0 {
		t.Fatalf("readers without an embed = %v, want none", readers)
	}
}
