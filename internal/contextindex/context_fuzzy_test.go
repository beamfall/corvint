package contextindex

import (
	"context"
	"fmt"
	"testing"
)

func fuzzyFixtureIndex(t *testing.T) *Index {
	t.Helper()
	files := map[string]string{
		"go.mod":                          "module example.test/fuzzy\n\ngo 1.27.0\n",
		"AGENTS.md":                       "# Standing instructions\n\nRead before editing.\n",
		"internal/unplannedread/store.go": "package unplannedread\n\n// store appends unplanned read ledger entries\nfunc store() {}\n",
		"tools/scopes/scopes.go":          "package scopes\n\n// scopes are loaded here\nfunc load() {}\n",
		".corvint/scopes.json":            "{\"scopes\": [\"one\"]}\n",
		"ci/narrow.sh":                    "#!/bin/sh\n# select packages\ngit merge-base origin/main HEAD\n",
		"ci/other.sh":                     "#!/bin/sh\n# merge and base are separate words here\n",
	}
	for index := range 9 {
		files[fmt.Sprintf("pkg/p%d/p.go", index)] = fmt.Sprintf("package p%d\n\n// unplanned ledger work\nfunc f() {}\n", index)
	}
	root := impactRepositoryWithFiles(t, files)
	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	return index
}
