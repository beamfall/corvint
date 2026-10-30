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
// (or, in sleep mode, signals readiness and sleeps), and `make` passes
// except for its rust-test target.
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
[ "$1" = rust-test ] && exit 1
exit 0
`

// sessionDeclaration is the fixture's corvint-test-commands/0 file, with
// any extra language members spliced in.
func sessionDeclaration(languages string) string {
	return `{"profile": "corvint-test-commands/0", "go": {"env": {"GOTOOLCHAIN": "local"}, "flags": ["-p", "1", "-count=1"]}, "languages": {` + languages + `}}` + "\n"
}

// sessionFixture is a two-package module whose HEAD changes package a.
func sessionFixture(t *testing.T, mode string) (root, base, log string) {
	t.Helper()
	root = t.TempDir()
	files := map[string]string{
		"go.mod":                    "module example.test/m\n\ngo 1.27\n",
		"a/a.go":                    "package a\n\nfunc A() int { return 1 }\n",
		"a/a_test.go":               "package a\n\nimport \"testing\"\n\nfunc TestA(t *testing.T) { _ = A() }\n",
		"b/b.go":                    "package b\n\nfunc B() int { return 2 }\n",
		"b/b_test.go":               "package b\n\nimport \"testing\"\n\nfunc TestB(t *testing.T) { _ = B() }\n",
		sessiontest.DeclarationPath: sessionDeclaration(""),
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
	if code != 0 || stderr != "corvint-test: NARROWED 1 execution units\n"+
		"corvint-test: whole-package go test planned; under script/heavy.sh run it with HEAVY_WHOLE_REASON='corvint test selected example.test/m/a whole: whole: DIRECT_SOURCE_CHANGE via a/a.go' (STR-V0-073)\n" {
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
		receipt.Invocations[0].AdapterManifest != sessiontest.AdapterManifestProfile ||
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
	if got := sessionRunnerLog(t, log); got != "go env GOVERSION\ngo test -p 1 -count=1 ./...\n" {
		t.Fatalf("runner calls:\n%s", got)
	}
	receipt := readSessionReceipt(t, receiptPath)
	if receipt.Scope != sessiontest.ScopeFull || receipt.Result != sessiontest.ResultPass || receipt.Summary != "FULL units passed: 1 of 1" ||
		len(receipt.Invocations) != 1 || receipt.Invocations[0].Adapter != sessiontest.AdapterDeclared || receipt.Invocations[0].Version != "go1.27.1" {
		t.Fatalf("receipt %+v", receipt)
	}
}

// STR-V0-003, STR-V0-004: with no plan, a declared language full command is
// found from the declaration, not the plan, and its failure fails the run;
// a {"none": reason} language is recorded and runs nothing.
func TestSTRV0003CLINoPlanRunsDeclaredLanguageCommand(t *testing.T) {
	root, _, log := sessionFixture(t, "pass")
	declaration := sessionDeclaration(`"rust": ["make", "rust-test"], "python": {"none": "fixtures only"}`)
	if err := os.WriteFile(filepath.Join(root, sessiontest.DeclarationPath), []byte(declaration), 0o644); err != nil {
		t.Fatal(err)
	}
	affectedGit(t, root, "commit", "-qam", "declare rust")
	receiptPath := filepath.Join(t.TempDir(), "r.json")
	manifestPath := filepath.Join(t.TempDir(), "m.json")
	code, stdout, stderr := runCLI(t, "--root", root, "test", "--base", strings.Repeat("e", 40), "--dry-run", "--manifest", manifestPath)
	if code != 0 || !strings.Contains(stdout, `"noSuite":["python: fixtures only"]`) {
		t.Fatalf("dry run code %d stdout %s stderr %q", code, stdout, stderr)
	}
	if m, _, err := sessiontest.ReadManifest(manifestPath); err != nil || len(m.NoSuite) != 1 || len(m.Units) != 2 {
		t.Fatalf("manifest %+v %v", m, err)
	}
	code, _, stderr = runCLI(t, "--root", root, "test", "--base", strings.Repeat("e", 40), "--receipt", receiptPath)
	if code != 1 {
		t.Fatalf("code %d stderr %q", code, stderr)
	}
	if got := sessionRunnerLog(t, log); got != "go env GOVERSION\nmake --version\ngo test -p 1 -count=1 ./...\nmake rust-test\n" {
		t.Fatalf("runner calls:\n%s", got)
	}
	if receipt := readSessionReceipt(t, receiptPath); receipt.Result != sessiontest.ResultFail || receipt.Summary != "FULL units passed: 1 of 2" {
		t.Fatalf("receipt %s %q", receipt.Result, receipt.Summary)
	}
}

// STR-V0-003: a declared repository-relative command runs from --root when
// the CLI is invoked from another directory (the test process's working
// directory is the package directory, not root).
func TestSTRV0003CLIDeclaredRelativeCommandRunsFromRoot(t *testing.T) {
	root, _, log := sessionFixture(t, "pass")
	script := filepath.Join(root, "tools", "fake-tests")
	if err := os.MkdirAll(filepath.Dir(script), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(script, []byte("#!/bin/sh\necho \"fake-tests $* in $(pwd)\" >> \"$SESSION_FAKE_LOG\"\n[ \"$1\" = --version ] && echo fake-tests 1\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	declaration := sessionDeclaration(`"python": ["./tools/fake-tests"]`)
	if err := os.WriteFile(filepath.Join(root, sessiontest.DeclarationPath), []byte(declaration), 0o644); err != nil {
		t.Fatal(err)
	}
	affectedGit(t, root, "add", ".")
	affectedGit(t, root, "commit", "-qm", "declare python")
	if cwd, _ := os.Getwd(); cwd == root {
		t.Fatal("the test already runs from root; the resolution would not be exercised")
	}
	receiptPath := filepath.Join(t.TempDir(), "r.json")
	code, _, stderr := runCLI(t, "--root", root, "test", "--base", strings.Repeat("e", 40), "--receipt", receiptPath)
	if code != 0 {
		t.Fatalf("code %d stderr %q", code, stderr)
	}
	resolvedRoot, _ := filepath.EvalSymlinks(root)
	got := sessionRunnerLog(t, log)
	if !strings.Contains(got, "fake-tests --version in "+resolvedRoot+"\n") || !strings.Contains(got, "fake-tests  in "+resolvedRoot+"\n") {
		t.Fatalf("runner calls:\n%s", got)
	}
	receipt := readSessionReceipt(t, receiptPath)
	if receipt.Result != sessiontest.ResultPass || len(receipt.Invocations) != 2 || receipt.Invocations[1].Runner != filepath.Join(resolvedRoot, "tools", "fake-tests") {
		t.Fatalf("receipt %s %+v", receipt.Result, receipt.Invocations)
	}
}

// STR-V0-003: a missing or malformed declaration is INCOMPLETE in every
// scope; nothing runs on a guessed full scope.
func TestSTRV0003CLIUnreadableDeclarationIsIncomplete(t *testing.T) {
	root, base, log := sessionFixture(t, "pass")
	if err := os.WriteFile(filepath.Join(root, sessiontest.DeclarationPath), []byte(`{"profile": "corvint-test-commands/0"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	affectedGit(t, root, "commit", "-qam", "break the declaration")
	receiptPath := filepath.Join(t.TempDir(), "r.json")
	code, _, stderr := runCLI(t, "--root", root, "test", "--base", base, "--receipt", receiptPath)
	if code != 3 || !strings.Contains(stderr, "the declared full scope is unreadable: "+sessiontest.DeclarationPath+": ") {
		t.Fatalf("code %d stderr %q", code, stderr)
	}
	if got := sessionRunnerLog(t, log); got != "" {
		t.Fatalf("an unreadable declaration launched runners:\n%s", got)
	}
	if receipt := readSessionReceipt(t, receiptPath); receipt.Result != sessiontest.ResultIncomplete || len(receipt.Invocations) != 0 {
		t.Fatalf("receipt %+v", receipt)
	}
}

// STR-V0-001: an output path that is tracked, Git metadata or a .corvint/
// ledger is refused before anything runs.
func TestSTRV0001CLIRefusesProtectedOutputPaths(t *testing.T) {
	root, base, log := sessionFixture(t, "pass")
	before, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"--receipt", filepath.Join(root, "go.mod")},
		{"--receipt", filepath.Join(root, "a")},
		{"--receipt", filepath.Join(root, ".git", "index")},
		{"--receipt", filepath.Join(root, ".git", "corvint", "traces.json")},
		{"--receipt", filepath.Join(root, ".corvint", "self-observations.jsonl")},
		{"--dry-run", "--manifest", filepath.Join(root, "go.mod")},
		{"--dry-run", "--manifest", filepath.Join(root, ".git", "config")},
	} {
		code, stdout, stderr := runCLI(t, append([]string{"--root", root, "test", "--base", base}, args...)...)
		if code != 2 || stdout != "" || !strings.Contains(stderr, args[len(args)-1]) {
			t.Errorf("%v: code %d stdout %q stderr %q", args, code, stdout, stderr)
		}
	}
	if after, _ := os.ReadFile(filepath.Join(root, "go.mod")); string(after) != string(before) {
		t.Fatal("the tracked go.mod changed")
	}
	if got := sessionRunnerLog(t, log); got != "" {
		t.Fatalf("a refused run launched runners:\n%s", got)
	}
	// A leaf symlink is the entry a write replaces, so it is refused even
	// when its target would be allowed; an unignored worktree file would
	// change the bound source identity.
	outside := t.TempDir()
	target := filepath.Join(outside, "target.json")
	if err := os.WriteFile(target, []byte("keep\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(outside, "link.json")); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"--receipt", filepath.Join(outside, "link.json")},
		{"--dry-run", "--manifest", filepath.Join(root, "manifest.json")},
	} {
		code, _, stderr := runCLI(t, append([]string{"--root", root, "test", "--base", base}, args...)...)
		if code != 2 || !strings.Contains(stderr, args[len(args)-1]) {
			t.Errorf("%v: code %d stderr %q", args, code, stderr)
		}
	}
	if kept, _ := os.ReadFile(target); string(kept) != "keep\n" {
		t.Fatal("the symlink target was overwritten")
	}
	if _, err := os.Lstat(filepath.Join(root, "manifest.json")); err == nil {
		t.Fatal("an unignored worktree manifest was written")
	}
	allowed := filepath.Join(root, ".git", "corvint", "test-run.1.json")
	if code, _, stderr := runCLI(t, "--root", root, "test", "--base", base, "--receipt", allowed); code != 0 {
		t.Fatalf("default receipt name refused: %d %s", code, stderr)
	}
	// The default destination passes the same guard: a linked
	// <git-dir>/corvint cannot redirect it into protected state.
	linked, linkedBase, linkedLog := sessionFixture(t, "pass")
	if err := os.MkdirAll(filepath.Join(linked, ".corvint"), 0o755); err != nil {
		t.Fatal(err)
	}
	_ = os.RemoveAll(filepath.Join(linked, ".git", "corvint"))
	if err := os.Symlink(filepath.Join(linked, ".corvint"), filepath.Join(linked, ".git", "corvint")); err != nil {
		t.Fatal(err)
	}
	code, _, stderr := runCLI(t, "--root", linked, "test", "--base", linkedBase)
	if code != 2 || !strings.Contains(stderr, "default receipt") || !strings.Contains(stderr, "under .corvint/") {
		t.Fatalf("linked default receipt: %d %s", code, stderr)
	}
	if _, err := os.Lstat(filepath.Join(linked, ".corvint", "test-run.json")); err == nil {
		t.Fatal("the default receipt was written through the link")
	}
	if got := sessionRunnerLog(t, linkedLog); got != "" {
		t.Fatalf("a refused run launched runners:\n%s", got)
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
		{"test", "--base", base, "--placement", "packed"},
		{"test", "--base", base, "--placement", "balanced", "--shard", "0/2"},
		{"test", "--base", base, "--placement", "balanced", "--profile", "ci", "--confine", "/x"},
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
	for _, args := range [][]string{
		{"test", "--base", base, "--placement", "balanced", "--shard", "1/3"},
		{"test", "--base", base, "--placement", "balanced", "--profile", "ci", "--confine", "/x", "--manifest", "/m"},
	} {
		if _, _, stderr := runCLI(t, args...); !strings.Contains(stderr, "STR-V0-008") {
			t.Errorf("%v: balanced refusal %q", args, stderr)
		}
	}
}

// STR-V0-008: a sharded balanced run executes against one dry-run manifest
// that every shard reproduces; verify-shards accepts the union; a shard that
// recomputes a different manifest is INCOMPLETE and runs nothing; the ci
// profile computes balanced placement with --dry-run and launches nothing.
func TestSTRV0008CLIBalancedShards(t *testing.T) {
	root, _, log := sessionFixture(t, "pass")
	base := strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD~1"))
	if err := os.WriteFile(filepath.Join(root, "b/b.go"), []byte("package b\n\nfunc B() int { return 4 }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	affectedGit(t, root, "commit", "-qam", "change b")
	dir := t.TempDir()
	manifest := filepath.Join(dir, "manifest.json")
	code, stdout, stderr := runCLI(t, "--root", root, "test", "--base", base, "--placement", "balanced", "--shard", "0/2", "--dry-run", "--manifest", manifest)
	if code != 0 || stderr != "corvint-test: NARROWED 2 execution units\n"+
		"corvint-test: whole-package go test planned; under script/heavy.sh run it with HEAVY_WHOLE_REASON='corvint test selected example.test/m/a whole: whole: DIRECT_SOURCE_CHANGE via a/a.go' (STR-V0-073)\n" || !strings.Contains(stdout, `"placement":"balanced"`) {
		t.Fatalf("dry run %d %q %s", code, stderr, stdout)
	}
	planned, _, err := sessiontest.ReadManifest(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if planned.Placement != sessiontest.PlacementBalanced || len(planned.Units) != 2 || planned.Units[0].Shard == planned.Units[1].Shard {
		t.Fatalf("balanced manifest %+v", planned.Units)
	}
	receipts := []string{}
	for _, shard := range []string{"0/2", "1/2"} {
		name := filepath.Join(dir, "run."+shard[:1]+".json")
		code, _, stderr := runCLI(t, "--root", root, "test", "--base", base, "--placement", "balanced", "--shard", shard, "--manifest", manifest, "--receipt", name)
		if code != 0 {
			t.Fatalf("shard %s: %d %s", shard, code, stderr)
		}
		if receipt := readSessionReceipt(t, name); receipt.Placement != sessiontest.PlacementBalanced || receipt.Summary != "selected units passed: 1 of 1; the full suite runs under make gate and the required CI checks" {
			t.Fatalf("shard %s receipt %s %q", shard, receipt.Placement, receipt.Summary)
		}
		receipts = append(receipts, name)
	}
	code, stdout, _ = runCLI(t, append([]string{"test", "verify-shards", "--manifest", manifest}, receipts...)...)
	if code != 0 || !strings.Contains(stdout, `"reasons":[],"result":"PASS"`) {
		t.Fatalf("verify-shards %d %s", code, stdout)
	}
	before := strings.Count(sessionRunnerLog(t, log), "go test")
	code, _, stderr = runCLI(t, "--root", root, "test", "--base", base, "--shard", "1/2", "--manifest", manifest, "--receipt", filepath.Join(dir, "universe.json"))
	if code != 3 || !strings.Contains(stderr, "it differs from the manifest this shard recomputed") {
		t.Fatalf("universe shard against a balanced manifest %d %q", code, stderr)
	}
	if after := strings.Count(sessionRunnerLog(t, log), "go test"); after != before {
		t.Fatalf("a mismatched shard ran go test (%d before, %d after)", before, after)
	}
	confine := filepath.Join(dir, "test-confine")
	code, stdout, stderr = runCLI(t, "--root", root, "test", "--base", base, "--placement", "balanced", "--profile", "ci", "--confine", confine, "--shard", "1/2", "--dry-run", "--manifest", filepath.Join(dir, "ci.json"))
	if code != 0 || !strings.Contains(stdout, `"placement":"balanced"`) || !strings.Contains(stdout, `"-exec","`+confine+`"`) {
		t.Fatalf("ci dry run %d %q %s", code, stderr, stdout)
	}
	if after := strings.Count(sessionRunnerLog(t, log), "go test"); after != before {
		t.Fatal("the ci dry run launched go test")
	}
}

// AFP-V0-050 (V1-1399): `corvint test` narrows an edit of the read-scope
// declaration against the plan's base, as the planner does: declaring a new
// package runs only that package, not every declared one.
func TestCorvintTestNarrowsReadScopeDeclarationEdit_AFPV0050(t *testing.T) {
	root, _, _ := sessionFixture(t, "pass")
	declaration := func(packages string) string {
		return `{"profile":"corvint-test-read-scopes/0","packages":{` + packages + `}}`
	}
	for name, body := range map[string]string{
		"c/c.go":                         "package c\n\nfunc C() int { return 4 }\n",
		"c/c_test.go":                    "package c\n\nimport \"testing\"\n\nfunc TestC(t *testing.T) { _ = C() }\n",
		".corvint/test-read-scopes.json": declaration(`"a":["docs/"],"b":["docs/"]`),
	} {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, name)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	affectedGit(t, root, "add", ".")
	affectedGit(t, root, "commit", "-qm", "declare a and b")
	base := strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
	if err := os.WriteFile(filepath.Join(root, ".corvint/test-read-scopes.json"), []byte(declaration(`"a":["docs/"],"b":["docs/"],"c":["docs/"]`)), 0o644); err != nil {
		t.Fatal(err)
	}
	affectedGit(t, root, "commit", "-qam", "declare c")
	code, stdout, stderr := runCLI(t, "--root", root, "test", "--base", base, "--dry-run", "--manifest", filepath.Join(t.TempDir(), "manifest.json"))
	var printed struct {
		Invocations []struct{ Argv []string } `json:"invocations"`
	}
	if code != 0 {
		t.Fatalf("code %d stderr %q", code, stderr)
	}
	if err := json.Unmarshal([]byte(stdout), &printed); err != nil {
		t.Fatal(err)
	}
	if len(printed.Invocations) != 1 || strings.Join(printed.Invocations[0].Argv, " ") != "go test -json -p 1 -count=1 example.test/m/c" {
		t.Fatalf("dry run printed %s", stdout)
	}
}
