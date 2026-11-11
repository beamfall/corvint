package contextindex

import (
	"context"
	"fmt"
	"testing"
)

// documentationFixture builds code files and documentation files that carry
// the same two task words; the documentation bodies are shorter, so BM25
// ranks every documentation file above every code file, and ties inside a
// class fall to the path.
func documentationFixture(t *testing.T, code, documentation int) *Index {
	t.Helper()
	return documentationFixtureWith(t, code, documentation, nil)
}

// documentationFixtureWith adds files that carry no task word, such as a
// governing instruction file, to the documentation fixture.
func documentationFixtureWith(t *testing.T, code, documentation int, extra map[string]string) *Index {
	t.Helper()
	files := map[string]string{"go.mod": "module example.test/share\n\ngo 1.27.0\n"}
	for index := range code {
		files[fmt.Sprintf("code/%02d.go", index+1)] = "package code\n\n// needle signal\n"
	}
	for index := range documentation {
		files[fmt.Sprintf("docs/%c.md", 'a'+index)] = "needle signal\n"
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

func contextUncertainty(t *testing.T, packet map[string]any) []string {
	t.Helper()
	lines := make([]string, 0)
	for _, line := range anySlice(contextCoverage(t, packet)["uncertainty"]) {
		lines = append(lines, line.(string))
	}
	return lines
}
