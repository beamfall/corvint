package dogfoodflow

import (
	"strings"
	"testing"
)

// TestDCWV0724MapMismatchNamesThePlansPassingHead (V1-4115): a plan that
// passed inputs-check, refused citation-plan-map-mismatch after a later commit
// re-prepares the map, names the HEAD it passed at, the current HEAD and the
// refresh command.
func TestDCWV0724MapMismatchNamesThePlansPassingHead(t *testing.T) {
	root, base := preflightRepository(t, nil)
	passed := testGit(t, root, "rev-parse", "HEAD")
	plan := writeInput(t, "1\tpre.go\t3:3\timplementation\n2\tpre.go\t20:20\timplementation\n")
	if code, stdout, stderr, _ := preflightRun(t, root, ChangeOptions{Base: base, Citations: plan}); code != 0 {
		t.Fatalf("first pass code=%d stdout=%s stderr=%s", code, stdout, stderr)
	}
	// A later commit drops the second hunk, so ordinal 2 names no hunk.
	content := testGit(t, root, "show", "HEAD:pre.go") + "\n"
	writeTree(t, root, map[string]string{"pre.go": strings.Replace(content, "\t// second line 6 changed\n", "\t// second line 6\n", 1)})
	testGit(t, root, "commit", "-q", "-am", "later")
	head := testGit(t, root, "rev-parse", "HEAD")
	code, stdout, stderr, _ := preflightRun(t, root, ChangeOptions{Base: base, Citations: plan})
	output := stdout + stderr
	if code == 0 || !strings.Contains(output, "citation-plan-map-mismatch") {
		t.Fatalf("code=%d stdout=%s stderr=%s", code, stdout, stderr)
	}
	want := "the plan passed inputs-check at HEAD " + passed + ", but HEAD is now " + head + " and a later commit changed the map: run corvint dogfood propose-citations " + base + " --refresh " + plan
	if !strings.Contains(output, want) || !strings.Contains(output, "(DCW-V0-724)") {
		t.Errorf("output lacks %q:\n%s", want, output)
	}
}
