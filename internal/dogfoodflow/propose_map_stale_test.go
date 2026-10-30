// SPDX-License-Identifier: Apache-2.0

package dogfoodflow

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/patch"
)

// TestProposeCitationsDCWV0441ContentAmendMakesMapStale pins the review
// finding on V1-3240: an amend that changes a hunk's content but keeps its
// path and BASE range re-hunks its ID, so a map prepared before the amend is
// stale and the draft comes from the current patch rather than carrying the
// map's obsolete ID. The same map is not stale before the amend.
func TestProposeCitationsDCWV0441ContentAmendMakesMapStale(t *testing.T) {
	root := t.TempDir()
	testGit(t, root, "init", "-q")
	file := "# Only\n\n" + numbered("line", 12)
	writeTree(t, root, map[string]string{"a.md": file})
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-q", "-m", "base")
	base := testGit(t, root, "rev-parse", "HEAD")
	writeTree(t, root, map[string]string{"a.md": strings.Replace(file, "line line 6\n", "line line 6 first\n", 1)})
	testGit(t, root, "commit", "-q", "-am", "change")

	propose := func() (string, string) {
		t.Helper()
		var stdout, stderr bytes.Buffer
		if code, err := ProposeCitations(context.Background(), ProposeOptions{Root: root, Base: base}, &stdout, &stderr); code != 0 || err != nil {
			t.Fatalf("ProposeCitations = %d, %v\n%s", code, err, stderr.String())
		}
		return stdout.String(), stderr.String()
	}
	draft, _ := propose()
	id, _, _ := strings.Cut(draft, "\t")
	parsed, err := patch.Parse([]byte(testGit(t, root, "diff", "--unified=3", base, "HEAD") + "\n"))
	if !strings.HasPrefix(id, "hunk:") || err != nil || len(parsed.Hunks) != 1 {
		t.Fatalf("draft %q, patch %v with %d hunks", draft, err, len(parsed.Hunks))
	}
	old := parsed.Hunks[0].OldRange
	document, err := json.Marshal(map[string]any{"baseRevision": base, "hunks": []map[string]any{{
		"id": id, "path": "a.md", "disposition": "unknown", "reason": "no-evidence",
		"oldRange": map[string]int64{"start": old.Start, "count": old.Count},
	}}})
	if err != nil {
		t.Fatal(err)
	}
	writeTree(t, root, map[string]string{".corvint/change.cem.json": string(document) + "\n"})
	if _, stderr := propose(); strings.Contains(stderr, "NOTE map-stale") {
		t.Fatalf("a current map was judged stale:\n%s", stderr)
	}

	writeTree(t, root, map[string]string{"a.md": strings.Replace(file, "line line 6\n", "line line 6 amended\n", 1)})
	testGit(t, root, "commit", "-q", "-a", "--amend", "--no-edit")
	stdout, stderr := propose()
	if !strings.Contains(stderr, "NOTE map-stale") || strings.Contains(stdout, id) {
		t.Fatalf("a content-only amend did not mark the map stale:\nstdout=%s\nstderr=%s", stdout, stderr)
	}
}
