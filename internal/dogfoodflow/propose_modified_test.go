package dogfoodflow

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// DCW-V0-962 amended by DWC-V0-926/927: keep useful ID/path anchors across
// a multi-file change, and abstain on arbitrary own-file lines and headings.
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
	// DWC-V0-926/927 supersede unanchored same-file, move, sibling and seal
	// guesses. Preserve the full scenario and check every remaining cite with
	// the CEM verifier; missing anchors must be explicit unknown rows.
	assertAnchoredDraft(t, root, base, stdout.String())

}
