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

// DCW-V0-117 (V1-2267): a rerun over a map that already cites a hunk refuses,
// before any cite or mark runs, a row that would add a different citation to
// that hunk or mark it unknown; replaying the same plan and a staged plan that
// names only hunks with an empty basis, unknown or mechanical, still cite.
func TestChangeDCWV0117RefusesARowThatRecitesACitedHunk(t *testing.T) {
	root := t.TempDir()
	testGit(t, root, "init", "-q")
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("one\ntwo\nthree\n"), 0o666); err != nil {
		t.Fatal(err)
	}
	testGit(t, root, "add", "a.txt")
	testGit(t, root, "commit", "-q", "-m", "base")
	base := testGit(t, root, "rev-parse", "HEAD")
	if err := os.WriteFile(filepath.Join(root, "b.txt"), []byte("changed\n"), 0o666); err != nil {
		t.Fatal(err)
	}
	testGit(t, root, "add", "b.txt")
	testGit(t, root, "commit", "-q", "-m", "change")
	// Hunk 1 cites a.txt lines 1:2 (bytes 0:8) as implementation; hunk 2 is
	// still unknown, as after a first staged plan; hunk 3 is mechanical, so
	// the map does not cite it either.
	prepared := `{"evidence": [{"id": "evidence:a", "path": "a.txt", "span": {"start": 0, "end": 8}}], "hunks": [` +
		`{"basis": [{"evidenceId": "evidence:a", "relation": "implementation"}], "disposition": "supported", "id": "hunk:a", "path": "b.txt", "reason": "evidence-backed"}, ` +
		`{"basis": [], "disposition": "unknown", "id": "hunk:b", "path": "b.txt", "reason": "no-evidence"}, ` +
		`{"basis": [], "disposition": "mechanical", "id": "hunk:c", "path": "b.txt", "reason": "formatting-only"}]}` + "\n"
	writes := 0
	steps := Runner{Path: "corvint", Run: func(_ context.Context, _ string, args []string, _, _ io.Writer) int {
		switch {
		case len(args) > 1 && args[0] == "cem" && args[1] == "prepare":
			if os.WriteFile(filepath.Join(root, ".corvint/change.cem.json"), []byte(prepared), 0o666) != nil {
				return 1
			}
			return 0
		case len(args) > 1 && args[0] == "cem" && (args[1] == "cite" || args[1] == "mark"):
			writes++
			return 0
		}
		return 1
	}}
	plan := filepath.Join(t.TempDir(), "cites.tsv")
	run := func(rows string) (string, string) {
		t.Helper()
		writes = 0
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
		t.Fatalf("report has no cem-cite row:\n%s", report)
		return "", ""
	}
	const refused = `"reason": "citation-plan-recites-cited-hunk"`
	for _, tc := range []struct {
		name, rows string
		refuse     bool
	}{
		{"replay", "1\ta.txt\t1:2\timplementation\n2\ta.txt\t3:3\tspecification\n", false},
		{"replay by hunk ID", "hunk:a\ta.txt\t1:2\timplementation\nhunk:b\ta.txt\t3:3\tspecification\n", false},
		{"staged plan names only the unknown hunk", "2\ta.txt\t3:3\tspecification\n", false},
		{"citation for a mechanical hunk", "2\ta.txt\t3:3\tspecification\n3\ta.txt\t1:1\tdependency\n", false},
		{"corrected span", "1\ta.txt\t1:1\timplementation\n2\ta.txt\t3:3\tspecification\n", true},
		{"corrected relation", "2\ta.txt\t3:3\tspecification\n1\ta.txt\t1:2\tspecification\n", true},
		{"unknown row for a supported hunk", "1\tunknown\tinsufficient-evidence\tnone fits\n2\ta.txt\t3:3\tspecification\n", true},
	} {
		got, stderr := run(tc.rows)
		if tc.refuse != strings.Contains(got, refused) || tc.refuse != (writes == 0) {
			t.Errorf("%s: cem-cite = %s after %d writes; refuse=%v", tc.name, got, writes, tc.refuse)
		}
		if tc.refuse && !strings.Contains(stderr, "would change hunk hunk:a, which .corvint/change.cem.json already cites") {
			t.Errorf("%s: fix line missing:\n%s", tc.name, stderr)
		}
	}
}
