package dogfoodflow

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// DCW-V0-389 (V1-3118, gh2011): recorded command output is data. The hunk in
// tool.go is named only by an inventory listing under a raw/ directory and by
// a .stdout and a .stderr capture outside one, so the draft falls back to a
// line of its own file instead of citing a listing of every path in the tree.
func TestProposeCitationsDCWV0389SkipsRecordedCommandOutput(t *testing.T) {
	root := t.TempDir()
	testGit(t, root, "init", "-q")
	tool := numbered("tool", 12)
	writeTree(t, root, map[string]string{
		"internal/x/tool.go": tool,
		"conformance/p/results/r/raw/init-inventory.cand.out": "internal/x/tool.go\n",
		"conformance/p/results/r/summary/run.stdout":          "ok internal/x/tool.go\n",
		"logs/run.stderr": "warn internal/x/tool.go\n",
	})
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-q", "-m", "base")
	base := testGit(t, root, "rev-parse", "HEAD")
	writeTree(t, root, map[string]string{"internal/x/tool.go": strings.Replace(tool, "tool line 6\n", "tool line 6 changed\n", 1)})
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
	if len(hunks) != 1 || len(rows) != 1 {
		t.Fatalf("draft has %d rows for %d hunks:\n%s", len(rows), len(hunks), stdout.String())
	}
	if _, rest, _ := strings.Cut(rows[0], "\t"); strings.Contains(rest, "/raw/") || strings.Contains(rest, ".stdout\t") || strings.Contains(rest, ".stderr\t") {
		t.Fatalf("row cites recorded output: %q", rows[0])
	}
}
