//go:build unix

package cli_test

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/cli"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// preflightInterruptEnv carries the root and argv of a preflight subprocess.
const preflightInterruptEnv = "TOL_PREFLIGHT_INTERRUPT"

// interruptedPreflight runs `preflight id --deep` with gates as a
// subprocess (the named test re-executed), waits until the temporary
// directory holding the worktree has every file in pidFiles, each holding a
// pid that a gate wrote, sends SIGTERM and waits for the exit. It returns
// the worktree, the pids and the subprocess's envelope; cleanup kills the
// pids' groups and removes a worktree left behind.
func interruptedPreflight(t *testing.T, test string, r *fixture.Repo, id string, gates []string, pidFiles ...string) (string, []int, *wire.Result, *exec.Cmd) {
	t.Helper()
	args := []string{r.Root, "preflight", id, "--deep"}
	for _, g := range gates {
		args = append(args, "--gate", g)
	}
	child := exec.Command(os.Args[0], "-test.run=^"+test+"$")
	child.Env = append(os.Environ(), preflightInterruptEnv+"="+strings.Join(args, "\x1f"))
	var out, errb bytes.Buffer
	child.Stdout, child.Stderr = &out, &errb
	child.WaitDelay = 2 * time.Second
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	exited := make(chan error, 1)
	go func() { exited <- child.Wait() }()
	pids, worktree := make([]int, len(pidFiles)), ""
	t.Cleanup(func() {
		for _, pid := range pids {
			if pid > 0 {
				_ = syscall.Kill(-pid, syscall.SIGKILL)
				_ = syscall.Kill(pid, syscall.SIGKILL)
			}
		}
		if worktree != "" {
			_ = exec.Command("git", "-C", r.Root, "worktree", "remove", "--force", worktree).Run()
		}
	})
	for deadline := time.Now().Add(60 * time.Second); slices.Contains(pids, 0); time.Sleep(20 * time.Millisecond) {
		for _, line := range strings.Split(gitOut(t, r.Root, "worktree", "list", "--porcelain"), "\n") {
			if p, ok := strings.CutPrefix(line, "worktree "); ok && strings.Contains(p, "corvint-tasks-preflight-") {
				worktree = p
			}
		}
		for i, name := range pidFiles {
			if raw, err := os.ReadFile(filepath.Join(filepath.Dir(worktree), name)); worktree != "" && err == nil {
				pids[i], _ = strconv.Atoi(strings.TrimSpace(string(raw)))
			}
		}
		if slices.Contains(pids, 0) && time.Now().After(deadline) {
			_ = child.Process.Kill()
			t.Fatalf("gates never started\n%s", errb.Bytes())
		}
	}
	if err := child.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	select {
	case <-exited:
	case <-time.After(60 * time.Second):
		_ = child.Process.Kill()
		t.Fatal("preflight did not exit after SIGTERM")
	}
	res, err := wire.DecodeResult(out.Bytes())
	if err != nil {
		t.Fatalf("envelope: %v (preflight %s)\nstdout=%s\nstderr=%s", err, child.ProcessState, out.Bytes(), errb.Bytes())
	}
	return worktree, pids, res, child
}

// runInterruptChild is the subprocess side of interruptedPreflight.
func runInterruptChild() {
	if spec := os.Getenv(preflightInterruptEnv); spec != "" {
		parts := strings.Split(spec, "\x1f")
		var out bytes.Buffer
		code := cli.Run(cli.Env{Cwd: parts[0], Args: parts[1:], Stdin: bytes.NewReader(nil), Stdout: &out, Stderr: os.Stderr})
		_, _ = os.Stdout.Write(out.Bytes())
		os.Exit(code)
	}
}

// waitGone fails unless kill(pid, 0) reports ESRCH within two seconds; a
// negative pid names a process group.
func waitGone(t *testing.T, pid int, what string) {
	t.Helper()
	for i := 0; ; i++ {
		if err := syscall.Kill(pid, 0); errors.Is(err, syscall.ESRCH) {
			return
		}
		if i > 200 {
			t.Fatalf("%s %d survived", what, pid)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// TestTOLV0027_PreflightDeepInterruptRetiresGateAndWorktree: SIGTERM to a
// preflight --deep process while its gate runs kills the gate's process
// group, removes the temporary worktree and refuses.
func TestTOLV0027_PreflightDeepInterruptRetiresGateAndWorktree(t *testing.T) {
	runInterruptChild()
	// The gate records its pid beside the temporary worktree, which the
	// parent finds through the repository's worktree list.
	r, id, _ := preflightRepo(t, cleanSpec, cleanSeed, deepGate("slow", "echo $$ > ../gate.pid; exec /bin/sleep 600", "900"))
	worktree, pids, res, child := interruptedPreflight(t, "TestTOLV0027_PreflightDeepInterruptRetiresGateAndWorktree", r, id, []string{"slow"}, "gate.pid")
	waitGone(t, -pids[0], "gate group")
	if _, err := os.Stat(worktree); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("temporary worktree %s left behind: %v", worktree, err)
	}
	if wt := gitOut(t, r.Root, "worktree", "list", "--porcelain"); strings.Count(wt, "worktree ") != 1 {
		t.Fatalf("worktree entry left behind:\n%s", wt)
	}
	if child.ProcessState.ExitCode() == 0 || res.Outcome != wire.OutcomeRefused {
		t.Fatalf("interrupted preflight: exit %d %+v", child.ProcessState.ExitCode(), res)
	}
}

// TestTOLV0027_PreflightDeepInterruptRetiresEarlierGateDescendants: a
// descendant an earlier gate left running in the background does not
// survive a SIGTERM that arrives while a later gate runs.
func TestTOLV0027_PreflightDeepInterruptRetiresEarlierGateDescendants(t *testing.T) {
	runInterruptChild()
	r, id, _ := preflightRepo(t, cleanSpec, cleanSeed,
		deepGate("spawn", "/bin/sleep 600 >/dev/null 2>&1 & echo $! > ../bg.pid; exit 0", "900"),
		deepGate("slow", "echo $$ > ../gate.pid; exec /bin/sleep 600", "900"))
	_, pids, res, _ := interruptedPreflight(t, "TestTOLV0027_PreflightDeepInterruptRetiresEarlierGateDescendants", r, id, []string{"spawn", "slow"}, "bg.pid", "gate.pid")
	waitGone(t, pids[0], "the earlier gate's background process")
	waitGone(t, -pids[1], "gate group")
	if res.Outcome != wire.OutcomeRefused {
		t.Fatalf("interrupted preflight: %+v", res)
	}
}

// TestTOLV0027_PreflightDeepRetiresAFinishedGatesDescendants: when a gate
// exits, what it left running in its process group is killed before the
// next gate starts.
func TestTOLV0027_PreflightDeepRetiresAFinishedGatesDescendants(t *testing.T) {
	r, id, _ := preflightRepo(t, cleanSpec, cleanSeed,
		deepGate("spawn", "/bin/sleep 600 >/dev/null 2>&1 & echo $! > ../bg.pid; exit 0", "900"),
		deepGate("report", "cat ../bg.pid; exit 1", "900"))
	x := atm(t, r.Root, nil, "preflight", id, "--deep", "--gate", "spawn", "--gate", "report")
	if x.res.Outcome != wire.OutcomeRefused || len(x.res.Items) != 1 || len(field(x.res.Items[0], "findings").Arr) != 1 {
		t.Fatalf("preflight: %s", x.stdout)
	}
	detail := field(field(x.res.Items[0], "findings").Arr[0], "detail").Str
	_, raw, _ := strings.Cut(detail, ": ")
	pid, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || pid <= 0 {
		t.Fatalf("no background pid in %q", detail)
	}
	t.Cleanup(func() { _ = syscall.Kill(pid, syscall.SIGKILL) })
	waitGone(t, pid, "the finished gate's background process")
}

// slowCleanFilterRepo is a preflight repository whose committed slow.txt
// has a clean filter that, in a preflight worktree only, records its pid
// beside the worktree and in pidFile and then sleeps for ten minutes. A gate
// that rewrites slow.txt at the same size makes the post-gate status hash it,
// which runs the filter.
func slowCleanFilterRepo(t *testing.T, gates ...string) (*fixture.Repo, string, string) {
	t.Helper()
	r, id, _ := preflightRepo(t, cleanSpec, cleanSeed, gates...)
	fixture.Write(t, filepath.Join(r.Root, ".gitattributes"), []byte("slow.txt filter=slow\n"))
	fixture.Write(t, filepath.Join(r.Root, "slow.txt"), []byte("slow\n"))
	git(t, r.Root, "add", ".gitattributes", "slow.txt")
	git(t, r.Root, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "commit", "-m", "slow")
	pidFile := filepath.Join(t.TempDir(), "filter.pid")
	git(t, r.Root, "config", "filter.slow.clean",
		`case "$PWD" in *corvint-tasks-preflight-*) echo $$ > ../filter.pid; echo $$ > '`+pidFile+`'; exec /bin/sleep 600;; esac; cat`)
	t.Cleanup(func() {
		if raw, err := os.ReadFile(pidFile); err == nil {
			if pid, _ := strconv.Atoi(strings.TrimSpace(string(raw))); pid > 0 {
				_ = syscall.Kill(pid, syscall.SIGKILL)
			}
		}
	})
	return r, id, pidFile
}

// TestTOLV0027_PreflightDeepInterruptStopsAHungStatus: SIGTERM while the
// post-gate status waits on a slow clean filter, before it has written a
// byte, kills the status's process group, removes the worktree and refuses.
func TestTOLV0027_PreflightDeepInterruptStopsAHungStatus(t *testing.T) {
	runInterruptChild()
	r, id, _ := slowCleanFilterRepo(t, deepGate("touch", "printf 'SLOW\\n' > slow.txt", "900"))
	worktree, pids, res, child := interruptedPreflight(t, "TestTOLV0027_PreflightDeepInterruptStopsAHungStatus", r, id, []string{"touch"}, "filter.pid")
	waitGone(t, pids[0], "the status's clean filter")
	if _, err := os.Stat(worktree); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("temporary worktree %s left behind: %v", worktree, err)
	}
	if wt := gitOut(t, r.Root, "worktree", "list", "--porcelain"); strings.Count(wt, "worktree ") != 1 {
		t.Fatalf("worktree entry left behind:\n%s", wt)
	}
	if child.ProcessState.ExitCode() == 0 || res.Outcome != wire.OutcomeRefused {
		t.Fatalf("interrupted preflight: exit %d %+v", child.ProcessState.ExitCode(), res)
	}
}

// TestTOLV0027_PreflightDeepStatusPastTheGateTimeoutFails: a post-gate
// status that does not finish within the gate's timeout is killed and the
// gate is a DEEP_CHECK_FAILED finding, never a clean pass; the worktree is
// removed.
func TestTOLV0027_PreflightDeepStatusPastTheGateTimeoutFails(t *testing.T) {
	r, id, pidFile := slowCleanFilterRepo(t, deepGate("touch", "printf 'SLOW\\n' > slow.txt", "2"))
	started := time.Now()
	x := atm(t, r.Root, nil, "preflight", id, "--deep", "--gate", "touch")
	if elapsed := time.Since(started); elapsed > 60*time.Second {
		t.Fatalf("preflight took %s", elapsed)
	}
	if x.res.Outcome != wire.OutcomeRefused || len(x.res.Items) != 1 {
		t.Fatalf("preflight: %s", x.stdout)
	}
	findings := field(x.res.Items[0], "findings").Arr
	if len(findings) != 1 || field(findings[0], "kind").Str != "DEEP_CHECK_FAILED" ||
		!strings.Contains(field(findings[0], "detail").Str, "could not be checked") {
		t.Fatalf("findings: %s", x.stdout)
	}
	if wt := gitOut(t, r.Root, "worktree", "list", "--porcelain"); strings.Count(wt, "worktree ") != 1 {
		t.Fatalf("worktree entry left behind:\n%s", wt)
	}
	raw, err := os.ReadFile(pidFile)
	if err != nil {
		t.Fatalf("the clean filter never ran: %v", err)
	}
	pid, _ := strconv.Atoi(strings.TrimSpace(string(raw)))
	waitGone(t, pid, "the status's clean filter")
}
