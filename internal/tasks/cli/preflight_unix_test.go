//go:build unix

package cli_test

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/cli"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// preflightInterruptEnv carries the root and argv of a preflight subprocess.
const preflightInterruptEnv = "TOL_PREFLIGHT_INTERRUPT"

// TestTOLV0027_PreflightDeepInterruptRetiresGateAndWorktree: SIGTERM to a
// preflight --deep process while its gate runs kills the gate's process
// group, removes the temporary worktree and refuses.
func TestTOLV0027_PreflightDeepInterruptRetiresGateAndWorktree(t *testing.T) {
	if spec := os.Getenv(preflightInterruptEnv); spec != "" {
		parts := strings.Split(spec, "\x1f")
		var out bytes.Buffer
		code := cli.Run(cli.Env{Cwd: parts[0], Args: parts[1:], Stdin: bytes.NewReader(nil), Stdout: &out, Stderr: os.Stderr})
		_, _ = os.Stdout.Write(out.Bytes())
		os.Exit(code)
	}
	// The gate records its pid in the temporary worktree, which the parent
	// finds through the repository's worktree list.
	r, id, _ := preflightRepo(t, cleanSpec, cleanSeed, deepGate("slow", "echo $$ > gate.pid; exec /bin/sleep 600", "900"))
	args := []string{r.Root, "preflight", id, "--deep", "--gate", "slow"}
	child := exec.Command(os.Args[0], "-test.run=^TestTOLV0027_PreflightDeepInterruptRetiresGateAndWorktree$")
	child.Env = append(os.Environ(), preflightInterruptEnv+"="+strings.Join(args, "\x1f"))
	var out, errb bytes.Buffer
	child.Stdout, child.Stderr = &out, &errb
	child.WaitDelay = 2 * time.Second
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	exited := make(chan error, 1)
	go func() { exited <- child.Wait() }()
	group, worktree := 0, ""
	t.Cleanup(func() {
		if group > 0 {
			_ = syscall.Kill(-group, syscall.SIGKILL)
		}
		if worktree != "" {
			_ = exec.Command("git", "-C", r.Root, "worktree", "remove", "--force", worktree).Run()
		}
	})
	for deadline := time.Now().Add(60 * time.Second); group == 0; time.Sleep(20 * time.Millisecond) {
		for _, line := range strings.Split(gitOut(t, r.Root, "worktree", "list", "--porcelain"), "\n") {
			if p, ok := strings.CutPrefix(line, "worktree "); ok && strings.Contains(p, "corvint-tasks-preflight-") {
				worktree = p
			}
		}
		if worktree != "" {
			if raw, err := os.ReadFile(filepath.Join(worktree, "gate.pid")); err == nil {
				group, _ = strconv.Atoi(strings.TrimSpace(string(raw)))
			}
		}
		if group == 0 && time.Now().After(deadline) {
			_ = child.Process.Kill()
			t.Fatalf("gate never started\n%s", errb.Bytes())
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
	for i := 0; ; i++ {
		if err := syscall.Kill(-group, 0); errors.Is(err, syscall.ESRCH) {
			break
		}
		if i > 200 {
			t.Fatalf("gate group %d survived the interrupt (preflight %s)\n%s", group, child.ProcessState, errb.Bytes())
		}
		time.Sleep(10 * time.Millisecond)
	}
	if _, err := os.Stat(worktree); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("temporary worktree %s left behind: %v", worktree, err)
	}
	if wt := gitOut(t, r.Root, "worktree", "list", "--porcelain"); strings.Count(wt, "worktree ") != 1 {
		t.Fatalf("worktree entry left behind:\n%s", wt)
	}
	res, err := wire.DecodeResult(out.Bytes())
	if err != nil {
		t.Fatalf("envelope: %v (preflight %s)\nstdout=%s\nstderr=%s", err, child.ProcessState, out.Bytes(), errb.Bytes())
	}
	if child.ProcessState.ExitCode() == 0 || res.Outcome != wire.OutcomeRefused {
		t.Fatalf("interrupted preflight: exit %d %s", child.ProcessState.ExitCode(), out.Bytes())
	}
}
