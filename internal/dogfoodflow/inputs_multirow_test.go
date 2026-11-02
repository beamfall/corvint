package dogfoodflow

import (
	"path/filepath"
	"strings"
	"testing"
)

// DCW-V0-690 (V1-1724): dogfood inputs-check refuses a citation plan with
// several bad rows of different codes in one pass, before any bind. Every
// refused row is named with its code: a governing instruction file the change
// modifies (cite-self-modified-authority), two changed lines
// (cite-span-not-stable) and a path absent at BASE (missing-evidence). The
// valid row is not named, and nothing is bound or committed.
func TestDCWV0690InputsCheckNamesEveryRefusedRowInOnePass(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	testGit(t, root, "init", "-q")
	content := "package pre\n\n// First is one.\nfunc First() int {\n" + numbered("\t// first", 12) + "\treturn 1\n}\n\n// Second is two.\nfunc Second() int {\n" + numbered("\t// second", 12) + "\treturn 2\n}\n"
	writeTree(t, root, map[string]string{"pre.go": content, "AGENTS.md": "# Rules\n\nkeep it short\n"})
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-q", "-m", "base")
	base := testGit(t, root, "rev-parse", "HEAD")
	changed := strings.Replace(content, "\t// first line 6\n", "\t// first line 6 changed\n", 1)
	changed = strings.Replace(changed, "\t// second line 6\n", "\t// second line 6 changed\n", 1)
	// The change edits AGENTS.md, so citing it as authority is self-modified.
	writeTree(t, root, map[string]string{"pre.go": changed, "AGENTS.md": "# Rules\n\nkeep it small\n"})
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-q", "-m", "change")
	head := testGit(t, root, "rev-parse", "HEAD")
	plan := writeInput(t, strings.Join([]string{
		"1\tAGENTS.md\t1:1\timplementation",
		"2\tpre.go\t10:10\timplementation",
		"3\tpre.go\t27:27\timplementation",
		"2\tabsent.go\t1:1\timplementation",
		"3\tpre.go\t3:3\timplementation",
	}, "\n")+"\n")
	code, stdout, stderr, labels := preflightRun(t, root, ChangeOptions{Base: base, Citations: plan})
	if code == 0 || !strings.Contains(stdout, "dogfood-inputs-check: prepare FAIL exit=") {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	output := stdout + stderr
	for _, want := range []string{
		"FAIL cem-cite-precheck refused plan rows=4",
		`row 1 hunk=1 path="AGENTS.md" lines=1:1 cite-self-modified-authority`,
		`row 2 hunk=2 path="pre.go" lines=10:10 cite-span-not-stable`,
		`row 3 hunk=3 path="pre.go" lines=27:27 cite-span-not-stable`,
		`row 4 hunk=2 path="absent.go" lines=1:1 missing-evidence`,
		"prepare cem-cite-precheck NOT_PRODUCED cite-self-modified-authority",
	} {
		if !strings.Contains(output, want) {
			t.Errorf("output lacks %q:\nstdout=%s\nstderr=%s", want, stdout, stderr)
		}
	}
	if strings.Contains(output, `row 5 hunk=3`) {
		t.Errorf("the valid row 5 was named as refused:\n%s", output)
	}
	assertNothingBound(t, root, head, labels)
}
