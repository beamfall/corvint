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

// TestChangeDCWV0883RefusesUniformStagedPlan (V1-3285): a staged plan
// (DCW-V0-037) whose rows cite one span, over a map whose other hunks already
// cite only that span, is refused citation-plan-uniform before any cite, as
// the single-plan route refuses it (DCW-V0-048). A second span, carried
// evidence with another span, or a hunk kept unknown is admitted.
func TestChangeDCWV0883RefusesUniformStagedPlan(t *testing.T) {
	root := t.TempDir()
	testGit(t, root, "init", "-q")
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("base\nline\n"), 0o666); err != nil {
		t.Fatal(err)
	}
	testGit(t, root, "add", "a.txt")
	testGit(t, root, "commit", "-q", "-m", "base")
	base := testGit(t, root, "rev-parse", "HEAD")
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("changed\nline\nmore\n"), 0o666); err != nil {
		t.Fatal(err)
	}
	testGit(t, root, "commit", "-q", "-am", "change")
	prepared, cites := "", 0
	steps := Runner{Path: "corvint", Run: func(_ context.Context, _ string, args []string, _, _ io.Writer) int {
		switch {
		case len(args) > 1 && args[0] == "cem" && args[1] == "prepare":
			if os.WriteFile(filepath.Join(root, ".corvint/change.cem.json"), []byte(prepared), 0o666) != nil {
				return 1
			}
			return 0
		case len(args) > 1 && args[0] == "cem" && args[1] == "cite":
			cites++
			return 0
		}
		return 1
	}}
	// hunk:a is already cited, by the evidence IDs given; b and c are unknown.
	cemMap := func(evidence string, basis ...string) string {
		entries := []string{}
		for _, id := range basis {
			entries = append(entries, `{"evidenceId": "`+id+`", "relation": "specification"}`)
		}
		return `{"evidence": [` + evidence + `], "hunks": [` +
			`{"disposition": "supported", "id": "hunk:a", "path": "a.txt", "reason": "cited", "basis": [` + strings.Join(entries, ", ") + `]}, ` +
			`{"disposition": "unknown", "id": "hunk:b", "path": "a.txt", "reason": "no-evidence"}, ` +
			`{"disposition": "unknown", "id": "hunk:c", "path": "a.txt", "reason": "no-evidence"}]}` + "\n"
	}
	one := `{"id": "e1", "path": "a.txt", "span": {"start": 1, "end": 2}}`
	two := one + `, {"id": "e2", "path": "a.txt", "span": {"start": 2, "end": 2}}`
	plan := filepath.Join(t.TempDir(), "stage-2.tsv")
	run := func(cemPrepared, rows string) (string, string) {
		t.Helper()
		prepared, cites = cemPrepared, 0
		if err := os.WriteFile(plan, []byte(rows), 0o666); err != nil {
			t.Fatal(err)
		}
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
				return strings.TrimSpace(line), stderr.String()
			}
		}
		return "", stderr.String()
	}
	uniform := "hunk:b\ta.txt\t1:2\timplementation\nhunk:c\ta.txt\t01:2\ttest-claim\n"
	got, stderr := run(cemMap(one, "e1"), uniform)
	if !strings.Contains(got, `"reason": "citation-plan-uniform"`) || cites != 0 {
		t.Fatalf("staged uniform plan: cem-cite = %s after %d cites; want citation-plan-uniform before any cite\n%s", got, cites, stderr)
	}
	if !strings.Contains(stderr, "hunks=3 span=a.txt:1:2") {
		t.Errorf("fix line does not name the hunk count and span:\n%s", stderr)
	}
	for name, tc := range map[string]struct{ cemMap, rows string }{
		"second span in the plan":  {cemMap(one, "e1"), "hunk:b\ta.txt\t1:2\timplementation\nhunk:c\ta.txt\t2:2\ttest-claim\n"},
		"carried hunk cites two":   {cemMap(two, "e1", "e2"), uniform},
		"carried hunk cites other": {cemMap(two, "e2"), uniform},
		"a hunk stays unknown":     {cemMap(one, "e1"), "hunk:b\ta.txt\t1:2\timplementation\nhunk:c\tunknown\tinsufficient-evidence\tlater\n"},
	} {
		if got, stderr := run(tc.cemMap, tc.rows); strings.Contains(got, "citation-plan-uniform") || cites == 0 {
			t.Errorf("%s: cem-cite = %s after %d cites; want the plan cited\n%s", name, got, cites, stderr)
		}
	}
}
