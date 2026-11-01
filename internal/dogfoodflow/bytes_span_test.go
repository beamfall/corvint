package dogfoodflow

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// DCW-V0-990 (V1-3637): a plan row's third field may be bytes:START:END, the
// byte span the CEM records, as dogfood-bind-range.sh already accepts. The
// fixture's line span 1:2 is bytes 0:8, so line and byte coordinates differ:
// the row is cited with --bytes, a replay holds the cited hunk, and a byte span
// the map does not hold is refused as a recite before any cite runs.
func TestChangeDCWV0990PlanRowsAcceptByteSpans(t *testing.T) {
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
	prepared := `{"evidence": [{"id": "evidence:a", "path": "a.txt", "span": {"start": 0, "end": 8}}], "hunks": [` +
		`{"basis": [{"evidenceId": "evidence:a", "relation": "implementation"}], "disposition": "supported", "id": "hunk:a", "path": "b.txt", "reason": "evidence-backed"}, ` +
		`{"basis": [], "disposition": "unknown", "id": "hunk:b", "path": "b.txt", "reason": "no-evidence"}]}` + "\n"
	var cites [][]string
	steps := Runner{Path: "corvint", Run: func(_ context.Context, _ string, args []string, _, _ io.Writer) int {
		switch {
		case len(args) > 1 && args[0] == "cem" && args[1] == "prepare":
			if os.WriteFile(filepath.Join(root, ".corvint/change.cem.json"), []byte(prepared), 0o666) != nil {
				return 1
			}
			return 0
		case len(args) > 1 && args[0] == "cem" && args[1] == "cite":
			cites = append(cites, slices.Clone(args))
			return 0
		}
		return 1
	}}
	plan := filepath.Join(t.TempDir(), "cites.tsv")
	run := func(rows string) string {
		t.Helper()
		cites = nil
		if err := os.WriteFile(plan, []byte(rows), 0o666); err != nil {
			t.Fatal(err)
		}
		if _, err := Change(context.Background(), ChangeOptions{Root: root, Base: base, Steps: steps, Citations: plan}, &bytes.Buffer{}); err != nil {
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
	if got := run("1\ta.txt\tbytes:0:8\timplementation\n2\ta.txt\tbytes:8:12\tspecification\n"); strings.Contains(got, "citation-plan-recites-cited-hunk") || len(cites) != 2 {
		t.Fatalf("byte-span replay: %s after %d cites", got, len(cites))
	}
	for index, want := range [][]string{{"--bytes", "0:8"}, {"--bytes", "8:12"}} {
		at := slices.Index(cites[index], "--bytes")
		if at < 0 || !slices.Equal(cites[index][at:at+2], want) || slices.Contains(cites[index], "--lines") {
			t.Errorf("cite %d args %q, want %q and no --lines", index+1, cites[index], want)
		}
	}
	// The line span that names the same bytes still cites with --lines.
	if run("1\ta.txt\t1:2\timplementation\n2\ta.txt\t3:3\tspecification\n"); len(cites) != 2 || !slices.Contains(cites[0], "--lines") {
		t.Fatalf("line-span plan: %q", cites)
	}
	// bytes 0:4 is not the cited 0:8, though it starts on the same line.
	if got := run("1\ta.txt\tbytes:0:4\timplementation\n2\ta.txt\tbytes:8:12\tspecification\n"); !strings.Contains(got, "citation-plan-recites-cited-hunk") || len(cites) != 0 {
		t.Fatalf("changed byte span: %s after %d cites", got, len(cites))
	}
}

// DCW-V0-990: a byte span and a line span are distinct uniform-citation spans.
func TestDCWV0990CanonicalSpanKeepsByteKind(t *testing.T) {
	lines, _ := canonicalSpan("a.txt", "1:2")
	byteSpan, ok := canonicalSpan("a.txt", "bytes:1:2")
	if !ok || lines == byteSpan || byteSpan != "a.txt:bytes:1:2" {
		t.Fatalf("canonical spans %q %q %v", lines, byteSpan, ok)
	}
}
