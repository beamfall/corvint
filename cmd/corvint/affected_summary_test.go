package main

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
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
		"dirty":           float64(len(plan["dirty"].([]any))),
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
	if summary["scope"] != "BOUNDED" || summary["selected"] != float64(0) || summary["unknown"] != float64(0) || !reflect.DeepEqual(summary["unknownByReason"], map[string]any{}) ||
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
