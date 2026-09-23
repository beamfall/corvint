package main

import (
	"encoding/json"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// caseFoldCLIContents are two tracked paths that differ only in case; a
// case-insensitive worktree holds only one of them.
var caseFoldCLIContents = map[string]string{
	"internal/token/token.go": "package token\n\nfunc MintToken() string { return \"lower\" }\n",
	"internal/token/Token.go": "package token\n\nfunc MintTokenUpper() string { return \"upper\" }\n",
}

// caseFoldCLIRepository commits caseFoldCLIContents through Git plumbing so
// the fixture also exists on a case-insensitive filesystem. It returns the
// root, the pre-collision base commit, the committed blob per path, and the
// paths `git status` reports dirty afterwards (sorted; empty on a
// case-sensitive filesystem).
func caseFoldCLIRepository(t *testing.T) (root, base string, blobs map[string]string, dirty []string) {
	t.Helper()
	root = cliRepository(t)
	cemWrite(t, root, "go.mod", "module example.test/fixture\n\ngo 1.27.0\n")
	cemGit(t, root, "add", ".")
	cemGit(t, root, "commit", "-qm", "module")
	base = cemGit(t, root, "rev-parse", "HEAD")
	blobs = make(map[string]string, len(caseFoldCLIContents))
	for _, path := range slices.Sorted(maps.Keys(caseFoldCLIContents)) {
		blob := filepath.Join(t.TempDir(), "blob")
		if err := os.WriteFile(blob, []byte(caseFoldCLIContents[path]), 0o644); err != nil {
			t.Fatal(err)
		}
		blobs[path] = cemGit(t, root, "hash-object", "-w", blob)
		cemGit(t, root, "update-index", "--add", "--cacheinfo", "100644,"+blobs[path]+","+path)
	}
	tree := cemGit(t, root, "write-tree")
	commit := cemGit(t, root, "commit-tree", tree, "-p", "HEAD", "-m", "case-fold collision")
	cemGit(t, root, "update-ref", "HEAD", commit)
	cemGit(t, root, "checkout", "-q", "--", ".")
	dirty = make([]string, 0)
	for _, line := range strings.Split(cemGit(t, root, "status", "--porcelain"), "\n") {
		if fields := strings.Fields(line); len(fields) == 2 {
			dirty = append(dirty, fields[1])
		}
	}
	slices.Sort(dirty)
	return root, base, blobs, dirty
}

// runCLITwice runs one read verb twice and fails unless both runs agree byte
// for byte: a case-fold collision must be answered deterministically.
func runCLITwice(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	code, stdout, stderr := runCLI(t, args...)
	again, stdoutAgain, stderrAgain := runCLI(t, args...)
	if code != again || stdout != stdoutAgain || stderr != stderrAgain {
		t.Fatalf("%v is not deterministic:\n%d %s%s\n%d %s%s", args, code, stdout, stderr, again, stdoutAgain, stderrAgain)
	}
	return code, stdout, stderr
}

// evidenceBlobs maps each result's first evidence path to its blob_hash.
func evidenceBlobs(t *testing.T, receipt map[string]any) map[string]string {
	t.Helper()
	blobs := make(map[string]string)
	for _, result := range receipt["results"].([]any) {
		evidence := result.(map[string]any)["evidence"].([]any)[0].(map[string]any)
		blobs[evidence["path"].(string)] = evidence["blob_hash"].(string)
	}
	return blobs
}

func decodeReceipt(t *testing.T, stdout string) map[string]any {
	t.Helper()
	var receipt map[string]any
	if err := json.Unmarshal([]byte(stdout), &receipt); err != nil {
		t.Fatalf("%v: %s", err, stdout)
	}
	return receipt
}

// assertFreshness checks the receipt's freshness block names exactly the
// paths Git reports dirty: `mixed-worktree` with the colliding path on a
// case-insensitive filesystem, `fresh` with none on a case-sensitive one.
func assertFreshness(t *testing.T, receipt map[string]any, dirty []string) {
	t.Helper()
	freshness := receipt["freshness"].(map[string]any)
	wantState := "fresh"
	if len(dirty) != 0 {
		wantState = "mixed-worktree"
	}
	mixed := make([]string, 0)
	for _, path := range freshness["mixed_paths"].([]any) {
		mixed = append(mixed, path.(string))
	}
	if freshness["state"] != wantState || !slices.Equal(mixed, dirty) {
		t.Fatalf("freshness = %v, want state %s over %v", freshness, wantState, dirty)
	}
}

// TestCaseFoldCollidingPathsAreDisclosedNotSilentlyMerged checks GPK-V0-006
// at the CLI: over two tracked paths differing only in case, index, query,
// context and path impact each carry both paths pinned to their own committed
// blob (never the other case's bytes), the worktree divergence is disclosed
// as `mixed-worktree` naming the colliding path, range impact refuses by name
// over that divergence, and every answer is deterministic.
func TestCaseFoldCollidingPathsAreDisclosedNotSilentlyMerged(t *testing.T) {
	t.Parallel()
	root, base, blobs, dirty := caseFoldCLIRepository(t)
	if code, _, stderr := runCLITwice(t, "--root", root, "index"); code != 0 {
		t.Fatalf("index exit %d: %s", code, stderr)
	}
	t.Run("query", func(t *testing.T) {
		code, stdout, stderr := runCLITwice(t, "--root", root, "query", "--task", "mint token", "--limit", "5")
		if code != 0 {
			t.Fatalf("exit %d: %s", code, stderr)
		}
		receipt := decodeReceipt(t, stdout)["context"].(map[string]any)
		if got := evidenceBlobs(t, receipt); !maps.Equal(got, blobs) {
			t.Fatalf("query evidence = %v, want %v", got, blobs)
		}
		assertFreshness(t, receipt, dirty)
	})
	t.Run("context", func(t *testing.T) {
		code, stdout, stderr := runCLITwice(t, "--root", root, "context", "--task", "mint token")
		if code != 0 {
			t.Fatalf("exit %d: %s", code, stderr)
		}
		if got := evidenceBlobs(t, decodeReceipt(t, stdout)); !maps.Equal(got, blobs) {
			t.Fatalf("context evidence = %v, want %v", got, blobs)
		}
	})
	for path, blob := range blobs {
		t.Run("impact "+path, func(t *testing.T) {
			code, stdout, stderr := runCLITwice(t, "--root", root, "impact", path)
			if code != 0 {
				t.Fatalf("exit %d: %s", code, stderr)
			}
			receipt := decodeReceipt(t, stdout)["context"].(map[string]any)
			if got := evidenceBlobs(t, receipt); !maps.Equal(got, map[string]string{path: blob}) {
				t.Fatalf("impact evidence = %v, want only %s at %s", got, path, blob)
			}
			assertFreshness(t, receipt, dirty)
		})
	}
	t.Run("range impact", func(t *testing.T) {
		code, _, stderr := runCLITwice(t, "--root", root, "impact", "--base", base)
		if len(dirty) == 0 {
			if code != 0 {
				t.Fatalf("clean case-sensitive worktree refused: exit %d: %s", code, stderr)
			}
			return
		}
		if code != 2 || !strings.Contains(stderr, `"code": "unsupported-impact-worktree"`) {
			t.Fatalf("exit %d stderr %q, want the named unsupported-impact-worktree refusal", code, stderr)
		}
	})
}
