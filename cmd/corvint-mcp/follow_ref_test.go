package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/mcp/bridge"
)

func gitTest(t *testing.T, root string, arguments ...string) string {
	t.Helper()
	output, err := exec.Command("git", append([]string{"-C", root}, arguments...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", arguments, err, output)
	}
	return strings.TrimSpace(string(output))
}

// followFixture is a repository whose checked-out branch `feature` is
// followed by a detached linked worktree.
func followFixture(t *testing.T) (main, lane string) {
	t.Helper()
	main = makeHostileRepository(t, "follow")
	gitTest(t, main, "checkout", "-q", "-b", "feature")
	lane = filepath.Join(filepath.Clean(t.TempDir()), "lane")
	gitTest(t, main, "worktree", "add", "-q", "--detach", lane, "feature")
	lane, err := filepath.EvalSymlinks(lane)
	if err != nil {
		t.Fatal(err)
	}
	return main, lane
}

func followHandler(t *testing.T, lane, ref string) *toolHandler {
	t.Helper()
	registry, registryErr := bridge.New(lane)
	if registryErr != nil {
		t.Fatal(registryErr)
	}
	follow, refusal := newFollowRef(context.Background(), "git", registry, ref)
	if refusal != "" {
		t.Fatal(refusal)
	}
	return &toolHandler{registry: registry, follow: follow}
}

func followCall(t *testing.T, handler *toolHandler) (map[string]any, map[string]any) {
	t.Helper()
	result, failure := handler.call(context.Background(), map[string]any{"_meta": map[string]any{}, "name": "corvint.status", "arguments": map[string]any{}})
	if failure != nil {
		t.Fatalf("protocol failure: %#v", failure)
	}
	content := result["content"].([]any)
	var block map[string]any
	if err := json.Unmarshal([]byte(content[len(content)-1].(map[string]any)["text"].(string)), &block); err != nil || block["schema"] != followRefSchema {
		t.Fatalf("no follow-ref block: %#v", content)
	}
	return result, block
}

func commitFile(t *testing.T, root, name, body string) string {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	gitTest(t, root, "add", name)
	gitTest(t, root, "commit", "-q", "-m", "move "+name)
	return gitTest(t, root, "rev-parse", "HEAD")
}

// MCPV0-039: a moved ref refreshes the dedicated worktree and its index
// snapshot before the next answer, which describes the new commit; the
// startup contract refuses anything but one plain root on a detached linked
// worktree with a resolvable ref.
func TestMCPV0039FollowRefRefreshesTreeAndIndexBeforeAnswer(t *testing.T) {
	skipUnqualifiedStatus(t)
	main, lane := followFixture(t)
	handler := followHandler(t, lane, "feature")
	first := gitTest(t, main, "rev-parse", "HEAD")
	result, block := followCall(t, handler)
	if block["state"] != "CURRENT" || block["commit"] != first || block["head"] != first || block["stale"] != false || block["ref"] != "feature" {
		t.Fatalf("first call: %#v", block)
	}
	if statusRevision(t, map[string]any{"result": result}) != first {
		t.Fatal("first answer not bound to the ref")
	}
	moved := commitFile(t, main, "NEW.md", "moved\n")
	result, block = followCall(t, handler)
	if block["state"] != "REFRESHED" || block["commit"] != moved || block["head"] != moved || block["stale"] != false || block["mutates"] != true {
		t.Fatalf("moved ref: %#v", block)
	}
	if statusRevision(t, map[string]any{"result": result}) != moved || gitTest(t, lane, "rev-parse", "HEAD") != moved {
		t.Fatal("answer or worktree not on the moved commit")
	}
	if _, fresh, err := contextindex.ProbeSnapshot(context.Background(), lane); err != nil || !fresh {
		t.Fatalf("index snapshot not refreshed for the moved commit: %v", err)
	}
	if err := exec.Command("git", "-C", lane, "symbolic-ref", "-q", "HEAD").Run(); err == nil {
		t.Fatal("worktree attached to a branch")
	}
	if _, block = followCall(t, handler); block["state"] != "CURRENT" || block["mutates"] != false {
		t.Fatalf("unchanged ref rewrote state: %#v", block)
	}

	refuse := func(arguments []string, want string) {
		t.Helper()
		var stdout, stderr bytes.Buffer
		if exit := run(context.Background(), arguments, strings.NewReader(""), &stdout, &stderr); exit != 2 || !strings.Contains(stderr.String(), want) {
			t.Fatalf("%v: exit=%d stderr=%q", arguments, exit, stderr.String())
		}
	}
	refuse([]string{"--root", main, "--follow-ref", "feature"}, followRefNotDedicated)
	attached := filepath.Join(filepath.Clean(t.TempDir()), "attached")
	gitTest(t, main, "worktree", "add", "-q", "-b", "other", attached)
	refuse([]string{"--root", attached, "--follow-ref", "feature"}, followRefNotDedicated)
	refuse([]string{"--root", lane, "--follow-ref", "no-such-branch"}, followRefMissing)
	refuse([]string{"--root", "a=" + lane, "--follow-ref", "feature"}, "invalid arguments")
	refuse([]string{"--root", lane, "--follow-ref", "feature", "--follow-ref", "feature"}, "invalid arguments")
	refuse([]string{"--root", lane, "--follow-ref", "-x"}, "invalid arguments")
	refuse([]string{"--version", "--follow-ref", "feature"}, "invalid arguments")
}

// MCPV0-040: a dirty or attached worktree is never modified; the answer
// describes the old head and is flagged stale. An index refresh failure is
// flagged too.
func TestMCPV0040DirtyTreeNeverModifiedAndFlaggedStale(t *testing.T) {
	skipUnqualifiedStatus(t)
	main, lane := followFixture(t)
	handler := followHandler(t, lane, "feature")
	old := gitTest(t, lane, "rev-parse", "HEAD")
	moved := commitFile(t, main, "NEW.md", "moved\n")
	for _, dirty := range []struct{ name, body string }{{"scratch.txt", "untracked\n"}, {"AGENTS.md", "edited\n"}} {
		path := filepath.Join(lane, dirty.name)
		before, _ := os.ReadFile(path)
		if err := os.WriteFile(path, []byte(dirty.body), 0o600); err != nil {
			t.Fatal(err)
		}
		result, block := followCall(t, handler)
		if block["state"] != "DIRTY" || block["stale"] != true || block["commit"] != moved || block["head"] != old || block["mutates"] != false {
			t.Fatalf("%s: %#v", dirty.name, block)
		}
		if got, _ := os.ReadFile(path); string(got) != dirty.body || gitTest(t, lane, "rev-parse", "HEAD") != old || statusRevision(t, map[string]any{"result": result}) != old {
			t.Fatalf("%s: dirty worktree modified", dirty.name)
		}
		if before == nil {
			_ = os.Remove(path)
		} else if err := os.WriteFile(path, before, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	gitTest(t, lane, "checkout", "-q", "-b", "local")
	if _, block := followCall(t, handler); block["state"] != "NOT_DETACHED" || block["stale"] != true || gitTest(t, lane, "rev-parse", "HEAD") != old {
		t.Fatalf("attached worktree: %#v", block)
	}
	gitTest(t, lane, "checkout", "-q", "--detach")
	handler.follow.refresh = func(context.Context, string) (bool, error) { return false, errors.New("index failed") }
	if _, block := followCall(t, handler); block["state"] != "INDEX_FAILED" || block["stale"] != true || block["head"] != moved {
		t.Fatalf("index failure: %#v", block)
	}
}

// MCPV0-041: a ref that stops resolving is a typed tool error that still
// echoes the binding; every response carries the resolved commit.
func TestMCPV0041MissingRefIsTypedErrorAndCommitEchoed(t *testing.T) {
	skipUnqualifiedStatus(t)
	main, lane := followFixture(t)
	head := gitTest(t, main, "rev-parse", "HEAD")
	gitTest(t, main, "update-ref", "refs/heads/follow", head)
	handler := followHandler(t, lane, "follow")
	handler.reasonClass = true
	if _, block := followCall(t, handler); block["commit"] != head {
		t.Fatalf("commit not echoed: %#v", block)
	}
	gitTest(t, main, "update-ref", "-d", "refs/heads/follow")
	result, block := followCall(t, handler)
	structured, _ := result["structuredContent"].(map[string]any)
	if result["isError"] != true || structured["code"] != followRefMissing || structured["reasonClass"] != unclassified || structured["tool"] != "corvint.status" {
		t.Fatalf("missing ref not a typed tool error: %#v", result)
	}
	if block["state"] != "MISSING" || block["stale"] != true || block["commit"] != "" || block["head"] != head || gitTest(t, lane, "rev-parse", "HEAD") != head {
		t.Fatalf("missing ref block: %#v", block)
	}
}
