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

	"github.com/Beamfall/corvint/internal/trace"
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

// TestLTPMV0517StoreAtMaximumBeyondTheWindow pins the operation budget of
// LTPM-V0-517: a valid store of trace.MaxTraceFiles traces, every one beyond
// the replay window and each on its own tree, plans and applies. The first
// version spent three probes and a tree listing per trace against the fixed
// 1,024-operation default and refused at about 250 traces.
func TestLTPMV0517StoreAtMaximumBeyondTheWindow(t *testing.T) {
	saved := maximumAncestry
	maximumAncestry = 2
	t.Cleanup(func() { maximumAncestry = saved })

	const strandedCount = 10
	root := fastImportRepository(t, trace.MaxTraceFiles-strandedCount+maximumAncestry, strandedCount)
	reachable := strings.Fields(git(t, root, "rev-list", "--skip=2", "HEAD"))
	stranded := strings.Fields(git(t, root, "rev-list", "side", "^HEAD"))
	if len(reachable) != trace.MaxTraceFiles-strandedCount || len(stranded) != strandedCount {
		t.Fatalf("fixture: %d reachable beyond the window, %d stranded", len(reachable), len(stranded))
	}
	git(t, root, "update-ref", "-d", "refs/heads/side")
	directory := filepath.Join(root, ".context-corvint", "traces")
	if err := os.MkdirAll(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, revision := range append(reachable, stranded...) {
		if err := os.WriteFile(filepath.Join(directory, revision+".jsonl"), traceRow(revision), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	result, err := Evaluate(context.Background(), root, Options{})
	if err != nil {
		t.Fatalf("dry run of a store at its maximum: %v", err)
	}
	if result.CandidateTraceFiles != trace.MaxTraceFiles || len(result.StrandedRevisions) != strandedCount {
		t.Fatalf("result: %d candidates, %d stranded; want %d and %d", result.CandidateTraceFiles, len(result.StrandedRevisions), trace.MaxTraceFiles, strandedCount)
	}
	digest := result.PlanDigest
	if _, err := Evaluate(context.Background(), root, Options{Apply: true, PlanDigest: &digest}); err != nil {
		t.Fatalf("apply of a store at its maximum: %v", err)
	}
}

// fastImportRepository builds, with one git fast-import, a main history of
// commits commits and a side branch of side commits off its root, each commit
// changing internal/example/value.go so every tree differs.
func fastImportRepository(t *testing.T, commits, side int) string {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	git(t, root, "init", "-q", "-b", "main")
	var stream strings.Builder
	mark := 0
	write := func(branch, parent string, value int) string {
		mark++
		fmt.Fprintf(&stream, "commit refs/heads/%s\nmark :%d\ncommitter T <t@example.test> 946684800 +0000\ndata 1\nc\n", branch, mark)
		if parent != "" {
			fmt.Fprintf(&stream, "from %s\n", parent)
		}
		body := fmt.Sprintf("package example\n\nconst Value = %d\n", value)
		fmt.Fprintf(&stream, "M 100644 inline .gitignore\ndata 18\n.context-corvint/\nM 100644 inline internal/example/value.go\ndata %d\n%s\n", len(body), body)
		return fmt.Sprintf(":%d", mark)
	}
	first := write("main", "", 0)
	for commit := 1; commit < commits; commit++ {
		write("main", "", commit)
	}
	parent := first
	for commit := 0; commit < side; commit++ {
		parent = write("side", parent, -1-commit)
	}
	command := exec.Command("git", "fast-import", "--quiet")
	command.Dir = root
	command.Stdin = strings.NewReader(stream.String())
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git fast-import: %v\n%s", err, output)
	}
	git(t, root, "checkout", "-q", "main")
	return root
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
