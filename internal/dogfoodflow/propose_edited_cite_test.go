package dogfoodflow

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestProposeCitationsDCWV0980EditedCitedLineRefreshPassesPrecheck pins
// V1-3984: a reviewed row cites a fixture line, a later commit edits that line
// in place, and --refresh no longer keeps the edited span. It re-pins the row
// to the nearest BASE line the change leaves intact, the earlier one on a tie,
// and the refreshed draft passes the real cem cite precheck.
func TestProposeCitationsDCWV0980EditedCitedLineRefreshPassesPrecheck(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	testGit(t, root, "init", "-q")
	code := "package pre\n\n// First is one.\nfunc First() int {\n" + numbered("\t// first", 12) + "\treturn 1\n}\n"
	fixture := numbered("fixture", 10)
	writeTree(t, root, map[string]string{"pre.go": code, "testdata/fixture.txt": fixture}) //corvint:fixture-path
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-q", "-m", "base")
	base := testGit(t, root, "rev-parse", "HEAD")
	writeTree(t, root, map[string]string{"pre.go": strings.Replace(code, "\t// first line 6\n", "\t// first line 6 changed\n", 1)})
	testGit(t, root, "commit", "-q", "-am", "code")

	var stdout, stderr bytes.Buffer
	if code, err := ProposeCitations(context.Background(), ProposeOptions{Root: root, Base: base}, &stdout, &stderr); code != 0 || err != nil {
		t.Fatalf("ProposeCitations = %d, %v\n%s", code, err, &stderr)
	}
	codeID, _, _ := strings.Cut(textLines(stdout.Bytes())[0], "\t")
	// The reviewed plan cites BASE line 6 of the fixture, which the lane
	// then edits in place.
	planPath := writeInput(t, codeID+"\ttestdata/fixture.txt\t6:6\ttest-claim\n")                                                             //corvint:fixture-path
	writeTree(t, root, map[string]string{"testdata/fixture.txt": strings.Replace(fixture, "fixture line 6\n", "fixture line 6 edited\n", 1)}) //corvint:fixture-path
	testGit(t, root, "commit", "-q", "-am", "fixture")

	stdout.Reset()
	stderr.Reset()
	if code, err := ProposeCitations(context.Background(), ProposeOptions{Root: root, Base: base, Refresh: planPath}, &stdout, &stderr); code != 0 || err != nil {
		t.Fatalf("ProposeCitations --refresh = %d, %v\n%s", code, err, &stderr)
	}
	refreshed := textLines(stdout.Bytes())
	output := stdout.String() + stderr.String()
	if containsLine(refreshed, codeID+"\ttestdata/fixture.txt\t6:6\ttest-claim") { //corvint:fixture-path
		t.Fatalf("the refresh kept the edited span 6:6:\n%s", output)
	}
	if !containsLine(refreshed, codeID+"\ttestdata/fixture.txt\t5:5\ttest-claim") || !strings.Contains(output, "re-pinned to the nearest surviving span testdata/fixture.txt:5:5") { //corvint:fixture-path
		t.Fatalf("the edited-line row was not re-pinned to fixture 5:5:\n%s", output)
	}
	next := filepath.Join(filepath.Dir(planPath), "plan.new")
	if err := os.WriteFile(next, stdout.Bytes(), 0o666); err != nil {
		t.Fatal(err)
	}
	if code, out, errText, _ := preflightRun(t, root, ChangeOptions{Base: base, Citations: next}); code != 0 || !strings.Contains(out, "inputs-check PASS") {
		t.Fatalf("the refreshed draft was refused: code=%d\nstdout=%s\nstderr=%s\nplan=%s", code, out, errText, stdout.String())
	}
}
