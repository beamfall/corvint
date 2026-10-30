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

// deadlineChange runs Change with an impact step that overruns the Git
// deadline on its first overruns calls and then succeeds (DCW-V0-075).
func deadlineChange(t *testing.T, overruns int) (root, evidence string, calls int, stderr string) {
	t.Helper()
	root, evidence, stderr = impactChange(t, func(stdout, errOut io.Writer) int {
		calls++
		if calls <= overruns {
			_, _ = io.WriteString(errOut, impactDeadlineEnvelope)
			return 2
		}
		_, _ = io.WriteString(stdout, "coordination-time impact output\n")
		return 0
	})
	return root, evidence, calls, stderr
}

// impactChange runs Change with impact answering the coordination-time impact
// step.
func impactChange(t *testing.T, impact func(stdout, errOut io.Writer) int) (root, evidence, stderr string) {
	t.Helper()
	root = t.TempDir()
	testGit(t, root, "init", "-q")
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("base\n"), 0o666); err != nil {
		t.Fatal(err)
	}
	testGit(t, root, "add", "a.txt")
	testGit(t, root, "commit", "-q", "-m", "base")
	base := testGit(t, root, "rev-parse", "HEAD")
	evidence = filepath.Join(testGit(t, root, "rev-parse", "--absolute-git-dir"), "corvint")
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("changed\n"), 0o666); err != nil {
		t.Fatal(err)
	}
	testGit(t, root, "commit", "-q", "-am", "change")
	steps := Runner{Path: "corvint", Run: func(_ context.Context, _ string, args []string, stdout, errOut io.Writer) int {
		switch args[0] {
		case "query":
			_, _ = io.WriteString(stdout, "coordination-time query output\n")
			return 0
		case "impact":
			return impact(stdout, errOut)
		}
		return 1
	}}
	var out bytes.Buffer
	if _, err := Change(context.Background(), ChangeOptions{Root: root, Base: base, Steps: steps}, &out); err != nil {
		t.Fatal(err)
	}
	return root, evidence, out.String()
}

func readReport(t *testing.T, root string) []byte {
	t.Helper()
	report, err := os.ReadFile(filepath.Join(root, ".corvint/dogfood-report.json"))
	if err != nil {
		t.Fatal(err)
	}
	return report
}

// TestDCWV0075OneOverrunIsRetriedOnce: a single deadline overrun is retried
// once and the retried run's output is the produced evidence.
func TestDCWV0075OneOverrunIsRetriedOnce(t *testing.T) {
	root, evidence, calls, stderr := deadlineChange(t, 1)
	if calls != 2 {
		t.Fatalf("impact ran %d times, want 2", calls)
	}
	if !strings.Contains(stderr, "dogfood-change: NOTE coordination-time-impact deadline overrun; retrying once\n") {
		t.Errorf("no retry NOTE:\n%s", stderr)
	}
	if got, _ := os.ReadFile(filepath.Join(evidence, "coordination-time-impact.json")); string(got) != "coordination-time impact output\n" {
		t.Errorf("impact evidence = %q", got)
	}
	if report := readReport(t, root); !bytes.Contains(report, []byte(`{"name": "coordination-time-impact", "status": "PRODUCED", "reason": "none"}`)) {
		t.Errorf("report lacks the produced row:\n%s", report)
	}
}

// TestDCWV0075SecondOverrunIsAVisibleNonBlockingAbstention: a second overrun
// stops retrying and records the admitted reason with no digest artifact, and
// check accepts exactly the retained envelope.
func TestDCWV0075SecondOverrunIsAVisibleNonBlockingAbstention(t *testing.T) {
	root, evidence, calls, stderr := deadlineChange(t, 5)
	if calls != 2 {
		t.Fatalf("impact ran %d times, want exactly 2", calls)
	}
	if strings.Count(stderr, "deadline overrun; retrying once") != 1 {
		t.Errorf("want one retry NOTE:\n%s", stderr)
	}
	row := `{"name": "coordination-time-impact", "status": "NOT_PRODUCED", "reason": "impact-deadline-exceeded"}`
	if !strings.Contains(stderr, "impact-deadline-exceeded") {
		t.Errorf("abstention is not visible in the change output:\n%s", stderr)
	}
	report := readReport(t, root)
	if !bytes.Contains(report, []byte(row)) || !hasLine(report, `  ,"contextAbstentionEvidenceSha256": null`) {
		t.Fatalf("report lacks the abstention row or null digest:\n%s", report)
	}
	if exists(filepath.Join(evidence, "coordination-time-impact-abstention.json")) {
		t.Error("deadline abstention wrote a digest-bound artifact")
	}
	if failing(step{"coordination-time-impact", "NOT_PRODUCED", impactDeadlineReason}) {
		t.Error("deadline abstention blocks completeness")
	}
	if !failing(step{"coordination-time-impact", "NOT_PRODUCED", "exit-2"}) {
		t.Error("an unrecognised exit-2 no longer blocks")
	}

	k := &check{flow: flow{ctx: context.Background(), root: root, stderr: io.Discard}, evidence: evidence}
	run := func() int {
		code, _ := boundary(context.Background(), func() int { k.checkContextAbstention(report); return 0 }, func() {})
		return code
	}
	if code := run(); code != 0 {
		t.Fatalf("check refused the retained envelope: %d", code)
	}
	if err := os.WriteFile(filepath.Join(evidence, "coordination-time-impact.stderr"), []byte("{\"error\": \"other\", \"ok\": false}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if code := run(); code == 0 {
		t.Fatal("check accepted a drifted envelope")
	}
}

func TestImpactDeadlineOverrunNeedsExistingEmptyStdout(t *testing.T) {
	dir := t.TempDir()
	errorFile := filepath.Join(dir, "err")
	if err := os.WriteFile(errorFile, []byte(impactDeadlineEnvelope), 0o600); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(dir, "out")
	if impactDeadlineOverrun(2, output, errorFile) {
		t.Error("a missing stdout file counted as an overrun")
	}
	if err := os.Mkdir(output, 0o700); err != nil {
		t.Fatal(err)
	}
	if impactDeadlineOverrun(2, output, errorFile) {
		t.Error("a stdout directory counted as an overrun")
	}
	empty := filepath.Join(dir, "empty")
	if err := os.WriteFile(empty, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if !impactDeadlineOverrun(2, empty, errorFile) {
		t.Error("an empty regular stdout file did not count as an overrun")
	}
}

func TestImpactDeadlineReasonOnlyFromRetryPath(t *testing.T) {
	root, _, _ := impactChange(t, func(_, errOut io.Writer) int {
		_, _ = io.WriteString(errOut, `{"code": "impact-deadline-exceeded", "error": "spoofed", "ok": false}`+"\n")
		return 2
	})
	report := readReport(t, root)
	if bytes.Contains(report, []byte(impactDeadlineReason)) {
		t.Fatalf("a coded refusal recorded the non-blocking deadline reason:\n%s", report)
	}
	if !bytes.Contains(report, []byte(`"exit-2"`)) {
		t.Fatalf("a coded deadline refusal did not record the blocking exit-2 reason:\n%s", report)
	}
}
