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
	command := exec.Command("git", append([]string{"-c", "maintenance.auto=false", "-c", "gc.auto=0", "-C", root}, args...)...)
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
// DCW-V0-066 (V1-1397): the refusal's fix line names the stale binding, the
// moved hunk and both remedies, not the generic mismatch causes.
func TestChangeRefusesAPlanWhoseOrdinalsMoved(t *testing.T) {
	root := t.TempDir()
	testGit(t, root, "init", "-q")
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("base\n"), 0o666); err != nil {
		t.Fatal(err)
	}
	// Each hunk cites its own span, so the plan is not uniform (DCW-V0-048).
	if err := os.WriteFile(filepath.Join(root, "b.txt"), []byte("base\n"), 0o666); err != nil {
		t.Fatal(err)
	}
	testGit(t, root, "add", "a.txt", "b.txt")
	testGit(t, root, "commit", "-q", "-m", "base")
	base := testGit(t, root, "rev-parse", "HEAD")
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("changed\n"), 0o666); err != nil {
		t.Fatal(err)
	}
	testGit(t, root, "commit", "-q", "-am", "change")
	// Row 1 cites ordinal 2, so the fix line must tell the plan row from the
	// ordinal it names.
	plan := filepath.Join(t.TempDir(), "cites.tsv")
	if err := os.WriteFile(plan, []byte("2\tb.txt\t1:1\tspecification\n1\ta.txt\t1:1\tspecification\n"), 0o666); err != nil {
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
	var stderr bytes.Buffer
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
		stderr.Reset()
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
	binding := filepath.Join(testGit(t, root, "rev-parse", "--absolute-git-dir"), "corvint", "citation-plan-binding")
	for _, want := range []string{
		"fix: stale binding: " + binding + " was recorded when these same plan bytes were cited against an earlier map, and plan row 1 (ordinal 2) named hunk hunk:b, which the current map holds at ordinal 1;",
		"rewrite it from the current .corvint/change.cem.json",
		"remove " + binding + " and rerun (DCW-V0-066)",
	} {
		if !strings.Contains(stderr.String(), want) {
			t.Errorf("stale-binding stderr lacks %q:\n%s", want, stderr.String())
		}
	}
	if strings.Contains(stderr.String(), "a row names an ordinal past its hunks") {
		t.Errorf("stale binding reported with the generic mismatch fix line:\n%s", stderr.String())
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
		{"disposition": "unknown", "id": "hunk:a", "path": "docs/a&b.md", "reason": "no-evidence"},
		{"disposition": "cited", "id": "hunk:b", "path": "b.txt", "reason": "cited"},
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

// DCW-V0-041: a refused citation row names the plan row and what a row may
// cite: any regular file at BASE, with one of the registered relations.
func TestChangeDCWV0041NamesCitationRowFix(t *testing.T) {
	c := &change{citationFailedRow: 3}
	for reason, want := range map[string]string{
		"invalid-arguments": "plan row 3: RELATION must be one of call-site, decision, dependency, implementation, incident, specification or test-claim",
		"missing-evidence":  "plan row 3 cites a path that is not a regular file at BASE; cite a source, test or spec file",
		// V1-1385: CEM-V1-114 refuses citing authority the change modifies.
		"cite-self-modified-authority": "plan row 3 cites a governing file (spec, decision or other authority) that this change itself modifies, which cannot govern it; cite a governing source the change leaves unmodified, or keep the hunk unknown",
	} {
		if hint := c.fixHint(step{name: "cem-cite", reason: reason}); !strings.Contains(hint, want) {
			t.Fatalf("%s hint %q", reason, hint)
		}
	}
	// V1-1147: a refused row names the rejected RELATION and span values.
	c.citationFailedValues = `RELATION "test" and START:END "1:2"`
	want := `plan row 3 (RELATION "test" and START:END "1:2"): RELATION must be one of call-site, decision, dependency, implementation, incident, specification or test-claim`
	if hint := c.fixHint(step{name: "cem-cite", reason: "invalid-arguments"}); !strings.Contains(hint, want) {
		t.Fatalf("invalid-arguments hint %q", hint)
	}
}

// DCW-V0-036: the public recorder code retains the internal aggregate fallback classification.
func TestLocalOutcomeTracePathLimitKeepsAdmissionReason(t *testing.T) {
	for _, code := range []string{"trace-path-limit-exceeded", "admitted-path-limit", "record-failed"} {
		t.Run(code, func(t *testing.T) {
			// The aggregate writer refuses symlinked ancestors; macOS TMPDIR has one.
			root, err := filepath.EvalSymlinks(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			called := false
			c := &change{flow: flow{ctx: context.Background(), root: root}, evidence: root,
				options: ChangeOptions{Verify: "go test ./...", Outcome: "passed", Steps: Runner{Run: func(_ context.Context, _ string, args []string, _, stderr io.Writer) int {
					if args[0] != "dogfood-record" {
						t.Fatalf("unexpected command %v", args)
					}
					called = true
					fmt.Fprintf(stderr, "{\"code\":%q,\"error\":\"overflow\",\"ok\":false}\n", code)
					return 2
				}}},
			}
			row := c.localOutcome("boundary")
			want := code
			if code == "trace-path-limit-exceeded" {
				want = "admitted-path-limit"
			}
			if !called || row != (step{"local-outcome", "NOT_PRODUCED", want}) {
				t.Fatalf("row=%+v called=%v", row, called)
			}
		})
	}
}

// DCW-V0-019 as amended by DCW-V0-046 (V1-1258, decision 0523 P3): a plan may
// keep a non-bootstrap hunk unknown with a recorded reason and a one-line note.
// dogfood change records the reason in the map through cem mark and admits
// that hunk beside the bootstrap allowance in cem status; an unknown hunk the
// plan leaves unnamed, or keeps with the prepared no-evidence reason or with no
// note, still refuses the whole plan and runs no cite or mark.
func TestChangeDCWV0046AdmitsReasonedUnknownRefusesUnreasoned(t *testing.T) {
	root := t.TempDir()
	testGit(t, root, "init", "-q")
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("base\n"), 0o666); err != nil {
		t.Fatal(err)
	}
	// A fully cited plan cites b.txt for one hunk, so it is not uniform (DCW-V0-048).
	if err := os.WriteFile(filepath.Join(root, "b.txt"), []byte("base\n"), 0o666); err != nil {
		t.Fatal(err)
	}
	testGit(t, root, "add", "a.txt", "b.txt")
	testGit(t, root, "commit", "-q", "-m", "base")
	base := testGit(t, root, "rev-parse", "HEAD")
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("changed\n"), 0o666); err != nil {
		t.Fatal(err)
	}
	testGit(t, root, "commit", "-q", "-am", "change")
	type hunk struct {
		Disposition string `json:"disposition"`
		ID          string `json:"id"`
		Path        string `json:"path"`
		Reason      string `json:"reason"`
	}
	type document struct {
		Hunks []hunk `json:"hunks"`
	}
	prepared := document{Hunks: []hunk{
		{"unknown", "hunk:a", "a.txt", "no-evidence"},
		{"unknown", "hunk:b", "a.txt", "no-evidence"},
		{"unknown", "hunk:c", "a.txt", "no-evidence"},
	}}
	flags := func(args []string) map[string]string {
		values := map[string]string{}
		for index := 2; index+1 < len(args); index += 2 {
			values[args[index]] = args[index+1]
		}
		return values
	}
	var calls [][]string
	failCite := false
	steps := Runner{Path: "corvint", Run: func(_ context.Context, _ string, args []string, _, _ io.Writer) int {
		calls = append(calls, append([]string{}, args...))
		if len(args) < 2 || args[0] != "cem" || failCite && args[1] == "cite" {
			return 1
		}
		values := flags(args)
		output := values["--output"]
		if output == "" {
			output = values["--map"]
		}
		switch args[1] {
		case "prepare":
			data, _ := json.Marshal(prepared)
			if os.MkdirAll(filepath.Join(root, ".corvint"), 0o777) != nil || os.WriteFile(filepath.Join(root, ".corvint/change.cem.json"), data, 0o666) != nil {
				return 1
			}
			return 0
		case "cite", "mark":
			var current document
			data, err := os.ReadFile(filepath.Join(root, values["--map"]))
			if err != nil || json.Unmarshal(data, &current) != nil {
				return 1
			}
			if args[1] == "mark" {
				for index := range current.Hunks {
					if fmt.Sprint(index+1) == values["--hunk"] || current.Hunks[index].ID == values["--hunk"] {
						current.Hunks[index].Reason = values["--reason"]
					}
				}
			}
			data, _ = json.Marshal(current)
			if os.WriteFile(filepath.Join(root, output), data, 0o666) != nil {
				return 1
			}
			return 0
		}
		return 1
	}}
	plan := filepath.Join(t.TempDir(), "cites.tsv")
	run := func(rows string) (string, [][]string) {
		t.Helper()
		calls = nil
		_ = os.Remove(filepath.Join(root, ".corvint/change.cem.json"))
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
				return strings.TrimSpace(line), calls
			}
		}
		t.Fatalf("report has no cem-cite row:\n%s", report)
		return "", nil
	}
	mutations := func(calls [][]string) []string {
		names := []string{}
		for _, call := range calls {
			if len(call) > 1 && call[0] == "cem" && (call[1] == "cite" || call[1] == "mark") {
				names = append(names, strings.Join(call[1:], " "))
			}
		}
		return names
	}
	maxUnknown := func(calls [][]string) string {
		for _, call := range calls {
			if len(call) > 1 && call[0] == "cem" && call[1] == "status" {
				return flags(call)["--max-unknown"]
			}
		}
		return "not-run"
	}

	// A world-readable file left at the evidence path is replaced, not reused.
	evidence := filepath.Join(root, ".git/corvint/cem-unknown-reasons.tsv")
	if os.MkdirAll(filepath.Dir(evidence), 0o700) != nil || os.WriteFile(evidence, []byte("stale\n"), 0o644) != nil || os.Chmod(evidence, 0o644) != nil {
		t.Fatal("cannot seed the evidence file")
	}
	reasoned := "1\ta.txt\t1:1\tspecification\n" +
		"2\tunknown\tinsufficient-evidence\tgenerated fixture bytes that no spec governs\n" +
		"hunk:c\tunknown\tconflicting-evidence\ttwo specs disagree on the answer\n"
	got, observed := run(reasoned)
	if !strings.Contains(got, `"status": "PRODUCED"`) {
		t.Fatalf("reasoned plan: cem-cite = %s; want PRODUCED", got)
	}
	ran := mutations(observed)
	if len(ran) != 3 || !strings.HasPrefix(ran[0], "cite ") ||
		!strings.Contains(ran[1], "mark --map .corvint/.cem-citations.") || !strings.Contains(ran[1], " --hunk 2 --disposition unknown --reason insufficient-evidence --output .corvint/.cem-citations.") ||
		!strings.HasPrefix(ran[2], "mark --map .corvint/.cem-citations.") || !strings.HasSuffix(ran[2], " --hunk hunk:c --disposition unknown --reason conflicting-evidence --output .corvint/change.cem.json") {
		t.Fatalf("reasoned plan ran %q", ran)
	}
	if limit := maxUnknown(observed); limit != "2" {
		t.Errorf("reasoned plan: cem status --max-unknown %s; want 2 (two reasoned unknown hunks, no bootstrap)", limit)
	}
	kept, err := os.ReadFile(evidence)
	notes := "hunk:b\tinsufficient-evidence\tgenerated fixture bytes that no spec governs\nhunk:c\tconflicting-evidence\ttwo specs disagree on the answer\n"
	if err != nil || string(kept) != notes {
		t.Errorf("unknown reasons evidence = %q, %v", kept, err)
	}
	if info, err := os.Stat(evidence); err != nil || info.Mode().Perm() != 0o600 {
		t.Errorf("unknown reasons evidence mode = %v, %v; want 0600", info, err)
	}

	// A resumed map whose hunks an earlier staged plan kept unknown with a
	// reason owes no row for them: the last plan names only the rest, and the
	// earlier stage's notes are kept.
	prepared.Hunks[1].Reason, prepared.Hunks[2].Reason = "insufficient-evidence", "conflicting-evidence"
	got, observed = run("1\ta.txt\t1:1\tspecification\n")
	prepared.Hunks[1].Reason, prepared.Hunks[2].Reason = "no-evidence", "no-evidence"
	if !strings.Contains(got, `"status": "PRODUCED"`) {
		t.Fatalf("resumed reasoned map: cem-cite = %s; want PRODUCED", got)
	}
	if ran := mutations(observed); len(ran) != 1 || !strings.HasPrefix(ran[0], "cite ") {
		t.Errorf("resumed reasoned map ran %q", ran)
	}
	if limit := maxUnknown(observed); limit != "2" {
		t.Errorf("resumed reasoned map: cem status --max-unknown %s; want 2", limit)
	}
	if kept, err := os.ReadFile(evidence); err != nil || string(kept) != notes {
		t.Errorf("resumed reasoned map: unknown reasons evidence = %q, %v; want the earlier notes", kept, err)
	}

	// A stage whose citation fails leaves the earlier notes as they were, so
	// they keep matching the reasons the map records.
	prepared.Hunks[1].Reason, prepared.Hunks[2].Reason = "insufficient-evidence", "conflicting-evidence"
	failCite = true
	got, _ = run("1\ta.txt\t1:1\tspecification\nhunk:b\tunknown\tconflicting-evidence\ta replacement note\n")
	failCite = false
	prepared.Hunks[1].Reason, prepared.Hunks[2].Reason = "no-evidence", "no-evidence"
	if strings.Contains(got, `"status": "PRODUCED"`) {
		t.Fatalf("failed citation stage: cem-cite = %s; want NOT_PRODUCED", got)
	}
	if kept, err := os.ReadFile(evidence); err != nil || string(kept) != notes {
		t.Errorf("failed citation stage: unknown reasons evidence = %q, %v; want the earlier notes", kept, err)
	}

	// Notes that cannot be replaced after the cite are reported; the map has
	// changed, the earlier notes are kept, and replaying the plan records them.
	newer := strings.Replace(reasoned, "generated fixture bytes that no spec governs", "a newer note", 1)
	if os.MkdirAll(evidence+".next/occupied", 0o700) != nil {
		t.Fatal("cannot occupy the staging path")
	}
	got, observed = run(newer)
	if !strings.Contains(got, `"reason": "unknown-reasons-not-recorded"`) || len(mutations(observed)) != 3 {
		t.Errorf("blocked publication: cem-cite = %s ran %q; want the cite and marks, then unknown-reasons-not-recorded", got, mutations(observed))
	}
	if kept, err := os.ReadFile(evidence); err != nil || string(kept) != notes {
		t.Errorf("blocked publication: unknown reasons evidence = %q, %v; want the earlier notes", kept, err)
	}
	if err := os.RemoveAll(evidence + ".next"); err != nil {
		t.Fatal(err)
	}
	if got, _ = run(newer); !strings.Contains(got, `"status": "PRODUCED"`) {
		t.Fatalf("replayed plan: cem-cite = %s; want PRODUCED", got)
	}
	if kept, err := os.ReadFile(evidence); err != nil || string(kept) != strings.Replace(notes, "generated fixture bytes that no spec governs", "a newer note", 1) {
		t.Errorf("replayed plan: unknown reasons evidence = %q, %v; want the newer notes", kept, err)
	}

	// A fresh map that keeps no hunk unknown drops the notes.
	got, _ = run("1\ta.txt\t1:1\tspecification\n2\ta.txt\t1:1\tspecification\n3\tb.txt\t1:1\tspecification\n")
	if !strings.Contains(got, `"status": "PRODUCED"`) {
		t.Fatalf("fully cited plan: cem-cite = %s; want PRODUCED", got)
	}
	if _, err := os.Lstat(evidence); !os.IsNotExist(err) {
		t.Errorf("a map with no reasoned unknown left %s: %v", evidence, err)
	}

	for _, refused := range []struct{ name, rows, reason string }{
		{"unnamed unknown hunk", "1\ta.txt\t1:1\tspecification\n2\tunknown\tinsufficient-evidence\tgenerated fixture bytes\n", "citation-plan-map-mismatch"},
		{"prepared reason", "1\ta.txt\t1:1\tspecification\n2\tunknown\tinsufficient-evidence\tgenerated fixture bytes\n3\tunknown\tno-evidence\tnot looked at\n", "invalid-citation-plan"},
		{"unregistered reason", "1\ta.txt\t1:1\tspecification\n2\tunknown\tinsufficient-evidence\tgenerated fixture bytes\n3\tunknown\tlater\tnot looked at\n", "invalid-citation-plan"},
		{"note names nothing", "1\ta.txt\t1:1\tspecification\n2\tunknown\tinsufficient-evidence\tgenerated fixture bytes\n3\tunknown\tconflicting-evidence\t- .\n", "invalid-citation-plan"},
	} {
		got, observed := run(refused.rows)
		if !strings.Contains(got, `"reason": "`+refused.reason+`"`) {
			t.Errorf("%s: cem-cite = %s; want %s", refused.name, got, refused.reason)
		}
		if ran := mutations(observed); len(ran) != 0 {
			t.Errorf("%s: refused plan ran %q", refused.name, ran)
		}
		if limit := maxUnknown(observed); limit != "0" {
			t.Errorf("%s: cem status --max-unknown %s; want 0", refused.name, limit)
		}
	}

	// Notes that cannot be retained leave the plan uncited (DCW-V0-046).
	if os.MkdirAll(filepath.Join(evidence, "occupied"), 0o700) != nil {
		t.Fatal("cannot occupy the evidence path")
	}
	got, observed = run(reasoned)
	if !strings.Contains(got, `"reason": "unknown-reasons-not-recorded"`) {
		t.Errorf("occupied evidence path: cem-cite = %s; want unknown-reasons-not-recorded", got)
	}
	if ran := mutations(observed); len(ran) != 0 {
		t.Errorf("occupied evidence path ran %q", ran)
	}
	if limit := maxUnknown(observed); limit != "0" {
		t.Errorf("occupied evidence path: cem status --max-unknown %s; want 0", limit)
	}
}

// TestPrepareUnknownReasonsDCWV0046RefusesOverflow: notes over the bound are
// refused before anything is cited, never truncated, and the file is kept.
func TestPrepareUnknownReasonsDCWV0046RefusesOverflow(t *testing.T) {
	root := t.TempDir()
	c := &change{flow: flow{root: root}, evidence: root}
	path := root + unknownReasons
	for _, size := range []int{maxUnknownReasonBytes, maxUnknownReasonBytes + 1} {
		previous := append(bytes.Repeat([]byte("x"), size-1), '\n')
		if err := os.WriteFile(path, previous, 0o600); err != nil {
			t.Fatal(err)
		}
		err := c.prepareUnknownReasons(nil)
		if (err != nil) != (size > maxUnknownReasonBytes) {
			t.Errorf("%d-byte notes: prepare error = %v", size, err)
		}
		if info, statErr := os.Stat(path); statErr != nil || info.Size() != int64(size) {
			t.Errorf("%d-byte notes: file after prepare = %v, %v; want it unchanged", size, info, statErr)
		}
	}
	plan := []byte("1\tunknown\tinsufficient-evidence\t" + strings.Repeat("n", maxUnknownReasonBytes) + "\n")
	if os.MkdirAll(filepath.Join(root, ".corvint"), 0o700) != nil ||
		os.WriteFile(filepath.Join(root, ".corvint/change.cem.json"), []byte(`{"hunks":[{"id":"hunk:a","disposition":"unknown","reason":"no-evidence"}]}`), 0o600) != nil ||
		os.Remove(path) != nil {
		t.Fatal("cannot seed the map")
	}
	if err := c.prepareUnknownReasons(plan); err == nil {
		t.Errorf("an over-bound plan note was prepared: %d bytes", len(c.unknownNotes))
	}
	if err := c.prepareUnknownReasons([]byte("1\tunknown\tinsufficient-evidence\tgenerated\n")); err != nil || string(c.unknownNotes) != "hunk:a\tinsufficient-evidence\tgenerated\n" {
		t.Errorf("prepared notes = %q, %v", c.unknownNotes, err)
	}
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Errorf("prepare wrote %s before any citation: %v", path, err)
	}
}

// TestPrepareUnknownReasonsDCWV0046AppliesRowsInPlanOrder: a later citation
// of a hunk an earlier stage kept unknown drops its note, and the last row for
// a hunk decides.
func TestPrepareUnknownReasonsDCWV0046AppliesRowsInPlanOrder(t *testing.T) {
	root := t.TempDir()
	c := &change{flow: flow{root: root}, evidence: root}
	if os.MkdirAll(filepath.Join(root, ".corvint"), 0o700) != nil ||
		os.WriteFile(filepath.Join(root, ".corvint/change.cem.json"), []byte(`{"hunks":[{"id":"hunk:a","disposition":"unknown","reason":"insufficient-evidence"},{"id":"hunk:b","disposition":"unknown","reason":"conflicting-evidence"}]}`), 0o600) != nil ||
		os.WriteFile(root+unknownReasons, []byte("hunk:a\tinsufficient-evidence\tearlier a\nhunk:b\tconflicting-evidence\tearlier b\n"), 0o600) != nil {
		t.Fatal("cannot seed the map and notes")
	}
	for _, step := range []struct{ plan, want string }{
		{"1\ta.txt\t1:1\tspecification\n", "hunk:b\tconflicting-evidence\tearlier b\n"},
		{"hunk:b\ta.txt\t1:1\tspecification\n1\ta.txt\t1:1\tspecification\n", ""},
		{"1\ta.txt\t1:1\tspecification\n1\tunknown\tconflicting-evidence\tlater a\n", "hunk:a\tconflicting-evidence\tlater a\nhunk:b\tconflicting-evidence\tearlier b\n"},
		{"2\tunknown\tinsufficient-evidence\tlater b\n2\ta.txt\t1:1\tspecification\n", "hunk:a\tinsufficient-evidence\tearlier a\n"},
	} {
		if err := c.prepareUnknownReasons([]byte(step.plan)); err != nil || string(c.unknownNotes) != step.want {
			t.Errorf("plan %q: notes = %q, %v; want %q", step.plan, c.unknownNotes, err, step.want)
		}
	}
}

// TestReasonedUnknownsDCWV0046CountsBootstrapOnceInTheAllowance: a reasoned
// hunk on an intent path absent at BASE is reported, but the bootstrap
// allowance already admits it, so the added allowance leaves it out.
func TestReasonedUnknownsDCWV0046CountsBootstrapOnceInTheAllowance(t *testing.T) {
	root := t.TempDir()
	testGit(t, root, "init", "-q")
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("base\n"), 0o666); err != nil {
		t.Fatal(err)
	}
	testGit(t, root, "add", "a.txt")
	testGit(t, root, "commit", "-q", "-m", "base")
	mapPath := filepath.Join(root, "change.cem.json")
	if err := os.WriteFile(mapPath, []byte(`{"hunks":[`+
		`{"disposition":"unknown","id":"hunk:s","path":"spec.md","reason":"insufficient-evidence"},`+
		`{"disposition":"unknown","id":"hunk:a","path":"a.txt","reason":"conflicting-evidence"},`+
		`{"disposition":"unknown","id":"hunk:b","path":"a.txt","reason":"no-evidence"},`+
		`{"disposition":"cited","id":"hunk:c","path":"a.txt","reason":"cited"}]}`), 0o666); err != nil {
		t.Fatal(err)
	}
	run := &flow{ctx: context.Background(), root: root, stderr: io.Discard, base: testGit(t, root, "rev-parse", "HEAD")}
	if all, allowance := run.reasonedUnknowns(mapPath, []string{"spec.md"}, true); all != 2 || allowance != 1 {
		t.Errorf("with bootstrap: reasonedUnknowns = %d, %d; want 2, 1", all, allowance)
	}
	if all, allowance := run.reasonedUnknowns(mapPath, []string{"spec.md"}, false); all != 2 || allowance != 2 {
		t.Errorf("without bootstrap: reasonedUnknowns = %d, %d; want 2, 2", all, allowance)
	}
}
