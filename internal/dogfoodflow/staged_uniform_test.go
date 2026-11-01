package dogfoodflow

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	cemcli "github.com/Beamfall/corvint/internal/cem/cli"
)

// TestChangeDCWV0883RefusesUniformStagedPlan (V1-3285): a staged plan
// (DCW-V0-037) whose rows cite one span, over a map whose other hunks already
// cite only that span, is refused citation-plan-uniform before any cite, as
// the single-plan route refuses it (DCW-V0-048). A second span, carried
// evidence with another span, or a hunk kept unknown is admitted. The maps
// are real cem prepare and cite output, whose evidence spans are BASE bytes
// while the plan's spans are BASE lines.
func TestChangeDCWV0883RefusesUniformStagedPlan(t *testing.T) {
	root, base := preflightRepository(t, map[string]string{"b.txt": "added\n"})
	target := testGit(t, root, "rev-parse", "HEAD")
	mapPath := filepath.Join(root, ".corvint/change.cem.json")
	// produce prepares the real map and cites its hunk 1 at each line span.
	produce := func(lines ...string) string {
		t.Helper()
		cem := func(args ...string) {
			t.Helper()
			var stdout, stderr bytes.Buffer
			if code := cemcli.Run(context.Background(), root, args, &stdout, &stderr); code != 0 {
				t.Fatalf("cem %q = %d\n%s%s", args, code, &stdout, &stderr)
			}
		}
		cem("prepare", "--base", base, "--target", target)
		for _, span := range lines {
			cem("cite", "--map", ".corvint/change.cem.json", "--hunk", "1", "--evidence-path", "pre.go", "--lines", span, "--relation", "specification")
		}
		data, err := os.ReadFile(mapPath)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Remove(mapPath); err != nil {
			t.Fatal(err)
		}
		if strings.Count(string(data), `"id":"hunk:`) != 3 {
			t.Fatalf("the prepared map does not hold three hunks:\n%s", data)
		}
		return string(data)
	}
	one, two, other := produce("3:3"), produce("3:3", "4:4"), produce("4:4")
	prepared, cites := "", 0
	steps := Runner{Path: "corvint", Run: func(_ context.Context, _ string, args []string, _, _ io.Writer) int {
		switch {
		case len(args) > 1 && args[0] == "cem" && args[1] == "prepare":
			if os.WriteFile(mapPath, []byte(prepared), 0o666) != nil {
				return 1
			}
			return 0
		case len(args) > 1 && args[0] == "cem" && args[1] == "cite":
			cites++
			return 0
		}
		return 1
	}}
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
	uniform := "2\tpre.go\t3:3\timplementation\n3\tpre.go\t03:3\ttest-claim\n"
	got, stderr := run(one, uniform)
	if !strings.Contains(got, `"reason": "citation-plan-uniform"`) || cites != 0 {
		t.Fatalf("staged uniform plan: cem-cite = %s after %d cites; want citation-plan-uniform before any cite\n%s", got, cites, stderr)
	}
	if !strings.Contains(stderr, "hunks=3 span=pre.go:3:3") {
		t.Errorf("fix line does not name the hunk count and span:\n%s", stderr)
	}
	for name, tc := range map[string]struct{ cemMap, rows string }{
		"second span in the plan":  {one, "2\tpre.go\t3:3\timplementation\n3\tpre.go\t4:4\ttest-claim\n"},
		"carried hunk cites two":   {two, uniform},
		"carried hunk cites other": {other, uniform},
		"a hunk stays unknown":     {one, "2\tpre.go\t3:3\timplementation\n3\tunknown\tinsufficient-evidence\tlater\n"},
	} {
		if got, stderr := run(tc.cemMap, tc.rows); strings.Contains(got, "citation-plan-uniform") || cites == 0 {
			t.Errorf("%s: cem-cite = %s after %d cites; want the plan cited\n%s", name, got, cites, stderr)
		}
	}
}
