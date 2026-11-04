// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/testplatform"
)

func keysFor(t *testing.T, steps []string) []string {
	t.Helper()
	l := open(*fixture(t), io.Discard, io.Discard)
	var keys []string
	for _, step := range steps {
		k, reason := l.key(step, nil)
		keys = append(keys, k+"|"+reason)
	}
	return keys
}

// TestSharedDigestKeysEqualOwnDigest_GLV0786 proves the fast path equals the
// slow path: every key a step derives from the shared digest file equals the
// key it derives from its own worktree digest, for a declared, an undeclared
// salted, a tree, a `@test:` and an unscoped step; a file for another root, a
// foreign schema and a malformed file are ignored, so the step digests itself.
func TestSharedDigestKeysEqualOwnDigest_GLV0786(t *testing.T) {
	testplatform.SkipPOSIXShell(t, "the fixture repository is driven through git")
	root := testScopeRepository(t, `{"profile":"corvint-test-read-scopes/0","packages":{"checks/chk":["docs/read.md"]}}`+"\n")
	writeAll(t, root, "notes/untracked.md", "untracked\n")
	withScope(t, "fixture-check", []string{testScopePrefix + "checks/chk"})
	steps := []string{"decision-numbers-check", "decision-numbers-check+aaa", "nosuchstep+bbb", "line-citations-check+ccc", "fixture-check+ddd", "nosuchstep"}
	setenv(t, sharedDigestEnv, "")
	slow := keysFor(t, steps)
	file := filepath.Join(t.TempDir(), "digest.json")
	var out bytes.Buffer
	if code := run(*fixture(t), []string{"digest", file}, &out, &out); code != 0 || !strings.Contains(out.String(), "DIGEST tree ") {
		t.Fatalf("digest: %d %q", code, out.String())
	}
	setenv(t, sharedDigestEnv, file)
	if l, ok := openShared(*fixture(t), io.Discard, io.Discard); !ok || l.tree == "" || len(l.entries) == 0 {
		t.Fatal("the shared digest was not used")
	}
	fast := keysFor(t, steps)
	for i := range steps {
		if fast[i] != slow[i] {
			t.Errorf("%s: shared key %q, own key %q", steps[i], fast[i], slow[i])
		}
	}
	for name, body := range map[string]string{
		"other root":     strings.Replace(mustRead(t, file), root, filepath.Join(root, "elsewhere"), 1),
		"foreign schema": strings.Replace(mustRead(t, file), sharedDigestSchema, "other/1", 1),
		"malformed":      "{",
	} {
		bad := filepath.Join(t.TempDir(), "bad.json")
		if err := os.WriteFile(bad, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		setenv(t, sharedDigestEnv, bad)
		if _, ok := openShared(*fixture(t), io.Discard, io.Discard); ok {
			t.Errorf("%s: shared digest used", name)
		}
		if got := keysFor(t, steps); strings.Join(got, ",") != strings.Join(slow, ",") {
			t.Errorf("%s: fallback keys differ", name)
		}
	}
}

// TestSharedDigestVerifyForgetsRecordsOfAMovedTree_GLV0786: an unchanged tree
// verifies and promotes the staged pass; a worktree edited after the digest
// fails verification, discards the pass it staged and loses the records
// promoted under the shared tree, so its next run misses.
func TestSharedDigestVerifyForgetsRecordsOfAMovedTree_GLV0786(t *testing.T) {
	testplatform.SkipPOSIXShell(t, "the ledger step under test is an sh -c command")
	root := fixtureRepository(t)
	write(t, root, ".gitignore", "COUNT\n")
	file := filepath.Join(t.TempDir(), "digest.json")
	setenv(t, sharedDigestEnv, "")
	if code, out := ledgerRun(t, "digest", file); code != 0 {
		t.Fatalf("digest: %q", out)
	}
	setenv(t, sharedDigestEnv, file)
	step := append([]string{"run", "decision-numbers-check+aaa"}, counting...)
	if _, out := ledgerRun(t, step...); !strings.Contains(out, "RECORD decision-numbers-check+aaa") {
		t.Fatalf("shared run: %q", out)
	}
	if code, out := ledgerRun(t, "digest", "--verify", file); code != 0 || !strings.Contains(out, "DIGEST unchanged") || !strings.Contains(out, "promoted 1 staged records") {
		t.Fatalf("unchanged verify: %d %q", code, out)
	}
	other := append([]string{"run", "decision-numbers-check+bbb"}, counting...)
	if _, out := ledgerRun(t, other...); !strings.Contains(out, "RECORD decision-numbers-check+bbb") {
		t.Fatalf("second shared run: %q", out)
	}
	write(t, root, "docs/decisions/0001.md", "moved\n")
	code, out := ledgerRun(t, "digest", "--verify", file)
	if code != 1 || !strings.Contains(out, "DIGEST changed during the run") || !strings.Contains(out, "discarded 1 staged records, removed 1 records") {
		t.Fatalf("moved verify: %d %q", code, out)
	}
	write(t, root, "docs/decisions/0001.md", "one\n")
	setenv(t, sharedDigestEnv, "")
	if _, out := ledgerRun(t, "plan", "decision-numbers-check+aaa", "decision-numbers-check+bbb"); !strings.Contains(out, "RUN decision-numbers-check+aaa: no recorded pass") || !strings.Contains(out, "RUN decision-numbers-check+bbb: no recorded pass") {
		t.Fatalf("a record of the moved run survived: %q", out)
	}
	if code, _ := ledgerRun(t, "digest"); code != 2 {
		t.Fatalf("digest without FILE: exit %d, want usage 2", code)
	}
}

// TestSharedDigestStagesPassesUntilVerify_GLV0786 replays the GL-V0-786 review
// finding: a pass recorded on the shared digest stays in the run's private
// stage, so a concurrent consumer, whether on its own digest or on another
// run's shared digest, finds no record of it; only digest --verify on an
// unchanged tree promotes it into the ledger. A run interrupted before
// --verify, whose stage is then removed or simply abandoned, leaves the
// ledger without the pass, and a fresh digest empties a reused stage.
func TestSharedDigestStagesPassesUntilVerify_GLV0786(t *testing.T) {
	testplatform.SkipPOSIXShell(t, "the ledger step under test is an sh -c command")
	root := fixtureRepository(t)
	write(t, root, ".gitignore", "COUNT\n")
	file := filepath.Join(t.TempDir(), "digest.json")
	setenv(t, sharedDigestEnv, "")
	if code, out := ledgerRun(t, "digest", file); code != 0 {
		t.Fatalf("digest: %q", out)
	}
	if info, err := os.Lstat(stageDirectory(file)); err != nil || !info.IsDir() || info.Mode().Perm() != 0o700 {
		t.Fatalf("stage: %v %v", info, err)
	}
	setenv(t, sharedDigestEnv, file)
	step := append([]string{"run", "decision-numbers-check+aaa"}, counting...)
	if _, out := ledgerRun(t, step...); !strings.Contains(out, "RECORD decision-numbers-check+aaa") || !strings.Contains(out, "staged until digest --verify") {
		t.Fatalf("shared run: %q", out)
	}
	if got := records(t); got != 0 {
		t.Fatalf("an unverified pass reached the ledger: %d records", got)
	}
	// A concurrent consumer on its own digest, and one on another run's shared digest.
	setenv(t, sharedDigestEnv, "")
	if _, out := ledgerRun(t, "plan", "decision-numbers-check+aaa"); !strings.Contains(out, "RUN decision-numbers-check+aaa: no recorded pass") {
		t.Fatalf("own-digest consumer saw an unverified pass: %q", out)
	}
	another := filepath.Join(t.TempDir(), "digest.json")
	if code, out := ledgerRun(t, "digest", another); code != 0 {
		t.Fatalf("second digest: %q", out)
	}
	setenv(t, sharedDigestEnv, another)
	if _, out := ledgerRun(t, "plan", "decision-numbers-check+aaa"); !strings.Contains(out, "RUN decision-numbers-check+aaa: no recorded pass") {
		t.Fatalf("another run saw an unverified pass: %q", out)
	}
	// Interruption: the run never verifies. Its abandoned stage is never read,
	// and a fresh digest through the same file empties it.
	setenv(t, sharedDigestEnv, "")
	if code, out := ledgerRun(t, "digest", file); code != 0 {
		t.Fatalf("redigest: %q", out)
	}
	if code, out := ledgerRun(t, "digest", "--verify", file); code != 0 || !strings.Contains(out, "promoted 0 staged records") {
		t.Fatalf("a pass of the interrupted run was promoted: %d %q", code, out)
	}
	if got := records(t); got != 0 {
		t.Fatalf("the interrupted run left %d records", got)
	}
	// The verified path: stage, verify, then every consumer hits.
	setenv(t, sharedDigestEnv, file)
	if _, out := ledgerRun(t, step...); !strings.Contains(out, "RECORD decision-numbers-check+aaa") {
		t.Fatalf("rerun: %q", out)
	}
	setenv(t, sharedDigestEnv, "")
	if code, out := ledgerRun(t, "digest", "--verify", file); code != 0 || !strings.Contains(out, "promoted 1 staged records") {
		t.Fatalf("verify: %d %q", code, out)
	}
	if entries, err := os.ReadDir(stageDirectory(file)); err != nil || len(entries) != 0 {
		t.Fatalf("the stage kept %d entries after verify: %v", len(entries), err)
	}
	if _, out := ledgerRun(t, "plan", "decision-numbers-check+aaa"); !strings.Contains(out, "HIT decision-numbers-check+aaa") {
		t.Fatalf("a verified pass missed: %q", out)
	}
	if got := runs(t, root); got != 2 {
		t.Fatalf("the step ran %d times, want 2", got)
	}
}

// TestSharedDigestRefusesAPassWhoseTreeMovedDuringTheStep_GLV0786: a step
// that ran on a worktree edited after the shared digest passes but records
// nothing, even when the edit is undone before digest --verify (A, B, A), so
// no pass is keyed on content the step did not test. The check made
// immediately before the step starts is the one that sees the edit.
func TestSharedDigestRefusesAPassWhoseTreeMovedDuringTheStep_GLV0786(t *testing.T) {
	testplatform.SkipPOSIXShell(t, "the ledger step under test is an sh -c command")
	root := fixtureRepository(t)
	write(t, root, ".gitignore", "COUNT\n")
	file := filepath.Join(t.TempDir(), "digest.json")
	setenv(t, sharedDigestEnv, "")
	if code, out := ledgerRun(t, "digest", file); code != 0 {
		t.Fatalf("digest: %q", out)
	}
	write(t, root, "docs/decisions/0001.md", "edited between the digest and the step\n")
	setenv(t, sharedDigestEnv, file)
	step := append([]string{"run", "decision-numbers-check+aaa"}, counting...)
	code, out := ledgerRun(t, step...)
	if code != 0 || !strings.Contains(out, "RECORD decision-numbers-check+aaa skipped: its inputs changed before the step started") {
		t.Fatalf("moved step: %d %q", code, out)
	}
	write(t, root, "docs/decisions/0001.md", "one\n")
	setenv(t, sharedDigestEnv, "")
	if code, out := ledgerRun(t, "digest", "--verify", file); code != 0 || !strings.Contains(out, "promoted 0 staged records") {
		t.Fatalf("verify after the edit was undone: %d %q", code, out)
	}
	if _, out := ledgerRun(t, "plan", "decision-numbers-check+aaa"); !strings.Contains(out, "RUN decision-numbers-check+aaa: no recorded pass") {
		t.Fatalf("a pass of the moved tree was recorded: %q", out)
	}
	if got := runs(t, root); got != 1 {
		t.Fatalf("the step ran %d times, want 1", got)
	}
}

// TestSharedDigestRefusesAPassOfAnEditRestoredBeforeTheStepReturns_GLV0786
// replays the second review's finding: digest A, edit a declared input to B
// before the step starts, let the step pass on B and restore A before it
// returns. The tree after the step is A again, but the check made immediately
// before execution saw B, so nothing is staged or promoted and the next run
// tests A.
func TestSharedDigestRefusesAPassOfAnEditRestoredBeforeTheStepReturns_GLV0786(t *testing.T) {
	testplatform.SkipPOSIXShell(t, "the ledger step under test is an sh -c command")
	root := fixtureRepository(t)
	write(t, root, ".gitignore", "COUNT\n")
	file := filepath.Join(t.TempDir(), "digest.json")
	setenv(t, sharedDigestEnv, "")
	if code, out := ledgerRun(t, "digest", file); code != 0 {
		t.Fatalf("digest: %q", out)
	}
	write(t, root, "docs/decisions/0001.md", "B: edited after the digest, before the step\n")
	setenv(t, sharedDigestEnv, file)
	code, out := ledgerRun(t, "run", "decision-numbers-check+aaa", "--", "sh", "-c", `echo ran >> COUNT; printf 'one\n' > docs/decisions/0001.md`)
	if code != 0 || !strings.Contains(out, "RECORD decision-numbers-check+aaa skipped: its inputs changed before the step started") {
		t.Fatalf("restored step: %d %q", code, out)
	}
	if got := mustRead(t, filepath.Join(root, "docs", "decisions", "0001.md")); got != "one\n" {
		t.Fatalf("the step did not restore A: %q", got)
	}
	setenv(t, sharedDigestEnv, "")
	if code, out := ledgerRun(t, "digest", "--verify", file); code != 0 || !strings.Contains(out, "DIGEST unchanged") || !strings.Contains(out, "promoted 0 staged records") {
		t.Fatalf("verify on the restored tree: %d %q", code, out)
	}
	if _, out := ledgerRun(t, "plan", "decision-numbers-check+aaa"); !strings.Contains(out, "RUN decision-numbers-check+aaa: no recorded pass") {
		t.Fatalf("a pass of B was keyed on A: %q", out)
	}
	if got := records(t); got != 0 {
		t.Fatalf("%d records, want 0", got)
	}
}

// TestOwnDigestRefusesAPassWhoseInputChangedWhileTheStepRan_GLV0786 replays
// V1-4547: a process on its own digest, with no shared digest, whose step
// edits a declared input while it runs records nothing, so restoring the input
// afterwards cannot reuse a pass keyed on content the step did not end on. An
// unedited step on the same digest still records and then hits: the untracked
// COUNT file each run writes is outside the step's declared inputs.
func TestOwnDigestRefusesAPassWhoseInputChangedWhileTheStepRan_GLV0786(t *testing.T) {
	testplatform.SkipPOSIXShell(t, "the ledger step under test is an sh -c command")
	root := fixtureRepository(t)
	setenv(t, sharedDigestEnv, "")
	code, out := ledgerRun(t, "run", "decision-numbers-check+aaa", "--", "sh", "-c", `echo ran >> COUNT; printf 'B\n' > docs/decisions/0001.md`)
	if code != 0 || !strings.Contains(out, "RECORD decision-numbers-check+aaa skipped: its inputs changed while the step ran") {
		t.Fatalf("editing step: %d %q", code, out)
	}
	if got := records(t); got != 0 {
		t.Fatalf("the editing step left %d records", got)
	}
	write(t, root, "docs/decisions/0001.md", "one\n")
	if _, out := ledgerRun(t, "plan", "decision-numbers-check+aaa"); !strings.Contains(out, "RUN decision-numbers-check+aaa: no recorded pass") {
		t.Fatalf("the restored input reused the editing step's pass: %q", out)
	}
	if _, out := ledgerRun(t, append([]string{"run", "decision-numbers-check+aaa"}, counting...)...); !strings.Contains(out, "RECORD decision-numbers-check+aaa ") || strings.Contains(out, "skipped") {
		t.Fatalf("an unedited step did not record: %q", out)
	}
	if _, out := ledgerRun(t, "plan", "decision-numbers-check+aaa"); !strings.Contains(out, "HIT decision-numbers-check+aaa") {
		t.Fatalf("the unedited pass missed: %q", out)
	}
	if got := runs(t, root); got != 2 {
		t.Fatalf("the step ran %d times, want 2", got)
	}
}

// TestOwnDigestRefusesAPassOfAnEditRestoredBeforeTheStepReturns_GLV0786: on
// its own digest, an input edited after the process digested the worktree but
// before its step starts (here, once it holds the step's lock), and restored
// by the step before it returns, records nothing.
func TestOwnDigestRefusesAPassOfAnEditRestoredBeforeTheStepReturns_GLV0786(t *testing.T) {
	testplatform.SkipPOSIXShell(t, "the ledger step under test is an sh -c command")
	root := fixtureRepository(t)
	setenv(t, sharedDigestEnv, "")
	lockHook = func(step, event string) {
		if step == "decision-numbers-check+aaa" && event == "flock" {
			write(t, root, "docs/decisions/0001.md", "B: edited after the digest, before the step\n")
		}
	}
	t.Cleanup(func() { lockHook = nil })
	code, out := ledgerRun(t, "run", "decision-numbers-check+aaa", "--", "sh", "-c", `echo ran >> COUNT; printf 'one\n' > docs/decisions/0001.md`)
	lockHook = nil
	if code != 0 || !strings.Contains(out, "RECORD decision-numbers-check+aaa skipped: its inputs changed before the step started") {
		t.Fatalf("restored step: %d %q", code, out)
	}
	if _, out := ledgerRun(t, "plan", "decision-numbers-check+aaa"); !strings.Contains(out, "RUN decision-numbers-check+aaa: no recorded pass") {
		t.Fatalf("a pass of B was keyed on A: %q", out)
	}
	if got := records(t); got != 0 {
		t.Fatalf("%d records, want 0", got)
	}
}

// TestGoTestRecordsAPackagePassOnlyOnAnUnmovedTree_GLV0786: a resolved
// package's pass is not recorded when an input was edited before the batch
// started and restored by the batch, or edited while the batch ran; the next
// run on the original tree then runs the package, and an unedited batch
// records it and hits.
func TestGoTestRecordsAPackagePassOnlyOnAnUnmovedTree_GLV0786(t *testing.T) {
	testplatform.SkipPOSIXShell(t, "the ledger step under test is an sh -c command")
	root := packageRepository(t, map[string]string{
		"go.mod":            "module example.com/fixture\n\ngo 1.27\n",
		"core/core.go":      "package core\n",
		"core/core_test.go": "package core\n\nimport \"testing\"\n\nfunc TestCore(t *testing.T) {}\n",
	})
	passing := `for pkg in "$@"; do case "$pkg" in -*) continue;; esac; printf '{"Action":"start","Package":"%s"}\n{"Action":"pass","Package":"%s"}\n' "$pkg" "$pkg"; done`
	core := packageStep + " example.com/fixture/core"
	lockHook = func(step, event string) {
		if step == core && event == "flock" {
			write(t, root, "core/core.go", "package core\n\n// B: edited before the batch\n")
		}
	}
	t.Cleanup(func() { lockHook = nil })
	code, out := ledgerRun(t, "go-test", "--", "sh", "-c", `printf 'package core\n' > core/core.go; `+passing)
	lockHook = nil
	if code != 0 || !strings.Contains(out, "RECORD "+core+" skipped: the worktree moved before the step started") {
		t.Fatalf("restored batch: %d %q", code, out)
	}
	code, out = ledgerRun(t, "go-test", "--", "sh", "-c", `printf 'package core\n\n// C: edited by the batch\n' > core/core.go; `+passing)
	if code != 0 || !strings.Contains(out, "RECORD "+core+" skipped: the worktree moved while the step ran") {
		t.Fatalf("editing batch: %d %q", code, out)
	}
	write(t, root, "core/core.go", "package core\n")
	if _, out := ledgerRun(t, "go-test", "--", "sh", "-c", passing); !strings.Contains(out, "RUN "+core+": no recorded pass") || !strings.Contains(out, "RECORD "+core+" ") || strings.Contains(out, "RECORD "+core+" skipped") {
		t.Fatalf("unedited batch: %q", out)
	}
	if _, out := ledgerRun(t, "go-test", "--", "sh", "-c", passing); !strings.Contains(out, "HIT "+core+" ") {
		t.Fatalf("the unedited pass missed: %q", out)
	}
}

func mustRead(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// TestStepLabelIsRecordedOutsideTheKey_ASCV0776 replays V1-4242: a pass
// recorded before labels existed (no GATE_LEDGER_LABEL) still hits when the
// caller now names the step, a new record carries the label, and two labels of
// one step share one key, so the label never changes what reuses.
func TestStepLabelIsRecordedOutsideTheKey_ASCV0776(t *testing.T) {
	testplatform.SkipPOSIXShell(t, "the ledger step under test is an sh -c command")
	root := fixtureRepository(t)
	write(t, root, ".gitignore", "COUNT\n")
	setenv(t, sharedDigestEnv, "")
	setenv(t, labelEnv, "")
	old := append([]string{"run", "batch-precheck+aaa"}, counting...)
	if _, out := ledgerRun(t, old...); !strings.Contains(out, "RECORD batch-precheck+aaa") {
		t.Fatalf("unlabelled record: %q", out)
	}
	setenv(t, labelEnv, "sync-check")
	if _, out := ledgerRun(t, old...); !strings.Contains(out, "HIT batch-precheck+aaa") {
		t.Fatalf("an unlabelled pass missed once a label was given: %q", out)
	}
	setenv(t, labelEnv, "  go run\n./conformance/use-cases-v0  ")
	if _, out := ledgerRun(t, append([]string{"run", "batch-precheck+bbb"}, counting...)...); !strings.Contains(out, "RECORD batch-precheck+bbb") {
		t.Fatalf("labelled record: %q", out)
	}
	labels := map[string]string{}
	files, _ := filepath.Glob(filepath.Join(ledgerDirectoryOf(t), "*.json"))
	for _, f := range files {
		var rec record
		if err := json.Unmarshal([]byte(mustRead(t, f)), &rec); err != nil {
			t.Fatal(err)
		}
		labels[rec.Step] = rec.Label
	}
	if labels["batch-precheck+aaa"] != "" || labels["batch-precheck+bbb"] != "go run ./conformance/use-cases-v0" {
		t.Fatalf("labels %q", labels)
	}
	setenv(t, labelEnv, "another name")
	if _, out := ledgerRun(t, append([]string{"run", "batch-precheck+bbb"}, counting...)...); !strings.Contains(out, "HIT batch-precheck+bbb") {
		t.Fatalf("a different label missed: %q", out)
	}
	if got := runs(t, root); got != 2 {
		t.Fatalf("the steps ran %d times, want 2", got)
	}
}
