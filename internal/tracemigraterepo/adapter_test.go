package tracemigraterepo

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestLTPMV0517DeepHistoryBoundsByTraceStore pins LTPM-V0-517 (V1-3034) with an
// injected replay window of two commits over a five-commit history: a canonical
// trace beyond the window and a trace stranded by an amend are both planned,
// where the pre-change adapter refused with "Git ancestry exceeds bounded trace
// replay limit"; a legacy tree name beyond the window refuses naming the step
// that retires it, and that step lets the dry run succeed.
func TestLTPMV0517DeepHistoryBoundsByTraceStore(t *testing.T) {
	saved := maximumAncestry
	maximumAncestry = 2
	t.Cleanup(func() { maximumAncestry = saved })

	root := deepRepository(t, 5)
	oldest := git(t, root, "rev-list", "--max-parents=0", "HEAD")
	oldestTree := git(t, root, "rev-parse", oldest+"^{tree}")
	stranded := git(t, root, "rev-parse", "HEAD")
	git(t, root, "commit", "-q", "--amend", "-m", "amended")
	directory := filepath.Join(root, ".context-corvint", "traces")
	if err := os.MkdirAll(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, revision := range []string{oldest, stranded} {
		if err := os.WriteFile(filepath.Join(directory, revision+".jsonl"), traceRow(revision), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	result, err := Evaluate(context.Background(), root, Options{})
	if err != nil {
		t.Fatalf("dry run over a history deeper than the window: %v", err)
	}
	if result.CandidateTraceFiles != 2 || len(result.StrandedRevisions) != 1 || result.StrandedRevisions[0] != stranded {
		t.Fatalf("result = %+v, want the canonical trace beyond the window and one stranded trace %s", result, stranded)
	}

	legacy := filepath.Join(directory, oldestTree+".jsonl")
	if err := os.WriteFile(legacy, traceRow(oldestTree), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = Evaluate(context.Background(), root, Options{})
	want := "move .context-corvint/traces/" + oldestTree + ".jsonl out of .context-corvint/ to retire it, then rerun corvint migrate-traces --dry-run (LTPM-V0-517)"
	if err == nil || !strings.Contains(err.Error(), want) || !strings.Contains(err.Error(), "bounded legacy tree replay limit 2") {
		t.Fatalf("legacy tree beyond the window: error = %v, want the recovery %q", err, want)
	}
	if err := os.Rename(legacy, filepath.Join(t.TempDir(), oldestTree+".jsonl")); err != nil {
		t.Fatal(err)
	}
	if _, err := Evaluate(context.Background(), root, Options{}); err != nil {
		t.Fatalf("the named recovery did not unblock the dry run: %v", err)
	}
}

func traceRow(revision string) []byte {
	basis := fmt.Sprintf(`{"changed_paths":["internal/example/value.go"],"opened_paths":["internal/example/value.go"],"outcome":"passed","revision":"%s","schema_version":1,"task":"verify example trace","verification":["git diff --check"]}`, revision)
	id := fmt.Sprintf("%x", sha256.Sum256([]byte(basis)))
	return []byte(strings.Replace(basis, `,"verification":`, `,"trace_id":"`+id+`","verification":`, 1) + "\n")
}

func deepRepository(t *testing.T, commits int) string {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "internal", "example"), 0o755); err != nil {
		t.Fatal(err)
	}
	for path, data := range map[string]string{
		".gitignore": ".context-corvint/\n",
		"go.mod":     "module example.test/traces\n\ngo 1.27.0\n",
	} {
		if err := os.WriteFile(filepath.Join(root, path), []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	git(t, root, "init", "-q")
	git(t, root, "config", "user.email", "corvint@example.test")
	git(t, root, "config", "user.name", "Corvint Test")
	for commit := 0; commit < commits; commit++ {
		value := fmt.Sprintf("package example\n\nconst Value = %d\n", commit)
		if err := os.WriteFile(filepath.Join(root, "internal", "example", "value.go"), []byte(value), 0o644); err != nil {
			t.Fatal(err)
		}
		git(t, root, "add", ".")
		git(t, root, "commit", "-qm", fmt.Sprintf("commit %d", commit))
	}
	return root
}

func git(t *testing.T, root string, arguments ...string) string {
	t.Helper()
	command := exec.Command("git", arguments...)
	command.Dir = root
	command.Env = append(os.Environ(), "GIT_AUTHOR_DATE=2000-01-01T00:00:00Z", "GIT_COMMITTER_DATE=2000-01-01T00:00:00Z")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", arguments, err, output)
	}
	return strings.TrimSpace(string(output))
}
