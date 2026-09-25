package dogfoodflow

import (
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func testGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", root}, args...)...)
	command.Env = append(os.Environ(), "LC_ALL=C", "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.invalid", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.invalid")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
	return strings.TrimSpace(string(output))
}

// TestChangeKeepsAgentPrechangeReceipts reproduces DCW-V0-026: the agent's
// pre-change receipts written before the change (docs/DOGFOOD.md section 1)
// survive a coordinator run, and the coordinator's own query and impact runs,
// made after the change, are never reported under a pre-change name.
func TestChangeKeepsAgentPrechangeReceipts(t *testing.T) {
	root := t.TempDir()
	testGit(t, root, "init", "-q")
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("base\n"), 0o666); err != nil {
		t.Fatal(err)
	}
	testGit(t, root, "add", "a.txt")
	testGit(t, root, "commit", "-q", "-m", "base")
	base := testGit(t, root, "rev-parse", "HEAD")
	evidence := filepath.Join(testGit(t, root, "rev-parse", "--absolute-git-dir"), "corvint")
	if err := os.MkdirAll(evidence, 0o777); err != nil {
		t.Fatal(err)
	}
	receipts := map[string][]byte{
		"prechange-query.json":  []byte("agent base-tree query receipt\n"),
		"prechange-impact.json": []byte("agent base-tree impact receipt\n"),
	}
	for name, data := range receipts {
		if err := os.WriteFile(filepath.Join(evidence, name), data, 0o666); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("changed\n"), 0o666); err != nil {
		t.Fatal(err)
	}
	testGit(t, root, "commit", "-q", "-am", "change")
	steps := Runner{Path: "corvint", Run: func(_ context.Context, _ string, args []string, stdout, _ io.Writer) int {
		if args[0] == "query" || args[0] == "impact" {
			_, _ = io.WriteString(stdout, "coordination-time "+args[0]+" output\n")
			return 0
		}
		return 1
	}}
	var stderr bytes.Buffer
	if _, err := Change(context.Background(), ChangeOptions{Root: root, Base: base, Steps: steps}, &stderr); err != nil {
		t.Fatal(err)
	}
	for name, want := range receipts {
		got, err := os.ReadFile(filepath.Join(evidence, name))
		if err != nil || !bytes.Equal(got, want) {
			t.Errorf("%s = %q, %v; want the agent receipt %q", name, got, err, want)
		}
	}
	for _, name := range []string{"coordination-time-query.json", "coordination-time-impact.json"} {
		if got, err := os.ReadFile(filepath.Join(evidence, name)); err != nil || !bytes.HasPrefix(got, []byte("coordination-time ")) {
			t.Errorf("%s = %q, %v; want the coordinator's output", name, got, err)
		}
	}
	report, err := os.ReadFile(filepath.Join(root, ".corvint/dogfood-report.json"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(report, []byte("prechange")) {
		t.Errorf("report labels a coordination-time run as pre-change:\n%s", report)
	}
	for _, want := range []string{`{"name": "coordination-time-query", "status": "PRODUCED", "reason": "none"}`, `{"name": "coordination-time-impact", "status": "PRODUCED", "reason": "none"}`} {
		if !bytes.Contains(report, []byte(want)) {
			t.Errorf("report lacks %s:\n%s", want, report)
		}
	}
}
