package dogfoodflow

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

// TestProposeCitationsDCWV0841SiblingsPathLinesAndDataReason pins V1-3673 and
// V1-3778: a new file absent at BASE is proposed the first stable line of the
// BASE sibling it follows (the file a test tests, in both the _test and test_
// forms, then the same file name in a sibling directory); a line naming a
// hunk's whole path anchors it when its requirement family comes only from
// the change's other files; and a data hunk names why it is never cited.
// A sibling the change itself modifies is never cited (DCW-V0-393).
func TestProposeCitationsDCWV0841SiblingsPathLinesAndDataReason(t *testing.T) {
	root := t.TempDir()
	testGit(t, root, "init", "-q")
	writeTree(t, root, map[string]string{
		"internal/y/y.go":     "package y\n\n// Y is the y contract.\nfunc Y() int {\n\treturn 1\n}\n",
		"internal/w/w.go":     "package w\n\n// W is the w contract.\nfunc W() int {\n\treturn 2\n}\n",
		"script/tool.py":      "\"\"\"Tool helper.\"\"\"\n\n\ndef tool():\n    return 1\n",
		"cmd/alpha/help.go":   "package main\n\n// Help prints the alpha help.\nfunc Help() string {\n\treturn \"alpha\"\n}\n",
		"tools/scopes.json":   "{\n  \"scopes\": [\n    \"a\"\n  ]\n}\n",
		"docs/guide.md":       "# Guide\n\nThe file `tools/scopes.json` lists the read scopes.\n\nNothing else.\n",
		"docs/notes.md":       "# Notes\n\nA note.\n",
		"internal/y/data.txt": "seed\n",
	})
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-q", "-m", "base")
	base := testGit(t, root, "rev-parse", "HEAD")
	writeTree(t, root, map[string]string{
		"internal/y/y_test.go":       "package y\n\nimport \"testing\"\n\nfunc TestY(t *testing.T) {\n\tif Y() != 1 {\n\t\tt.Fatal(\"y\")\n\t}\n}\n",
		"script/test_tool.py":        "from tool import tool\n\n\ndef test_tool():\n    assert tool() == 1\n",
		"cmd/beta/help.go":           "package main\n\n// Help prints the beta help.\nfunc Help() string {\n\treturn \"beta\"\n}\n",
		"tools/scopes.json":          "{\n  \"scopes\": [\n    \"a\",\n    \"b\"\n  ]\n}\n",
		"docs/notes.md":              "# Notes\n\nA note.\nABC-V0-001 and ABC-V0-002 describe the scopes.\n",
		"internal/w/w.go":            "package w\n\n// W is the w contract.\nfunc W() int {\n\treturn 3\n}\n",
		"internal/w/w_test.go":       "package w\n\nimport \"testing\"\n\nfunc TestW(t *testing.T) {\n\tif W() != 3 {\n\t\tt.Fatal(\"w\")\n\t}\n}\n",
		"internal/y/testdata/z.json": "{\"z\": 1}\n",
	})
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-q", "-m", "change")

	var stdout, stderr bytes.Buffer
	if code, err := ProposeCitations(context.Background(), ProposeOptions{Root: root, Base: base}, &stdout, &stderr); code != 0 || err != nil {
		t.Fatalf("ProposeCitations = %d, %v\n%s", code, err, stderr.String())
	}
	paths := map[string]string{}
	for _, row := range textLines(stdout.Bytes()) {
		id, _, _ := strings.Cut(row, "\t")
		for _, line := range strings.Split(stderr.String(), "\n") {
			if _, rest, ok := strings.Cut(line, "hunk "+id+" edits "); ok {
				name, _, _ := strings.Cut(rest, " at BASE ")
				paths[name] = row
			}
		}
	}
	want := map[string]string{
		"internal/y/y_test.go": "\tinternal/y/y.go\t4:4\t",
		"script/test_tool.py":  "\tscript/tool.py\t4:4\t",
		"cmd/beta/help.go":     "\tcmd/alpha/help.go\t4:4\t",
		"tools/scopes.json":    "\tdocs/guide.md\t3:3\t",
	}
	for name, cite := range want {
		row, ok := paths[name]
		if !ok || !strings.Contains(row, cite) {
			t.Errorf("row for %s = %q; want it to cite %q\nstdout=%s\nstderr=%s", name, row, cite, stdout.String(), stderr.String())
		}
	}
	for _, name := range []string{"internal/y/y_test.go", "script/test_tool.py", "cmd/beta/help.go"} {
		if !strings.Contains(stderr.String(), name+" is new at BASE and follows the pattern of its BASE sibling") {
			t.Errorf("no sibling reason names %s:\n%s", name, stderr.String())
		}
	}
	if row := paths["internal/w/w_test.go"]; !strings.Contains(row, "\tunknown\tinsufficient-evidence\t") {
		t.Errorf("a test whose sibling the change modifies was proposed %q", row)
	}
	if row := paths["internal/y/testdata/z.json"]; !strings.Contains(row, "\tunknown\tinsufficient-evidence\t") || !strings.Contains(row, "a data record is never cited") || !strings.Contains(row, "DCW-V0-841") {
		t.Errorf("data row = %q; want unknown naming the DCW-V0-841 data reason", row)
	}
}
