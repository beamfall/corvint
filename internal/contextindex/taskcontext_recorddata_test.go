package contextindex

import (
	"context"
	"fmt"
	"testing"
)

// recordDataFixture builds one strong code file, five weaker code files and
// record-data files whose BM25 strength lies between them: each body carries
// the two task words, and a longer body ranks lower. Ties inside a class fall
// to the path.
func recordDataFixture(t *testing.T, data int, extra map[string]string) *Index {
	t.Helper()
	files := map[string]string{
		"go.mod":     "module example.test/records\n\ngo 1.27.0\n",
		"code/01.go": "package code\n\n// needle signal\n",
	}
	for index := 2; index <= 6; index++ {
		files[fmt.Sprintf("code/%02d.go", index)] = "package code\n\n// needle signal alpha beta gamma delta epsilon zeta eta theta iota kappa lambda mu\n"
	}
	for index := range data {
		files[fmt.Sprintf("data/%c.json", 'a'+index)] = "{\"needle\": \"signal\", \"one\": \"two three four\"}\n"
	}
	for path, body := range extra {
		files[path] = body
	}
	index, err := Build(context.Background(), impactRepositoryWithFiles(t, files))
	if err != nil {
		t.Fatal(err)
	}
	return index
}
