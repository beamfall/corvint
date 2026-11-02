package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// runAffectedForm runs corvint affected with options, on separate streams or,
// when merged, on one file for both (as after 2>&1).
func runAffectedForm(t *testing.T, root string, merged bool, options ...string) (string, string, int) {
	t.Helper()
	arguments := append([]string{"--root", root, "affected"}, options...)
	if !merged {
		var stdout, stderr bytes.Buffer
		code := runContext(context.Background(), arguments, strings.NewReader(""), &stdout, &stderr)
		return stdout.String(), stderr.String(), code
	}
	file, err := os.Create(filepath.Join(t.TempDir(), "merged"))
	if err != nil {
		t.Fatal(err)
	}
	code := runContext(context.Background(), arguments, strings.NewReader(""), file, file)
	_ = file.Close()
	output, err := os.ReadFile(file.Name())
	if err != nil {
		t.Fatal(err)
	}
	return string(output), "", code
}

// ACX-V0-750 (V1-4214, NEW-C1): a stale binary's notice, with the rebuild
// recovery, the uninstalled-build alternative and the plan-unchanged
// statement, reaches every affected output form on separate and merged
// streams; a binary built at HEAD carries none of it, and the plan is the
// same either way.
func TestStaleBinaryNoticeReachesEveryAffectedForm_ACXV0750(t *testing.T) {
	root, built := selfBinaryRepository(t)
	stubSelfBuildInfo(t, selfBinaryInfo("example.test/self", built))
	forms := []struct {
		options []string
		json    bool
	}{
		{nil, true},
		{[]string{"--full"}, true},
		{[]string{"--summary"}, true},
		{[]string{"--format", "text"}, false},
		{[]string{"--summary", "--format", "text"}, false},
		{[]string{"--by-path", "--path", "self.go"}, true},
		{[]string{"--by-path", "--path", "self.go", "--format", "text"}, false},
	}
	hints := []string{"go install ./cmd/corvint", "go run ./cmd/corvint", "this notice changes no plan, selection or exit status"}
	for _, form := range forms {
		stdout, stderr, code := runAffectedForm(t, root, false, form.options...)
		if code != 0 || strings.Contains(stdout+stderr, "go run ./cmd/corvint") || strings.Contains(stdout+stderr, "degraded") || strings.Contains(stdout+stderr, "egradationDetails") {
			t.Fatalf("%v: a binary built at HEAD carried a notice (exit %d):\n%s%s", form.options, code, stdout, stderr)
		}
	}

	commitSelfBinaryFiles(t, root, map[string]string{"self.go": "package self\n\nfunc Self() int { return 2 }\n"})
	freshness := selfBinaryStale(context.Background(), root)
	notice := selfBinaryNotice(freshness)
	if freshness.Code != selfBinaryStaleCode || !strings.HasPrefix(notice, freshness.Detail) {
		t.Fatalf("the fixture binary is not stale: %+v", freshness)
	}
	for _, hint := range hints {
		if !strings.Contains(notice, hint) {
			t.Fatalf("the notice lacks %q: %s", hint, notice)
		}
	}
	for _, form := range forms {
		for _, merged := range []bool{false, true} {
			stdout, stderr, code := runAffectedForm(t, root, merged, form.options...)
			if code != 0 {
				t.Fatalf("%v merged=%v: exit %d\n%s%s", form.options, merged, code, stdout, stderr)
			}
			line := "corvint: DEGRADED " + selfBinaryStaleCode + ": " + notice + "\n"
			if merged == strings.Contains(stdout+stderr, "corvint: DEGRADED") || !merged && stderr != line {
				t.Fatalf("%v merged=%v: want stderr exactly %q, got stdout %q stderr %q", form.options, merged, line, stdout, stderr)
			}
			if !form.json {
				want := "\ndegraded " + selfBinaryStaleCode + "\ndegraded-detail " + selfBinaryStaleCode + " " + notice + "\n"
				if !strings.Contains(stdout, want) {
					t.Fatalf("%v merged=%v: text lacks %q:\n%s", form.options, merged, want, stdout)
				}
				continue
			}
			var receipt struct {
				Degradations       []string          `json:"degradations"`
				DegradationDetails map[string]string `json:"degradationDetails"`
			}
			if err := json.Unmarshal([]byte(stdout), &receipt); err != nil {
				t.Fatalf("%v merged=%v: not one JSON document: %v\n%s", form.options, merged, err, stdout)
			}
			if len(receipt.Degradations) != 1 || receipt.Degradations[0] != selfBinaryStaleCode || len(receipt.DegradationDetails) != 1 || receipt.DegradationDetails[selfBinaryStaleCode] != notice {
				t.Fatalf("%v merged=%v: degradations %v details %v", form.options, merged, receipt.Degradations, receipt.DegradationDetails)
			}
		}
	}
	// The notice changes no plan: at one HEAD and base, the stale binary and
	// one built at HEAD give the same summary, tiers and range.
	head := strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
	stale, _, staleCode := runAffectedForm(t, root, false, "--summary", "--base", built)
	stubSelfBuildInfo(t, selfBinaryInfo("example.test/self", head))
	current, currentStderr, currentCode := runAffectedForm(t, root, false, "--summary", "--base", built)
	var before, after map[string]any
	if staleCode != 0 || currentCode != 0 || strings.Contains(currentStderr, "DEGRADED") || json.Unmarshal([]byte(stale), &before) != nil || json.Unmarshal([]byte(current), &after) != nil {
		t.Fatalf("summaries: exit %d, %d\n%s\n%s%s", staleCode, currentCode, stale, current, currentStderr)
	}
	for _, member := range []string{"summary", "tiers", "range"} {
		left, _ := json.Marshal(before[member])
		right, _ := json.Marshal(after[member])
		if !bytes.Equal(left, right) || string(left) == "null" {
			t.Fatalf("the notice changed %s: %s -> %s", member, left, right)
		}
	}
	if summary, _ := after["summary"].(map[string]any); summary["line"] == nil || !strings.Contains(current, "tier0") {
		t.Fatalf("the base plan selects nothing to compare:\n%s", current)
	}
}

// ACX-V0-751 (V1-4039): each invocation judges the binary once and prints at
// most one DEGRADED line, the same line every time; nothing carries over
// between invocations, so a replay of N planner calls prints N identical
// lines that a driver dedupes on the degradations member or suppresses by
// merging the streams (AFP-V0-871). corvint test --dry-run behaves alike.
func TestStaleBinaryNoticeOncePerInvocation_ACXV0751(t *testing.T) {
	root, built := selfBinaryRepository(t)
	stubSelfBuildInfo(t, selfBinaryInfo("example.test/self", built))
	commitSelfBinaryFiles(t, root, map[string]string{"self.go": "package self\n\nfunc Self() int { return 2 }\n"})
	status := affectedGit(t, root, "status", "--porcelain", "--ignored")
	var lines []string
	for range 2 {
		for _, options := range [][]string{nil, {"--by-path", "--path", "self.go"}, {"--summary", "--format", "text"}} {
			_, stderr, code := runAffectedForm(t, root, false, options...)
			if code != 0 || strings.Count(stderr, "DEGRADED") != 1 {
				t.Fatalf("%v: want one DEGRADED line, exit %d:\n%s", options, code, stderr)
			}
			lines = append(lines, stderr)
		}
		var stdout, stderr bytes.Buffer
		runContext(context.Background(), []string{"--root", root, "test", "--base", built, "--dry-run"}, strings.NewReader(""), &stdout, &stderr)
		if strings.Count(stderr.String(), "corvint-test: DEGRADED") != 1 || !strings.Contains(stderr.String(), "corvint-test: DEGRADED "+selfBinaryStaleCode+": "+selfBinaryNotice(selfBinaryStale(context.Background(), root))+"\n") {
			t.Fatalf("corvint test: want one notice line:\n%s", stderr.String())
		}
	}
	for _, line := range lines[1:] {
		if line != lines[0] {
			t.Fatalf("the notice differs between invocations:\n%s\n%s", lines[0], line)
		}
	}
	if after := affectedGit(t, root, "status", "--porcelain", "--ignored"); after != status {
		t.Fatalf("the notice left state behind:\nbefore %q\nafter  %q", status, after)
	}
}
