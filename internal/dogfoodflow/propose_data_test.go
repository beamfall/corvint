package dogfoodflow

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// DCW-V0-381 (V1-2423): a draft cites no data path and no longer path that
// merely ends in the hunk's path, and prefers a file the change leaves
// untouched. The hunk in table.txt is named only by a benchmark corpus and a
// test fixture, so its row never cites either; the README.md hunk is named
// by docs/specs/README.md (another file), by docs/aaa.md (which the change
// also edits) and by docs/guide.md, which the draft cites.
func TestProposeCitationsDCWV0381SkipsDataAndPartialPaths(t *testing.T) {
	root := t.TempDir()
	testGit(t, root, "init", "-q")
	readme := "# Readme\n\n" + numbered("readme", 12)
	table := numbered("row", 12)
	other := "# Other\n\nSee README.md for setup.\n" + numbered("other", 12)
	writeTree(t, root, map[string]string{
		"README.md":                     readme,
		"internal/x/table.txt":          table,
		"benchmarks/x/corpus.json":      "{\n  \"path\": \"internal/x/table.txt\",\n  \"other\": \"README.md\"\n}\n",
		"internal/x/testdata/fixture":   "internal/x/table.txt\n",
		"docs/specs/README.md":          "# Specs\n\nThis is docs/specs/README.md.\n",
		"docs/aaa.md":                   other,
		"docs/guide.md":                 "# Guide\n\nStart with README.md first.\n",
		".taskman/tickets/V1-0001.json": "{\"title\": \"README.md and internal/x/table.txt\"}\n",
	})
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-q", "-m", "base")
	base := testGit(t, root, "rev-parse", "HEAD")
	writeTree(t, root, map[string]string{
		"README.md":            strings.Replace(readme, "readme line 6\n", "readme line 6 changed\n", 1),
		"internal/x/table.txt": strings.Replace(table, "row line 6\n", "row line 6 changed\n", 1),
		"docs/aaa.md":          strings.Replace(other, "other line 10\n", "other line 10 changed\n", 1),
	})
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-q", "-m", "change")
	hunks := diffHunks(t, root, base)
	document, err := json.Marshal(map[string]any{"baseRevision": base, "hunks": hunks})
	if err != nil {
		t.Fatal(err)
	}
	writeTree(t, root, map[string]string{".corvint/change.cem.json": string(document) + "\n"})
	var stdout, stderr bytes.Buffer
	run := &proposal{
		flow:    flow{ctx: context.Background(), prefix: "dogfood-propose-citations", root: root, stderr: &stderr},
		options: ProposeOptions{Root: root, Base: base}, stdout: &stdout, blobs: map[string][]byte{}, lines: map[string][][]byte{},
	}
	if code, err := boundary(context.Background(), run.run, func() {}); err != nil || code != 0 {
		t.Fatalf("ProposeCitations = %d, %v\n%s", code, err, stderr.String())
	}
	rows := textLines(stdout.Bytes())
	if len(hunks) != 3 || len(rows) != 3 {
		t.Fatalf("draft has %d rows for %d hunks:\n%s", len(rows), len(hunks), stdout.String())
	}
	for index, row := range rows {
		_, rest, _ := strings.Cut(row, "\t")
		for _, prefix := range []string{"benchmarks/", "internal/x/testdata/", ".taskman/", "docs/specs/README.md\t"} {
			if strings.HasPrefix(rest, prefix) {
				t.Errorf("row cites an excluded path: %q", row)
			}
		}
		if hunks[index].Path == "README.md" && rest != "docs/guide.md\t3:3\tspecification" {
			t.Errorf("README.md row = %q; want the untouched guide", row)
		}
	}
}
