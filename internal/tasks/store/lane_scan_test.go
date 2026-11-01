package store

import (
	"os/exec"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestTCDV0731_ScanLaneCommitsReadsEveryMergedNonMergeCommit: a scan reports each commit's parent
// count, and the lane scan of a merge reads every non-merge commit its
// second parent reaches and its first parent does not, a nested lane
// merge's commits included; a single-parent commit has no lane range
// (TCD-V0-731).
func TestTCDV0731_ScanLaneCommitsReadsEveryMergedNonMergeCommit(t *testing.T) {
	if args := LaneLogArgs("FIRST", "SECOND", 3); !slices.Contains(args, "--no-merges") || slices.Contains(args, "--first-parent") || args[len(args)-2] != "SECOND" || args[len(args)-1] != "^FIRST" {
		t.Fatalf("lane argv %v", args)
	}
	root := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		c := exec.Command("git", append([]string{"-c", "user.name=t", "-c", "user.email=t@example.invalid"}, args...)...)
		c.Dir = root
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	run("init", "-q", "-b", "main")
	run("commit", "-q", "--allow-empty", "-m", "base")
	run("checkout", "-q", "-b", "side")
	run("commit", "-q", "--allow-empty", "-m", "lane one")
	run("checkout", "-q", "-b", "sub")
	run("commit", "-q", "--allow-empty", "-m", "sub lane")
	run("checkout", "-q", "side")
	run("merge", "-q", "--no-ff", "-m", "Merge sub", "sub")
	run("checkout", "-q", "main")
	run("commit", "-q", "--allow-empty", "-m", "main moves")
	run("merge", "-q", "--no-ff", "-m", "Merge side", "side")
	head, err := CloseoutRevision(root, "main")
	if err != nil {
		t.Fatal(err)
	}
	scan, err := ScanCommits(root, head, "", 10)
	parents := map[string]int{}
	for _, c := range scan.Commits {
		parents[c.Subject] = c.Parents
	}
	if err != nil || parents["Merge side"] != 2 || parents["base"] != 0 || parents["lane one"] != 1 {
		t.Fatalf("parents: %+v %v", scan, err)
	}
	lane, err := ScanLaneCommits(root, head, 10)
	var subjects []string
	for _, c := range lane.Commits {
		subjects = append(subjects, c.Subject)
	}
	slices.Sort(subjects)
	if err != nil || strings.Join(subjects, ",") != "lane one,sub lane" {
		t.Fatalf("lane commits %v %v", subjects, err)
	}
	first, err := CloseoutRevision(root, "main^1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ScanLaneCommits(root, first, 10); wire.CodeOf(err) != wire.CodeMissingEvidence {
		t.Fatalf("single-parent lane scan: %v", err)
	}
}
