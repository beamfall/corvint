package dogfoodflow

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

// TestProposeCitationsDCWV0440SameChangeRequirementStaysUnknown pins V1-3151:
// a change that adds a requirement, code naming it and a new test named after
// it drafts no whole-file line-1 row and no row on a JSON or .agent-evidence/
// record that merely lists the code path; the code and test hunks get unknown
// rows naming the requirement this change adds, since same-change intent is
// not BASE evidence. Fails on BASE, which cited docs/worklist.json 1:1 for the
// code hunk and drafted the new test file's row without the requirement.
func TestProposeCitationsDCWV0440SameChangeRequirementStaysUnknown(t *testing.T) {
	root := t.TempDir()
	testGit(t, root, "init", "-q")
	spec := "# X v0\n\n## Requirements\n\n- `XYZ-V0-001`: the widget MUST frob.\n" + numbered("requirement", 8)
	code := "package x\n\n// Gadget is a function.\nfunc Gadget() int {\n" + numbered("\t// gadget", 12) + "\treturn 1\n}\n"
	writeTree(t, root, map[string]string{
		"docs/specs/x-v0.md":   spec,
		"internal/x/gadget.go": code,
		"docs/worklist.json":   `{"paths":["internal/x/gadget.go"]}` + "\n",
		"docs/LIST.txt":        "internal/x/gadget.go\n",
		// An untouched multiline BASE document naming the path competes as an
		// anchor; the same-change requirement still keeps the row unknown.
		"docs/design/gadget.md":                              "# Gadget\n\n## Traceability\n\n| `internal/x/gadget.go` `Gadget` | spins |\n| state | none |\n",
		".agent-evidence/native-taskman/affected-dirty.json": "{\n  \"dirty\": [\n    \"internal/x/gadget.go\"\n  ]\n}\n",
		"protocol/x/MIGRATION.json":                          "{\n  \"note\": \"one\",\n  \"path\": \"internal/x/gadget.go\",\n  \"end\": true\n}\n",
	})
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-q", "-m", "base")
	base := testGit(t, root, "rev-parse", "HEAD")
	writeTree(t, root, map[string]string{
		"docs/specs/x-v0.md":        strings.Replace(spec, "- `XYZ-V0-001`: the widget MUST frob.\n", "- `XYZ-V0-001`: the widget MUST frob.\n- `XYZ-V0-002`: the gadget MUST spin.\n", 1),
		"internal/x/gadget.go":      strings.Replace(code, "\t// gadget line 6\n", "\t// gadget line 6 spins, as XYZ-V0-002 requires\n", 1),
		"internal/x/gadget_test.go": "package x\n\nimport \"testing\"\n\nfunc TestXYZV0002GadgetSpins(t *testing.T) {\n\tif Gadget() != 1 {\n\t\tt.Fatal(\"no spin\")\n\t}\n}\n",
	})
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-q", "-m", "change")

	// No map yet: the hunk-ID template path drafts the same rows (DCW-V0-350).
	var stdout, stderr bytes.Buffer
	if code, err := ProposeCitations(context.Background(), ProposeOptions{Root: root, Base: base}, &stdout, &stderr); code != 0 || err != nil {
		t.Fatalf("ProposeCitations = %d, %v\n%s", code, err, stderr.String())
	}
	rows := textLines(stdout.Bytes())
	if len(rows) != 3 {
		t.Fatalf("draft has %d rows; want 3:\n%s\n%s", len(rows), stdout.String(), stderr.String())
	}
	edits := map[string]string{}
	for _, row := range rows {
		id, _, _ := strings.Cut(row, "\t")
		for _, name := range []string{"docs/specs/x-v0.md", "internal/x/gadget.go", "internal/x/gadget_test.go"} {
			if strings.Contains(stderr.String(), "hunk "+id+" edits "+name+" at BASE ") {
				edits[id] = name
			}
		}
	}
	for _, row := range rows {
		fields := strings.Split(row, "\t")
		if fields[2] == "1:1" || strings.HasSuffix(fields[1], ".json") || strings.HasPrefix(fields[1], ".agent-evidence/") || fields[1] == "docs/LIST.txt" {
			t.Errorf("row cites a whole-file or data record: %q", row)
		}
		switch name := edits[fields[0]]; name {
		case "internal/x/gadget.go", "internal/x/gadget_test.go":
			if !reasonedUnknownRow(fields) || !strings.Contains(fields[3], "requirement XYZ-V0-002, which this change itself adds in docs/specs/x-v0.md") ||
				!strings.Contains(fields[3], "(DCW-V0-440)") {
				t.Errorf("row for %s = %q; want an unknown row naming the same-change requirement XYZ-V0-002", name, row)
			}
		case "docs/specs/x-v0.md":
			if !reasonedUnknownRow(fields) {
				t.Errorf("spec row = %q; want unknown", row)
			}
		default:
			t.Errorf("row %q maps to no hunk:\n%s", row, stderr.String())
		}
	}
}
