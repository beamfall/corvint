package dogfoodflow

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

// TestProposeCitationsDCWV0548RepinOnlyHunkCitesTheRepinRule reproduces
// V1-3545: a hunk that only moved line-citation pins, as `--repin` rewrites
// them, was drafted to a same-file anchor. It now cites the DCG-V0-045
// definition line; a hunk that also changes prose keeps its own-file draft.
// Fails on BASE, which drafted guide.md for both hunks.
func TestProposeCitationsDCWV0548RepinOnlyHunkCitesTheRepinRule(t *testing.T) {
	root := t.TempDir()
	testGit(t, root, "init", "-q")
	guide := "# Guide\n\n" + numbered("intro", 6) +
		"The reader is at `internal/a.go:10-12@0123abcd`.\n" + numbered("middle", 8) +
		"The writer is at `internal/b.go:4@89abcdef` and is plain.\n" + numbered("tail", 6)
	writeTree(t, root, map[string]string{
		"docs/guide.md": guide,
		"docs/specs/documentation-citation-gate-v0.md": "# Documentation citation gate\n\n## Requirements\n\n" + numbered("pre", 3) +
			"- `DCG-V0-045`: `--repin [--base <rev>]` MUST also relocate a pinned citation.\n" + numbered("post", 3),
	})
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-q", "-m", "base")
	base := testGit(t, root, "rev-parse", "HEAD")
	changed := strings.Replace(guide, "`internal/a.go:10-12@0123abcd`", "`internal/a.go:14-16@4567cdef`", 1)
	changed = strings.Replace(changed, "`internal/b.go:4@89abcdef` and is plain", "`internal/b.go:6@fedcba98` and is rewritten", 1)
	writeTree(t, root, map[string]string{"docs/guide.md": changed})
	testGit(t, root, "commit", "-q", "-am", "repin")

	var stdout, stderr bytes.Buffer
	if code, err := ProposeCitations(context.Background(), ProposeOptions{Root: root, Base: base}, &stdout, &stderr); code != 0 || err != nil {
		t.Fatalf("ProposeCitations = %d, %v\n%s", code, err, stderr.String())
	}
	rows := textLines(stdout.Bytes())
	if len(rows) != 2 {
		t.Fatalf("draft has %d rows; want 2:\n%s\n%s", len(rows), stdout.String(), stderr.String())
	}
	repin, prose := strings.Split(rows[0], "\t"), strings.Split(rows[1], "\t")
	if repin[1] != "docs/specs/documentation-citation-gate-v0.md" || repin[2] != "8:8" || repin[3] != "specification" {
		t.Errorf("repin-only hunk row = %q; want the DCG-V0-045 definition 8:8\n%s", repin, stderr.String())
	}
	if !strings.Contains(stderr.String(), "changes only line-citation pins") || !strings.Contains(stderr.String(), "(DCW-V0-548)") {
		t.Errorf("stderr does not explain the repin draft:\n%s", stderr.String())
	}
	if prose[1] == "docs/specs/documentation-citation-gate-v0.md" {
		t.Errorf("a hunk that also edits prose was drafted to the repin rule: %q", prose)
	}
}
