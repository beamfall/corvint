package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// AFP-V0-510: the affected-plan/1 summary restates the plan's scope and
// counts, groups the unknowns by reason and carries the same as one line;
// --full carries none.
func TestAFPV0510AffectedSummaryRestatesThePlan(t *testing.T) {
	t.Parallel()
	root := affectedFixtureRepository(t)
	appendFile(t, filepath.Join(root, "core", "core.go"), "\n// edited\n")
	if err := os.WriteFile(filepath.Join(root, "notes.txt"), []byte("scratch\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	receipt, _, stderr, code := runAffectedCLI(t, root)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	plan := receipt["plan"].(map[string]any)
	byReason := map[string]any{}
	for _, item := range plan["unknown"].([]any) {
		reason := item.(map[string]any)["reason"].(string)
		count, _ := byReason[reason].(float64)
		byReason[reason] = count + 1
	}
	if byReason["UNOWNED_DIRTY_PATH"] == nil || plan["scope"] != "UNKNOWN" || len(plan["selected"].([]any)) == 0 {
		t.Fatalf("fixture plan: %v", plan)
	}
	excluded := plan["excluded"].(map[string]any)["count"]
	want := map[string]any{
		"committed":       float64(0),
		"dirty":           float64(len(plan["dirty"].([]any))),
		"worktree":        float64(len(plan["dirty"].([]any))),
		"excluded":        excluded,
		"scope":           "UNKNOWN",
		"selected":        float64(len(plan["selected"].([]any))),
		"unknown":         float64(len(plan["unknown"].([]any))),
		"unknownByReason": byReason,
	}
	want["line"] = fmt.Sprintf("scope=UNKNOWN dirty=%d selected=%d excluded=%d unknown=%d", int(want["dirty"].(float64)), int(want["selected"].(float64)), int(excluded.(float64)), int(want["unknown"].(float64)))
	for _, reason := range sortedKeysOf(byReason) {
		want["line"] = want["line"].(string) + fmt.Sprintf(" %s=%d", reason, int(byReason[reason].(float64)))
	}
	if !reflect.DeepEqual(receipt["summary"], want) {
		t.Fatalf("summary = %v, want %v", receipt["summary"], want)
	}

	clean := affectedFixtureRepository(t)
	receipt, _, stderr, code = runAffectedCLI(t, clean)
	if code != 0 {
		t.Fatalf("clean exit %d: %s", code, stderr)
	}
	summary := receipt["summary"].(map[string]any)
	if summary["scope"] != "BOUNDED" || summary["worktree"] != float64(0) || summary["committed"] != float64(0) || summary["selected"] != float64(0) || summary["unknown"] != float64(0) || !reflect.DeepEqual(summary["unknownByReason"], map[string]any{}) ||
		summary["line"] != fmt.Sprintf("scope=BOUNDED dirty=0 selected=0 excluded=%d unknown=0", int(summary["excluded"].(float64))) {
		t.Fatalf("clean summary = %v", summary)
	}

	full, _, stderr, code := runAffectedCLI(t, root, "--full")
	if code != 0 {
		t.Fatalf("--full exit %d: %s", code, stderr)
	}
	if _, present := full["summary"]; present {
		t.Fatal("the affected-plan/0 --full document must stay unchanged")
	}
}

func sortedKeysOf(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && keys[j] < keys[j-1]; j-- {
			keys[j], keys[j-1] = keys[j-1], keys[j]
		}
	}
	return keys
}

// AFP-V0-820: with --base the committed range is unioned into plan.dirty
// (AFP-V0-010), so a clean lane tree still reads dirty=N; the summary splits
// that count into worktree (the status set alone) and committed (the range),
// and a clean tree with a committed range reads worktree=0, in the JSON
// summary and on the text header.
func TestAFPV0820CleanTreeWithARangeReadsWorktreeZero(t *testing.T) {
	t.Parallel()
	root := affectedFixtureRepository(t)
	base := strings.TrimSpace(materializationGit(t, root, "rev-parse", "HEAD"))
	appendFile(t, filepath.Join(root, "core", "core.go"), "\n// committed\n")
	commitAll(t, root, "edit core")
	receipt, _, stderr, code := runAffectedCLI(t, root, "--base", base)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	summary := receipt["summary"].(map[string]any)
	committed := float64(len(receipt["range"].(map[string]any)["paths"].([]any)))
	if summary["worktree"] != float64(0) || committed == 0 || summary["committed"] != committed || summary["dirty"] != committed {
		t.Fatalf("clean tree with a range: summary = %v, range paths = %v", summary, committed)
	}

	appendFile(t, filepath.Join(root, "core", "core.go"), "\n// uncommitted\n")
	receipt, _, stderr, code = runAffectedCLI(t, root, "--base", base)
	if code != 0 {
		t.Fatalf("mixed exit %d: %s", code, stderr)
	}
	if summary := receipt["summary"].(map[string]any); summary["worktree"] != float64(1) || summary["committed"] != committed {
		t.Fatalf("worktree edit on a range: summary = %v", summary)
	}

	var text, errText bytes.Buffer
	if code := runContext(context.Background(), []string{"--root", root, "affected", "--base", base, "--format", "text"}, strings.NewReader(""), &text, &errText); code != 0 {
		t.Fatalf("text exit %d: %s", code, errText.String())
	}
	if header, _, _ := strings.Cut(text.String(), "\n"); !strings.Contains(header, " worktree=1 revision=") {
		t.Fatalf("text header = %q", header)
	}
}
