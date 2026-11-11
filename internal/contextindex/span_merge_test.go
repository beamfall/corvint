package contextindex

import (
	"context"
	"strings"
	"testing"
)

// spanModeFixture holds two adjacent named functions, one that fits under
// the per-span cap but not a small budget, and one past the cap that names
// zqmarker past its first contextSpanMaxLines lines (line 140).
func spanModeFixture(t *testing.T) *Index {
	t.Helper()
	work := "package work\n\nfunc AlphaStep() int {\n\treturn 1\n}\nfunc BetaStep() int {\n\treturn 2\n}\n\n" +
		"func MediumWork() int {\n" + strings.Repeat("\t_ = 0\n", 29) + "\treturn 0\n}\n\n" +
		"func HugeWork() int {\n" + strings.Repeat("\t_ = 0\n", 96) + "\t_ = zqmarker\n" + strings.Repeat("\t_ = 0\n", 2) + "\treturn 0\n}\n"
	root := impactRepositoryWithFiles(t, map[string]string{
		"go.mod":           "module example.test/work\n\ngo 1.27.0\n",
		"pkg/work/work.go": work,
		"docs/notes.md":    "# notes\n",
	})
	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	return index
}
