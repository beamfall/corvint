package main

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/evalrepo"
)

func TestManifestAndRegisteredEvidence(t *testing.T) {
	root := testCorvintRoot(t)
	m, _, err := loadManifest(filepath.Join(root, "benchmarks", "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Repositories) != 5 {
		t.Fatalf("repositories=%d", len(m.Repositories))
	}
	public, cases := 0, 0
	for _, repo := range m.Repositories {
		if repo.Public {
			public++
		}
		raw, err := os.ReadFile(filepath.Join(root, repo.Cases))
		if err != nil {
			t.Fatal(err)
		}
		var payload map[string]any
		if err = json.Unmarshal(raw, &payload); err != nil {
			t.Fatal(err)
		}
		cases += len(maps(payload["cases"]))
	}
	if public != 4 || cases < 25 {
		t.Fatalf("public=%d cases=%d", public, cases)
	}
	for path, partition := range map[string]string{"blind-v2-manifest.json": "heldout-v2", "blind-v3-manifest.json": "heldout-v3"} {
		raw, err := os.ReadFile(filepath.Join(root, "benchmarks", path))
		if err != nil {
			t.Fatal(err)
		}
		evidence := firstRunEvidence[sha(raw)]
		if stringValue(evidence["partition"]) != partition {
			t.Fatalf("%s evidence=%v", path, evidence)
		}
	}
	fixture, err := os.ReadFile(filepath.Join(root, "benchmarks", "learned-trace-fixture-v0.json"))
	if err != nil {
		t.Fatal(err)
	}
	if sha(fixture) != learnedTraceFixtureSHA256 {
		t.Fatal("learned fixture digest changed")
	}
}

func TestCommittedSplitManifestRegistersReleaseCorpora(t *testing.T) {
	root := testCorvintRoot(t)
	splits, err := loadSplitManifest(filepath.Join(root, "benchmarks", "eval-split-v0.json"))
	if err != nil {
		t.Fatal(err)
	}
	m, _, err := loadManifest(filepath.Join(root, "benchmarks", "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	dev, total := 0, 0
	for _, repo := range m.Repositories {
		raw, err := os.ReadFile(filepath.Join(root, repo.Cases))
		if err != nil {
			t.Fatal(err)
		}
		if registered, err := splits.VerifyCorpus(repo.Cases, raw); !registered || err != nil {
			t.Fatalf("%s registered=%v err=%v", repo.Cases, registered, err)
		}
		var payload map[string]any
		if err = json.Unmarshal(raw, &payload); err != nil {
			t.Fatal(err)
		}
		dev += len(readableCases(maps(payload["cases"]), evalrepo.PurposeTune))
		total += len(maps(payload["cases"]))
	}
	if dev == 0 || dev == total {
		t.Fatalf("dev=%d total=%d", dev, total)
	}
}

func TestSelectorPathsAndPartitions(t *testing.T) {
	if got := selectorPath("symbol:packages/core/errors.ts:treeifyError"); got != "packages/core/errors.ts" {
		t.Fatal(got)
	}
	if got := selectorPath("reverse-import:tests/test_app.py"); got != "tests/test_app.py" {
		t.Fatal(got)
	}
	if got := selectorPath("feature:sessions"); got != "" {
		t.Fatal(got)
	}
	evidence := firstRunEvidence["1365eec079f861d8abe03123afdeefa4bd9da23f51bb2c7efce4366f553c87b1"]
	if evaluationPartition("blind-v1", nil) != "development-after-observation" || evaluationPartition("heldout-v2", evidence) != "development-after-first-run" || evaluationPartition("heldout-v3", nil) != "unattested-heldout" {
		t.Fatal("partition translation changed")
	}
}

func TestThresholdsFailClosedAndSubsetBlocksRelease(t *testing.T) {
	m := map[string]any{"repositories": 5, "skipped_repository_ids": []string{}, "public_repositories": 4, "cases": 25, "positive_cases": 25, "revision_pinned_cases": 25, "critical_evidence_total": 25, "must_exclude_total": 1, "epistemic_state_cases": 1, "budgeted_cases": 25, "must_exclude_violations": 0, "repository_evaluations_ready": true, "recall": 1.0, "critical_evidence_misses": 1, "serialized_result_byte_weighted_precision": .9, "top5_task_success": .9, "abstention_accuracy": 1.0, "epistemic_state_accuracy": 1.0, "expected_state_accuracy": 1.0, "budget_compliance": 1.0}
	checks := thresholds(m)
	if checks["zero_critical_misses"] || allTrue(checks) {
		t.Fatal(checks)
	}
	m["critical_evidence_misses"] = 0
	m["skipped_repository_ids"] = []string{"beamfall"}
	if thresholds(m)["all_manifest_repositories_included"] {
		t.Fatal("subset passed complete-manifest check")
	}
}

func TestAuditReplaysBaselineAndLearnedMetrics(t *testing.T) {
	payload := map[string]any{"cases": []any{map[string]any{"id": "case", "ground_truth": []any{}, "expected": map[string]any{"must_include": []any{"path:app.py"}, "must_exclude": []any{"path:wrong.py"}, "state": "READY"}}}}
	metrics := map[string]any{"cases": 1, "must_read_total": 1, "must_read_hits": 1, "critical_evidence_total": 0, "critical_evidence_misses": 0, "must_exclude_total": 1, "must_exclude_violations": 1, "epistemic_state_cases": 0, "epistemic_state_hits": 0}
	evaluation := map[string]any{"metrics": metrics, "cases": []any{map[string]any{"id": "case", "state": "READY", "missing": []any{}, "unexpected": []any{"path:wrong.py"}, "actual": []any{map[string]any{"selector": "path:app.py"}, map[string]any{"selector": "path:wrong.py"}}}}}
	counts, err := auditEvaluation(payload, evaluation)
	if err != nil {
		t.Fatal(err)
	}
	if counts["must_exclude_violations"] != 1 || counts["expected_state_hits"] != 1 {
		t.Fatal(counts)
	}
	metrics["must_exclude_violations"] = 0
	if _, err = auditEvaluation(payload, evaluation); err == nil || !strings.Contains(err.Error(), "does not replay") {
		t.Fatalf("err=%v", err)
	}
	metrics["must_exclude_violations"] = 1
	learned := cloneMap(evaluation)
	learnedMetrics := cloneMap(metrics)
	learnedMetrics["must_read_hits"] = 0
	learned["metrics"] = learnedMetrics
	if _, err = auditEvaluation(payload, learned); err == nil || !strings.Contains(err.Error(), "must_read_hits") {
		t.Fatalf("learned err=%v", err)
	}
}

func TestValidateCasesRequiresExactPinnedSourceProof(t *testing.T) {
	repo, commit := fixtureRepository(t, "def run():\n    return 1\n\ndef stop():\n    return 0\n")
	entry := repository{ID: "fixture", Commit: commit, Public: true}
	casePath := filepath.Join(repo, "cases.json")
	writeCases := func(ground []any, selector string) error {
		t.Helper()
		payload := map[string]any{
			"schemaVersion": 1,
			"commit":        commit,
			"cases": []any{map[string]any{
				"id": "case", "mode": "query", "text": "stop", "budget_bytes": 4000,
				"rationale": "proof", "ground_truth": ground,
				"expected": map[string]any{"must_include": []any{selector}, "critical": []any{selector}},
			}},
		}
		raw, _ := json.Marshal(payload)
		if err := os.WriteFile(casePath, raw, 0o600); err != nil {
			t.Fatal(err)
		}
		_, _, err := validateCases(context.Background(), entry, repo, casePath)
		return err
	}
	if err := writeCases(nil, "symbol:app.py:run"); err == nil || !strings.Contains(err.Error(), "ground_truth") {
		t.Fatalf("err=%v", err)
	}
	if err := writeCases([]any{map[string]any{"path": "app.py", "line": 1, "contains": "def run"}}, "symbol:app.py:stop"); err == nil || !strings.Contains(err.Error(), "exact source proof") {
		t.Fatalf("err=%v", err)
	}
	if err := os.WriteFile(filepath.Join(repo, "app.py"), []byte("def changed():\n    return 2\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := writeCases([]any{map[string]any{"path": "app.py", "line": 1, "contains": "def changed"}}, "symbol:app.py:run"); err == nil || !strings.Contains(err.Error(), "ground truth moved") {
		t.Fatalf("err=%v", err)
	}
}

func TestLearnedTraceGateOrderAndDistinguishability(t *testing.T) {
	baseline := map[string]any{"critical_evidence_misses": 0, "abstention_accuracy": 1.0, "epistemic_state_accuracy": 1.0, "serialized_result_byte_weighted_precision": .82, "recall": .75, "top5_task_success": .8}
	fixture := cloneMap(baseline)
	fixture["serialized_result_byte_weighted_precision"] = .79
	report := learnedTraceReport(baseline, fixture, map[string]int{"serialized_result_byte_weighted_precision": 1})
	if report["ready"].(bool) {
		t.Fatal(report)
	}
	rows := maps(report["delta"])
	got := []string{}
	for _, row := range rows {
		got = append(got, stringValue(row["metric"]))
	}
	want := []string{"critical_evidence_misses", "abstention_accuracy", "epistemic_state_accuracy", "serialized_result_byte_weighted_precision", "recall", "top_five_task_success"}
	if !reflect.DeepEqual(got, want) || rows[3]["delta"] != "not distinguished" {
		t.Fatalf("rows=%v", rows)
	}
}

func TestExactBaselineUsesLegacyTermExpansion(t *testing.T) {
	got := set(termsFor("generate HTTPClients synchronously")...)
	for _, want := range []string{"generate", "gen", "http", "clients", "client", "synchronously"} {
		if _, ok := got[want]; !ok {
			t.Fatalf("missing %s in %v", want, got)
		}
	}
}

func TestGitCancellationReapsDescendant(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX process-group regression")
	}
	bin := t.TempDir()
	pidFile := filepath.Join(bin, "child.pid")
	script := "#!/bin/sh\nsleep 30 &\necho $! > '" + pidFile + "'\nwait\n"
	if err := os.WriteFile(filepath.Join(bin, "git"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { _, err := git(ctx, bin, "status"); done <- err }()
	deadline := time.Now().Add(2 * time.Second)
	var pid int
	for time.Now().Before(deadline) {
		raw, err := os.ReadFile(pidFile)
		if err == nil {
			pid, _ = strconv.Atoi(strings.TrimSpace(string(raw)))
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if pid == 0 {
		t.Fatal("fake git descendant did not start")
	}
	cancel()
	if err := <-done; err == nil || !strings.Contains(err.Error(), "process-cancelled") {
		t.Fatalf("err=%v", err)
	}
	for deadline = time.Now().Add(2 * time.Second); time.Now().Before(deadline); {
		if processGone(pid) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("descendant %d survived cancellation", pid)
}

func fixtureRepository(t *testing.T, body string) (string, string) {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "app.py"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"init", "-q"}, {"config", "user.email", "corvint@example.test"}, {"config", "user.name", "Corvint Test"}, {"add", "app.py"}, {"commit", "-qm", "fixture"}} {
		command := exec.Command("git", args...)
		command.Dir = root
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, output)
		}
	}
	commit, err := git(context.Background(), root, "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	return root, commit
}
func testCorvintRoot(t *testing.T) string {
	t.Helper()
	root, err := corvintRoot()
	if err != nil {
		t.Fatal(err)
	}
	return root
}
