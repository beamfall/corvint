package dogfoodflow

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/patch"
	"github.com/Beamfall/corvint/internal/cem/verify"
)

// DCW-V0-924 (V1-3792): a new file that names the requirement ID it
// implements reaches the anchored tier through the BASE line defining that ID,
// the documented alternative to a span naming its path, which no BASE line can
// hold; dogfood-check therefore warns it (an anchor is available) rather than
// counting it unanchorable, while an uncited new file stays in the
// unanchorable NOTE, whose fix names that alternative.
func TestDCWV0924NewFileAnchorsThroughTheRequirementItNames(t *testing.T) {
	root := t.TempDir()
	testGit(t, root, "init", "-q")
	definition := "- `XYZ-V0-045`: the snapshot MUST be byte-stable.\n"
	spec := "# X v0\n\n## Requirements\n\n" + definition + numbered("requirement", 12) +
		"\n## Traceability\n\n| `XYZ-V0-045` | `internal/y/` snapshot | implemented |\n" + numbered("trace", 6)
	writeTree(t, root, map[string]string{"docs/specs/x-v0.md": spec})
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-q", "-m", "base")
	base := testGit(t, root, "rev-parse", "HEAD")
	writeTree(t, root, map[string]string{
		"internal/y/snapshot.go": "package y\n\n// Snapshot implements `XYZ-V0-045`.\nfunc Snapshot() int { return 1 }\n",
		"internal/y/uncited.go":  "package y\n\n// Uncited is a function.\nfunc Uncited() int { return 2 }\n",
	})
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-q", "-m", "change")
	target := testGit(t, root, "rev-parse", "HEAD")
	raw := testGit(t, root, "diff", "--full-index", "--no-renames", "--no-indent-heuristic", "--diff-algorithm=myers",
		"--unified=3", "--inter-hunk-context=0", "--src-prefix=a/", "--dst-prefix=b/", base, "HEAD") + "\n"
	parsed, err := patch.Parse([]byte(raw))
	if err != nil || len(parsed.Hunks) != 2 {
		t.Fatalf("canonical patch: %v (%d hunks)", err, len(parsed.Hunks))
	}
	snapshot, uncited := parsed.Hunks[0], parsed.Hunks[1]
	if snapshot.DisplayPath != "internal/y/snapshot.go" || uncited.DisplayPath != "internal/y/uncited.go" {
		t.Fatalf("hunk order: %s, %s", snapshot.DisplayPath, uncited.DisplayPath)
	}
	if kinds, ids := verify.SpanAnchors(snapshot, []byte(definition)); !slices.Equal(kinds, []string{verify.AnchorID}) || !slices.Contains(ids, "XYZ-V0-045") {
		t.Fatalf("new file naming the ID: anchors = %v, ids = %v; want id via XYZ-V0-045", kinds, ids)
	}
	if kinds, _ := verify.SpanAnchors(uncited, []byte(definition)); len(kinds) != 0 {
		t.Fatalf("uncited new file: anchors = %v, want none", kinds)
	}
	type mapHunk struct {
		ID       string         `json:"id"`
		Path     string         `json:"path"`
		OldRange map[string]int `json:"oldRange"`
	}
	mapped, paths, tiers := []mapHunk{}, []string{}, []string{}
	for _, hunk := range parsed.Hunks {
		mapped = append(mapped, mapHunk{hunk.ID, hunk.DisplayPath, map[string]int{"start": int(hunk.OldRange.Start), "count": int(hunk.OldRange.Count)}})
		paths, tiers = append(paths, hunk.DisplayPath), append(tiers, "cited")
	}
	document, err := json.Marshal(map[string]any{"baseRevision": base, "hunks": mapped})
	if err != nil {
		t.Fatal(err)
	}
	writeTree(t, root, map[string]string{".corvint/change.cem.json": string(document) + "\n"})
	line := tierStatusLine(paths, tiers)
	for index, hunk := range parsed.Hunks {
		line = bytes.ReplaceAll(line, []byte(fmt.Sprintf("hunk:sha256:%064x", index+1)), []byte(hunk.ID))
	}
	var stderr bytes.Buffer
	run := &check{flow: flow{ctx: context.Background(), prefix: "dogfood-check", root: root, stderr: &stderr, base: base, target: target}}
	got := run.tierWarnings(line)
	joined := strings.Join(got, "\n")
	if len(got) != 2 || !strings.Contains(got[0], `path="internal/y/snapshot.go"`) || strings.Contains(got[0], "uncited.go") ||
		got[1] != "dogfood-check: NOTE tier-below-anchored-unanchorable hunks=1 "+tierBelowAnchoredUnavailable ||
		!strings.Contains(joined, "name its ID in the file") || stderr.Len() != 0 {
		t.Fatalf("tier lines = %q (stderr %q); want a warning for snapshot.go and one NOTE, naming the ID route, for uncited.go", got, stderr.String())
	}
}
