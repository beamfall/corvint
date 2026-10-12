//go:build darwin || linux

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/sessiontest"
)

// sessionFakeRunners is a recording `go` and `make` on PATH: probes answer
// the fixture universe, `go test` reports each named package as passing
// (or, in sleep mode, signals readiness and sleeps), and `make` passes.
const sessionFakeGo = `#!/bin/sh
echo "go $*" >> "$SESSION_FAKE_LOG"
case "$1" in
env) echo go1.27.1; exit 0;;
list) echo example.test/m/a; echo example.test/m/b; exit 0;;
esac
if [ "$SESSION_FAKE_MODE" = sleep ]; then touch "$SESSION_FAKE_LOG.started"; sleep 60; fi
for arg in "$@"; do
  case "$arg" in example.test/*) echo "{\"Action\":\"pass\",\"Package\":\"$arg\"}";; esac
done
`

const sessionFakeMake = `#!/bin/sh
echo "make $*" >> "$SESSION_FAKE_LOG"
[ "$1" = --version ] && echo "GNU Make 4.4"
exit 0
`

// sessionFixture is a two-package module whose HEAD changes package a.
func sessionFixture(t *testing.T, mode string) (root, base, log string) {
	t.Helper()
	root = t.TempDir()
	files := map[string]string{
		"go.mod":      "module example.test/m\n\ngo 1.27\n",
		"a/a.go":      "package a\n\nfunc A() int { return 1 }\n",
		"a/a_test.go": "package a\n\nimport \"testing\"\n\nfunc TestA(t *testing.T) { _ = A() }\n",
		"b/b.go":      "package b\n\nfunc B() int { return 2 }\n",
		"b/b_test.go": "package b\n\nimport \"testing\"\n\nfunc TestB(t *testing.T) { _ = B() }\n",
		"Makefile":    "go-test:\n\tGOTOOLCHAIN=local go test -p 1 -count=1 ./...\n",
	}
	for name, body := range files {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, name)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, arguments := range [][]string{
		{"init", "-q"}, {"config", "user.email", "corvint@example.test"}, {"config", "user.name", "Corvint Test"},
		{"config", "maintenance.auto", "false"}, {"config", "gc.auto", "0"}, {"add", "."}, {"commit", "-qm", "base"},
	} {
		affectedGit(t, root, arguments...)
	}
	base = strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
	if err := os.WriteFile(filepath.Join(root, "a/a.go"), []byte("package a\n\nfunc A() int { return 3 }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	affectedGit(t, root, "commit", "-qam", "change a")
	bin := t.TempDir()
	for name, body := range map[string]string{"go": sessionFakeGo, "make": sessionFakeMake} {
		if err := os.WriteFile(filepath.Join(bin, name), []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	log = filepath.Join(t.TempDir(), "runners.log")
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("SESSION_FAKE_LOG", log)
	t.Setenv("SESSION_FAKE_MODE", mode)
	return root, base, log
}

func sessionRunnerLog(t *testing.T, log string) string {
	t.Helper()
	data, err := os.ReadFile(log)
	if errors.Is(err, os.ErrNotExist) {
		return ""
	}
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func readSessionReceipt(t *testing.T, name string) sessiontest.Receipt {
	t.Helper()
	receipt, err := sessiontest.ReadReceipt(name)
	if err != nil {
		t.Fatal(err)
	}
	return receipt
}

// STR-V0-001, STR-V0-018: a dry run plans in process, writes only the named
// manifest, prints the argv and launches no runner.
func TestSTRV0018CLIDryRunLaunchesNoRunner(t *testing.T) {
	root, base, log := sessionFixture(t, "pass")
	manifest := filepath.Join(t.TempDir(), "manifest.json")
	code, stdout, stderr := runCLI(t, "--root", root, "test", "--base", base, "--dry-run", "--manifest", manifest)
	if code != 0 || stderr != "corvint-test: NARROWED 1 execution units\n" {
		t.Fatalf("code %d stderr %q", code, stderr)
	}
	var printed struct {
		Invocations []struct{ Argv []string } `json:"invocations"`
		Scope       string                    `json:"scope"`
	}
	if err := json.Unmarshal([]byte(stdout), &printed); err != nil {
		t.Fatal(err)
	}
	if printed.Scope != "NARROWED" || len(printed.Invocations) != 1 ||
		strings.Join(printed.Invocations[0].Argv, " ") != "go test -json -p 1 -count=1 example.test/m/a" {
		t.Fatalf("dry run printed %s", stdout)
	}
	if got := sessionRunnerLog(t, log); got != "" {
		t.Fatalf("a dry run launched runners:\n%s", got)
	}
	if _, _, err := sessiontest.ReadManifest(manifest); err != nil {
		t.Fatal(err)
	}
	if status := affectedGit(t, root, "status", "--porcelain", "--ignored"); status != "" {
		t.Fatalf("a dry run changed the repository: %q", status)
	}
	if _, err := os.Stat(filepath.Join(root, ".git", "corvint")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("a dry run wrote under .git/corvint")
	}
}

// STR-V0-005, STR-V0-010, STR-V0-011: a narrowed run launches only the
// selected package and writes the default receipt.
func TestSTRV0010CLIRunWritesReceipt(t *testing.T) {
	root, base, log := sessionFixture(t, "pass")
	code, _, stderr := runCLI(t, "--root", root, "test", "--base", base)
	receiptPath := filepath.Join(root, ".git", "corvint", "test-run.json")
	if code != 0 || !strings.Contains(stderr, "corvint-test: PASS selected units passed: 1 of 1; the full suite runs under make gate and the required CI checks; receipt ") {
		t.Fatalf("code %d stderr %q", code, stderr)
	}
	if got := sessionRunnerLog(t, log); got != "go env GOVERSION\ngo list ./...\ngo test -json -p 1 -count=1 example.test/m/a\n" {
		t.Fatalf("runner calls:\n%s", got)
	}
	receipt := readSessionReceipt(t, receiptPath)
	head := strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
	if receipt.Result != sessiontest.ResultPass || receipt.Scope != sessiontest.ScopeNarrowed || receipt.Base != base ||
		receipt.Revision != head || len(receipt.Invocations) != 1 || receipt.Invocations[0].Version != "go1.27.1" ||
		receipt.Invocations[0].Outcomes[0] != (sessiontest.Outcome{Outcome: "PASS", Slice: "whole", Unit: "go:example.test/m/a"}) ||
		!strings.HasPrefix(receipt.Tool, "corvint ") || len(receipt.Incomplete) != 0 {
		t.Fatalf("receipt %+v", receipt)
	}
	if status := affectedGit(t, root, "status", "--porcelain"); status != "" {
		t.Fatalf("a run changed the worktree: %q", status)
	}
}

// STR-V0-003: with no plan the declared full scope runs, read independently
// of the planner.
func TestSTRV0003CLINoPlanRunsDeclaredScope(t *testing.T) {
	root, _, log := sessionFixture(t, "pass")
	missing := strings.Repeat("e", 40)
	receiptPath := filepath.Join(t.TempDir(), "r.json")
	code, _, stderr := runCLI(t, "--root", root, "test", "--base", missing, "--receipt", receiptPath)
	if code != 0 || !strings.Contains(stderr, "corvint-test: FALLBACK no plan: ") {
		t.Fatalf("code %d stderr %q", code, stderr)
	}
	if got := sessionRunnerLog(t, log); got != "make --version\nmake go-test\n" {
		t.Fatalf("runner calls:\n%s", got)
	}
	receipt := readSessionReceipt(t, receiptPath)
	if receipt.Scope != sessiontest.ScopeFull || receipt.Result != sessiontest.ResultPass || receipt.Summary != "FULL units passed: 1 of 1" {
		t.Fatalf("receipt %+v", receipt)
	}
}

// STR-V0-007, STR-V0-009: every shard of one manifest, the empty one
// included, writes a receipt, and verify-shards accepts only their union.
func TestSTRV0009CLIShardsAndVerify(t *testing.T) {
	root, base, _ := sessionFixture(t, "pass")
	dir := t.TempDir()
	manifest := filepath.Join(dir, "manifest.json")
	if code, _, stderr := runCLI(t, "--root", root, "test", "--base", base, "--shard", "0/2", "--dry-run", "--manifest", manifest); code != 0 {
		t.Fatalf("dry run %d %s", code, stderr)
	}
	receipts := []string{}
	empty := 0
	for _, shard := range []string{"0/2", "1/2"} {
		code, _, stderr := runCLI(t, "--root", root, "test", "--base", base, "--shard", shard, "--manifest", manifest)
		if code != 0 {
			t.Fatalf("shard %s: %d %s", shard, code, stderr)
		}
		name := filepath.Join(root, ".git", "corvint", "test-run."+shard[:1]+".json")
		if readSessionReceipt(t, name).Summary == "selected units passed: 0 of 0; the full suite runs under make gate and the required CI checks" {
			empty++
		}
		receipts = append(receipts, name)
	}
	if empty != 1 {
		t.Fatalf("%d empty shards, want 1", empty)
	}
	code, stdout, _ := runCLI(t, append([]string{"test", "verify-shards", "--manifest", manifest}, receipts...)...)
	if code != 0 || !strings.Contains(stdout, `"reasons":[],"result":"PASS"`) {
		t.Fatalf("verify-shards %d %s", code, stdout)
	}
	code, stdout, _ = runCLI(t, "test", "verify-shards", "--manifest", manifest, receipts[0])
	if code != 3 || !strings.Contains(stdout, `"result":"INCOMPLETE"`) || !strings.Contains(stdout, "has no accepted receipt") {
		t.Fatalf("one receipt %d %s", code, stdout)
	}
	code, stdout, _ = runCLI(t, "test", "verify-shards", "--manifest", manifest, receipts[0], manifest)
	if code != 3 || !strings.Contains(stdout, "unreadable receipt") {
		t.Fatalf("manifest as receipt %d %s", code, stdout)
	}
	other := filepath.Join(dir, "other.json")
	if code, _, _ := runCLI(t, "--root", root, "test", "--base", base, "--dry-run", "--manifest", other); code != 0 {
		t.Fatal("unsharded dry run failed")
	}
	receiptPath := filepath.Join(dir, "mismatch.json")
	code, _, stderr := runCLI(t, "--root", root, "test", "--base", base, "--shard", "0/2", "--manifest", other, "--receipt", receiptPath)
	if code != 3 || !strings.Contains(stderr, "the supplied manifest is not usable: it differs from the manifest this shard recomputed") {
		t.Fatalf("foreign manifest %d %q", code, stderr)
	}
}

// STR-V0-014: a signal kills the owned group, starts no fallback, records
// INCOMPLETE and exits 128+n.
func TestSTRV0014CLISignalKillsRunAndExits(t *testing.T) {
	root, base, log := sessionFixture(t, "sleep")
	receiptPath := filepath.Join(t.TempDir(), "r.json")
	type result struct {
		code   int
		stderr string
	}
	done := make(chan result, 1)
	go func() {
		var stdout, stderr bytes.Buffer
		code := runContext(context.Background(), []string{"--root", root, "test", "--base", base, "--receipt", receiptPath}, strings.NewReader(""), &stdout, &stderr)
		done <- result{code, stderr.String()}
	}()
	for deadline := time.Now().Add(30 * time.Second); ; time.Sleep(20 * time.Millisecond) {
		if _, err := os.Stat(log + ".started"); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the fake go test never started")
		}
	}
	if err := syscall.Kill(os.Getpid(), syscall.SIGHUP); err != nil {
		t.Fatal(err)
	}
	var got result
	select {
	case got = <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("the run did not stop after SIGHUP")
	}
	if got.code != 129 {
		t.Fatalf("exit %d, stderr %q", got.code, got.stderr)
	}
	receipt := readSessionReceipt(t, receiptPath)
	if receipt.Result != sessiontest.ResultIncomplete || !strings.Contains(strings.Join(receipt.Incomplete, "\n"), "terminated by signal hangup; owned process groups killed; no fallback started") {
		t.Fatalf("receipt %s %v", receipt.Result, receipt.Incomplete)
	}
	if calls := sessionRunnerLog(t, log); strings.Count(calls, "go test") != 1 || strings.Contains(calls, "make") {
		t.Fatalf("runner calls after the signal:\n%s", calls)
	}
}

func TestSTRV0008CLIUsageRefusals(t *testing.T) {
	base := strings.Repeat("a", 40)
	for _, args := range [][]string{
		{"test", "--base", base, "--placement", "balanced"},
		{"test", "--base", base, "--require-clean"},
		{"test", "--base", "HEAD"},
		{"test", "--base", base, "--profile", "ci"},
		{"test", "--base", base, "--confine", "/x"},
		{"test", "--base", base, "--shard", "0/65"},
		{"test", "--base", base, "--shard", "2/2"},
		{"test", "verify-shards", "--manifest", "m.json"},
	} {
		code, stdout, stderr := runCLI(t, args...)
		if code != 2 || stdout != "" || stderr == "" {
			t.Errorf("%v: code %d stdout %q", args, code, stdout)
		}
	}
	if _, _, stderr := runCLI(t, "test", "--base", base, "--placement", "balanced"); !strings.Contains(stderr, "STR-V0-008") {
		t.Fatalf("balanced refusal %q", stderr)
	}
}
