package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/mcp/bridge"
	"github.com/Beamfall/corvint/internal/mcp/protocol"
)

// MCPV0-039: Git runs with the sanitized environment, so an inherited
// GIT_DIR, GIT_WORK_TREE or GIT_INDEX_FILE cannot redirect the move to
// another checkout.
func TestMCPV0039InheritedGitEnvironmentCannotRedirectMove(t *testing.T) {
	skipUnqualifiedStatus(t)
	main, lane := followFixture(t)
	handler := followHandler(t, lane, "feature")
	other := filepath.Join(filepath.Clean(t.TempDir()), "other")
	gitTest(t, main, "worktree", "add", "-q", "--detach", other, "feature")
	old := gitTest(t, other, "rev-parse", "HEAD")
	otherGitDir := gitTest(t, other, "rev-parse", "--path-format=absolute", "--git-dir")
	otherIndex := filepath.Join(otherGitDir, "index")
	indexBefore, err := os.ReadFile(otherIndex)
	if err != nil {
		t.Fatal(err)
	}
	moved := commitFile(t, main, "NEW.md", "moved\n")
	for key, value := range map[string]string{"GIT_DIR": otherGitDir, "GIT_WORK_TREE": other, "GIT_INDEX_FILE": otherIndex} {
		t.Setenv(key, value)
	}
	_, block := followCall(t, handler)
	for _, key := range []string{"GIT_DIR", "GIT_WORK_TREE", "GIT_INDEX_FILE"} {
		os.Unsetenv(key)
	}
	if block["state"] != "REFRESHED" || block["head"] != moved {
		t.Fatalf("move: %#v", block)
	}
	if gitTest(t, lane, "rev-parse", "HEAD") != moved || gitTest(t, other, "rev-parse", "HEAD") != old {
		t.Fatal("inherited Git environment redirected the move")
	}
	if indexAfter, err := os.ReadFile(otherIndex); err != nil || !bytes.Equal(indexBefore, indexAfter) {
		t.Fatalf("other checkout's index rewritten: %v", err)
	}
}

// MCPV0-039: the follower is bound to the registry's canonical root and its
// identities; retargeting the configured path or replacing the root never
// moves another checkout.
func TestMCPV0039FollowRefPinnedToCanonicalRootIdentity(t *testing.T) {
	skipUnqualifiedStatus(t)
	main, lane := followFixture(t)
	lane2 := filepath.Join(filepath.Dir(lane), "lane2")
	gitTest(t, main, "worktree", "add", "-q", "--detach", lane2, "feature")
	old := gitTest(t, lane, "rev-parse", "HEAD")
	link := filepath.Join(filepath.Dir(lane), "link")
	if err := os.Symlink(lane, link); err != nil {
		t.Fatal(err)
	}
	registry, registryErr := bridge.New(link)
	if registryErr != nil {
		t.Fatal(registryErr)
	}
	follow, refusal := newFollowRef(context.Background(), "git", registry, "feature")
	if refusal != "" {
		t.Fatal(refusal)
	}
	handler := &toolHandler{registry: registry, follow: follow}
	if err := os.Remove(link); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(lane2, link); err != nil {
		t.Fatal(err)
	}
	moved := commitFile(t, main, "NEW.md", "moved\n")
	if _, block := followCall(t, handler); block["state"] != "REFRESHED" || block["head"] != moved {
		t.Fatalf("pinned move: %#v", block)
	}
	if gitTest(t, lane, "rev-parse", "HEAD") != moved || gitTest(t, lane2, "rev-parse", "HEAD") != old {
		t.Fatal("retargeted path moved another checkout")
	}

	away := lane + "-away"
	if err := os.Rename(lane, away); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(lane2, lane); err != nil {
		t.Fatal(err)
	}
	again := commitFile(t, main, "AGAIN.md", "again\n")
	if _, block := followCall(t, handler); block["state"] != "ROOT_CHANGED" || block["stale"] != true || block["mutates"] != false || block["commit"] != again {
		t.Fatalf("replaced root: %#v", block)
	}
	if gitTest(t, lane2, "rev-parse", "HEAD") != old || gitTest(t, away, "rev-parse", "HEAD") != moved {
		t.Fatal("replaced root mutated a checkout")
	}
}

// MCPV0-039: the move and the answer run under one lock, so a concurrent
// call cannot move the worktree while an answer is being produced.
func TestMCPV0039AnswerHoldsTheFollowLock(t *testing.T) {
	skipUnqualifiedStatus(t)
	main, lane := followFixture(t)
	handler := followHandler(t, lane, "feature")
	old := gitTest(t, lane, "rev-parse", "HEAD")
	started, release, second := make(chan followState, 1), make(chan struct{}), make(chan followState, 1)
	firstDone := make(chan struct{})
	go func() {
		defer close(firstDone)
		handler.follow.bound(context.Background(), func(state followState, _ string) (map[string]any, *protocol.RPCError) {
			started <- state
			<-release
			return nil, nil
		})
	}()
	if state := <-started; state.Head != old {
		t.Fatalf("first: %#v", state)
	}
	moved := commitFile(t, main, "NEW.md", "moved\n")
	go handler.follow.bound(context.Background(), func(state followState, _ string) (map[string]any, *protocol.RPCError) {
		second <- state
		return nil, nil
	})
	select {
	case state := <-second:
		t.Fatalf("second call ran during the first answer: %#v", state)
	case <-time.After(2 * time.Second):
	}
	if gitTest(t, lane, "rev-parse", "HEAD") != old {
		t.Fatal("worktree moved under an answer")
	}
	close(release)
	<-firstDone
	select {
	case state := <-second:
		if state.Head != moved || state.State != "REFRESHED" {
			t.Fatalf("second: %#v", state)
		}
	case <-time.After(2 * time.Minute):
		t.Fatal("second call never ran")
	}
}

// MCPV0-040: the dirty and attached checks run even when HEAD already equals
// the ref's commit, and neither writes the index.
func TestMCPV0040ChecksRunWhenHeadAlreadyCurrent(t *testing.T) {
	skipUnqualifiedStatus(t)
	_, lane := followFixture(t)
	handler := followHandler(t, lane, "feature")
	head := gitTest(t, lane, "rev-parse", "HEAD")
	refreshes := 0
	handler.follow.refresh = func(context.Context, string) (bool, error) { refreshes++; return true, nil }
	scratch := filepath.Join(lane, "scratch.txt")
	if err := os.WriteFile(scratch, []byte("untracked\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, block := followCall(t, handler); block["state"] != "DIRTY" || block["stale"] != true || block["head"] != head || block["mutates"] != false {
		t.Fatalf("dirty at current head: %#v", block)
	}
	if err := os.Remove(scratch); err != nil {
		t.Fatal(err)
	}
	gitTest(t, lane, "checkout", "-q", "-b", "local")
	if _, block := followCall(t, handler); block["state"] != "NOT_DETACHED" || block["stale"] != true || block["mutates"] != false {
		t.Fatalf("attached at current head: %#v", block)
	}
	if refreshes != 0 {
		t.Fatalf("index written %d times by a dirty or attached worktree", refreshes)
	}
}

// MCPV0-040: the move never overwrites an ignored local file that the new
// commit tracks; the collision is a stale answer on the old head.
func TestMCPV0040MoveNeverOverwritesIgnoredFiles(t *testing.T) {
	skipUnqualifiedStatus(t)
	main, lane := followFixture(t)
	handler := followHandler(t, lane, "feature")
	old := gitTest(t, lane, "rev-parse", "HEAD")
	common := gitTest(t, main, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err := os.MkdirAll(filepath.Join(common, "info"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(common, "info", "exclude"), []byte("gen.txt\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	local := filepath.Join(lane, "gen.txt")
	if err := os.WriteFile(local, []byte("local precious\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(main, "gen.txt"), []byte("tracked\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	gitTest(t, main, "add", "-f", "gen.txt")
	gitTest(t, main, "commit", "-q", "-m", "track gen.txt")
	moved := gitTest(t, main, "rev-parse", "HEAD")
	if _, block := followCall(t, handler); block["state"] != "CHECKOUT_FAILED" || block["stale"] != true || block["commit"] != moved || block["head"] != old {
		t.Fatalf("ignored collision: %#v", block)
	}
	if got, _ := os.ReadFile(local); string(got) != "local precious\n" || gitTest(t, lane, "rev-parse", "HEAD") != old {
		t.Fatal("ignored file overwritten or HEAD moved")
	}
}
