package dogfoodflow

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// TestProposeCitationsDCWV0382DedupesSpansMapsHunksAndRedraftsStaleMap pins
// V1-3118: no two drafted rows cite the same BASE span, even when one line is
// the only anchor naming every hunk's file; stderr names the file and BASE
// range each row's hunk edits; and a map a later commit made stale is not
// drafted from, because its ordinals and ranges no longer match the patch the
// bind prepares, so the draft is keyed by the current patch's hunk IDs.
func TestProposeCitationsDCWV0382DedupesSpansMapsHunksAndRedraftsStaleMap(t *testing.T) {
	root := t.TempDir()
	testGit(t, root, "init", "-q")
	code := "package x\n\n" + numbered("// c", 80)
	writeTree(t, root, map[string]string{
		"pkg/code.go": code,
		"README.md":   "# Readme\n\n" + numbered("intro", 6) + "The handler lives in pkg/code.go.\n" + numbered("outro", 6),
	})
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-q", "-m", "base")
	base := testGit(t, root, "rev-parse", "HEAD")
	edit := func(lines ...string) string {
		text := code
		for _, line := range lines {
			text = strings.Replace(text, "// c line "+line+"\n", "// c line "+line+" changed\n", 1)
		}
		return text
	}
	writeTree(t, root, map[string]string{"pkg/code.go": edit("10", "30", "50")})
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-q", "-m", "change")
	writeMap := func() {
		document, err := json.Marshal(map[string]any{"baseRevision": base, "hunks": diffHunks(t, root, base)})
		if err != nil {
			t.Fatal(err)
		}
		writeTree(t, root, map[string]string{".corvint/change.cem.json": string(document) + "\n"})
	}
	run := func() (string, string) {
		t.Helper()
		var stdout, stderr bytes.Buffer
		status, err := ProposeCitations(context.Background(), ProposeOptions{Root: root, Base: base}, &stdout, &stderr)
		if err != nil || status != 0 {
			t.Fatalf("ProposeCitations = %d, %v\n%s", status, err, stderr.String())
		}
		return stdout.String(), stderr.String()
	}
	writeMap()
	out, errText := run()
	seen := map[string]string{}
	rows := textLines([]byte(out))
	if len(rows) != 3 {
		t.Fatalf("rows = %d, want 3:\n%s", len(rows), out)
	}
	for _, row := range rows {
		fields := strings.Split(row, "\t")
		if unknownRow(fields) {
			continue
		}
		span := fields[1] + ":" + fields[2]
		if previous, ok := seen[span]; ok {
			t.Errorf("rows %s and %s both cite %s:\n%s", previous, fields[0], span, out)
		}
		seen[span] = fields[0]
	}
	for _, want := range []string{"hunk 1 edits pkg/code.go at BASE 9,7", "hunk 2 edits pkg/code.go at BASE 29,7", "hunk 3 edits pkg/code.go at BASE 49,7"} {
		if !strings.Contains(errText, want) {
			t.Errorf("stderr lacks the hunk mapping %q:\n%s", want, errText)
		}
	}
	if strings.Contains(errText, "map-stale") {
		t.Errorf("a current map was judged stale:\n%s", errText)
	}

	// A commit after the prepare adds a hunk and moves none: the map's ordinals
	// would name other hunks once the bind re-prepares, so the draft is keyed
	// by the current patch's hunk IDs instead.
	writeTree(t, root, map[string]string{"pkg/code.go": edit("10", "30", "50", "70")})
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-q", "-m", "later")
	out, errText = run()
	if !strings.Contains(errText, "NOTE map-stale") || !strings.Contains(errText, "map=STALE keyed=hunk-id") {
		t.Fatalf("a stale map was drafted from:\n%s", errText)
	}
	rows = textLines([]byte(out))
	if len(rows) != 4 {
		t.Fatalf("stale-map rows = %d, want the current patch's 4:\n%s", len(rows), out)
	}
	for _, row := range rows {
		key, _, _ := strings.Cut(row, "\t")
		if len(key) < 40 || !strings.Contains(errText, "hunk "+key+" edits pkg/code.go at BASE ") {
			t.Errorf("stale-map row is not keyed and mapped by hunk ID: %q\n%s", row, errText)
		}
	}

	// A commit after the prepare reverts a hunk the map holds: every current
	// hunk is still in the map, but the map's extra hunk would shift its
	// ordinals, so it is stale too.
	writeMap()
	if _, errText = run(); strings.Contains(errText, "map-stale") {
		t.Fatalf("a re-prepared map was judged stale:\n%s", errText)
	}
	writeTree(t, root, map[string]string{"pkg/code.go": edit("10", "30", "50")})
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-q", "-m", "revert")
	out, errText = run()
	if !strings.Contains(errText, "NOTE map-stale") || !strings.Contains(errText, "map=STALE keyed=hunk-id") {
		t.Fatalf("a map holding a reverted hunk was drafted from:\n%s", errText)
	}
	if rows = textLines([]byte(out)); len(rows) != 3 {
		t.Fatalf("reverted-hunk rows = %d, want the current patch's 3:\n%s", len(rows), out)
	}
}
