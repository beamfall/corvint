package dogfoodflow

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// TestProposeCitationsDCWV0962DraftsNearestStableLineAsProposal pins
// V1-3284 and V1-3428 (amending DCW-V0-439 and DCW-V0-846): a hunk no line of
// an untouched file anchors gets its modified file's nearest stable line,
// preferring the enclosing heading or declaration, drafted on stdout as its
// cite row and marked "drafted proposal" on stderr, with no span drafted
// twice (DCW-V0-382). The unrelated spec is never cited.
func TestProposeCitationsDCWV0962DraftsNearestStableLineAsProposal(t *testing.T) {
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
	spans := map[string]bool{}
	for index, row := range rows {
		fields := strings.Split(row, "\t")
		if strings.Contains(row, "unrelated-v0.md") {
			t.Errorf("row %d cites the unrelated spec: %q", index+1, row)
		}
		if len(fields) != 4 || fields[0] != fmt.Sprint(index+1) || fields[1] != hunks[index].Path || spans[fields[1]+":"+fields[2]] {
			t.Errorf("row %d = %q; want a distinct cite of its own file %s", index+1, row, hunks[index].Path)
		}
		spans[fields[1]+":"+fields[2]] = true
		if mark := "  hunk " + fields[0] + " edits " + hunks[index].Path + " at BASE "; !strings.Contains(stderr.String(), mark) {
			t.Errorf("row %d: stderr lacks %q", index+1, mark)
		}
		if mark := "  drafted proposal: " + row + "\n    the nearest stable BASE span of the file this hunk edits, a same-file proposal rather than an anchor (DCW-V0-393): confirm it, or replace it"; !strings.Contains(stderr.String(), mark) {
			t.Errorf("row %d is not marked as a proposal:\n%s", index+1, stderr.String())
		}
	}
	if rows[0] != "1\tdocs/specs/x-v0.md\t3:3\tspecification" {
		t.Errorf("spec row does not draft its enclosing heading: %q", rows[0])
	}
	if !strings.Contains(stderr.String(), "  proposals=4 rows cite the nearest stable BASE span of the file their hunk edits") {
		t.Errorf("draft header does not count the proposals:\n%s", stderr.String())
	}
	// DCW-V0-845 (V1-3302): the header names the line basis and the BASE sha.
	if basis := "  line basis: every START:END in a plan row is a BASE line number, counted in each file as it is at BASE " + base + ", not at HEAD or in the working tree (DCW-V0-845)\n"; !strings.Contains(stderr.String(), basis) {
		t.Errorf("draft header does not name the line basis:\n%s", stderr.String())
	}
}
