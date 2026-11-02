// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/testplatform"
)

func keysFor(t *testing.T, steps []string) []string {
	t.Helper()
	l := open(io.Discard, io.Discard)
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
	t.Setenv(sharedDigestEnv, "")
	slow := keysFor(t, steps)
	file := filepath.Join(t.TempDir(), "digest.json")
	var out bytes.Buffer
	if code := run([]string{"digest", file}, &out, &out); code != 0 || !strings.Contains(out.String(), "DIGEST tree ") {
		t.Fatalf("digest: %d %q", code, out.String())
	}
	t.Setenv(sharedDigestEnv, file)
	if l, ok := openShared(io.Discard, io.Discard); !ok || l.tree == "" || len(l.entries) == 0 {
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
		t.Setenv(sharedDigestEnv, bad)
		if _, ok := openShared(io.Discard, io.Discard); ok {
			t.Errorf("%s: shared digest used", name)
		}
		if got := keysFor(t, steps); strings.Join(got, ",") != strings.Join(slow, ",") {
			t.Errorf("%s: fallback keys differ", name)
		}
	}
}

// TestSharedDigestVerifyForgetsRecordsOfAMovedTree_GLV0786: an unchanged tree
// verifies; a worktree edited after the digest fails verification and loses
// the records written under the shared tree, so its next run misses.
func TestSharedDigestVerifyForgetsRecordsOfAMovedTree_GLV0786(t *testing.T) {
	testplatform.SkipPOSIXShell(t, "the ledger step under test is an sh -c command")
	root := fixtureRepository(t)
	write(t, root, ".gitignore", "COUNT\n")
	file := filepath.Join(t.TempDir(), "digest.json")
	t.Setenv(sharedDigestEnv, "")
	if code, out := ledgerRun(t, "digest", file); code != 0 {
		t.Fatalf("digest: %q", out)
	}
	t.Setenv(sharedDigestEnv, file)
	step := append([]string{"run", "decision-numbers-check+aaa"}, counting...)
	if _, out := ledgerRun(t, step...); !strings.Contains(out, "RECORD decision-numbers-check+aaa") {
		t.Fatalf("shared run: %q", out)
	}
	if code, out := ledgerRun(t, "digest", "--verify", file); code != 0 || !strings.Contains(out, "DIGEST unchanged") {
		t.Fatalf("unchanged verify: %d %q", code, out)
	}
	write(t, root, "docs/decisions/0001.md", "moved\n")
	code, out := ledgerRun(t, "digest", "--verify", file)
	if code != 1 || !strings.Contains(out, "DIGEST changed during the run") || !strings.Contains(out, "removed 1 records") {
		t.Fatalf("moved verify: %d %q", code, out)
	}
	write(t, root, "docs/decisions/0001.md", "one\n")
	t.Setenv(sharedDigestEnv, "")
	if _, out := ledgerRun(t, step...); !strings.Contains(out, "RUN decision-numbers-check+aaa: no recorded pass") {
		t.Fatalf("a record of the moved run survived: %q", out)
	}
	if code, _ := ledgerRun(t, "digest"); code != 2 {
		t.Fatalf("digest without FILE: exit %d, want usage 2", code)
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
