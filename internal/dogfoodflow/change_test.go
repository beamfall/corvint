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

// V1-0239: rerunning the same ordinal plan after a later commit reordered the
// map's hunks keeps its row count but would cite each row against another hunk.
func TestChangeRefusesAPlanWhoseOrdinalsMoved(t *testing.T) {
	root := t.TempDir()
	testGit(t, root, "init", "-q")
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("base\n"), 0o666); err != nil {
		t.Fatal(err)
	}
	testGit(t, root, "add", "a.txt")
	testGit(t, root, "commit", "-q", "-m", "base")
	base := testGit(t, root, "rev-parse", "HEAD")
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("changed\n"), 0o666); err != nil {
		t.Fatal(err)
	}
	testGit(t, root, "commit", "-q", "-am", "change")
	plan := filepath.Join(t.TempDir(), "cites.tsv")
	if err := os.WriteFile(plan, []byte("1\ta.txt\t1:1\tspecification\n2\ta.txt\t1:1\tspecification\n"), 0o666); err != nil {
		t.Fatal(err)
	}
	cemMap := func(ids ...string) string {
		hunks := []string{}
		for _, id := range ids {
			hunks = append(hunks, "    {\n      \"disposition\": \"unknown\",\n      \"id\": \""+id+"\",\n      \"path\": \"a.txt\"\n    }")
		}
		return "{\n  \"hunks\": [\n" + strings.Join(hunks, ",\n") + "\n  ]\n}\n"
	}
	cemPrepared := ""
	steps := Runner{Path: "corvint", Run: func(_ context.Context, _ string, args []string, _, _ io.Writer) int {
		switch {
		case len(args) > 1 && args[0] == "cem" && args[1] == "prepare":
			if err := os.MkdirAll(filepath.Join(root, ".corvint"), 0o777); err != nil {
				return 1
			}
			if err := os.WriteFile(filepath.Join(root, ".corvint/change.cem.json"), []byte(cemPrepared), 0o666); err != nil {
				return 1
			}
			return 0
		case len(args) > 1 && args[0] == "cem" && args[1] == "cite":
			return 0
		}
		return 1
	}}
	citeRow := func(ids ...string) string {
		cemPrepared = cemMap(ids...)
		var stderr bytes.Buffer
		if _, err := Change(context.Background(), ChangeOptions{Root: root, Base: base, Steps: steps, Citations: plan}, &stderr); err != nil {
			t.Fatal(err)
		}
		report, err := os.ReadFile(filepath.Join(root, ".corvint/dogfood-report.json"))
		if err != nil {
			t.Fatal(err)
		}
		for _, line := range strings.Split(string(report), "\n") {
			if strings.Contains(line, `"name": "cem-cite"`) {
				return strings.TrimSpace(line)
			}
		}
		t.Fatalf("report has no cem-cite row:\n%s", report)
		return ""
	}
	if got := citeRow("hunk:a", "hunk:b"); !strings.Contains(got, `"status": "PRODUCED"`) {
		t.Fatalf("first pass cem-cite = %s; want PRODUCED", got)
	}
	if got := citeRow("hunk:b", "hunk:a"); !strings.Contains(got, `"reason": "citation-plan-map-mismatch"`) {
		t.Errorf("rerun after the hunks swapped: cem-cite = %s; want citation-plan-map-mismatch", got)
	}
}
