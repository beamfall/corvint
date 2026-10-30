package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

type laneRun struct {
	code           int
	stdout, stderr string
}

// concurrently starts the same dogfood command in every lane before waiting
// for any of them, so the runs overlap.
func (run portableRun) concurrently(t *testing.T, dirs []string, extra func(int) []string, args ...string) []laneRun {
	t.Helper()
	commands := make([]*exec.Cmd, len(dirs))
	stdout, stderr := make([]strings.Builder, len(dirs)), make([]strings.Builder, len(dirs))
	for i, dir := range dirs {
		command, err := startCopiedBinary(func() *exec.Cmd {
			command := exec.Command(run.binary, args...)
			command.Dir = dir
			command.Env = append(append([]string{}, run.env...), extra(i)...)
			command.Stdout, command.Stderr = &stdout[i], &stderr[i]
			return command
		})
		if err != nil {
			t.Fatal(err)
		}
		commands[i] = command
	}
	results := make([]laneRun, len(dirs))
	for i, command := range commands {
		err := command.Wait()
		if exit, ok := err.(*exec.ExitError); ok {
			results[i].code = exit.ExitCode()
		} else if err != nil {
			t.Fatal(err)
		}
		results[i].stdout, results[i].stderr = stdout[i].String(), stderr[i].String()
	}
	return results
}

// DCW-V0-044: two lanes of one repository, each in its own `git worktree add`
// worktree, run change, bind, check and seal at the same time and neither
// refuses: each worktree keeps its own local trace store, evidence directory
// and operation lock, and the primary worktree's store is never written. Run
// in lane A's worktree instead, lane B is refused by lane A's stranded traces,
// and the refusal names the per-lane worktree.
func TestDogfoodDCWV0044WorktreeLanesRunConcurrently(t *testing.T) {
	t.Parallel()
	run := portableDogfoodRunner(t)
	root, base := portableDogfoodRepo(t)
	parent, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	lanes := []string{filepath.Join(parent, "lane-a"), filepath.Join(parent, "lane-b")}
	cemGit(t, root, "worktree", "add", "-q", "-b", "lane-a", lanes[0], "main")
	cemGit(t, root, "worktree", "add", "-q", "-b", "lane-b", lanes[1], base)
	cemWrite(t, lanes[1], "fixture/extra.go", "package fixture\nfunc Extra() int { return 3 }\n")
	cemGit(t, lanes[1], "add", ".")
	cemGit(t, lanes[1], "commit", "-qm", "extra")
	work := cemGit(t, lanes[1], "rev-parse", "HEAD")
	inputsDir := t.TempDir()
	intents := filepath.Join(inputsDir, "intents")
	plans := []string{filepath.Join(inputsDir, "a.tsv"), filepath.Join(inputsDir, "b.tsv")}
	for path, content := range map[string]string{
		intents:  "intent.md\n",
		plans[0]: "1\tfixture/fixture.go\t1:1\timplementation\n1\tintent.md\t5:5\tspecification\n",
		plans[1]: "1\tfixture/fixture.go\t1:1\tdependency\n",
	} {
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	inputs := func(i int) []string {
		return []string{"DOGFOOD_TASK=Lane " + string(rune('A'+i)) + ".", "DOGFOOD_VERIFY=true", "DOGFOOD_OUTCOME=passed",
			"DOGFOOD_CITATIONS=" + plans[i], "DOGFOOD_INTENTS_FILE=" + intents}
	}
	none := func(int) []string { return nil }

	for i, result := range run.concurrently(t, lanes, inputs, "dogfood", "change", base) {
		if result.code != 1 || !strings.Contains(result.stderr, "local-outcome: record-index-failed") {
			t.Fatalf("lane %d first pass exit=%d stderr=%s", i, result.code, result.stderr)
		}
	}
	for _, lane := range lanes {
		cemGit(t, lane, "add", ".corvint/change.cem.json")
		cemGit(t, lane, "commit", "-qm", "chore: bind change evidence")
	}
	for step, args := range [][]string{{"dogfood", "change", base}, {"dogfood", "check", base}, {"dogfood", "seal", base}} {
		extra := none
		if step == 0 {
			extra = inputs
		}
		for i, result := range run.concurrently(t, lanes, extra, args...) {
			if result.code != 0 || strings.Contains(result.stderr, "REFUSE") {
				t.Fatalf("lane %d %v exit=%d stdout=%s stderr=%s", i, args, result.code, result.stdout, result.stderr)
			}
		}
	}
	for _, lane := range lanes {
		if traces, err := os.ReadDir(filepath.Join(lane, ".context-corvint", "traces")); err != nil || len(traces) == 0 {
			t.Fatalf("%s has no trace store of its own: %v", lane, err)
		}
		if status := cemGit(t, lane, "status", "--porcelain"); status != "" {
			t.Fatalf("%s left %q", lane, status)
		}
	}
	if _, err := os.Lstat(filepath.Join(root, ".context-corvint")); !os.IsNotExist(err) {
		t.Fatalf("a lane wrote the primary worktree's trace store: %v", err)
	}

	// The shared-store failure V1-1209 reported: lane B's work run in lane A's
	// worktree, whose traces name commits lane B cannot reach.
	cemGit(t, lanes[0], "checkout", "-q", "--detach", work)
	code, _, stderr := run.exec(t, lanes[0], inputs(1), "dogfood", "change", base)
	if code != 1 || !strings.Contains(stderr, "  local trace store: a recorded trace names a commit no longer reachable from HEAD; ") ||
		!strings.Contains(stderr, "run each lane in its own worktree (git worktree add) or clone") {
		t.Fatalf("shared worktree exit=%d stderr=%s", code, stderr)
	}
}
