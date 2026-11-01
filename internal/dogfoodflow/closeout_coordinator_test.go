package dogfoodflow

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// DCW-V0-907 (V1-1629, V1-3361, V1-3475): with ConcurrentVerifiers set, the
// base and tree cem status commands run at once. Each fake waits until the
// other has started, so a sequential pair would time out; both results come
// back unchanged for the exact comparison. Without the option the pair runs
// one after the other, as before.
func TestDCWV0907CheckRunsTheVerifierPairConcurrently(t *testing.T) {
	baseStarted, treeStarted := make(chan struct{}), make(chan struct{})
	fake := func(started, other chan struct{}, out string) Runner {
		return Runner{Path: out, Run: func(ctx context.Context, root string, args []string, stdout, stderr io.Writer) int {
			close(started)
			select {
			case <-other:
			case <-time.After(5 * time.Second):
				_, _ = io.WriteString(stderr, "the other verifier never started")
				return 9
			}
			_, _ = io.WriteString(stdout, out+" "+strings.Join(args, " "))
			return 0
		}}
	}
	c := &check{flow: flow{ctx: context.Background(), prefix: "dogfood-check", stderr: io.Discard}, options: CheckOptions{
		BaseVerifier: fake(baseStarted, treeStarted, "base"), TreeVerifier: fake(treeStarted, baseStarted, "tree"), ConcurrentVerifiers: true,
	}}
	base, tree := c.verifyPair("cem", "status")
	if base.status != 0 || tree.status != 0 || string(base.stdout) != "base cem status" || string(tree.stdout) != "tree cem status" {
		t.Fatalf("base=%d %q %q tree=%d %q %q", base.status, base.stdout, base.stderr, tree.status, tree.stdout, tree.stderr)
	}

	order := []string{}
	sequential := func(name string) Runner {
		return Runner{Path: name, Run: func(context.Context, string, []string, io.Writer, io.Writer) int {
			order = append(order, name)
			return 0
		}}
	}
	c.options = CheckOptions{BaseVerifier: sequential("base"), TreeVerifier: sequential("tree")}
	c.verifyPair("cem", "status")
	if strings.Join(order, ",") != "base,tree" {
		t.Fatalf("without ConcurrentVerifiers the pair ran %v, want base then tree", order)
	}
}

// ASD-V0-907 (V1-1629): finish-lane prints the worktree's dogfood-change run
// count from the log script/dogfood-change.sh appends to, or NOT_OBSERVED.
func TestFinishLaneASDV0907PrintsTheDogfoodChangeRunCount(t *testing.T) {
	gitDir := t.TempDir()
	var stderr bytes.Buffer
	lane := &finishLane{flow: flow{ctx: context.Background(), prefix: "dogfood-finish-lane", stderr: &stderr, gitDir: gitDir}}
	lane.reportRuns()
	if want := "dogfood-finish-lane: RUNS dogfood-change=NOT_OBSERVED log=" + filepath.Join(gitDir, "corvint", dogfoodChangeRuns) + "\n"; stderr.String() != want {
		t.Fatalf("stderr = %q, want %q", stderr.String(), want)
	}
	if err := os.MkdirAll(filepath.Join(gitDir, "corvint"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gitDir, "corvint", dogfoodChangeRuns), []byte("t1\tb\th\nt2\tb\th\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	stderr.Reset()
	lane.reportRuns()
	if !strings.HasPrefix(stderr.String(), "dogfood-finish-lane: RUNS dogfood-change=2 log=") {
		t.Fatalf("stderr = %q, want a count of 2", stderr.String())
	}
}

// DCW-V0-908 (V1-3188): after a seal, finish-lane names the exact report it
// bound by digest and says its review and the native completion are still to
// come; an unreadable report prints NOT_OBSERVED.
func TestFinishLaneDCWV0908NamesTheReportToReview(t *testing.T) {
	root := t.TempDir()
	var stderr bytes.Buffer
	lane := &finishLane{flow: flow{ctx: context.Background(), prefix: "dogfood-finish-lane", root: root, stderr: &stderr}}
	report := filepath.Join(root, ".corvint", "dogfood-report.json")
	lane.reportReview()
	if !strings.HasPrefix(stderr.String(), "dogfood-finish-lane: REVIEW report="+report+" sha256=NOT_OBSERVED reviewed=NOT_OBSERVED\n") {
		t.Fatalf("stderr = %q", stderr.String())
	}
	if err := os.MkdirAll(filepath.Dir(report), 0o777); err != nil {
		t.Fatal(err)
	}
	data := []byte("{\"dogfoodCheck\":\"PASS\"}\n")
	if err := os.WriteFile(report, data, 0o666); err != nil {
		t.Fatal(err)
	}
	stderr.Reset()
	lane.reportReview()
	want := fmt.Sprintf("dogfood-finish-lane: REVIEW report=%s sha256=%x reviewed=NOT_OBSERVED\n", report, sha256.Sum256(data))
	if !strings.HasPrefix(stderr.String(), want) || !strings.Contains(stderr.String(), "finish-lane records neither (DCW-V0-908)") {
		t.Fatalf("stderr = %q, want %q and the next-step line", stderr.String(), want)
	}
}
