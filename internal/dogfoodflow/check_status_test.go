package dogfoodflow

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// cleanChange runs dogfood check's cleanliness probe for base..HEAD.
func cleanChange(t *testing.T, root, base string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	run := newCheck(context.Background(), "dogfood-check", CheckOptions{Root: root}, &stdout, &stderr)
	run.base, run.target = base, testGit(t, root, "rev-parse", "HEAD")
	code, err := boundary(context.Background(), func() int { run.requireCleanChange(); return 0 }, func() {})
	if err != nil {
		t.Fatal(err)
	}
	return code, stdout.String(), stderr.String()
}

// DCW-V0-045 (V1-1207): dogfood check's cleanliness probe is Core's hardened
// status, so a repository-configured clean filter never runs; the check
// refuses worktree-status-unavailable instead. Without the filter it keeps
// its dirty-worktree, uncommitted-change and no-change outcomes.
func TestCheckDCWV0045CleanProbeNeverRunsRepositoryFilters(t *testing.T) {
	root := t.TempDir()
	write := func(path, data string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(root, path), []byte(data), 0o666); err != nil {
			t.Fatal(err)
		}
	}
	testGit(t, root, "init", "-q")
	write("a.txt", "a\n")
	write(".gitattributes", "a.txt filter=hostile\n")
	testGit(t, root, "add", "-A")
	testGit(t, root, "commit", "-q", "-m", "base")
	base := testGit(t, root, "rev-parse", "HEAD")
	write("b.txt", "b\n")
	testGit(t, root, "add", "-A")
	testGit(t, root, "commit", "-q", "-m", "change")

	marker := filepath.Join(t.TempDir(), "filter-ran")
	testGit(t, root, "config", "filter.hostile.clean", "touch '"+marker+"'; cat")
	later := time.Now().Add(time.Hour)
	if err := os.Chtimes(filepath.Join(root, "a.txt"), later, later); err != nil {
		t.Fatal(err)
	}
	code, _, stderr := cleanChange(t, root, base)
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("dogfood check ran a repository clean filter: %v", err)
	}
	if code != 2 || !strings.HasPrefix(stderr, "dogfood-check: REFUSE worktree-status-unavailable\n  ") ||
		!strings.HasSuffix(stderr, "  fix: remove the repository configuration the reason names, or run dogfood check in a plain clone (git clone --no-hardlinks)\n") {
		t.Fatalf("filter exit=%d stderr=%q", code, stderr)
	}

	testGit(t, root, "config", "--unset", "filter.hostile.clean")
	if code, stdout, stderr := cleanChange(t, root, base); code != 0 || stdout != "" || stderr != "" {
		t.Fatalf("clean change exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	order := "  required order: commit the change; corvint dogfood change <sha>; commit .corvint/change.cem.json; corvint dogfood change <sha>; corvint dogfood check <sha>\n"
	write("c.txt", "untracked\n")
	if code, _, stderr := cleanChange(t, root, base); code != 2 || stderr != "dogfood-check: REFUSE dirty-worktree\n"+order {
		t.Fatalf("dirty exit=%d stderr=%q", code, stderr)
	}
	head := testGit(t, root, "rev-parse", "HEAD")
	if code, _, stderr := cleanChange(t, root, head); code != 2 || stderr != "dogfood-check: REFUSE uncommitted-change-not-in-base-target\n"+order {
		t.Fatalf("uncommitted exit=%d stderr=%q", code, stderr)
	}
	if err := os.Remove(filepath.Join(root, "c.txt")); err != nil {
		t.Fatal(err)
	}
	if code, stdout, _ := cleanChange(t, root, head); code != 0 || stdout != "dogfood-check: PASS no-change-clean-worktree\n" {
		t.Fatalf("no change exit=%d stdout=%q", code, stdout)
	}
}
