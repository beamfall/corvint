package dogfoodflow

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestDogfoodDCWV0849RefusesBaseNotAncestorOfHead pins V1-3680: with a BASE
// HEAD lacks, propose-citations (with and without --refresh) and dogfood
// change refuse base-not-ancestor (DCW-V0-946) with exit 2, print no draft and
// write no map, instead of drafting or binding the revert of BASE's commits.
func TestDogfoodDCWV0849RefusesBaseNotAncestorOfHead(t *testing.T) {
	root, base := preflightRepository(t, nil)
	head := testGit(t, root, "rev-parse", "HEAD")
	branch := testGit(t, root, "rev-parse", "--abbrev-ref", "HEAD")
	testGit(t, root, "checkout", "-q", "-b", "mainline", base)
	writeTree(t, root, map[string]string{"mainline.txt": "mainline\n"})
	testGit(t, root, "add", "mainline.txt")
	testGit(t, root, "commit", "-q", "-m", "mainline")
	ahead := testGit(t, root, "rev-parse", "HEAD")
	testGit(t, root, "checkout", "-q", branch)
	if testGit(t, root, "rev-parse", "HEAD") != head {
		t.Fatal("checkout did not restore HEAD")
	}
	plan := writeInput(t, "1\tunknown\tinsufficient-evidence\tnote\n")
	for name, options := range map[string]ProposeOptions{
		"draft":   {Root: root, Base: ahead},
		"refresh": {Root: root, Base: ahead, Refresh: plan},
	} {
		var stdout, stderr bytes.Buffer
		if code, _ := ProposeCitations(context.Background(), options, &stdout, &stderr); code != 2 || stdout.Len() != 0 || !strings.Contains(stderr.String(), "REFUSE base-not-ancestor base=") {
			t.Errorf("propose-citations %s: code=%d stdout=%q\n%s", name, code, stdout.String(), stderr.String())
		}
	}
	// finish-lane keeps its own head-not-ahead-of-base refusal; dogfood
	// change outside it now refuses the same BASE by name.
	var changeErr bytes.Buffer
	steps := Runner{Path: "corvint", Run: func(context.Context, string, []string, io.Writer, io.Writer) int {
		t.Error("dogfood change ran a step for a BASE HEAD lacks")
		return 1
	}}
	code, _ := Change(context.Background(), ChangeOptions{Root: root, Base: ahead, Citations: plan, Verify: "go version", Outcome: "passed", IntentsFile: writeInput(t, "#no-intent-declared\n"), Steps: steps}, &changeErr)
	if code != 2 || !strings.Contains(changeErr.String(), "REFUSE base-not-ancestor base=") {
		t.Errorf("dogfood change: code=%d\n%s", code, changeErr.String())
	}
	if _, err := os.Lstat(filepath.Join(root, ".corvint", "change.cem.json")); err == nil {
		t.Error("the refused change wrote .corvint/change.cem.json")
	}
	var stdout, stderr bytes.Buffer
	if code, err := ProposeCitations(context.Background(), ProposeOptions{Root: root, Base: base}, &stdout, &stderr); code != 0 || err != nil {
		t.Errorf("an ancestor BASE was refused: %d, %v\n%s", code, err, stderr.String())
	}
}
