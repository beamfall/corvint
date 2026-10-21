package dogfoodflow

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// V1-2261: the bind commit (dogfood-finish) and the seal commit keep the
// caller's environment, so exported GIT_AUTHOR_* and GIT_COMMITTER_* identity
// outranks a different global user identity.
func TestDogfoodCommitsHonourExportedGitIdentity(t *testing.T) {
	home := t.TempDir()
	global := filepath.Join(home, ".gitconfig")
	if err := os.WriteFile(global, []byte("[user]\n\tname = Global Person\n\temail = global@example.invalid\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	t.Setenv("GIT_CONFIG_GLOBAL", global)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_AUTHOR_NAME", "Lane Author")
	t.Setenv("GIT_AUTHOR_EMAIL", "author@example.invalid")
	t.Setenv("GIT_COMMITTER_NAME", "Lane Committer")
	t.Setenv("GIT_COMMITTER_EMAIL", "committer@example.invalid")

	for name, commit := range map[string]func(root string, args ...string) int{
		"finish-lane": func(root string, args ...string) int {
			lane := &finishLane{flow: flow{ctx: context.Background(), root: root, stderr: io.Discard}}
			return lane.gitPassthrough(args...)
		},
		"seal": func(root string, args ...string) int {
			sealer := &check{flow: flow{ctx: context.Background(), root: root, stderr: io.Discard}, stdout: io.Discard}
			return sealer.gitPassthrough(args...)
		},
	} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			if output, err := exec.Command("git", "-C", root, "init", "-q").CombinedOutput(); err != nil {
				t.Fatalf("git init: %v %s", err, output)
			}
			if code := commit(root, "commit", "-q", "--allow-empty", "-m", "chore: identity"); code != 0 {
				t.Fatalf("commit status %d", code)
			}
			output, err := exec.Command("git", "-C", root, "log", "-1", "--format=%an <%ae>|%cn <%ce>").Output()
			if err != nil {
				t.Fatal(err)
			}
			want := "Lane Author <author@example.invalid>|Lane Committer <committer@example.invalid>"
			if got := strings.TrimSpace(string(output)); got != want {
				t.Fatalf("commit identity = %q, want %q", got, want)
			}
		})
	}
}
