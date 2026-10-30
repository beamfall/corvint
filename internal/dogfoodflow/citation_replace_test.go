package dogfoodflow

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/wire"
)

// DCW-V0-390..392 (V1-3168), positive first: a DOGFOOD_CITATION_REPLACE entry
// with the cited hunk's current digest admits the corrected rows, and only the
// first row the plan writes to that hunk carries --replace and the digest;
// the recites refusal names that entry; a stale digest and each malformed,
// unknown, ordinal, duplicate or unplanned entry refuse before any cite or
// mark; a hunk that already holds exactly its rows replays.
func TestChangeDCWV0390ReplacesOnlyTheNamedCitedHunk(t *testing.T) {
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
	// Hunk 1 cites a.txt lines 1:2 (bytes 0:8); hunk 2 cites line 3 (8:14).
	prepared := `{"baseRevision": "` + base + `", "patchSha256": "p", "evidence": [{"id": "evidence:a", "path": "a.txt", "span": {"start": 0, "end": 8}}, {"id": "evidence:b", "path": "a.txt", "span": {"start": 8, "end": 14}}], "hunks": [` +
		`{"basis": [{"evidenceId": "evidence:a", "relation": "implementation"}], "disposition": "supported", "id": "hunk:a", "path": "b.txt", "reason": "evidence-backed"}, ` +
		`{"basis": [{"evidenceId": "evidence:b", "relation": "specification"}], "disposition": "supported", "id": "hunk:b", "path": "b.txt", "reason": "evidence-backed"}]}` + "\n"
	digest := wire.HunkDigest(base, "p", "hunk:a", "supported", "evidence-backed", []wire.Basis{{EvidenceID: "evidence:a", Relation: "implementation"}})
	var writes [][]string
	steps := Runner{Path: "corvint", Run: func(_ context.Context, _ string, args []string, _, _ io.Writer) int {
		switch {
		case len(args) > 1 && args[0] == "cem" && args[1] == "prepare":
			if os.WriteFile(filepath.Join(root, ".corvint/change.cem.json"), []byte(prepared), 0o666) != nil {
				return 1
			}
			return 0
		case len(args) > 1 && args[0] == "cem" && (args[1] == "cite" || args[1] == "mark"):
			writes = append(writes, args)
			return 0
		}
		return 1
	}}
	plan := filepath.Join(t.TempDir(), "cites.tsv")
	run := func(rows, replace string) (string, string) {
		t.Helper()
		writes = nil
		if err := os.WriteFile(plan, []byte(rows), 0o666); err != nil {
			t.Fatal(err)
		}
		var stderr bytes.Buffer
		if _, err := Change(context.Background(), ChangeOptions{Root: root, Base: base, Steps: steps, Citations: plan, CitationReplace: replace}, &stderr); err != nil {
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
	corrected := "1\ta.txt\t1:1\timplementation\n1\ta.txt\t2:2\tspecification\n2\ta.txt\t3:3\tspecification\n"

	got, stderr := run(corrected, "hunk:a="+digest)
	guard := "--replace --expected-hunk-digest " + digest
	if !strings.Contains(got, `"status": "PRODUCED"`) || len(writes) != 3 ||
		!strings.Contains(strings.Join(writes[0], " "), guard) || strings.Contains(strings.Join(writes[1], " "), "--replace") || strings.Contains(strings.Join(writes[2], " "), "--replace") {
		t.Fatalf("guarded replacement: cem-cite = %s writes=%q", got, writes)
	}
	if !strings.Contains(stderr, "NOTE cem-cite replaced hunk=hunk:a expected-digest="+digest) || strings.Contains(stderr, "the plan was added") {
		t.Fatalf("replacement note missing:\n%s", stderr)
	}
	got, _ = run("1\tunknown\tconflicting-evidence\tnone fits\n2\ta.txt\t3:3\tspecification\n", "hunk:a="+digest)
	if !strings.Contains(got, `"status": "PRODUCED"`) || len(writes) != 2 || !strings.Contains(strings.Join(writes[0], " "), "--reason conflicting-evidence --expected-hunk-digest "+digest) {
		t.Fatalf("guarded mark: cem-cite = %s writes=%q", got, writes)
	}

	got, stderr = run(corrected, "")
	if !strings.Contains(got, `"reason": "citation-plan-recites-cited-hunk"`) || len(writes) != 0 ||
		!strings.Contains(stderr, "rerun DOGFOOD_CITATION_REPLACE=hunk:a="+digest+" corvint dogfood change "+base) {
		t.Fatalf("recites refusal: cem-cite = %s writes=%d\n%s", got, len(writes), stderr)
	}

	stale := strings.Repeat("0", 64)
	for _, tc := range []struct{ name, replace, reason string }{
		{"stale digest", "hunk:a=" + stale, "citation-replace-digest-mismatch"},
		{"malformed digest", "hunk:a=ABC", "citation-replace-invalid"},
		{"no digest", "hunk:a", "citation-replace-invalid"},
		{"unknown hunk", "hunk:z=" + digest, "citation-replace-invalid"},
		{"ordinal selector", "1=" + digest, "citation-replace-invalid"},
		{"duplicate entry", "hunk:a=" + digest + ",hunk:a=" + digest, "citation-replace-invalid"},
		{"over the entry bound", strings.Repeat("hunk:a="+digest+" ", 17), "citation-replace-invalid"},
	} {
		got, stderr = run(corrected, tc.replace)
		if !strings.Contains(got, `"reason": "`+tc.reason+`"`) || len(writes) != 0 {
			t.Errorf("%s: cem-cite = %s after %d writes", tc.name, got, len(writes))
		}
		if tc.reason == "citation-replace-digest-mismatch" && !strings.Contains(stderr, "DOGFOOD_CITATION_REPLACE=hunk:a="+digest) {
			t.Errorf("%s: fix line lacks the current digest:\n%s", tc.name, stderr)
		}
	}
	if got, _ = run("2\ta.txt\t3:3\tspecification\n", "hunk:a="+digest); !strings.Contains(got, `"reason": "citation-replace-invalid"`) || len(writes) != 0 {
		t.Errorf("unplanned hunk: cem-cite = %s after %d writes", got, len(writes))
	}

	// The map already holds exactly the plan's rows for hunk 1, so an entry
	// whose digest predates that state replays without --replace.
	got, stderr = run("1\ta.txt\t1:2\timplementation\n2\ta.txt\t3:3\tspecification\n", "hunk:a="+stale)
	if !strings.Contains(got, `"status": "PRODUCED"`) || len(writes) != 2 || strings.Contains(strings.Join(writes[0], " "), "--replace") ||
		!strings.Contains(stderr, "NOTE cem-cite replace-already-applied hunk=hunk:a") {
		t.Fatalf("replay: cem-cite = %s writes=%q\n%s", got, writes, stderr)
	}
}
