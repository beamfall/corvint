package witness

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/cem/gitrun"
	"github.com/Beamfall/corvint/internal/contextindex"
)

// The whole report rests on the claim that no V0 authority closes an
// obligation. That claim must be table data a reader can audit, never a
// constant folded into the verdict.
func TestNoV0AuthorityCloses(t *testing.T) {
	if len(authorities) == 0 {
		t.Fatal("the authority table must enumerate every class the report can assign")
	}
	for _, item := range authorities {
		if item.Closing {
			t.Errorf("%s is marked closing; CF-V0-014 admits no closing authority in V0", item.Class)
		}
		if item.Citation == "" {
			t.Errorf("%s carries no citation for its closing determination", item.Class)
		}
	}
	if anyClosingAuthority() {
		t.Error("anyClosingAuthority disagrees with the table")
	}
}

func TestClosesIsTableDrivenNotConstant(t *testing.T) {
	original := authorities
	t.Cleanup(func() { authorities = original })
	authorities = append(append([]Authority{}, original...), Authority{Class: "OWNING_VERIFIER", Closing: true, Citation: "test"})
	if !closes("OWNING_VERIFIER") {
		t.Error("closes must read the table, so admitting a new authority is a row rather than a rewrite")
	}
	if closes("CALLER_REPORTED") {
		t.Error("CALLER_REPORTED must never close")
	}
	if closes("NOT_A_CLASS") {
		t.Error("an unknown class must not close")
	}
}

func TestCoverageWidensNotRunForWeakLanguages(t *testing.T) {
	cases := []struct {
		name       string
		item       change
		tier       string
		wantReason bool
	}{
		{"go", change{status: "M", path: "internal/a/a.go", newMode: "100644"}, "DEEP", false},
		{"go at repository root", change{status: "M", path: "main.go", newMode: "100644"}, "DEEP", true},
		{"python", change{status: "M", path: "src/a.py", newMode: "100644"}, "SHALLOW", true},
		{"typescript", change{status: "M", path: "web/a.ts", newMode: "100644"}, "GENERIC", true},
		{"markdown", change{status: "M", path: "docs/a.md", newMode: "100644"}, "NONE", true},
		{"deleted go", change{status: "D", path: "internal/a/a.go", newMode: "100644"}, "NONE", true},
		{"symlink", change{status: "A", path: "internal/a/link", newMode: "120000"}, "NONE", true},
		{"gitlink", change{status: "A", path: "vendorish", newMode: "160000"}, "NONE", true},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			tier, reason := coverage(testCase.item, nil)
			if tier != testCase.tier {
				t.Errorf("tier = %q, want %q", tier, testCase.tier)
			}
			if (reason != "") != testCase.wantReason {
				t.Errorf("reason = %q, want non-empty = %t", reason, testCase.wantReason)
			}
		})
	}
}

func TestClassifyDerivesAuthorityFromRepositoryFacts(t *testing.T) {
	changed := map[string]bool{"internal/a/a.go": true}
	cases := []struct{ relation, evidencePath, want string }{
		{"test-claim", "docs/spec.md", "CALLER_REPORTED"},
		{"test-claim", "internal/a/a.go", "CALLER_REPORTED"},
		{"specification", "internal/a/a.go", "SELF_AUTHORED"},
		{"specification", "docs/spec.md", "PRODUCER_DECLARED"},
		{"implementation", "", "PRODUCER_DECLARED"},
	}
	for _, testCase := range cases {
		if got := classify(testCase.relation, testCase.evidencePath, changed); got != testCase.want {
			t.Errorf("classify(%q, %q) = %q, want %q", testCase.relation, testCase.evidencePath, got, testCase.want)
		}
	}
}

// NOT_RUN must be sticky: an obligation whose blast radius is unknown cannot be
// closed, because the extent a witness would have to cover is itself unknown.
func TestDecideNeverUpgradesNotRun(t *testing.T) {
	obligations := []Obligation{
		{ID: "a", Verdict: NotRun, Reason: "ADMISSION_REFUSED", Witnesses: []Witness{{Closing: true}}},
		{ID: "b", Witnesses: []Witness{{Closing: false}}},
		{ID: "c", Witnesses: []Witness{{Closing: false}, {Closing: true}}},
		{ID: "d", Witnesses: []Witness{}},
	}
	decide(obligations)
	want := []Verdict{NotRun, Unproven, Closed, Unproven}
	for position, item := range obligations {
		if item.Verdict != want[position] {
			t.Errorf("%s verdict = %s, want %s", item.ID, item.Verdict, want[position])
		}
	}
	if obligations[0].Reason != "ADMISSION_REFUSED" {
		t.Errorf("a NOT_RUN reason must survive decide, got %q", obligations[0].Reason)
	}
	if obligations[1].Reason != "NO_CLOSING_AUTHORITY" {
		t.Errorf("an unproven obligation must name why, got %q", obligations[1].Reason)
	}
}

// The denominator is the universe admitted, not the result set produced: a path
// no engine could analyse is still an obligation, and it is named.
func TestChangeObligationsAdmitEveryPath(t *testing.T) {
	changes := []change{
		{status: "M", path: "internal/a/a.go", newMode: "100644"},
		{status: "M", path: "internal/b/b.go", newMode: "100644"},
		{status: "M", path: "src/a.py", newMode: "100644"},
		{status: "M", path: "docs/a.md", newMode: "100644"},
	}
	admission := stage{status: "COMPUTED", paths: []string{"internal/a/a.go", "internal/b/b.go"}}
	closure := stage{status: "PARTIAL", refused: map[string]string{"internal/b/b.go": "refused"}}
	obligations := changeObligations(changes, admission, closure, nil)
	if len(obligations) != len(changes) {
		t.Fatalf("opened %d obligations for %d changed paths; no input may be dropped", len(obligations), len(changes))
	}
	reasons := map[string]string{}
	verdicts := map[string]Verdict{}
	for _, item := range obligations {
		reasons[item.Path], verdicts[item.Path] = item.Reason, item.Verdict
	}
	if verdicts["internal/a/a.go"] != "" {
		t.Errorf("an admitted, non-refused path must stay undecided until decide runs")
	}
	if verdicts["internal/b/b.go"] != NotRun || reasons["internal/b/b.go"] != "CLOSURE_REFUSED" {
		t.Errorf("a path whose closure was refused must be NOT_RUN, got %s/%s", verdicts["internal/b/b.go"], reasons["internal/b/b.go"])
	}
	if verdicts["src/a.py"] != NotRun || reasons["src/a.py"] != "LANGUAGE_COVERAGE_SHALLOW_PYTHON" {
		t.Errorf("a weakly covered language must widen NOT_RUN, got %s/%s", verdicts["src/a.py"], reasons["src/a.py"])
	}
	if verdicts["docs/a.md"] != NotRun || reasons["docs/a.md"] != "LANGUAGE_COVERAGE_NONE" {
		t.Errorf("an uncovered language must widen NOT_RUN, got %s/%s", verdicts["docs/a.md"], reasons["docs/a.md"])
	}
}

func TestChangeObligationsNameARefusedAdmission(t *testing.T) {
	changes := []change{{status: "M", path: "internal/a/a.go", newMode: "100644"}}
	obligations := changeObligations(changes, stage{status: "REFUSED", detail: "dirty worktree"}, stage{refused: map[string]string{}}, nil)
	if len(obligations) != 1 || obligations[0].Verdict != NotRun || obligations[0].Reason != "ADMISSION_REFUSED" {
		t.Fatalf("a refused admission must leave every path counted and named, got %+v", obligations)
	}
}

func TestImpactObligationsUnionReceiptsAndExcludeChangedPaths(t *testing.T) {
	closure := stage{receipts: []map[string]any{
		receiptWith("path", "internal/a/a.go", "reverse-import", "internal/c/c.go"),
		receiptWith("path", "internal/b/b.go", "reverse-import", "internal/c/c.go"),
		receiptWith("test", "internal/a/a_test.go", "reverse-import", "internal/b/b.go"),
	}}
	changes := []change{{status: "M", path: "internal/a/a.go"}, {status: "M", path: "internal/b/b.go"}}
	obligations := impactObligations(closure, changes, nil)
	got := make([]string, 0, len(obligations))
	for _, item := range obligations {
		got = append(got, item.ID)
	}
	want := "impact:reverse-import:internal/c/c.go,impact:test:internal/a/a_test.go"
	if strings.Join(got, ",") != want {
		t.Errorf("impact obligations = %v, want %s (deduplicated, changed paths excluded)", got, want)
	}
}

func receiptWith(pairs ...string) map[string]any {
	rows := make([]any, 0, len(pairs)/2)
	for position := 0; position+1 < len(pairs); position += 2 {
		rows = append(rows, map[string]any{"kind": pairs[position], "id": pairs[position+1]})
	}
	return map[string]any{"results": rows}
}

// The engine truncates its result slice before counting it, so its own
// omitted_results reads zero however much was discarded. Truncation has to be
// detected from the ceiling itself.
func TestAtCeilingDetectsTruncationTheReceiptCannotReport(t *testing.T) {
	full := map[string]any{"coverage": map[string]any{"included_results": impactLimit, "omitted_results": 0}}
	if !atCeiling(full) {
		t.Error("a receipt filled to the ranking ceiling must be treated as possibly truncated")
	}
	partial := map[string]any{"coverage": map[string]any{"included_results": impactLimit - 1, "omitted_results": 0}}
	if atCeiling(partial) {
		t.Error("a receipt below the ceiling is complete")
	}
	if atCeiling(map[string]any{}) {
		t.Error("a receipt with no coverage block must not be claimed truncated")
	}
}

func TestSummarizeRatios(t *testing.T) {
	obligations := []Obligation{
		{Verdict: Closed, Witnesses: []Witness{{Closing: true}}},
		{Verdict: Unproven, Witnesses: []Witness{{Closing: false}, {Closing: false}}},
		{Verdict: Unproven, Witnesses: []Witness{}},
		{Verdict: NotRun, Witnesses: []Witness{}},
	}
	summary := summarize(obligations)
	if summary.Opened != 4 || summary.Closed != 1 || summary.Unproven != 2 || summary.NotRun != 1 {
		t.Fatalf("summary counts = %+v", summary)
	}
	if summary.Analysed != 3 {
		t.Errorf("analysed = %d, want 3 (opened minus not-run)", summary.Analysed)
	}
	if summary.WitnessesExamined != 3 || summary.WitnessesClosing != 1 {
		t.Errorf("witness counts = %d/%d, want 3/1", summary.WitnessesExamined, summary.WitnessesClosing)
	}
	if summary.DeterminablePerMille != 750 {
		t.Errorf("determinable = %d per mille, want 750", summary.DeterminablePerMille)
	}
	if summary.ProvenPerMille != 333 {
		t.Errorf("proven = %d per mille, want 333", summary.ProvenPerMille)
	}
}

func TestSummarizeEmptyRangeDoesNotDivide(t *testing.T) {
	summary := summarize(nil)
	if summary.Opened != 0 || summary.DeterminablePerMille != 0 || summary.ProvenPerMille != 0 {
		t.Fatalf("an empty range must summarize to zeroes without dividing, got %+v", summary)
	}
}

func TestSortObligationsIsTotalAndDeterministic(t *testing.T) {
	obligations := []Obligation{
		{ID: "impact:test:b", Kind: KindImpact, Path: "b"},
		{ID: "change:b", Kind: KindChange, Path: "b"},
		{ID: "impact:reverse-import:a", Kind: KindImpact, Path: "a"},
		{ID: "change:a", Kind: KindChange, Path: "a", Witnesses: []Witness{
			{HunkID: "h2", Relation: "specification", EvidencePath: "z"},
			{HunkID: "h1", Relation: "test-claim", EvidencePath: "y"},
		}},
	}
	sortObligations(obligations)
	got := make([]string, 0, len(obligations))
	for _, item := range obligations {
		got = append(got, item.ID)
	}
	want := "change:a,change:b,impact:reverse-import:a,impact:test:b"
	if strings.Join(got, ",") != want {
		t.Errorf("order = %v, want %s", got, want)
	}
	if obligations[0].Witnesses[0].HunkID != "h1" {
		t.Error("witnesses must sort by hunk identity so the bytes are stable")
	}
}

func TestPreconditionsNameWhatIsMissing(t *testing.T) {
	report := &Report{
		Range:   Range{Admission: "REFUSED", AdmissionDetail: "dirty worktree", Closure: "EMPTY", ClosureDetail: "nothing admitted"},
		Sources: []Source{{Name: "cem", Status: "UNBOUND", Detail: "bound elsewhere"}},
		Summary: Summary{Opened: 3, NotRun: 3},
	}
	got := identifiers(preconditions(report))
	for _, want := range []string{
		"WITNESS-P1-NO-CLOSING-AUTHORITY",
		"WITNESS-P2-AUTHORITY-LABELS-ARE-LITERALS",
		"WITNESS-P3-NO-BOUND-EVIDENCE-MAP",
		"WITNESS-P4-NO-BLAST-RADIUS",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("preconditions %s missing %s", got, want)
		}
	}
	if strings.Contains(got, "WITNESS-P5") {
		t.Error("the partial-coverage precondition is redundant when the whole admission was refused")
	}
}

func TestPreconditionsReportPartialCoverageAndTruncation(t *testing.T) {
	report := &Report{
		Range:   Range{Admission: "COMPUTED", Closure: "TRUNCATED", ClosureDetail: "2 paths at the ceiling"},
		Sources: []Source{{Name: "cem", Status: "BOUND"}},
		Summary: Summary{Opened: 10, NotRun: 2},
	}
	got := identifiers(preconditions(report))
	for _, want := range []string{
		"WITNESS-P5-PARTIAL-LANGUAGE-COVERAGE",
		"WITNESS-P6-INCOMPLETE-REVERSE-DEPENDENCY-CLOSURE",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("preconditions %s missing %s", got, want)
		}
	}
	if strings.Contains(got, "WITNESS-P3") {
		t.Error("a bound evidence map must not raise the missing-map precondition")
	}
}

func identifiers(items []Precondition) string {
	result := make([]string, 0, len(items))
	for _, item := range items {
		result = append(result, item.ID)
	}
	return strings.Join(result, ",")
}

// TestCoverageReportsDroppedFactsAsUncovered pins the receipt-visible half of
// the silent-drop fix. A Python source the index parsed and a Python source the
// index dropped previously reported the same SHALLOW tier, which overstated
// what had been analysed for the dropped one.
func TestCoverageReportsDroppedFactsAsUncovered(t *testing.T) {
	dropped := map[string]string{
		"src/dropped.py": contextindex.UnparsedPythonGrammar,
		"src/binary.py":  contextindex.UnparsedNotText,
	}
	parsed := change{status: "M", path: "src/parsed.py", newMode: "100644"}
	if tier, reason := coverage(parsed, dropped); tier != "SHALLOW" || reason != "LANGUAGE_COVERAGE_SHALLOW_PYTHON" {
		t.Errorf("parsed python coverage = %q/%q, want SHALLOW/LANGUAGE_COVERAGE_SHALLOW_PYTHON", tier, reason)
	}
	for path, want := range dropped {
		item := change{status: "M", path: path, newMode: "100644"}
		tier, reason := coverage(item, dropped)
		if tier != "NONE" {
			t.Errorf("%s tier = %q, want NONE", path, tier)
		}
		if reason != want {
			t.Errorf("%s reason = %q, want %q", path, reason, want)
		}
	}
}

// TestChangeObligationsCarryTheDropReason proves the reason reaches an
// obligation rather than stopping at the coverage helper.
func TestChangeObligationsCarryTheDropReason(t *testing.T) {
	changes := []change{{status: "M", path: "src/dropped.py", newMode: "100644"}}
	dropped := map[string]string{"src/dropped.py": contextindex.UnparsedPythonGrammar}
	obligations := changeObligations(changes, stage{status: "COMPUTED"}, stage{refused: map[string]string{}}, dropped)
	if len(obligations) != 1 {
		t.Fatalf("obligations = %#v", obligations)
	}
	if obligations[0].Coverage != "NONE" || obligations[0].Reason != contextindex.UnparsedPythonGrammar {
		t.Fatalf("obligation coverage=%q reason=%q, want NONE/%s", obligations[0].Coverage, obligations[0].Reason, contextindex.UnparsedPythonGrammar)
	}
}

// TestReadCEMSourceGitFailureIsInvalidNotAbsent proves a git failure that is
// not "the path is absent" (budget exhaustion here stands in for the size,
// timeout, and start-failure cases that share the same non-exit-code error
// shape) reports INVALID with the failure reason, not ABSENT's "nothing is
// committed" — that claim is reserved for a genuine missing path.
func TestReadCEMSourceGitFailureIsInvalidNotAbsent(t *testing.T) {
	index := &contextindex.Index{Root: ".", Revision: strings.Repeat("0", 40)}
	budget := gitrun.NewBudget(0, time.Minute)
	source, document := readCEMSource(context.Background(), budget, index, Options{})
	if source.Status != "INVALID" {
		t.Fatalf("git failure status = %q, want INVALID", source.Status)
	}
	if source.Detail == "" || strings.Contains(source.Detail, "no Change Evidence Map is committed") {
		t.Fatalf("git failure detail = %q, must carry the failure reason, not the absent-path claim", source.Detail)
	}
	if document != nil {
		t.Fatalf("an INVALID source must not carry a parsed document")
	}
}

// TestReadCEMSourceNonBlobAtMapPathIsInvalidNotAbsent proves a tree committed
// at the map path reports INVALID: something is committed there, so the
// "no Change Evidence Map is committed" claim would be invented certainty. A
// path with nothing committed stays ABSENT.
func TestReadCEMSourceNonBlobAtMapPathIsInvalidNotAbsent(t *testing.T) {
	git, err := exec.LookPath("git")
	if err != nil {
		t.Skipf("git unavailable: %v", err)
	}
	root := initTestRepo(t, git)
	if err := os.MkdirAll(filepath.Join(root, ".corvint", "change.cem.json"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".corvint", "change.cem.json", "x"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	tree := ""
	for _, arguments := range [][]string{
		{"add", "."},
		{"-c", "user.name=Fixture", "-c", "user.email=fixture@example.test", "commit", "--quiet", "-m", "tree at map path"},
		{"rev-parse", "HEAD^{tree}"},
	} {
		command := exec.Command(git, arguments...)
		command.Dir = root
		command.Env = gitEnvironment()
		output, err := command.Output()
		if err != nil {
			t.Fatalf("git %s: %v", strings.Join(arguments, " "), err)
		}
		tree = strings.TrimSpace(string(output))
	}
	index := &contextindex.Index{Root: root, Revision: tree}
	source, document := readCEMSource(context.Background(), gitrun.NewDefaultBudget(), index, Options{})
	if source.Status != "INVALID" || document != nil {
		t.Fatalf("tree at map path status = %q detail = %q, want INVALID", source.Status, source.Detail)
	}
	absent, _ := readCEMSource(context.Background(), gitrun.NewDefaultBudget(), index, Options{CEMPath: ".corvint/missing.cem.json"})
	if absent.Status != "ABSENT" {
		t.Fatalf("missing map path status = %q detail = %q, want ABSENT", absent.Status, absent.Detail)
	}
}

// TestRunDoesNotLeakAmbientGitDir proves run's gitrun.Options carries the
// scrubbed environment gitEnvironment builds, so an ambient GIT_DIR or
// GIT_WORK_TREE set in this process's own environment cannot redirect the
// child Git invocation away from the root run was given. Without the Env
// wiring, Git resolves GIT_DIR/GIT_WORK_TREE ahead of the process's working
// directory and would report the decoy repository's toplevel instead.
func TestRunDoesNotLeakAmbientGitDir(t *testing.T) {
	git, err := exec.LookPath("git")
	if err != nil {
		t.Skipf("git unavailable: %v", err)
	}

	root := initTestRepo(t, git)
	decoy := initTestRepo(t, git)

	t.Setenv("GIT_DIR", filepath.Join(decoy, ".git"))
	t.Setenv("GIT_WORK_TREE", decoy)

	budget := gitrun.NewDefaultBudget()
	raw, err := run(context.Background(), budget, root, 4096, "rev-parse", "--show-toplevel")
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	got, err := filepath.EvalSymlinks(strings.TrimSpace(string(raw)))
	if err != nil {
		t.Fatalf("resolve reported toplevel: %v", err)
	}
	want, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatalf("resolve root: %v", err)
	}
	if got != want {
		t.Fatalf("run reported toplevel %q, want %q (ambient GIT_DIR/GIT_WORK_TREE leaked to the child)", got, want)
	}
}

// initTestRepo creates and commits a minimal repository under a fresh
// temporary directory, using the same scrubbed environment run uses so the
// fixture itself never depends on ambient Git state.
func initTestRepo(t *testing.T, git string) string {
	t.Helper()
	root := t.TempDir()
	for _, arguments := range [][]string{
		{"init", "--quiet"},
		{"-c", "user.name=Fixture", "-c", "user.email=fixture@example.test", "commit", "--quiet", "--allow-empty", "-m", "fixture"},
	} {
		command := exec.Command(git, arguments...)
		command.Dir = root
		command.Env = gitEnvironment()
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v: %s", strings.Join(arguments, " "), err, output)
		}
	}
	return root
}

func TestBaseTreeIgnoresGraftedAncestry(t *testing.T) {
	for _, location := range []string{"repository", "ambient"} {
		t.Run("AGW-V0-002 PUB-V0-019 "+location, func(t *testing.T) {
			git, err := exec.LookPath("git")
			if err != nil {
				t.Fatal(err)
			}
			root := initTestRepo(t, git)
			fixtureGit := func(args ...string) string {
				t.Helper()
				argv := append([]string{"-c", "advice.graftFileDeprecated=false", "-c", "user.name=Fixture", "-c", "user.email=fixture@example.test"}, args...)
				command := exec.Command(git, argv...)
				command.Dir, command.Env = root, gitEnvironment()
				raw, err := command.CombinedOutput()
				if err != nil {
					t.Fatalf("fixture git %v: %v: %s", args, err, raw)
				}
				return strings.TrimSpace(string(raw))
			}
			parent := fixtureGit("rev-parse", "HEAD")
			fixtureGit("commit", "--quiet", "--allow-empty", "-m", "child")
			target := fixtureGit("rev-parse", "HEAD")
			tree := fixtureGit("rev-parse", "HEAD^{tree}")
			unrelated := fixtureGit("commit-tree", tree, "-m", "unrelated")
			graftPath := filepath.Join(root, ".git", "info", "grafts")
			if location == "ambient" {
				graftPath = filepath.Join(t.TempDir(), "grafts")
			}
			graft := []byte(target + " " + unrelated + "\n")
			if err := os.WriteFile(graftPath, graft, 0o600); err != nil {
				t.Fatal(err)
			}
			if location == "ambient" {
				t.Setenv("GIT_GRAFT_FILE", graftPath)
			}
			control := exec.Command(git, "-c", "advice.graftFileDeprecated=false", "merge-base", "--is-ancestor", unrelated, target)
			control.Dir = root
			control.Env = []string{"PATH=" + os.Getenv("PATH"), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=" + os.DevNull, "GIT_NO_REPLACE_OBJECTS=1", "GIT_GRAFT_FILE=" + graftPath}
			if raw, err := control.CombinedOutput(); err != nil {
				t.Fatalf("fixture did not forge ancestry: %v %s", err, raw)
			}
			index := &contextindex.Index{Root: root, Revision: tree, CommitRevision: target}
			if _, err := resolveBaseTree(context.Background(), gitrun.NewDefaultBudget(), index, parent); err != nil {
				t.Errorf("real parent refused: %v", err)
			}
			if _, err := resolveBaseTree(context.Background(), gitrun.NewDefaultBudget(), index, unrelated); err == nil {
				t.Error("graft-invented parent accepted")
			}
			after, err := os.ReadFile(graftPath)
			if err != nil || string(after) != string(graft) {
				t.Fatalf("graft evidence changed: %v", err)
			}
		})
	}
}

// TestPacketCoverageEqualsEveryCompiledReceipt pins AGW-V0-003: the report
// carries one coverage entry per packet it compiled, each equal to that
// packet's own coverage block under the packet's field names.
func TestPacketCoverageEqualsEveryCompiledReceipt(t *testing.T) {
	index, base := packetFixture(t, "example.test/fixture")
	report, err := Compile(context.Background(), index, Options{Base: base})
	if err != nil {
		t.Fatal(err)
	}
	admission, err := contextindex.RangeImpact(context.Background(), index, base, impactLimit)
	if err != nil {
		t.Fatal(err)
	}
	closure, err := contextindex.Impact(index, []string{"a/a.go"}, impactLimit)
	if err != nil {
		t.Fatal(err)
	}
	want := []map[string]any{coverageOf("admission", "", admission), coverageOf("closure", "a/a.go", closure)}
	if got := decodedPacketCoverage(t, report); !reflect.DeepEqual(got, want) {
		t.Fatalf("packetCoverage = %v, want %v", got, want)
	}
	rendered := Render(report)
	line := fmt.Sprintf("closure   packet_bytes=%v budget_bytes=null within_budget=true", want[1]["packet_bytes"])
	if !strings.Contains(rendered, "\nPACKETS\n") || !strings.Contains(rendered, line) {
		t.Fatalf("rendered report lacks the closure packet line %q\n%s", line, rendered)
	}
}

// A refused admission compiles no packet, so the list is empty and still
// present: a reader must never have to tell null from "none compiled".
func TestPacketCoverageOfARefusedAdmissionIsEmptyNotNull(t *testing.T) {
	index, base := packetFixture(t, "fixture")
	report, err := Compile(context.Background(), index, Options{Base: base})
	if err != nil {
		t.Fatal(err)
	}
	if report.Range.Admission != "REFUSED" {
		t.Fatalf("fixture admission = %s, want REFUSED", report.Range.Admission)
	}
	encoded, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(encoded), `"packetCoverage":[]`) {
		t.Fatalf("a refused admission must encode packetCoverage as []: %s", encoded)
	}
	if !strings.Contains(Render(report), "PACKETS\n  (none: no packet was compiled)") {
		t.Error("the rendered report does not name that no packet was compiled")
	}
}

// A receipt without a readable coverage block is reported unreadable and
// carries no numbers, never zeros read from a missing or mistyped value.
func TestPacketCoverageRefusesAnUnreadableBlock(t *testing.T) {
	complete := map[string]any{"packet_bytes": 10, "budget_bytes": nil, "within_budget": true, "included_results": 1, "omitted_results": 0}
	for name, receipt := range map[string]map[string]any{
		"absent":             {},
		"mistyped bytes":     {"coverage": withValue(complete, "packet_bytes", "10")},
		"mistyped budget":    {"coverage": withValue(complete, "budget_bytes", "none")},
		"missing within":     {"coverage": withValue(complete, "within_budget", nil)},
		"mistyped omitted":   {"coverage": withValue(complete, "omitted_results", 0.0)},
		"not a coverage map": {"coverage": "x"},
	} {
		encoded, err := json.Marshal(packetCoverage("closure", "a.go", receipt))
		if err != nil {
			t.Fatal(err)
		}
		if string(encoded) != `{"stage":"closure","path":"a.go","status":"NOT_PRODUCED","reason":"packet-coverage-unreadable"}` {
			t.Errorf("%s: %s", name, encoded)
		}
	}
	encoded, _ := json.Marshal(packetCoverage("admission", "", map[string]any{"coverage": complete}))
	if string(encoded) != `{"stage":"admission","status":"PRODUCED","packet_bytes":10,"budget_bytes":null,"within_budget":true,"included_results":1,"omitted_results":0}` {
		t.Errorf("a readable block: %s", encoded)
	}
	if text := packetText(packetCoverage("closure", "a.go", map[string]any{})); text != "NOT_PRODUCED packet-coverage-unreadable" {
		t.Errorf("an unreadable entry renders %q", text)
	}
	if text := packetText(packetCoverage("closure", "a.go", map[string]any{"coverage": withValue(complete, "included_results", impactLimit)})); !strings.HasSuffix(text, " at-ranking-ceiling") {
		t.Errorf("a packet at the ranking ceiling renders %q without the marker", text)
	}
}

func withValue(source map[string]any, key string, value any) map[string]any {
	copied := map[string]any{}
	for name, item := range source {
		copied[name] = item
	}
	copied[key] = value
	return copied
}

// packetFixture commits a two-package module and a change to a/a.go. A module
// path without a slash makes the committed-range engine refuse admission.
func packetFixture(t *testing.T, module string) (*contextindex.Index, string) {
	t.Helper()
	git, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	root := initTestRepo(t, git)
	fixtureGit := func(args ...string) string {
		t.Helper()
		command := exec.Command(git, append([]string{"-c", "user.name=Fixture", "-c", "user.email=fixture@example.test"}, args...)...)
		command.Dir, command.Env = root, gitEnvironment()
		raw, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("fixture git %v: %v: %s", args, err, raw)
		}
		return strings.TrimSpace(string(raw))
	}
	files := map[string]string{
		"go.mod":    "module " + module + "\n\ngo 1.22\n",
		"a/a.go":    "package a\n\n// A is used by b.\nfunc A() int { return 1 }\n",
		"b/b.go":    "package b\n\nimport \"" + module + "/a\"\n\n// B calls A.\nfunc B() int { return a.A() }\n",
		"README.md": "fixture\n",
	}
	for name, body := range files {
		if err := os.MkdirAll(filepath.Join(root, filepath.Dir(name)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	fixtureGit("add", ".")
	fixtureGit("commit", "--quiet", "-m", "base")
	base := fixtureGit("rev-parse", "HEAD")
	if err := os.WriteFile(filepath.Join(root, "a/a.go"), []byte("package a\n\n// A is used by b.\nfunc A() int { return 2 }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	fixtureGit("commit", "--quiet", "-am", "change")
	index, err := contextindex.Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	return index, base
}

func decodedPacketCoverage(t *testing.T, report *Report) []map[string]any {
	t.Helper()
	encoded, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		PacketCoverage []map[string]any `json:"packetCoverage"`
	}
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	return decoded.PacketCoverage
}

// coverageOf is the expected projection, decoded through JSON the way a
// receipt reader sees it.
func coverageOf(stageName, changedPath string, receipt map[string]any) map[string]any {
	coverageMap := receipt["coverage"].(map[string]any)
	projected := map[string]any{"stage": stageName, "status": "PRODUCED"}
	if changedPath != "" {
		projected["path"] = changedPath
	}
	for _, key := range []string{"packet_bytes", "budget_bytes", "within_budget", "included_results", "omitted_results"} {
		projected[key] = coverageMap[key]
	}
	raw, _ := json.Marshal(projected)
	decoded := map[string]any{}
	_ = json.Unmarshal(raw, &decoded)
	return decoded
}
