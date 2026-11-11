// SPDX-License-Identifier: Apache-2.0
package watch

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func write(t *testing.T, root, rel, body string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func run(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%v: %v: %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func commit(t *testing.T, root, message string) string {
	t.Helper()
	run(t, root, "git", "add", ".")
	run(t, root, "git", "-c", "user.name=t", "-c", "user.email=t@invalid", "-c", "commit.gpgsign=false", "commit", "-qm", message)
	return run(t, root, "git", "rev-parse", "HEAD")
}

// module is a committed one-package module whose TestFail fails.
func module(t *testing.T) string {
	root := t.TempDir()
	write(t, root, "go.mod", "module example.com/m\n\ngo 1.22\n")
	write(t, root, "p/p.go", "package p\n\nfunc F() int { return 1 }\n")
	write(t, root, "p/p_test.go", "package p\n\nimport \"testing\"\n\nfunc TestPass(t *testing.T) {}\n\nfunc TestFail(t *testing.T) { t.Fail() }\n")
	run(t, root, "git", "init", "-q")
	commit(t, root, "m")
	return root
}

func awaitSave(t *testing.T, ctx context.Context, s *Source) {
	t.Helper()
	select {
	case _, ok := <-s.C:
		if !ok {
			t.Fatalf("watcher stopped before save: %v", s.Err())
		}
	case <-ctx.Done():
		t.Fatal("save was missed")
	}
}

// quietSaves consumes queued notifications until a bounded quiet interval,
// so an earlier save cannot satisfy the next assertion. No filesystem polling.
func quietSaves(t *testing.T, ctx context.Context, s *Source) {
	t.Helper()
	timer := time.NewTimer(30 * time.Millisecond)
	defer timer.Stop()
	for {
		select {
		case _, ok := <-s.C:
			if !ok {
				t.Fatalf("watcher stopped: %v", s.Err())
			}
			timer.Reset(30 * time.Millisecond)
		case <-timer.C:
			return
		case <-ctx.Done():
			t.Fatal("watcher never became quiet")
		}
	}
}
