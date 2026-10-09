package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// TestContextInALinkedWorktreeAnswersForItsOwnHead is V1-0998 (TCP-V0-001,
// DIRTY-CACHE-001, DIRTY-CACHE-013): `context --root LINKED` builds over the
// linked worktree's own `HEAD^{tree}`, not the primary checkout's, both when
// the shared store holds only the primary's snapshot (a miss) and when it
// holds the linked tree's own (a hit). The packet's `revision` is that tree
// OID, never the commit, which is how the reported mismatch arose: commit
// 0b5096ca has tree 21f255f3. Neither run writes the store or either worktree.
func TestContextInALinkedWorktreeAnswersForItsOwnHead(t *testing.T) {
	t.Parallel()
	root := queryCLIRepository(t)
	linked := filepath.Join(t.TempDir(), "linked")
	gitFixture(t, root, "worktree", "add", "-q", "-b", "linked-head", linked, "HEAD")
	if err := os.WriteFile(filepath.Join(linked, "internal", "parser", "lexer.go"),
		[]byte("package parser\n\n// LexWorktreeOnly exists only at the linked worktree's HEAD.\nfunc LexWorktreeOnly() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitFixture(t, linked, "add", ".")
	gitFixture(t, linked, "commit", "-qm", "move the linked worktree to another tree")
	primaryTree := gitFixture(t, root, "rev-parse", "HEAD^{tree}")
	linkedCommit := gitFixture(t, linked, "rev-parse", "HEAD")
	linkedTree := gitFixture(t, linked, "rev-parse", "HEAD^{tree}")
	if primaryTree == linkedTree || gitFixture(t, root, "rev-parse", "HEAD") == linkedCommit {
		t.Fatal("fixture: the linked worktree's HEAD must differ from the primary checkout's")
	}
	common, err := filepath.EvalSymlinks(gitFixture(t, linked, "rev-parse", "--path-format=absolute", "--git-common-dir"))
	if err != nil {
		t.Fatal(err)
	}
	store := filepath.Join(common, "corvint", "index")
	// snapshots is every file under the shared store with its content digest.
	snapshots := func() []string {
		t.Helper()
		var found []string
		err := filepath.WalkDir(store, func(path string, entry fs.DirEntry, err error) error {
			if err != nil || entry.IsDir() {
				return err
			}
			body, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			digest := sha256.Sum256(body)
			found = append(found, path+" "+hex.EncodeToString(digest[:]))
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		return found
	}
	const task = "where is LexWorktreeOnly defined in the lexer"
	contextAt := func(at string, wantHit bool) map[string]any {
		t.Helper()
		// The CLI path does not expose the loader outcome; the same compile does.
		if _, hit, err := compileTaskContext(context.Background(), taskContextOptions{root: at, task: task, limit: 5, lsp: "off", lspSet: true}, loadContextSnapshot); err != nil || hit != wantHit {
			t.Fatalf("V1-0998: context --root %s snapshot hit=%t err=%v, want hit=%t", at, hit, err, wantHit)
		}
		before, primaryStatus, linkedStatus := snapshots(), gitFixture(t, root, "status", "--porcelain"), gitFixture(t, linked, "status", "--porcelain")
		code, stdout, stderr := runContextCommand(t, "--root", at, "context", "--task", task, "--limit", "5")
		if code != 0 {
			t.Fatalf("context --root %s exit %d: %s", at, code, stderr)
		}
		if after := snapshots(); !slices.Equal(before, after) ||
			gitFixture(t, root, "status", "--porcelain") != primaryStatus || gitFixture(t, linked, "status", "--porcelain") != linkedStatus {
			t.Fatalf("TCP-V0-001: context wrote state: store %v -> %v", before, after)
		}
		return decodeObject(t, stdout)
	}
	names := func(packet map[string]any) bool {
		rows, _ := packet["results"].([]any)
		for _, row := range rows {
			if fields, _ := row.(map[string]any); fields["id"] == "internal/parser/lexer.go" {
				return true
			}
		}
		return false
	}

	// Only the primary checkout's tree is in the shared store: a miss.
	runIndexForTest(t, root, false)
	for _, phase := range []string{"miss", "hit"} {
		packet := contextAt(linked, phase == "hit")
		if packet["revision"] != linkedTree || packet["mutates"] != false || !names(packet) {
			t.Fatalf("V1-0998 %s: linked context revision=%v want linked tree %s (primary tree %s, commit %s), lexer named=%t",
				phase, packet["revision"], linkedTree, primaryTree, linkedCommit, names(packet))
		}
		if phase == "miss" {
			if receipt := writingIndexReceipt(t, runIndexForTest(t, linked, false)); receipt["tree"] != linkedTree || receipt["store"] != store {
				t.Fatalf("V1-0998: linked index receipt = %v", receipt)
			}
		}
	}
	if packet := contextAt(root, true); packet["revision"] != primaryTree || names(packet) {
		t.Fatalf("V1-0998: primary context revision=%v want %s, lexer named=%t", packet["revision"], primaryTree, names(packet))
	}
}
