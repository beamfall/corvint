package gitancestry

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestRewrittenNamesGraftsShallowAndUnreadable_EEPV0028(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	run := func(ctx context.Context, args ...string) ([]byte, error) {
		command := exec.CommandContext(ctx, "git", args...)
		command.Dir = root
		return command.Output()
	}
	ctx := context.Background()
	for _, args := range [][]string{{"init", "-q"}, {"-c", "user.email=c@example.test", "-c", "user.name=C", "commit", "-q", "--allow-empty", "-m", "one"}} {
		if out, err := run(ctx, args...); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	if got := Rewritten(ctx, run); got != "" {
		t.Fatalf("plain checkout = %q, want trusted", got)
	}
	head, _ := run(ctx, "rev-parse", "HEAD")
	shallow := filepath.Join(root, ".git", "shallow")
	if err := os.WriteFile(shallow, head, 0o644); err != nil {
		t.Fatal(err)
	}
	if got := Rewritten(ctx, run); got != "checkout is shallow" {
		t.Errorf("shallow = %q", got)
	}
	if err := os.Remove(shallow); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".git", "info", "grafts"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if got := Rewritten(ctx, run); got != "checkout rewrites ancestry with info/grafts" {
		t.Errorf("grafts = %q", got)
	}
	failing := func(context.Context, ...string) ([]byte, error) { return nil, errors.New("no git") }
	if got := Rewritten(ctx, failing); got != "checkout Git directory unreadable" {
		t.Errorf("unreadable = %q", got)
	}
}
