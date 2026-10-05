package dogfoodflow

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/Beamfall/corvint/internal/dogfoodoperation"
	"github.com/Beamfall/corvint/internal/tracerecordrepo"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
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

// V1-0316: dogfood change names an agent pre-change receipt that is absent, has
// no tree, or was written against another tree than the base; none blocks.
func TestChangeNotesAbsentOrStaleAgentReceipts(t *testing.T) {
	root := t.TempDir()
	testGit(t, root, "init", "-q")
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("base\n"), 0o666); err != nil {
		t.Fatal(err)
	}
	testGit(t, root, "add", "a.txt")
	testGit(t, root, "commit", "-q", "-m", "base")
	base := testGit(t, root, "rev-parse", "HEAD")
	baseTree := testGit(t, root, "rev-parse", "HEAD^{tree}")
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("changed\n"), 0o666); err != nil {
		t.Fatal(err)
	}
	testGit(t, root, "commit", "-q", "-am", "change")
	headTree := testGit(t, root, "rev-parse", "HEAD^{tree}")
	evidence := filepath.Join(testGit(t, root, "rev-parse", "--absolute-git-dir"), "corvint")
	if err := os.MkdirAll(evidence, 0o777); err != nil {
		t.Fatal(err)
	}
	steps := Runner{Path: "corvint", Run: func(context.Context, string, []string, io.Writer, io.Writer) int { return 1 }}
	receipt := func(tree string) string { return `{"context":{"revision":"` + tree + `"}}` + "\n" }
	cases := []struct {
		name, query, impact string
		want, absent        []string
	}{
		{"absent", "", "", []string{
			"dogfood-change: NOTE prechange-query NOT_OBSERVED agent-receipt-absent",
			"dogfood-change: NOTE prechange-impact NOT_OBSERVED agent-receipt-absent",
		}, nil},
		{"stale and unknown", receipt(headTree), `{"code": "invalid-arguments", "ok": false}` + "\n", []string{
			"dogfood-change: NOTE prechange-query STALE agent-receipt-not-base-tree tree=" + headTree + " base-tree=" + baseTree,
			"dogfood-change: NOTE prechange-impact NOT_OBSERVED agent-receipt-tree-unknown",
		}, nil},
		// V1-0743: a receipt that is not JSON, or exceeds the 4 MiB bound, is
		// named apart from a well-formed receipt that carries no tree.
		{"malformed and over bound", "{\"context\":\n", "{}" + strings.Repeat(" ", 4194304) + "\n", []string{
			"dogfood-change: NOTE prechange-query NOT_OBSERVED agent-receipt-malformed",
			"dogfood-change: NOTE prechange-impact NOT_OBSERVED agent-receipt-over-bound",
		}, []string{"agent-receipt-tree-unknown"}},
		{"base tree", receipt(baseTree), receipt(baseTree), nil, []string{"prechange-query", "prechange-impact"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for name, data := range map[string]string{"prechange-query.json": tc.query, "prechange-impact.json": tc.impact} {
				path := filepath.Join(evidence, name)
				_ = os.Remove(path)
				if data == "" {
					continue
				}
				if err := os.WriteFile(path, []byte(data), 0o666); err != nil {
					t.Fatal(err)
				}
			}
			var stderr bytes.Buffer
			if _, err := Change(context.Background(), ChangeOptions{Root: root, Base: base, Steps: steps}, &stderr); err != nil {
				t.Fatal(err)
			}
			lines := strings.Split(stderr.String(), "\n")
			for _, want := range tc.want {
				if !slices.Contains(lines, want) {
					t.Errorf("stderr lacks %q:\n%s", want, stderr.String())
				}
			}
			for _, name := range tc.absent {
				if strings.Contains(stderr.String(), "NOTE "+name+" ") || strings.Contains(stderr.String(), " "+name+"\n") {
					t.Errorf("stderr names %s:\n%s", name, stderr.String())
				}
			}
		})
	}
}

// V1-0743: an empty citation plan on a map that still owes a hunk, and an
// intents or verify file over its bound, each refuse with a named code instead
// of a silent no-op or an unbounded read; the empty plan cites nothing.
func TestChangeRefusesEmptyPlanAndOverBoundInputs(t *testing.T) {
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
	inputs := t.TempDir()
	write := func(name string, data []byte) string {
		path := filepath.Join(inputs, name)
		if err := os.WriteFile(path, data, 0o666); err != nil {
			t.Fatal(err)
		}
		return path
	}
	cited := false
	prepared := "{\n  \"hunks\": [\n    {\n      \"disposition\": \"unknown\",\n      \"id\": \"hunk:a\",\n      \"path\": \"a.txt\"\n    }\n  ]\n}\n"
	steps := Runner{Path: "corvint", Run: func(_ context.Context, _ string, args []string, _, _ io.Writer) int {
		switch {
		case len(args) > 1 && args[0] == "cem" && args[1] == "prepare":
			if err := os.MkdirAll(filepath.Join(root, ".corvint"), 0o777); err != nil {
				return 1
			}
			if err := os.WriteFile(filepath.Join(root, ".corvint/change.cem.json"), []byte(prepared), 0o666); err != nil {
				return 1
			}
			return 0
		case len(args) > 1 && args[0] == "cem" && args[1] == "cite":
			cited = true
			return 0
		}
		return 1
	}}
	options := ChangeOptions{
		Root: root, Base: base, Steps: steps, Outcome: "passed",
		Citations:   write("cites.tsv", nil),
		IntentsFile: write("intents", bytes.Repeat([]byte("a\n"), maxIntentManifestBytes/2+1)),
		VerifyFile:  write("verify", bytes.Repeat([]byte("\n"), maxVerifyFileBytes+1)),
	}
	var stderr bytes.Buffer
	if _, err := Change(context.Background(), options, &stderr); err != nil {
		t.Fatal(err)
	}
	report, err := os.ReadFile(filepath.Join(root, ".corvint/dogfood-report.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`{"name": "cem-cite", "status": "NOT_PRODUCED", "reason": "empty-citation-plan"}`,
		`{"name": "ocm-aggregate", "status": "NOT_PRODUCED", "reason": "intent-manifest-over-bound"}`,
		`{"name": "local-outcome", "status": "NOT_PRODUCED", "reason": "verify-file-over-bound"}`,
	} {
		if !bytes.Contains(report, []byte(want)) {
			t.Errorf("report lacks %s:\n%s", want, report)
		}
	}
	for _, want := range []string{"DOGFOOD_CITATIONS names an empty file", "DOGFOOD_INTENTS_FILE is larger than", "DOGFOOD_VERIFY_FILE is larger than"} {
		if !strings.Contains(stderr.String(), want) {
			t.Errorf("stderr lacks the fix %q:\n%s", want, stderr.String())
		}
	}
	if cited {
		t.Error("an empty plan ran cem cite")
	}
	// A map that owes no hunk, as local completion reruns it after strict CEM
	// status (LCP-V0), still admits the empty plan as a zero-citation pass, and
	// inputs of exactly their bound are not over it.
	prepared = strings.Replace(prepared, `"unknown"`, `"cited"`, 1)
	options.IntentsFile = write("intents", bytes.Repeat([]byte("a\n"), maxIntentManifestBytes/2))
	options.VerifyFile = write("verify", bytes.Repeat([]byte("\n"), maxVerifyFileBytes))
	if _, err := Change(context.Background(), options, &stderr); err != nil {
		t.Fatal(err)
	}
	if report, err = os.ReadFile(filepath.Join(root, ".corvint/dogfood-report.json")); err != nil {
		t.Fatal(err)
	}
	if want := `{"name": "cem-cite", "status": "PRODUCED", "reason": "none"}`; !bytes.Contains(report, []byte(want)) {
		t.Errorf("empty plan on a bound map: report lacks %s:\n%s", want, report)
	}
	for _, refused := range []string{"intent-manifest-over-bound", "verify-file-over-bound"} {
		if bytes.Contains(report, []byte(refused)) {
			t.Errorf("an input of exactly its bound reported %s:\n%s", refused, report)
		}
	}
}

// A compact map names the same hunks as the indent-2 map cem prepare writes, so
// a citation plan binds to either layout.
func TestMapHunksReadsAnyJSONLayout(t *testing.T) {
	cemMap := map[string]any{"hunks": []map[string]any{
		{"basis": []any{}, "disposition": "unknown", "id": "hunk:a", "path": "docs/a&b.md", "reason": "no-evidence"},
		{"basis": []any{}, "disposition": "cited", "id": "hunk:b", "path": "b.txt", "reason": "cited"},
	}}
	indented, err := json.MarshalIndent(cemMap, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	compact, err := json.Marshal(cemMap)
	if err != nil {
		t.Fatal(err)
	}
	want := []map[string]string{
		{"disposition": "unknown", "id": "hunk:a", "path": "docs/a&b.md"},
		{"disposition": "cited", "id": "hunk:b", "path": "b.txt"},
	}
	for name, data := range map[string][]byte{"indent-2": indented, "compact": compact} {
		if got := mapHunks(data); !reflect.DeepEqual(got, want) {
			t.Errorf("%s map: mapHunks = %v; want %v", name, got, want)
		}
	}
	if got := mapHunks([]byte(`{"hunks": [`)); len(got) != 0 {
		t.Errorf("truncated map: mapHunks = %v; want no hunks", got)
	}
}

func TestAggregateWritersShareOperationLockBeforeCleanup(t *testing.T) {
	root := t.TempDir()
	testGit(t, root, "init", "-q")
	testGit(t, root, "commit", "--allow-empty", "-qm", "base")
	base := testGit(t, root, "rev-parse", "HEAD")
	gitDir := testGit(t, root, "rev-parse", "--absolute-git-dir")
	_, unlock, err := dogfoodoperation.Acquire(context.Background(), gitDir)
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()
	sentinel := filepath.Join(gitDir, "corvint", "local-outcome.stderr")
	if err := os.WriteFile(sentinel, []byte("preserve failure"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"change", "check", "seal"} {
		t.Run(mode, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			var code int
			var err error
			switch mode {
			case "change":
				code, err = Change(context.Background(), ChangeOptions{Root: root, Base: base}, &stderr)
			case "check":
				code, err = Check(context.Background(), CheckOptions{Root: root, Base: base}, &stdout, &stderr)
			case "seal":
				code, err = Seal(context.Background(), CheckOptions{Root: root, Base: base}, &stdout, &stderr)
			}
			if err != nil || code != 2 || !strings.Contains(stderr.String(), "operation-in-progress") {
				t.Fatalf("code=%d err=%v stderr=%s", code, err, stderr.String())
			}
			if raw, err := os.ReadFile(sentinel); err != nil || string(raw) != "preserve failure" {
				t.Fatalf("prior evidence changed: %s %v", raw, err)
			}
		})
	}
}

func TestAggregateReportClosedVersionAndWhitespace(t *testing.T) {
	root := t.TempDir()
	testGit(t, root, "init", "-q")
	testGit(t, root, "commit", "--allow-empty", "-qm", "base")
	base := testGit(t, root, "rev-parse", "HEAD")
	for i := 0; i < 201; i++ {
		if err := os.WriteFile(filepath.Join(root, fmt.Sprintf("f%03d.go", i)), []byte(fmt.Sprintf("package fixture\nconst V%d = %d\n", i, i)), 0600); err != nil {
			t.Fatal(err)
		}
	}
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-qm", "source")
	plan := strings.Repeat("a", 64)
	expected := tracerecordrepo.AggregateExpected{ObjectFormat: "sha1", Base: base, Target: testGit(t, root, "rev-parse", "HEAD"), Tree: testGit(t, root, "rev-parse", "HEAD^{tree}"), AdmissionPolicy: tracerecordrepo.AggregateAdmissionPolicy, Task: "Local completion " + plan, Verification: []string{"go test ./..."}, Outcome: "passed"}
	receipt, err := tracerecordrepo.ProduceAggregateOutcome(context.Background(), root, expected)
	if err != nil {
		t.Fatal(err)
	}
	outcome, err := tracerecordrepo.ParseAggregateOutcome(receipt)
	if err != nil {
		t.Fatal(err)
	}
	binding, err := tracerecordrepo.AggregateBinding(outcome)
	if err != nil {
		t.Fatal(err)
	}
	old := map[string]any{"profile": "corvint-dogfood-change/0", "base": base, "target": expected.Target, "complete": false, "steps": []AggregateReportStep{{"local-outcome", "NOT_PRODUCED", "admitted-path-limit"}, {"coordination-time-query", "PRODUCED", "none"}}, "ocmStatus": nil, "localOutcomeEvidenceSha256": nil, "contextAbstentionEvidenceSha256": nil, "queryAbstentionEvidenceSha256": nil, "anchor": AggregateReportAnchor{State: "NOT_OBSERVED"}, "ocmLinkPlan": nil, "dogfoodPolicy": AggregateReportPolicy{}, "packetCoverage": []any{}, "dogfoodCheck": nil}
	oldRaw, _ := json.Marshal(old)
	raw, err := PrepareAggregateReport(oldRaw, AggregateEnrollment{strings.Repeat("b", 64), plan + "-001", plan, binding}, receipt)
	if err != nil {
		t.Fatal(err)
	}
	value, err := ParseAggregateReport(raw)
	if err != nil || value.CompletionState != "complete" {
		t.Fatal(value.CompletionState, err)
	}
	var members map[string]any
	if err := json.Unmarshal(raw, &members); err != nil {
		t.Fatal(err)
	}
	if _, ok := members["complete"]; ok {
		t.Fatal("legacy completeness leaked")
	}
	reformatted, _ := json.MarshalIndent(members, "", "  ")
	if _, err := ParseAggregateReport(reformatted); err != nil {
		t.Fatal("structural reader rejected whitespace", err)
	}
	for _, test := range []struct {
		name   string
		mutate func(map[string]any)
	}{
		{"hybrid", func(m map[string]any) { m["complete"] = true }},
		{"downgrade", func(m map[string]any) { m["profile"] = "corvint-dogfood-change/0" }},
		{"unknown", func(m map[string]any) { m["futureAuthority"] = true }},
		{"missing", func(m map[string]any) { delete(m, "completionState") }},
		{"null-state", func(m map[string]any) { m["completionState"] = nil }},
		{"unbound-enrollment", func(m map[string]any) { m["enrollment"].(map[string]any)["extra"] = true }},
	} {
		t.Run(test.name, func(t *testing.T) {
			var m map[string]any
			_ = json.Unmarshal(raw, &m)
			test.mutate(m)
			changed, _ := json.Marshal(m)
			if _, err := ParseAggregateReport(changed); err == nil {
				t.Fatal("mutation accepted")
			}
		})
	}
}
