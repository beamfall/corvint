package dogfoodflow

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// DCW-V0-333 (V1-2423): a draft never cites a bookkeeping path. A line in the
// task store, docs/BUILD-LOG.md or a build-log entry that names a hunk's path
// is no anchor candidate, so the code hunk gets an unknown row rather than a
// line of the file it modifies (DCW-V0-439), and a hunk in a bookkeeping file gets an unknown row instead of a cite of
// itself. Every cite row of the draft is then usable as written.
func TestProposeCitationsDCWV0333SkipsBookkeepingPaths(t *testing.T) {
	root := t.TempDir()
	testGit(t, root, "init", "-q")
	source := "package x\n\n// Named is a function.\nfunc Named() int {\n" + numbered("\t// Named", 20) + "\treturn 1\n}\n"
	record := func(title string) string {
		return "# " + title + "\n\nThis entry changes internal/x/named.go.\n" + numbered(title, 12)
	}
	ticket := "{\n  \"title\": \"fix internal/x/named.go\",\n" + numbered("  \"note\": \"line", 12) + "  \"status\": \"OPEN\"\n}\n"
	writeTree(t, root, map[string]string{
		"internal/x/named.go":            source,
		".taskman/tickets/V1-0001.json":  ticket,
		"docs/BUILD-LOG.md":              record("Build log"),
		"docs/build-log/2026-10-01-x.md": record("Entry"),
		"docs/specs/unrelated-v0.md":     "# Unrelated\n" + numbered("spec", 4),
	})
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-q", "-m", "base")
	base := testGit(t, root, "rev-parse", "HEAD")
	writeTree(t, root, map[string]string{
		"internal/x/named.go":            strings.Replace(source, "\t// Named line 10\n", "\t// Named line 10 changed\n", 1),
		".taskman/tickets/V1-0001.json":  strings.Replace(ticket, "\"status\": \"OPEN\"", "\"status\": \"DONE\"", 1),
		"docs/build-log/2026-10-01-x.md": strings.Replace(record("Entry"), "Entry line 6\n", "Entry line 6 amended\n", 1),
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
	bookkeeping := "\tunknown\tinsufficient-evidence\tproposed: "
	for index, row := range rows {
		ordinal, rest, _ := strings.Cut(row, "\t")
		for _, prefix := range []string{".taskman/", "docs/BUILD-LOG.md", "docs/build-log/"} {
			if strings.HasPrefix(rest, prefix) {
				t.Errorf("row %s cites a bookkeeping path: %q", ordinal, row)
			}
		}
		switch path := hunks[index].Path; path {
		case "internal/x/named.go":
			if !strings.HasPrefix("\t"+rest, bookkeeping) || !strings.Contains(rest, "nearest stable line internal/x/named.go 4:4 dependency is a review hint only") {
				t.Errorf("code row = %q; want a DCW-V0-439 unknown row hinting its own declaration line", row)
			}
		default:
			if want := bookkeeping + path + " is a bookkeeping record (DCW-V0-333)"; !strings.HasPrefix("\t"+rest, want) {
				t.Errorf("row for %s = %q; want an unknown bookkeeping row", path, row)
			}
		}
	}
}
