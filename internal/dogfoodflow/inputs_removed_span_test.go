package dogfoodflow

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestASDV0577InputsCheckReportsACitedSpecSpanTheHeadRemoves (V1-3503): when
// the candidate head no longer holds a spec span the plan cites, as after a
// merge of main that drops a requirement, the pre-flight names that plan row
// and its span as cite-span-not-stable before any bind attempt.
func TestASDV0577InputsCheckReportsACitedSpecSpanTheHeadRemoves(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	testGit(t, root, "init", "-q")
	code := "package pre\n\n// First is one.\nfunc First() int {\n" + numbered("\t// first", 12) + "\treturn 1\n}\n"
	spec := "# Spec\n\n- `TSP-V0-862`: removed by the merge.\n- `TSP-V0-863`: removed by the merge.\n\n" + numbered("kept", 12)
	writeTree(t, root, map[string]string{"pre.go": code, "spec.md": spec})
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-q", "-m", "base")
	base := testGit(t, root, "rev-parse", "HEAD")
	writeTree(t, root, map[string]string{
		"pre.go":  strings.Replace(code, "\t// first line 6\n", "\t// first line 6 changed\n", 1),
		"spec.md": strings.Replace(spec, "- `TSP-V0-862`: removed by the merge.\n- `TSP-V0-863`: removed by the merge.\n", "", 1),
	})
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-q", "-m", "merge drops the cited requirements")
	head := testGit(t, root, "rev-parse", "HEAD")

	plan := writeInput(t, "1\tspec.md\t3:4\tspecification\n2\tpre.go\t3:3\timplementation\n")
	exit, stdout, stderr, labels := preflightRun(t, root, ChangeOptions{Base: base, Citations: plan})
	if exit == 0 || strings.Contains(stdout, "inputs-check PASS") {
		t.Fatalf("a plan citing a removed spec span passed: code=%d stdout=%q stderr=%q", exit, stdout, stderr)
	}
	for _, want := range []string{"FAIL cem-cite-precheck refused plan rows=1", `row 1 hunk=1 path="spec.md" lines=3:4 cite-span-not-stable`, "prepare cem-cite-precheck NOT_PRODUCED cite-span-not-stable"} {
		if !strings.Contains(stdout+stderr, want) {
			t.Errorf("output lacks %q:\nstdout=%s\nstderr=%s", want, stdout, stderr)
		}
	}
	assertNothingBound(t, root, head, labels)
}
