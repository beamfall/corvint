package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

// projectOperationsTask matches at least two project-operation terms, so the
// standalone command routes it to the authority-start profile.
const projectOperationsTask = "orient the contributor on the roadmap workflow"

// wideningRepository extends the authority fixture with neighbours the
// task-context slot ranking names for repositoryQueryTask but the confident
// symbol match does not: a test, a sibling source file and a document.
func wideningRepository(t *testing.T) string {
	t.Helper()
	root := queryCLIRepository(t)
	files := map[string]string{
		"internal/parser/token_test.go": "package parser\n\nimport \"testing\"\n\nfunc TestParseTokenDelimiter(t *testing.T) { ParseToken(\"a,b\") }\n",
		"internal/parser/lexer.go":      "package parser\n\n// Lex splits input on validated delimiters.\nfunc Lex(input string) []string { return nil }\n",
		"docs/parser.md":                "# Parser\n\nThe token parser validates delimiters before lexing.\n",
	}
	for relative, contents := range files {
		file := filepath.Join(root, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	commitAll(t, root, "parser neighbours")
	return root
}

// wideningIdentifierRepository adds legacy sources that spell the identifier
// wideningFusionTask names. No symbol extractor covers them, so only the
// lexical identifier channel ranks them by that spelling; the test-named ones
// stay beneath the confident tier, so the GPK-V0-092 identifier gate admits
// several widening rows.
func wideningIdentifierRepository(t *testing.T) string {
	t.Helper()
	root := wideningRepository(t)
	for _, name := range []string{"DelimiterRules", "DelimiterRulesConfig", "DelimiterRulesTest", "DelimiterRulesSmokeTest", "DelimiterRulesIntegrationTest"} {
		legacy := filepath.Join(root, "legacy", name+".java")
		if err := os.MkdirAll(filepath.Dir(legacy), 0o755); err != nil {
			t.Fatal(err)
		}
		contents := fmt.Sprintf("package legacy;\n\n// %s calls validateDelimiterRules on every token.\npublic class %s {\n  static boolean run(String input) { return validateDelimiterRules(input); }\n}\n", name, name)
		if err := os.WriteFile(legacy, []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// The governing file names the identifier too, so the lexical channel
	// ranks it; the widening tier must still leave it to the authority tier.
	agents, err := os.OpenFile(filepath.Join(root, "AGENTS.md"), os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := agents.WriteString("\nRun validateDelimiterRules before every parser change.\n"); err != nil {
		t.Fatal(err)
	}
	if err := agents.Close(); err != nil {
		t.Fatal(err)
	}
	commitAll(t, root, "legacy rules")
	return root
}

func commitAll(t *testing.T, root, message string) {
	t.Helper()
	for _, arguments := range [][]string{{"add", "-A"}, {"commit", "-qm", message}} {
		command := exec.Command("git", arguments...)
		command.Dir = root
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", arguments, err, output)
		}
	}
}

// queryRun is one in-process `corvint query`: the decoded receipt plus the
// exact bytes of the `context` member as the command wrote them.
type queryRun struct {
	receipt map[string]any
	raw     []byte
	stdout  []byte
	stderr  string
	code    int
}

func runQueryInProcess(t *testing.T, root string, arguments ...string) queryRun {
	t.Helper()
	var stdout, stderr bytes.Buffer
	argv := append([]string{"--root", root, "query"}, arguments...)
	code := run(argv, strings.NewReader(""), &stdout, &stderr)
	result := queryRun{stdout: stdout.Bytes(), stderr: stderr.String(), code: code}
	if code != 0 {
		return result
	}
	result.receipt, result.raw = decodeContextMember(t, stdout.Bytes())
	return result
}

func decodeContextMember(t *testing.T, output []byte) (map[string]any, []byte) {
	t.Helper()
	var envelope struct {
		Context json.RawMessage `json:"context"`
	}
	if err := json.Unmarshal(output, &envelope); err != nil {
		t.Fatalf("decode: %v: %s", err, output)
	}
	var receipt map[string]any
	if err := json.Unmarshal(envelope.Context, &receipt); err != nil {
		t.Fatalf("decode context: %v: %s", err, envelope.Context)
	}
	return receipt, envelope.Context
}

// assertPacketBytesDescribeReceipt compares coverage.packet_bytes and
// within_budget with the bytes the command actually wrote for the receipt.
func assertPacketBytesDescribeReceipt(t *testing.T, receipt map[string]any, raw []byte, budget int) {
	t.Helper()
	coverage := objectField(t, receipt, "coverage")
	if coverage["packet_bytes"] != float64(len(raw)) {
		t.Fatalf("packet_bytes=%v written=%d", coverage["packet_bytes"], len(raw))
	}
	if budget > 0 && (len(raw) > budget || coverage["within_budget"] != true || coverage["budget_bytes"] != float64(budget)) {
		t.Fatalf("budget=%d written=%d coverage=%v", budget, len(raw), coverage)
	}
}

func evidencePaths(t *testing.T, rows []any) map[string]bool {
	t.Helper()
	named := map[string]bool{}
	for _, row := range rows {
		for _, evidence := range arrayField(t, row.(map[string]any), "evidence") {
			named[evidence.(map[string]any)["path"].(string)] = true
		}
	}
	return named
}

// TestStandaloneQueryWidensBeneathConfidentResults pins GPK-V0-090: the
// standalone `corvint query` receipt carries an additive `widening` member of
// task-context rows the confident `results[]` do not already name, bounded by
// the requested limit, free of governing rows, and described by packet_bytes.
// The task names an identifier, as GPK-V0-092 requires of every widening row.
func TestStandaloneQueryWidensBeneathConfidentResults(t *testing.T) {
	t.Parallel()
	root := wideningIdentifierRepository(t)
	run := runQueryInProcess(t, root, "--task", wideningFusionTask, "--limit", "10")
	if run.code != 0 {
		t.Fatalf("exit=%d stderr=%s", run.code, run.stderr)
	}
	receipt := run.receipt
	widening := objectField(t, receipt, "widening")
	if widening["profile"] != "task-context" || widening["disclosure"] != queryWideningDisclosure {
		t.Fatalf("widening=%v", widening)
	}
	results := arrayField(t, receipt, "results")
	rows := arrayField(t, widening, "results")
	if widening["state"] != "READY" || len(rows) == 0 || len(results)+len(rows) > 10 {
		t.Fatalf("state=%v confident=%d widening=%d", widening["state"], len(results), len(rows))
	}
	named := evidencePaths(t, results)
	seen := map[string]bool{}
	for _, row := range rows {
		item := row.(map[string]any)
		id := item["id"].(string)
		if named[id] || seen[id] || item["kind"] == "governing" || id == "AGENTS.md" {
			t.Fatalf("widening row %v repeats a confident path, itself or carries authority", item)
		}
		seen[id] = true
	}
	if _, ok := widening["omitted_results"].(float64); !ok {
		t.Fatalf("omitted_results=%#v", widening["omitted_results"])
	}
	assertPacketBytesDescribeReceipt(t, receipt, run.raw, 0)
}

// TestStandaloneQueryWideningFillsQuota pins the GPK-V0-090 rule for a limit
// the confident rows already fill: the ranking still runs over the limit-sized
// window, an eligible row is counted in omitted_results with state
// QUOTA_FILLED, and NO_CANDIDATES is said only when the window held none.
// wideningMentionTask names a documentation path so the task-context ranking
// places a row the repository profile never ranks confidently at the head of
// its window, right after the reserved governing row, and names the identifier
// that page spells so the GPK-V0-092 identifier gate admits it.
const wideningMentionTask = repositoryQueryTask + " described in docs/parser.md under delimiterRuleTable"

// TestStandaloneQueryWideningFillsQuota pins the GPK-V0-090 rule that the
// widening ranking always runs: when the confident rows fill the limit, an
// eligible candidate in the window is counted in `omitted_results` under
// `QUOTA_FILLED`, and `NO_CANDIDATES` means the window held nothing eligible.
// The fixture adds a second confident declaration so a two-row window has
// room for the mentioned documentation row beside the reserved governing row.
func TestStandaloneQueryWideningFillsQuota(t *testing.T) {
	t.Parallel()
	root := wideningRepository(t)
	file := filepath.Join(root, "internal", "parser", "token_validate.go")
	contents := "package parser\n\n// ValidateToken validates token parser delimiters.\nfunc ValidateToken(input string) bool { return input != \"\" }\n"
	if err := os.WriteFile(file, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
	page := "# Parser\n\nThe token parser validates delimiters before lexing; delimiterRuleTable holds the rules.\n"
	if err := os.WriteFile(filepath.Join(root, "docs", "parser.md"), []byte(page), 0o644); err != nil {
		t.Fatal(err)
	}
	commitAll(t, root, "second parser declaration")
	wide := runQueryInProcess(t, root, "--task", wideningMentionTask, "--limit", "10")
	confident := len(arrayField(t, wide.receipt, "results"))
	if confident < 2 || len(arrayField(t, objectField(t, wide.receipt, "widening"), "results")) == 0 {
		t.Fatalf("fixture needs two confident rows and widening rows: %v", wide.receipt)
	}
	filled := 0
	for limit := 1; limit <= confident; limit++ {
		run := runQueryInProcess(t, root, "--task", wideningMentionTask, "--limit", strconv.Itoa(limit))
		if run.code != 0 {
			t.Fatalf("limit %d exit=%d stderr=%s", limit, run.code, run.stderr)
		}
		if len(arrayField(t, run.receipt, "results")) != limit {
			t.Fatalf("limit %d left %d confident rows; the quota is not filled", limit, len(arrayField(t, run.receipt, "results")))
		}
		widening := objectField(t, run.receipt, "widening")
		omitted := widening["omitted_results"].(float64)
		if len(arrayField(t, widening, "results")) != 0 {
			t.Fatalf("limit %d showed widening rows past the quota: %v", limit, widening)
		}
		switch widening["state"] {
		case "QUOTA_FILLED":
			if omitted < 1 {
				t.Fatalf("limit %d QUOTA_FILLED without an omitted row: %v", limit, widening)
			}
			filled++
		case "NO_CANDIDATES":
			if omitted != 0 {
				t.Fatalf("limit %d NO_CANDIDATES with omitted rows: %v", limit, widening)
			}
		default:
			t.Fatalf("limit %d widening state %v", limit, widening["state"])
		}
		assertPacketBytesDescribeReceipt(t, run.receipt, run.raw, 0)
	}
	if filled == 0 {
		t.Fatalf("no limit in 1..%d met an eligible row in its window; the fixture cannot show QUOTA_FILLED", confident)
	}
}

// TestStandaloneQueryWideningYieldsBudgetToResults pins the GPK-V0-090 budget
// rule: a budgeted receipt still fits, widening rows are dropped from the tail
// before any confident row, the confident rows that remain are byte-for-byte
// a prefix of the unbudgeted ones, and the member disappears when no row fits.
func TestStandaloneQueryWideningYieldsBudgetToResults(t *testing.T) {
	t.Parallel()
	root := wideningIdentifierRepository(t)
	unbudgeted := runQueryInProcess(t, root, "--task", wideningFusionTask, "--limit", "10")
	confident := arrayField(t, unbudgeted.receipt, "results")
	wideRows := arrayField(t, objectField(t, unbudgeted.receipt, "widening"), "results")
	widened, dropped := 0, false
	// Budgets relative to the unbudgeted receipt: one that drops the member,
	// one in the middle, one that keeps part of the member and one just under
	// the unbudgeted size (a budgeted receipt first sheds optional members).
	for _, budget := range []int{1500, len(unbudgeted.raw) * 3 / 4, len(unbudgeted.raw) * 7 / 8, len(unbudgeted.raw) - 1} {
		run := runQueryInProcess(t, root, "--task", wideningFusionTask, "--limit", "10", "--budget-bytes", strconv.Itoa(budget))
		if run.code != 0 {
			t.Fatalf("budget %d exit=%d stderr=%s", budget, run.code, run.stderr)
		}
		receipt := run.receipt
		assertPacketBytesDescribeReceipt(t, receipt, run.raw, budget)
		kept := arrayField(t, receipt, "results")
		if len(kept) > len(confident) || !reflect.DeepEqual(kept, confident[:len(kept)]) {
			t.Fatalf("budget %d changed the confident rows: kept %d of %d, prefix equal %v", budget, len(kept), len(confident), reflect.DeepEqual(kept, confident[:min(len(kept), len(confident))]))
		}
		if widening, ok := receipt["widening"].(map[string]any); ok {
			rows := arrayField(t, widening, "results")
			if len(rows) == 0 {
				t.Fatalf("budget %d kept an empty %v widening member: %v", budget, widening["state"], widening)
			}
			if len(kept) != len(confident) {
				t.Fatalf("budget %d dropped a confident row while %d widening rows remained", budget, len(rows))
			}
			if !reflect.DeepEqual(rows, wideRows[:len(rows)]) {
				t.Fatalf("budget %d widening rows are not a prefix of the unbudgeted ones", budget)
			}
			if len(rows) < len(wideRows) {
				if widening["state"] != "BUDGETED" || widening["omitted_results"].(float64) < float64(len(wideRows)-len(rows)) {
					t.Fatalf("budget %d dropped %d widening rows without saying so: %v", budget, len(wideRows)-len(rows), widening)
				}
				dropped = true
			}
			widened += len(rows)
		}
	}
	if widened == 0 || !dropped {
		t.Fatalf("budgets admitted %d widening rows and tail-dropped %v; the fixture cannot show the tail-drop rule", widened, dropped)
	}
}

// TestHarnessUserPromptCarriesNoWidening pins the GPK-V0-090 boundary: the
// harness user-prompt packet over the same repository and task keeps its
// shape, so hidden host context neither grows nor changes bytes.
func TestHarnessUserPromptCarriesNoWidening(t *testing.T) {
	t.Parallel()
	root := queryCLIRepository(t)
	input, _ := json.Marshal(map[string]string{"task": repositoryQueryTask})
	stdout := runHarnessEvent(t, root, "user-prompt", string(input))
	receipt, _ := decodeContextMember(t, stdout)
	if _, present := receipt["widening"]; present {
		t.Fatalf("harness user-prompt packet carries widening: %s", stdout)
	}
	if _, present := receipt["degradations"]; present {
		t.Fatalf("harness user-prompt packet carries degradations: %s", stdout)
	}
}

// TestProjectOperationsQueryDegradesWithoutRootAgentsFile pins GPK-V0-091: a
// project-operations task over a repository with no uniquely highest root
// AGENTS.md answers through the repository profile with the degradation named
// in `degradations` and in coverage.uncertainty, instead of refusing.
func TestProjectOperationsQueryDegradesWithoutRootAgentsFile(t *testing.T) {
	t.Parallel()
	root := ledgerRepository(t)
	for _, budget := range []int{0, 1500} {
		arguments := []string{"--task", projectOperationsTask, "--limit", "5"}
		if budget > 0 {
			arguments = append(arguments, "--budget-bytes", strconv.Itoa(budget))
		}
		run := runQueryInProcess(t, root, arguments...)
		if run.code != 0 {
			t.Fatalf("budget %d exit=%d stderr=%s", budget, run.code, run.stderr)
		}
		receipt, stdout := run.receipt, run.stdout
		if intent := objectField(t, receipt, "intent"); intent["id"] != "project-operations" {
			t.Fatalf("intent=%v", intent)
		}
		degradations := arrayField(t, receipt, "degradations")
		if len(degradations) != 1 || degradations[0] != "authority-start-unavailable" {
			t.Fatalf("degradations=%v", degradations)
		}
		if !bytes.Contains(stdout, []byte(`"authority-start profile unavailable: no uniquely highest-ranked root AGENTS.md`)) {
			t.Fatalf("uncertainty does not state the degradation: %s", stdout)
		}
		if receipt["mode"] != "query" || receipt["state"] == "" {
			t.Fatalf("receipt=%v", receipt)
		}
		assertPacketBytesDescribeReceipt(t, receipt, run.raw, budget)
	}
}

// TestProjectOperationsQueryStillRefusesSymlinkAuthority pins the GPK-V0-091
// boundary: only the missing-root refusal degrades; a root AGENTS.md that is a
// symlink keeps its unsupported-query-authority refusal.
func TestProjectOperationsQueryStillRefusesSymlinkAuthority(t *testing.T) {
	t.Parallel()
	root := queryCLIRepository(t)
	if err := os.Remove(filepath.Join(root, "AGENTS.md")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("script/roadmap.sh", filepath.Join(root, "AGENTS.md")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	commitAll(t, root, "symlink authority")
	run := runQueryInProcess(t, root, "--task", authorityStartPrompt, "--limit", "1")
	if run.code == 0 || !strings.Contains(run.stderr+string(run.stdout), "unsupported-query-authority") {
		t.Fatalf("exit=%d stdout=%s stderr=%s", run.code, run.stdout, run.stderr)
	}
}

// wideningFusionTask names one identifier that only a source with no symbol
// extractor spells, so the lexical identifier channel is the sole list that
// can rank it (GPK-V0-092).
const wideningFusionTask = repositoryQueryTask + " in validateDelimiterRules"

// TestStandaloneQueryWideningFusesLexicalChannel pins GPK-V0-092: the
// widening rows are the reciprocal-rank fusion of the task-context ranking and
// the lexical identifier channel, each row explains its position with its
// rank in each list and the task identifiers it names, the member discloses
// the ranking and what the channel searched for, and a source the structural
// ranking cannot reach is still a candidate when it spells the identifier.
func TestStandaloneQueryWideningFusesLexicalChannel(t *testing.T) {
	t.Parallel()
	root := wideningIdentifierRepository(t)
	run := runQueryInProcess(t, root, "--task", wideningFusionTask, "--limit", "10")
	if run.code != 0 {
		t.Fatalf("exit=%d stderr=%s", run.code, run.stderr)
	}
	widening := objectField(t, run.receipt, "widening")
	ranking := objectField(t, widening, "ranking")
	if ranking["method"] != "rrf" || ranking["constant"] != float64(queryFusionConstant) {
		t.Fatalf("ranking=%v", ranking)
	}
	lexical := objectField(t, widening, "lexical")
	if !reflect.DeepEqual(lexical["identifiers"], []any{"validateDelimiterRules"}) {
		t.Fatalf("lexical=%v, want the task identifier searched", lexical)
	}
	rows := arrayField(t, widening, "results")
	if widening["state"] != "READY" || len(rows) == 0 {
		t.Fatalf("state=%v rows=%d", widening["state"], len(rows))
	}
	confident := evidencePaths(t, arrayField(t, run.receipt, "results"))
	var legacyRow map[string]any
	for _, row := range rows {
		item := row.(map[string]any)
		if confident[item["id"].(string)] {
			t.Fatalf("widening row %v repeats a confident result", item["id"])
		}
		if item["id"] == "AGENTS.md" || item["kind"] == "governing" {
			t.Fatalf("widening row %v carries the reserved governing file through the lexical channel", item["id"])
		}
		fusion := objectField(t, item, "fusion")
		rrf, _ := fusion["rrf_micro"].(float64)
		_, structural := fusion["structural_rank"].(float64)
		_, lexicalRank := fusion["lexical_rank"].(float64)
		if rrf <= 0 || !(structural || lexicalRank) || !strings.Contains(fusion["reason"].(string), "rrf ") {
			t.Fatalf("widening row %v lacks a fusion explanation", item)
		}
		if item["id"] == "legacy/DelimiterRules.java" {
			legacyRow = item
		}
	}
	// The source spelling the identifier is a candidate: confident when the
	// slot ranking already pinned it, otherwise a widening row explained by
	// the identifier it names.
	switch {
	case confident["legacy/DelimiterRules.java"]:
	case legacyRow == nil:
		t.Fatalf("legacy/DelimiterRules.java missing from results and widening rows: %v", rows)
	default:
		fusion := objectField(t, legacyRow, "fusion")
		if !reflect.DeepEqual(fusion["identifiers"], []any{"validateDelimiterRules"}) || !strings.Contains(fusion["reason"].(string), "names validateDelimiterRules") {
			t.Fatalf("legacy fusion=%v, want the named identifier explained", fusion)
		}
	}
	if _, present := widening["uncertainty"]; present {
		t.Fatalf("uncertainty=%v for a task that names an identifier", widening["uncertainty"])
	}
	// Fused order is by score, descending; the ranking the member declares
	// must be the ranking the rows carry.
	previous := 2e6
	for _, row := range rows {
		rrf := objectField(t, row.(map[string]any), "fusion")["rrf_micro"].(float64)
		if rrf > previous {
			t.Fatalf("widening rows are not in fused order: %v", rows)
		}
		previous = rrf
	}
	assertPacketBytesDescribeReceipt(t, run.receipt, run.raw, 0)

	// A task that names no identifier says so: its lexical candidates rest on
	// prose terms (product invariant 2).
	prose := runQueryInProcess(t, root, "--task", repositoryQueryTask, "--limit", "10")
	if prose.code != 0 {
		t.Fatalf("exit=%d stderr=%s", prose.code, prose.stderr)
	}
	proseWidening := objectField(t, prose.receipt, "widening")
	uncertainty, _ := proseWidening["uncertainty"].([]any)
	if len(uncertainty) != 1 || !strings.Contains(uncertainty[0].(string), "names no identifier") {
		t.Fatalf("uncertainty=%v, want the identifier-free disclosure", proseWidening["uncertainty"])
	}
}
