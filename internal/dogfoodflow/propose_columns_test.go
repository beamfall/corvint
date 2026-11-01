package dogfoodflow

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// TestDCWV0730ProposeCitationsNamesThePlanColumns (V1-3261): the draft names
// the plan's columns, and that the cited path is not the edited path, on
// stderr, while stdout keeps only four-field plan rows.
func TestDCWV0730ProposeCitationsNamesThePlanColumns(t *testing.T) {
	root := t.TempDir()
	testGit(t, root, "init", "-q")
	code := "package x\n\n" + numbered("// c", 20)
	writeTree(t, root, map[string]string{"pkg/code.go": code})
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-q", "-m", "base")
	base := testGit(t, root, "rev-parse", "HEAD")
	writeTree(t, root, map[string]string{"pkg/code.go": strings.Replace(code, "// c line 10\n", "// c line 10 changed\n", 1)})
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-q", "-m", "change")
	document, err := json.Marshal(map[string]any{"baseRevision": base, "hunks": diffHunks(t, root, base)})
	if err != nil {
		t.Fatal(err)
	}
	writeTree(t, root, map[string]string{".corvint/change.cem.json": string(document) + "\n"})
	var stdout, stderr bytes.Buffer
	if status, err := ProposeCitations(context.Background(), ProposeOptions{Root: root, Base: base}, &stdout, &stderr); err != nil || status != 0 {
		t.Fatalf("ProposeCitations = %d, %v\n%s", status, err, stderr.String())
	}
	if !strings.Contains(stderr.String(), "  "+planColumnsNote+"\n") {
		t.Errorf("stderr lacks the column note:\n%s", stderr.String())
	}
	for _, row := range textLines(stdout.Bytes()) {
		if strings.HasPrefix(row, "#") || len(strings.Split(row, "\t")) != 4 {
			t.Errorf("stdout row is not a four-field plan row: %q", row)
		}
	}
}
