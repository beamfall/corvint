package dogfoodflow

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// TestProposeCitationsDCWV0439NeverDraftsModifiedFileLines pins V1-3145: a
// draft never cites a nearby line of a file the change modifies. A hunk whose
// only candidates are its own file's lines gets an unknown row naming the
// nearest stable line as a review hint; a contract line in a modified file is
// drafted only when no hunk of that file overlaps or shifts it, so a line the
// spec's hunk moves is not cited for y.go; an untouched file's line naming
// the path is still cited, and the unrelated spec never is. Fails on BASE,
// which drafted the spec hunk against its own heading and y.go against the
// shifted spec line.
func TestProposeCitationsDCWV0439NeverDraftsModifiedFileLines(t *testing.T) {
	root := t.TempDir()
	testGit(t, root, "init", "-q")
	spec := "# X v0\n\n## Requirements\n\n- `XYZ-V0-001`: the widget MUST frob.\n" + numbered("requirement", 30) +
		"\nThe module `internal/x/y.go` implements the frobber.\n" + numbered("tail", 4)
	source := func(name string) string {
		return "package x\n\n// " + name + " is a function.\nfunc " + name + "() int {\n" + numbered("\t// "+name, 30) + "\treturn 1\n}\n"
	}
	writeTree(t, root, map[string]string{
		"docs/specs/x-v0.md": spec, "internal/x/x.go": source("X"), "internal/x/y.go": source("Y"),
		"docs/guide.md":              "# Guide\n\n" + numbered("guide", 4) + "The frobber lives in internal/x/x.go.\n" + numbered("more", 4),
		"docs/specs/unrelated-v0.md": "# Unrelated\n\n" + numbered("unrelated", 6),
	})
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-q", "-m", "base")
	base := testGit(t, root, "rev-parse", "HEAD")
	writeTree(t, root, map[string]string{
		"docs/specs/x-v0.md": strings.Replace(spec, "requirement line 20\n", "requirement line 20 amended\n", 1),
		"internal/x/x.go": strings.NewReplacer("\t// X line 10\n", "\t// X line 10 frobs, as XYZ-V0-001 requires\n",
			"\t// X line 25\n", "\t// X line 25 changed\n").Replace(source("X")),
		"internal/x/y.go": strings.Replace(source("Y"), "\t// Y line 10\n", "\t// Y line 10 changed\n", 1),
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
	if code, err := ProposeCitations(context.Background(), ProposeOptions{Root: root, Base: base}, &stdout, &stderr); code != 0 || err != nil {
		t.Fatalf("ProposeCitations = %d, %v\n%s", code, err, stderr.String())
	}
	rows := textLines(stdout.Bytes())
	if len(hunks) != 4 || len(rows) != 4 {
		t.Fatalf("draft has %d rows for %d hunks:\n%s", len(rows), len(hunks), stdout.String())
	}
	specLines := splitRecords([]byte(testGit(t, root, "show", base+":docs/specs/x-v0.md") + "\n"))
	guideLines := splitRecords([]byte(testGit(t, root, "show", base+":docs/guide.md") + "\n"))
	want := []string{
		"unknown",
		"docs/specs/x-v0.md\t" + lineOf(t, specLines, "- `XYZ-V0-001`:"),
		"docs/guide.md\t" + lineOf(t, guideLines, "The frobber lives"),
		"unknown",
	}
	for index, row := range rows {
		fields := strings.Split(row, "\t")
		if strings.Contains(row, "unrelated-v0.md") {
			t.Errorf("row %d cites the unrelated spec: %q", index+1, row)
		}
		if want[index] == "unknown" {
			if !reasonedUnknownRow(fields) || !strings.Contains(fields[3], hunks[index].Path+" is modified by this change, so no line of it is drafted (DCW-V0-439)") {
				t.Errorf("row %d = %q; want a DCW-V0-439 unknown row for %s", index+1, row, hunks[index].Path)
			}
			continue
		}
		if got := fields[1] + "\t" + fields[2]; got != want[index] {
			t.Errorf("row %d = %q; want %q", index+1, row, want[index])
		}
	}
	if !strings.Contains(rows[0], "nearest stable line docs/specs/x-v0.md 3:3 specification is a review hint only") {
		t.Errorf("spec row does not hint its enclosing heading: %q", rows[0])
	}
}
