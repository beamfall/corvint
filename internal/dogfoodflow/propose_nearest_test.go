package dogfoodflow

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestProposeCitationsDCWV0980DeletedLineGetsNearestSurvivingSpan pins the
// cit-u18 friction: a row citing a test line the change deletes was printed
// unknown (DCW-V0-840), so the lane rewrote it by hand. Now it is re-pinned to
// the nearest BASE line the change leaves intact, the note names the move,
// and the re-pinned draft passes the real cem cite precheck. A row citing a
// file the change deletes has no surviving line and stays unknown.
func TestProposeCitationsDCWV0980DeletedLineGetsNearestSurvivingSpan(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	testGit(t, root, "init", "-q")
	content := "package pre\n\nimport \"testing\"\n\nfunc TestPre(t *testing.T) {\n" + numbered("\t// check", 10) + "}\n"
	writeTree(t, root, map[string]string{"pre_test.go": content, "gone.txt": "gone one\ngone two\n"})
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-q", "-m", "base")
	base := testGit(t, root, "rev-parse", "HEAD")
	// BASE line 10 is "check line 5"; the change deletes it and gone.txt.
	writeTree(t, root, map[string]string{"pre_test.go": strings.Replace(content, "\t// check line 5\n", "", 1)})
	testGit(t, root, "rm", "-q", "gone.txt")
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-q", "-m", "change")

	var stdout, stderr bytes.Buffer
	if code, err := ProposeCitations(context.Background(), ProposeOptions{Root: root, Base: base}, &stdout, &stderr); code != 0 || err != nil {
		t.Fatalf("ProposeCitations = %d, %v\n%s", code, err, stderr.String())
	}
	rows := textLines(stdout.Bytes())
	if len(rows) != 2 {
		t.Fatalf("draft rows = %q\n%s", rows, stderr.String())
	}
	goneID, _, _ := strings.Cut(rows[0], "\t")
	testID, _, _ := strings.Cut(rows[1], "\t")
	planPath := writeInput(t, goneID+"\tgone.txt\t1:2\ttest-claim\n"+testID+"\tpre_test.go\t10:10\ttest-claim\n")
	stdout.Reset()
	stderr.Reset()
	if code, err := ProposeCitations(context.Background(), ProposeOptions{Root: root, Base: base, Refresh: planPath}, &stdout, &stderr); code != 0 || err != nil {
		t.Fatalf("ProposeCitations --refresh = %d, %v\n%s", code, err, stderr.String())
	}
	refreshed := textLines(stdout.Bytes())
	output := stdout.String() + stderr.String()
	if !containsLine(refreshed, testID+"\tpre_test.go\t9:9\ttest-claim") || !strings.Contains(output, "re-pinned to the nearest surviving span pre_test.go:9:9") || !strings.Contains(output, "DCW-V0-980") {
		t.Errorf("the deleted-line row was not re-pinned to pre_test.go 9:9:\n%s", output)
	}
	if !containsPrefix(refreshed, goneID+"\tunknown\tinsufficient-evidence\t") || !strings.Contains(output, "DCW-V0-840") {
		t.Errorf("the deleted-file row did not stay unknown:\n%s", output)
	}
	next := filepath.Join(filepath.Dir(planPath), "plan.new")
	if err := os.WriteFile(next, stdout.Bytes(), 0o666); err != nil {
		t.Fatal(err)
	}
	if code, out, errText, _ := preflightRun(t, root, ChangeOptions{Base: base, Citations: next}); code != 0 {
		t.Fatalf("the re-pinned draft was refused: code=%d\nstdout=%s\nstderr=%s\nplan=%s", code, out, errText, stdout.String())
	}
}

// TestSurvivingPartsDCWV0980 pins the split order: the longest part of a
// span no hunk removes first, the earlier part on a tie.
func TestSurvivingPartsDCWV0980(t *testing.T) {
	for _, test := range []struct {
		start, end int
		removed    []removedLines
		want       [][2]int
	}{
		{1, 10, []removedLines{{"h", 3, 4}}, [][2]int{{5, 10}, {1, 2}}},
		{1, 5, []removedLines{{"h", 3, 3}}, [][2]int{{1, 2}, {4, 5}}},
		{2, 2, []removedLines{{"h", 2, 2}}, [][2]int{}},
		{1, 4, []removedLines{{"h", 8, 9}}, [][2]int{{1, 4}}},
	} {
		got := survivingParts(test.start, test.end, test.removed)
		if len(got) != len(test.want) {
			t.Errorf("survivingParts(%d, %d) = %v, want %v", test.start, test.end, got, test.want)
			continue
		}
		for index := range got {
			if got[index] != test.want[index] {
				t.Errorf("survivingParts(%d, %d) = %v, want %v", test.start, test.end, got, test.want)
			}
		}
	}
}
