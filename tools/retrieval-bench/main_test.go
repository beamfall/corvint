package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"
)

var startedAt = time.Now()

// fixtureSnapshot is a snapshot without .git: a tiny repository whose gold
// file is findable by grep and whose given file must never be a success.
func fixtureSnapshot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		"src/help.rs":        "fn quote_default_value() {}\n",
		"src/format.rs":      "fn format_error() {}\n",
		"tests/help_test.rs": "#[test] fn quote_empty_default_value_in_help() {}\n",
		"README.md":          "clap help output\n",
		"assets/logo.bin":    "PNG\x00\x00binary quote default help",
	}
	for path, content := range files {
		full := filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

const fixtureSamples = `{"id":"pos","task_type":"code2test","repo":"clap-rs/clap","base_commit":"68b5ff900bae8ee1a0e328c1a2301a7985e4f1c6","query":{"pr_title":"Quote empty default values in help output","changed_file":"src/help.rs"},"gold":{"related_tests":["tests/help_test.rs"],"negative_distractors":["src/format.rs"]},"metadata":{}}
{"id":"ctx","task_type":"comment2context","repo":"clap-rs/clap","base_commit":"68b5ff900bae8ee1a0e328c1a2301a7985e4f1c6","query":{"path":"src/help.rs","review_comment":"quote default value"},"gold":{"root_cause_files":["src/help.rs","tests/help_test.rs"]},"metadata":{}}
{"id":"nat","task_type":"abstention","repo":"clap-rs/clap","base_commit":"68b5ff900bae8ee1a0e328c1a2301a7985e4f1c6","query":{"text":"zzzz qqqq"},"gold":{"files":[],"no_gold":true,"reason":"upstream_dependency"},"metadata":{"organic":true}}
{"id":"cf","task_type":"abstention","repo":"clap-rs/clap","base_commit":"68b5ff900bae8ee1a0e328c1a2301a7985e4f1c6","query":{"text":"help output"},"gold":{"files":[],"no_gold":true},"metadata":{"organic":false}}
`

func fakeCorvint(answers map[string]arm) retriever {
	return func(ctx context.Context, corvintGo, root, task string, limit int) (arm, error) {
		if _, err := os.Stat(filepath.Join(root, ".git")); err != nil {
			return arm{}, err
		}
		keys := make([]string, 0, len(answers))
		for key := range answers {
			keys = append(keys, key)
		}
		sort.Slice(keys, func(left, right int) bool { return len(keys[left]) > len(keys[right]) })
		for _, key := range keys {
			if strings.Contains(task, key) {
				return answers[key], nil
			}
		}
		return arm{State: "OUT_OF_SCOPE", Abstained: true}, nil
	}
}

// fakeImpact answers by changed file; any other path is refused the way
// `corvint impact` refuses a path it has no rule for.
func fakeImpact(answers map[string]arm) impactRetriever {
	return func(ctx context.Context, corvintGo, root, changed string, limit int) (arm, error) {
		if _, err := os.Stat(filepath.Join(root, ".git")); err != nil {
			return arm{}, err
		}
		answer, known := answers[changed]
		if !known {
			return arm{}, errors.New("corvint impact: exit status 2: unsupported-impact-path-suffix")
		}
		return answer, nil
	}
}

// fakeAffected answers by changed file; any other path is refused the way
// `corvint affected` refuses a path the copy does not hold.
func fakeAffected(answers map[string]arm) affectedRetriever {
	return func(ctx context.Context, corvintGo, root, changed string, limit int) (arm, error) {
		if _, err := os.Stat(filepath.Join(root, ".git")); err != nil {
			return arm{}, err
		}
		answer, known := answers[changed]
		if !known {
			return arm{}, errors.New("corvint affected: open " + changed + ": no such file or directory")
		}
		return answer, nil
	}
}

func fixtureOptions(t *testing.T, snapshot string) options {
	t.Helper()
	samples := filepath.Join(t.TempDir(), "samples.jsonl")
	if err := os.WriteFile(samples, []byte(fixtureSamples), 0o644); err != nil {
		t.Fatal(err)
	}
	corvintGo := filepath.Join(t.TempDir(), "corvint")
	if err := os.WriteFile(corvintGo, []byte("#!/bin/sh\necho Corvint test\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return options{samples: samples, snapshots: map[string]string{"clap-rs/clap@68b5ff900bae8ee1a0e328c1a2301a7985e4f1c6": snapshot}, corvintGo: corvintGo, limit: 5, taskTypes: map[string]bool{}, arms: allArms()}
}

// TestBenchScoresAllArmsAndRemovesItsCopies: the snapshot has no .git, so
// the harness materializes one Git copy for every sample sharing the commit,
// scores the four arms per the bench's rules, and leaves no copy behind.
func TestBenchScoresAllArmsAndRemovesItsCopies(t *testing.T) {
	// bench() stages its snapshot copy under os.MkdirTemp("", ...), i.e. the
	// process-wide temp dir; a concurrent test process can create its own
	// "corvint-retrieval-bench-*" entry there between this test's run and the
	// leftover-copy check below, producing a false "copy left behind". Give
	// this test a private TMPDIR so both the copy and the check are scoped
	// to it alone.
	t.Setenv("TMPDIR", t.TempDir())
	snapshot := fixtureSnapshot(t)
	answers := map[string]arm{
		"Quote empty default": {Ranked: []string{"src/help.rs", "tests/help_test.rs"}, State: "READY"},
		"quote default value": {Ranked: []string{"src/help.rs", "src/format.rs"}, State: "READY"},
		"help output":         {Ranked: []string{"README.md"}, State: "READY"},
	}
	impacts := map[string]arm{"src/help.rs": {Ranked: []string{"src/help.rs", "tests/help_test.rs"}, State: "READY", PacketBytes: 42}}
	affecteds := map[string]arm{"src/help.rs": {Ranked: []string{"tests/help_test.rs", "src/help.rs"}, State: "UNKNOWN", PacketBytes: 77}}
	report, err := bench(context.Background(), fixtureOptions(t, snapshot), fakeCorvint(answers), fakeCorvint(answers), fakeImpact(impacts), fakeAffected(affecteds))
	if err != nil {
		t.Fatal(err)
	}
	details := report["details"].([]sampleReport)
	if len(details) != 4 {
		t.Fatalf("details = %d", len(details))
	}
	byID := map[string]sampleReport{}
	for _, detail := range details {
		byID[detail.ID] = detail
	}
	positive := byID["pos"]
	if positive.Stratum != "positive" || strings.Join(positive.Gold, ",") != "tests/help_test.rs" {
		t.Fatalf("pos: %+v", positive)
	}
	if got := positive.Metrics["corvint"]; got["mrr@k"] != 0.5 || got["recall@5"] != 1 || got["hit@k"] != 1 || got["selective_success"] != 1 || got["hard_negative_hits@k"] != 0 {
		t.Fatalf("pos corvint metrics: %v", got)
	}
	if context := positive.Arms["context"]; strings.Join(context.Ranked, ",") != "src/help.rs,tests/help_test.rs" || context.Abstained || context.Error != "" {
		t.Fatalf("pos context: %+v", context)
	}
	grep := positive.Arms["grep"]
	if len(grep.Ranked) == 0 || grep.Ranked[0] != "tests/help_test.rs" || grep.Abstained {
		t.Fatalf("pos grep: %+v", grep)
	}
	for _, path := range grep.Ranked {
		if path == "assets/logo.bin" {
			t.Fatal("grep ranked a binary file")
		}
	}
	impact := positive.Arms["impact"]
	if strings.Join(impact.Ranked, ",") != "tests/help_test.rs" || impact.Abstained || impact.Error != "" || impact.PacketBytes != 42 {
		t.Fatalf("pos impact kept the changed file or lost the packet: %+v", impact)
	}
	if got := positive.Metrics["impact"]; got["hit@k"] != 1 || got["mrr@k"] != 1 || got["selective_success"] != 1 {
		t.Fatalf("pos impact metrics: %v", got)
	}
	affected := positive.Arms["affected"]
	if strings.Join(affected.Ranked, ",") != "tests/help_test.rs" || affected.Abstained || affected.Error != "" || affected.State != "UNKNOWN" || affected.PacketBytes != 77 {
		t.Fatalf("pos affected kept the changed file or lost the plan: %+v", affected)
	}
	if got := positive.Metrics["affected"]; got["hit@k"] != 1 || got["mrr@k"] != 1 || got["selective_success"] != 1 {
		t.Fatalf("pos affected metrics: %v", got)
	}
	context := byID["ctx"]
	if impact := context.Arms["impact"]; impact.Error != "no changed_file in query" || len(impact.Ranked) != 0 {
		t.Fatalf("ctx impact without changed_file: %+v", impact)
	}
	if affected := context.Arms["affected"]; affected.Error != "no changed_file in query" || len(affected.Ranked) != 0 {
		t.Fatalf("ctx affected without changed_file: %+v", affected)
	}
	if strings.Join(context.Given, ",") != "src/help.rs" || strings.Join(context.Gold, ",") != "src/help.rs,tests/help_test.rs" {
		t.Fatalf("ctx gold/given: %+v", context)
	}
	if ranked := context.Arms["corvint"].Ranked; strings.Join(ranked, ",") != "src/format.rs" {
		t.Fatalf("ctx corvint kept the given file: %v", ranked)
	}
	if got := context.Metrics["corvint"]; got["hard_negative_hits@k"] != 0 || got["hit@k"] != 0 || got["selective_success"] != 0 {
		t.Fatalf("ctx corvint metrics: %v", got)
	}
	for _, path := range context.Arms["grep"].Ranked {
		if path == "src/help.rs" {
			t.Fatal("grep ranked the given file")
		}
	}
	natural, counterfactual := byID["nat"], byID["cf"]
	if natural.Stratum != "natural_no_gold" || counterfactual.Stratum != "counterfactual_no_gold" {
		t.Fatalf("strata: %s %s", natural.Stratum, counterfactual.Stratum)
	}
	if natural.Metrics["corvint"]["selective_success"] != 1 || natural.Metrics["grep"]["selective_success"] != 1 {
		t.Fatalf("natural no-gold should be an abstention for both arms: %v", natural.Metrics)
	}
	if counterfactual.Metrics["corvint"]["selective_success"] != 0 || counterfactual.Metrics["grep"]["selective_success"] != 0 {
		t.Fatalf("counterfactual answered by both arms should fail: %v", counterfactual.Metrics)
	}
	arms := report["arms"].(map[string]any)
	corvintSummary := arms["corvint"].(map[string]any)
	all := corvintSummary["all"].(map[string]any)
	if all["n"] != 4 || all["positives"] != 2 || all["selective_success"] != 0.5 {
		t.Fatalf("corvint all: %v", all)
	}
	intervals := corvintSummary["intervals"].(map[string]any)
	selective := intervals["selective_success"].(map[string]any)
	if selective["n"] != 4 || selective["rate"] != 0.5 || selective["low"].(float64) >= 0.5 || selective["high"].(float64) <= 0.5 {
		t.Fatalf("selective interval: %v", selective)
	}
	if hit := intervals["hit@5"].(map[string]any); hit["n"] != 2 {
		t.Fatalf("hit interval: %v", hit)
	}
	if contextSummary := arms["context"].(map[string]any); contextSummary["errors"] != 0 || contextSummary["all"].(map[string]any)["n"] != 4 {
		t.Fatalf("context summary: %v", contextSummary)
	}
	impactSummary := arms["impact"].(map[string]any)
	if impactSummary["errors"] != 3 || impactSummary["all"].(map[string]any)["n"] != 4 {
		t.Fatalf("impact summary: %v", impactSummary)
	}
	affectedSummary := arms["affected"].(map[string]any)
	if affectedSummary["errors"] != 3 || affectedSummary["all"].(map[string]any)["n"] != 4 || affectedSummary["task:code2test"].(map[string]any)["hit@k"] != 1.0 {
		t.Fatalf("affected summary: %v", affectedSummary)
	}
	matches, _ := filepath.Glob(filepath.Join(os.TempDir(), "corvint-retrieval-bench-*"))
	for _, match := range matches {
		if info, err := os.Stat(match); err == nil && info.ModTime().After(startedAt) {
			t.Fatalf("copy left behind: %s", match)
		}
	}
	if _, err := os.Stat(filepath.Join(snapshot, ".git")); !os.IsNotExist(err) {
		t.Fatal("the snapshot itself was turned into a repository")
	}
}

// TestRunIsDeterministic: two runs over the same inputs are byte-identical
// once the measured wall times, the one section a rerun may change, are
// removed.
func TestRunIsDeterministic(t *testing.T) {
	snapshot := fixtureSnapshot(t)
	configuration := fixtureOptions(t, snapshot)
	answers := map[string]arm{"Quote empty": {Ranked: []string{"tests/help_test.rs"}, State: "READY"}}
	impacts := map[string]arm{"src/help.rs": {Ranked: []string{"tests/help_test.rs", "src/help.rs"}, State: "READY", PacketBytes: 7}}
	affecteds := map[string]arm{"src/help.rs": {Ranked: []string{"tests/help_test.rs"}, State: "BOUNDED", PacketBytes: 9}}
	encode := func() []byte {
		report, err := bench(context.Background(), configuration, fakeCorvint(answers), fakeCorvint(answers), fakeImpact(impacts), fakeAffected(affecteds))
		if err != nil {
			t.Fatal(err)
		}
		delete(report, "latency")
		for _, detail := range report["details"].([]sampleReport) {
			for name, answer := range detail.Arms {
				answer.WallMillis = 0
				detail.Arms[name] = answer
			}
		}
		encoded, err := json.Marshal(report)
		if err != nil {
			t.Fatal(err)
		}
		return encoded
	}
	if first, second := encode(), encode(); !bytes.Equal(first, second) {
		t.Fatalf("reruns differ:\n%s\n%s", first, second)
	}
}

// TestGoldAndGivenFollowTheBenchRules pins baseline.py's target_gold_files
// and given_files, including object-shaped paths and the no-gold override.
func TestGoldAndGivenFollowTheBenchRules(t *testing.T) {
	for _, test := range []struct {
		name  string
		item  sample
		gold  string
		given string
	}{
		{"explicit files win", sample{TaskType: "code2test", Gold: map[string]any{"files": []any{map[string]any{"path": "a.go"}, "a.go", "b.go"}, "related_tests": []any{"t.go"}}}, "a.go,b.go", ""},
		{"code2test", sample{TaskType: "code2test", Gold: map[string]any{"related_tests": []any{"t.go"}, "root_cause_files": []any{"r.go"}}}, "t.go", ""},
		{"comment2context context", sample{TaskType: "comment2context", Query: map[string]any{"path": "seen.go"}, Gold: map[string]any{"must_context_files": []any{"c.go"}, "root_cause_files": []any{"r.go"}}}, "c.go", "seen.go"},
		{"comment2context fallback", sample{TaskType: "comment2context", Query: map[string]any{"given_file": "g.go"}, Gold: map[string]any{"root_cause_files": []any{"r.go"}}}, "r.go", "g.go"},
		{"trace2code", sample{TaskType: "trace2code", Gold: map[string]any{"root_cause_files": []any{"r.go"}, "related_tests": []any{"t.go"}}}, "r.go", ""},
		{"edit2ripple given", sample{TaskType: "edit2ripple", Gold: map[string]any{"given_files": []any{"anchor.go"}, "related_tests": []any{"t.go"}}}, "t.go", "anchor.go"},
		{"no gold", sample{TaskType: "abstention", Gold: map[string]any{"no_gold": true, "files": []any{"x.go"}}}, "", ""},
	} {
		if gold := strings.Join(goldFiles(test.item), ","); gold != test.gold {
			t.Errorf("%s: gold %q, want %q", test.name, gold, test.gold)
		}
		if given := strings.Join(givenFiles(test.item), ","); given != test.given {
			t.Errorf("%s: given %q, want %q", test.name, given, test.given)
		}
	}
}

// TestQueryTextMatchesTheBenchAndCorvintSeesAtMostTwoThousandCharacters.
func TestQueryTextMatchesTheBenchAndCorvintSeesAtMostTwoThousandCharacters(t *testing.T) {
	item := sample{Query: map[string]any{"pr_title": "b<c", "changed_file": "a.go", "n": 1.0, "f": 1.5, "l": []any{"x", nil, true, map[string]any{}}}}
	if text := queryText(item); text != `{"changed_file": "a.go", "f": 1.5, "l": ["x", null, true, {}], "n": 1.0, "pr_title": "b<c"}` {
		t.Fatalf("queryText = %q", text)
	}
	if text := queryText(sample{}); text != "{}" {
		t.Fatalf("queryText of no query = %q", text)
	}
	long := strings.Repeat("é", maxQueryChars+1)
	task, truncated := truncateQuery(long)
	if !truncated || len([]rune(task)) != maxQueryChars {
		t.Fatalf("truncate: %d runes, truncated=%v", len([]rune(task)), truncated)
	}
	if _, truncated := truncateQuery("short"); truncated {
		t.Fatal("short text truncated")
	}
	if got := strings.Join(terms("QuoteEmpty default_value help a"), ","); got != "default,empty,help,quote,value" {
		t.Fatalf("terms = %q", got)
	}
}

// TestJudgeContextUsesTheFullBenchQueryText: context has its own larger input
// bound, so it sees the canonical bench query rather than query's 2,000-rune prefix.
func TestJudgeContextUsesTheFullBenchQueryText(t *testing.T) {
	item := sample{Query: map[string]any{"detail": strings.Repeat("x", maxQueryChars+1)}, Gold: map[string]any{"files": []any{"result.go"}}}
	full := queryText(item)
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	contextCalled := false
	contextPacket := retriever(func(ctx context.Context, corvintGo, gotRoot, task string, limit int) (arm, error) {
		contextCalled = true
		if gotRoot != root || task != full || len([]rune(task)) <= maxQueryChars {
			t.Fatalf("context task was truncated or routed to the wrong root: %d runes", len([]rune(task)))
		}
		return arm{Ranked: []string{"result.go"}, State: "READY"}, nil
	})
	report := judge(context.Background(), options{limit: 5, arms: allArms()}, fakeCorvint(nil), contextPacket, fakeImpact(nil), fakeAffected(nil), item, root)
	if !contextCalled || report.Arms["context"].Error != "" || report.Metrics["context"]["hit@k"] != 1 {
		t.Fatalf("context arm: %+v", report)
	}
}

// TestParsePacketRanksEachResultOncePerPath and reads abstention from state.
func TestParsePacketRanksEachResultOncePerPath(t *testing.T) {
	packet := `{"context":{"state":"READY","results":[{"evidence":[{"path":"a.go"},{"path":"z.md"}]},{"evidence":[{"path":"a.go"}]},{"evidence":[]},{"evidence":[{"path":"b.go"}]}]}}`
	answer, err := parsePacket([]byte(packet))
	if err != nil || strings.Join(answer.Ranked, ",") != "a.go,b.go" || answer.Abstained || answer.PacketBytes != len(packet) {
		t.Fatalf("answer %+v err %v", answer, err)
	}
	if answer, _ := parsePacket([]byte(`{"context":{"state":"OUT_OF_SCOPE","results":[]}}`)); !answer.Abstained {
		t.Fatal("OUT_OF_SCOPE is an abstention")
	}
	if _, err := parsePacket([]byte("{")); err == nil {
		t.Fatal("unreadable packet accepted")
	}
}

// TestParseContextPacketRanksResultIDs: context has its own top-level packet
// and ranks result IDs, rather than reusing query's nested envelope.
func TestParseContextPacketRanksResultIDs(t *testing.T) {
	packet := `{"state":"READY","results":[{"id":"a.go","evidence":[{"path":"a.go"}]},{"id":"a.go","evidence":[{"path":"a.go"}]},{"id":"b_test.go","evidence":[{"path":"b_test.go"}]}]}`
	answer, err := parseContextPacket([]byte(packet))
	if err != nil || strings.Join(answer.Ranked, ",") != "a.go,b_test.go" || answer.Abstained || answer.PacketBytes != len(packet) {
		t.Fatalf("answer %+v err %v", answer, err)
	}
	if answer, _ := parseContextPacket([]byte(`{"state":"NO_CANDIDATES","results":[]}`)); !answer.Abstained {
		t.Fatal("NO_CANDIDATES is an abstention")
	}
	if _, err := parseContextPacket([]byte("{")); err == nil {
		t.Fatal("unreadable packet accepted")
	}
}

// TestParseImpactPacketRanksEveryEvidencePathOnce: every evidence row counts,
// in packet order, and abstention waits for judgeImpact.
func TestParseImpactPacketRanksEveryEvidencePathOnce(t *testing.T) {
	packet := `{"context":{"state":"READY","results":[{"evidence":[{"path":"a.go"},{"path":"a_test.go"}]},{"evidence":[{"path":"a_test.go"},{"path":"b_test.go"}]},{"evidence":[]}]}}`
	answer, err := parseImpactPacket([]byte(packet))
	if err != nil || strings.Join(answer.Ranked, ",") != "a.go,a_test.go,b_test.go" || answer.State != "READY" || answer.PacketBytes != len(packet) {
		t.Fatalf("answer %+v err %v", answer, err)
	}
	if _, err := parseImpactPacket([]byte("{")); err == nil {
		t.Fatal("unreadable packet accepted")
	}
	only := fakeImpact(map[string]arm{"a.go": {Ranked: []string{"a.go"}, State: "READY"}})
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	item := sample{Query: map[string]any{"changed_file": "a.go"}}
	if answer := judgeImpact(context.Background(), options{limit: 5}, only, item, root, nil); !answer.Abstained || len(answer.Ranked) != 0 {
		t.Fatalf("a packet holding only the changed file is an abstention: %+v", answer)
	}
	if answer := judgeImpact(context.Background(), options{limit: 5}, only, sample{Query: map[string]any{"changed_file": "z.rs"}}, root, nil); answer.Error == "" || answer.Abstained {
		t.Fatalf("a refusal is an error, not an abstention: %+v", answer)
	}
}

// TestParseAffectedPlanRanksEverySelectedTestOnce: every selected unit's
// tests count, in plan order, the scope is the state, and abstention waits
// for judgeAffected.
func TestParseAffectedPlanRanksEverySelectedTestOnce(t *testing.T) {
	document := `{"plan":{"scope":"UNKNOWN","selected":[{"tests":["a_test.go","b_test.go"]},{"tests":["b_test.go","c_test.go"]},{"tests":[]}]}}`
	answer, err := parseAffectedPlan([]byte(document))
	if err != nil || strings.Join(answer.Ranked, ",") != "a_test.go,b_test.go,c_test.go" || answer.State != "UNKNOWN" || answer.PacketBytes != len(document) {
		t.Fatalf("answer %+v err %v", answer, err)
	}
	if _, err := parseAffectedPlan([]byte("{")); err == nil {
		t.Fatal("unreadable plan accepted")
	}
	only := fakeAffected(map[string]arm{"a.go": {Ranked: []string{"a.go"}, State: "BOUNDED"}})
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	item := sample{Query: map[string]any{"changed_file": "a.go"}}
	if answer := judgeAffected(context.Background(), options{limit: 5}, only, item, root, nil); !answer.Abstained || len(answer.Ranked) != 0 {
		t.Fatalf("a plan selecting only the changed file is an abstention: %+v", answer)
	}
	if answer := judgeAffected(context.Background(), options{limit: 5}, only, sample{Query: map[string]any{"changed_file": "z.rs"}}, root, nil); answer.Error == "" || answer.Abstained {
		t.Fatalf("a refusal is an error, not an abstention: %+v", answer)
	}
}

// TestRunAffectedDirtiesThenRestoresTheCopy drives the real affected arm
// through a shell script standing in for corvint on a temporary Git
// repository: the script sees the changed file dirty, the file is restored
// byte for byte, the copy is clean afterwards, and a refusal (exit 2) is an
// error that still restores the file.
func TestRunAffectedDirtiesThenRestoresTheCopy(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the fake corvint is a POSIX shell script")
	}
	ctx := context.Background()
	root := t.TempDir()
	original := []byte("package a\n\nfunc A() {}\n")
	if err := os.WriteFile(filepath.Join(root, "a.go"), original, 0o644); err != nil {
		t.Fatal(err)
	}
	for _, arguments := range [][]string{{"init", "-q"}, {"add", "-A"}, {"commit", "-q", "-m", "base"}} {
		if _, err := git(ctx, root, arguments...); err != nil {
			t.Fatal(err)
		}
	}
	status := filepath.Join(t.TempDir(), "status")
	t.Setenv("AFFECTED_STATUS", status)
	script := filepath.Join(t.TempDir(), "corvint")
	body := `#!/bin/sh
git -C "$2" status --porcelain > "$AFFECTED_STATUS"
if [ -n "$AFFECTED_REFUSE" ]; then printf '{"code": "unsupported-affected-status", "ok": false}' >&2; exit 2; fi
printf '{"plan":{"scope":"UNKNOWN","selected":[{"tests":["a_test.go"],"witness":{}},{"tests":["a_test.go","b_test.go"]}]}}'
`
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	answer, err := runAffected(ctx, script, root, "a.go", 5)
	if err != nil || strings.Join(answer.Ranked, ",") != "a_test.go,b_test.go" || answer.State != "UNKNOWN" || answer.PacketBytes == 0 {
		t.Fatalf("affected: %+v %v", answer, err)
	}
	if seen, _ := os.ReadFile(status); !strings.Contains(string(seen), " M a.go") {
		t.Fatalf("the script did not see the file dirty: %q", seen)
	}
	assertRestored(t, ctx, root, original)
	t.Setenv("AFFECTED_REFUSE", "1")
	if _, err := runAffected(ctx, script, root, "a.go", 5); err == nil || !strings.Contains(err.Error(), "unsupported-affected-status") {
		t.Fatalf("refusal: %v", err)
	}
	assertRestored(t, ctx, root, original)
	if _, err := runAffected(ctx, script, root, "missing.go", 5); err == nil {
		t.Fatal("a path the copy does not hold was accepted")
	}
	assertRestored(t, ctx, root, original)
}

func assertRestored(t *testing.T, ctx context.Context, root string, original []byte) {
	t.Helper()
	restored, err := os.ReadFile(filepath.Join(root, "a.go"))
	if err != nil || !bytes.Equal(restored, original) {
		t.Fatalf("a.go not restored byte for byte: %q %v", restored, err)
	}
	if status, err := git(ctx, root, "status", "--porcelain"); err != nil || status != "" {
		t.Fatalf("copy left dirty: %q %v", status, err)
	}
}

// TestResolveSnapshotNeedsExactlyOneCorpusMatch.
func TestResolveSnapshotNeedsExactlyOneCorpusMatch(t *testing.T) {
	corpus := t.TempDir()
	for _, name := range []string{"clap-rs__clap__68b5ff900bae", "clap-rs__clap__deadbeefdead", "other__68b5ff900bae"} {
		if err := os.Mkdir(filepath.Join(corpus, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	item := sample{Repo: "clap-rs/clap", BaseCommit: "68b5ff900bae8ee1a0e328c1a2301a7985e4f1c6"}
	resolved, err := resolveSnapshot(options{corpus: corpus}, item)
	if err != nil || filepath.Base(resolved) != "clap-rs__clap__68b5ff900bae" {
		t.Fatalf("resolved %q err %v", resolved, err)
	}
	if _, err := resolveSnapshot(options{corpus: corpus}, sample{Repo: "clap-rs/clap", BaseCommit: "0000"}); err == nil {
		t.Fatal("no match accepted")
	}
	if resolved, _ := resolveSnapshot(options{snapshots: map[string]string{"clap-rs/clap@0000": "/explicit"}}, sample{Repo: "clap-rs/clap", BaseCommit: "0000"}); resolved != "/explicit" {
		t.Fatalf("override ignored: %q", resolved)
	}
}

// TestParseOptionsRejectsCallerMistakes.
func TestParseOptionsRejectsCallerMistakes(t *testing.T) {
	for _, arguments := range [][]string{
		{},
		{"--samples", "s.jsonl"},
		{"--samples", "s.jsonl", "--corpus", "c", "--limit", "0"},
		{"--samples", "s.jsonl", "--snapshot", "nope"},
		{"--samples", "s.jsonl", "--snapshot", "@=/tmp/x"},
		{"--samples", "s.jsonl", "--corpus", "c", "--max-samples", "-1"},
		{"--samples", "s.jsonl", "--corpus", "c", "extra"},
	} {
		if _, err := parseOptions(arguments); err == nil {
			t.Errorf("%v accepted", arguments)
		}
	}
	configuration, err := parseOptions([]string{"--samples", "s.jsonl", "--snapshot", "o/n@c=/p", "--task-type", "code2test"})
	if err != nil || configuration.snapshots["o/n@c"] != "/p" || !configuration.taskTypes["code2test"] || configuration.limit != defaultLimit {
		t.Fatalf("%+v %v", configuration, err)
	}
}

// TestScoreFollowsTheBenchDenominators: precision divides by the answered
// length, MRR is over the k-long ranking, and recall@k exists only for k ≤ limit.
func TestScoreFollowsTheBenchDenominators(t *testing.T) {
	answer := arm{Ranked: []string{"x.go", "a_test.go"}}
	got := score(answer, []string{"a_test.go", "b_test.go"}, []string{"x.go"}, "positive", 10)
	if got["precision@k"] != 0.5 || got["mrr@k"] != 0.5 || got["recall@5"] != 0.5 || got["recall@10"] != 0.5 || got["hard_negative_hits@k"] != 1 {
		t.Fatalf("score = %v", got)
	}
	if _, present := got["recall@20"]; present {
		t.Fatal("recall@20 reported for a 10-long ranking")
	}
	if empty := score(arm{Abstained: true}, []string{"a_test.go"}, nil, "positive", 10); empty["precision@k"] != 0 || empty["selective_success"] != 0 {
		t.Fatalf("empty score = %v", empty)
	}
}

// TestReadSamplesSkipsUnlabeledNoGold mirrors the selective evaluator: a
// sample with neither gold nor a no_gold label is skipped and counted.
func TestReadSamplesSkipsUnlabeledNoGold(t *testing.T) {
	path := filepath.Join(t.TempDir(), "s.jsonl")
	lines := `{"id":"a","repo":"o/n","base_commit":"c","task_type":"code2test","gold":{"related_tests":["t.go"]}}
{"id":"b","repo":"o/n","base_commit":"c","task_type":"code2test","gold":{}}
{"id":"c","repo":"o/n","base_commit":"c","task_type":"code2test","gold":{"no_gold":true}}
`
	if err := os.WriteFile(path, []byte(lines), 0o644); err != nil {
		t.Fatal(err)
	}
	samples, _, skipped, err := readSamples(options{samples: path, taskTypes: map[string]bool{}})
	if err != nil || len(samples) != 2 || samples[0].ID != "a" || samples[1].ID != "c" || skipped["no_gold_unlabeled"] != 1 {
		t.Fatalf("samples %d skipped %v err %v", len(samples), skipped, err)
	}
}

// TestWriteChunkRowsRebuildsOnlyTheFileRows: a release chunk file yields the
// files the bench evaluated over, never symbol rows or escaping paths.
func TestWriteChunkRowsRebuildsOnlyTheFileRows(t *testing.T) {
	corpus := t.TempDir()
	dir := filepath.Join(corpus, "clap-rs__clap")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	chunks := filepath.Join(dir, "68b5ff900bae8ee1a0e328c1a2301a7985e4f1c6.chunks.jsonl")
	rows := `{"kind":"file","path":"src/help.rs","text":"fn quote_default_value() {}\n"}
{"kind":"symbol","path":"src/help.rs","symbol":"quote_default_value","text":"fn quote_default_value() {}"}
{"kind":"file","path":"tests/help_test.rs","text":"#[test] fn quote_empty_default_value_in_help() {}\n"}
{"kind":"file","path":"../escape.rs","text":"nope"}
{"kind":"file","path":"/abs.rs","text":"nope"}
`
	if err := os.WriteFile(chunks, []byte(rows), 0o644); err != nil {
		t.Fatal(err)
	}
	item := sample{Repo: "clap-rs/clap", BaseCommit: "68b5ff900bae8ee1a0e328c1a2301a7985e4f1c6"}
	resolved, err := resolveSnapshot(options{corpus: corpus}, item)
	if err != nil || resolved != chunks {
		t.Fatalf("resolved %q err %v", resolved, err)
	}
	space := newWorkspaces()
	root, err := space.open(context.Background(), options{corpus: corpus}, item)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	_ = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if entry.IsDir() && entry.Name() == ".git" {
			return filepath.SkipDir
		}
		if !entry.IsDir() {
			relative, _ := filepath.Rel(root, path)
			got = append(got, relative)
		}
		return nil
	})
	if strings.Join(got, ",") != "src/help.rs,tests/help_test.rs" {
		t.Fatalf("files = %v", got)
	}
	if head, err := git(context.Background(), root, "rev-parse", "HEAD"); err != nil || head == "" {
		t.Fatalf("no commit: %v", err)
	}
	if _, err := os.Stat(filepath.Join(corpus, "escape.rs")); !os.IsNotExist(err) {
		t.Fatal("a row escaped the destination")
	}
	space.close()
	if _, err := os.Stat(root); !os.IsNotExist(err) {
		t.Fatal("temporary copy left behind")
	}
}

// TestRunCorvintGoBoundsAndReportsTheChild drives the real runners through a
// shell script standing in for corvint: a packet is parsed, a refusal on
// stderr becomes an error, and output past the bound is refused.
func TestRunCorvintGoBoundsAndReportsTheChild(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the fake corvint is a POSIX shell script")
	}
	script := filepath.Join(t.TempDir(), "corvint")
	body := `#!/bin/sh
case "$3" in
  query) printf '{"context":{"state":"READY","results":[{"evidence":[{"path":"a.go"}]}]}}' ;;
  context) printf '{"state":"READY","results":[{"id":"a.go"},{"id":"a_test.go"}]}' ;;
  impact)
    if [ "$4" = "refuse.rs" ]; then printf '{"code": "unsupported-impact-path-suffix", "ok": false}' >&2; exit 2; fi
    if [ "$4" = "huge.go" ]; then head -c 9000000 /dev/zero | tr '\0' 'x'; exit 0; fi
    printf '{"context":{"state":"READY","results":[{"evidence":[{"path":"a.go"},{"path":"a_test.go"}]},{"evidence":[{"path":"a.go"}]}]}}' ;;
esac
`
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if answer, err := runCorvint(ctx, script, "/r", "task", 5); err != nil || strings.Join(answer.Ranked, ",") != "a.go" {
		t.Fatalf("query: %+v %v", answer, err)
	}
	if answer, err := runContext(ctx, script, "/r", "task", 5); err != nil || strings.Join(answer.Ranked, ",") != "a.go,a_test.go" || answer.PacketBytes == 0 {
		t.Fatalf("context: %+v %v", answer, err)
	}
	if answer, err := runImpact(ctx, script, "/r", "a.go", 5); err != nil || strings.Join(answer.Ranked, ",") != "a.go,a_test.go" || answer.PacketBytes == 0 {
		t.Fatalf("impact: %+v %v", answer, err)
	}
	if _, err := runImpact(ctx, script, "/r", "refuse.rs", 5); err == nil || !strings.Contains(err.Error(), "unsupported-impact-path-suffix") {
		t.Fatalf("refusal: %v", err)
	}
	if _, err := runImpact(ctx, script, "/r", "huge.go", 5); err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("oversized: %v", err)
	}
}

// TestGrepIdentTakesIdentifiersFromQueryValuesAndMatchesWholeWords: the
// query's keys never become terms, the identifier and the code-file stem
// are the only terms, matches are whole-word and case-sensitive, a path hit
// counts once, and the given file is excluded.
func TestGrepIdentTakesIdentifiersFromQueryValuesAndMatchesWholeWords(t *testing.T) {
	query := map[string]any{"review_comment": "quote_default_value drops the changed_line", "path": "src/help.rs", "nested": []any{map[string]any{"pr_body": "see fmtError"}}}
	if got := strings.Join(queryIdentifiers(query), ","); got != "changed_line,fmtError,help,quote_default_value" {
		t.Fatalf("identifiers = %s", got)
	}
	index := indexCorpus(fixtureSnapshot(t))
	answer := runGrepIdent(index, []string{"help", "quote_default_value"}, []string{"src/help.rs"}, 5)
	if strings.Join(answer.Ranked, ",") != "README.md,tests/help_test.rs" || answer.Abstained {
		t.Fatalf("grep-ident: %+v", answer)
	}
	if empty := runGrepIdent(index, nil, nil, 5); !empty.Abstained || len(empty.Ranked) != 0 {
		t.Fatalf("no identifiers should abstain: %+v", empty)
	}
	if wholeWordCount("get_value get_value_x xget_value get_value", "get_value") != 2 {
		t.Fatal("whole-word count crossed an identifier boundary")
	}
}

// TestBM25RanksShorterMatchingFilesFirstAndAbstainsWithoutHits: both files
// holding every ident term score, the shorter one higher; a term set the
// corpus lacks abstains; the given file never ranks.
func TestBM25RanksShorterMatchingFilesFirstAndAbstainsWithoutHits(t *testing.T) {
	index := indexCorpus(fixtureSnapshot(t))
	if index.meanLength <= 0 || index.documentFrequency["quote"] != 2 {
		t.Fatalf("index: mean %v df(quote) %d", index.meanLength, index.documentFrequency["quote"])
	}
	answer := runBM25(index, identifierTerms([]string{"quote_default_value"}), nil, 5)
	if strings.Join(answer.Ranked, ",") != "src/help.rs,tests/help_test.rs" || answer.Abstained || answer.TopScore <= 0 {
		t.Fatalf("bm25:ident: %+v", answer)
	}
	if given := runBM25(index, identifierTerms([]string{"quote_default_value"}), []string{"src/help.rs"}, 5); strings.Join(given.Ranked, ",") != "tests/help_test.rs" {
		t.Fatalf("bm25 ranked the given file: %+v", given)
	}
	if none := runBM25(index, terms("zzzz qqqq"), nil, 5); !none.Abstained || none.TopScore != 0 {
		t.Fatalf("no hit should abstain: %+v", none)
	}
}

// TestPairedDifferencesAreDeterministicAndCountWins: the paired block holds
// the mean difference, a fixed-seed interval around it, win/loss/tie counts,
// per-repository means only for repositories with five samples, folds from
// the frozen map, and nothing for an arm the reports lack.
func TestPairedDifferencesAreDeterministicAndCountWins(t *testing.T) {
	reports := make([]sampleReport, 0, 6)
	contextRecall := []float64{1, 0.5, 0, 1, 0.5, 0.5}
	grepRecall := []float64{0.5, 0.5, 0, 0, 1, 0}
	for index := range contextRecall {
		reports = append(reports, sampleReport{ID: string(rune('a' + index)), TaskType: "code2test", Repo: "gin-gonic/gin", Stratum: "positive", Partition: partition("gin-gonic/gin"), Metrics: map[string]metrics{
			"context": {"recall@10": contextRecall[index]},
			"grep":    {"recall@10": grepRecall[index]},
		}})
	}
	reports = append(reports, sampleReport{ID: "nogold", TaskType: "code2test", Repo: "gin-gonic/gin", Stratum: "natural_no_gold", Partition: "A", Metrics: map[string]metrics{"context": {"abstained": 1}, "grep": {"abstained": 0}}})
	first, second := paired(reports), paired(reports)
	block := first["task:code2test/fold:A"].(map[string]any)["context"].(map[string]any)["grep"].(map[string]any)["recall@10"].(map[string]any)
	if block["n"] != 6 || block["mean_diff"] != 0.25 || block["wins"] != 3 || block["losses"] != 1 || block["ties"] != 2 {
		t.Fatalf("paired block: %v", block)
	}
	low, high := block["low"].(float64), block["high"].(float64)
	if low > 0.25 || high < 0.25 || low < -0.5 || high > 1 {
		t.Fatalf("interval [%v, %v] does not bracket the mean", low, high)
	}
	if repos := block["repos"].(map[string]any)["gin-gonic/gin"].(map[string]any); repos["n"] != 6 || repos["mean_diff"] != 0.25 {
		t.Fatalf("per-repo: %v", repos)
	}
	if _, present := first["all"].(map[string]any)["context"].(map[string]any)["bm25:ident"]; present {
		t.Fatal("an absent arm produced a pair")
	}
	firstBytes, _ := json.Marshal(first)
	secondBytes, _ := json.Marshal(second)
	if !bytes.Equal(firstBytes, secondBytes) {
		t.Fatalf("paired statistics differ between calls:\n%s\n%s", firstBytes, secondBytes)
	}
	if partition("unknown/repo") != "unassigned" || partition("clap-rs/clap") != "B" || len(foldMapDigest()) != 64 {
		t.Fatal("fold map")
	}
}

func TestSummarizeRefusesUnknownArmsAndMergesNullArms(t *testing.T) {
	directory := t.TempDir()
	writeReport := func(name, details string) string {
		path := filepath.Join(directory, name)
		if err := os.WriteFile(path, []byte(`{"samples_sha256":"s","limit":10,"details":[`+details+`]}`), 0o644); err != nil {
			t.Fatal(err)
		}
		return path
	}
	unknown := writeReport("unknown.json", `{"id":"a","repo":"gin-gonic/gin","stratum":"positive","arms":{"future":{"wall_ms":5}},"metrics":{"future":{"recall@10":1}}}`)
	if _, err := resummarize(options{summaries: []string{unknown}}); err == nil || !strings.Contains(err.Error(), "future") {
		t.Fatalf("resummarize error = %v, want the unknown arm refused", err)
	}
	empty := writeReport("empty.json", `{"id":"a","repo":"gin-gonic/gin","stratum":"positive","arms":null,"metrics":null}`)
	later := writeReport("later.json", `{"id":"a","repo":"gin-gonic/gin","stratum":"positive","arms":{"grep":{"ranked":["x"]}},"metrics":{"grep":{"recall@10":1}}}`)
	report, err := resummarize(options{summaries: []string{empty, later}})
	if err != nil {
		t.Fatal(err)
	}
	if details := report["details"].([]sampleReport); len(details[0].Arms) != 1 || details[0].Metrics["grep"]["recall@10"] != 1 {
		t.Fatalf("merged details = %+v", details)
	}
}

func TestBootstrapIntervalTailsAreEqual(t *testing.T) {
	differences := make([]float64, 40)
	for index := range differences {
		differences[index] = 1 / float64(index+3)
	}
	low, high := bootstrap(differences)
	negated := make([]float64, len(differences))
	for index, value := range differences {
		negated[index] = -value
	}
	negatedLow, negatedHigh := bootstrap(negated)
	if negatedLow != -high || negatedHigh != -low {
		t.Fatalf("interval [%v, %v] mirrored to [%v, %v]; the tails drop unequal resample counts", low, high, negatedLow, negatedHigh)
	}
}
