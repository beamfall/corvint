//go:build darwin || linux

package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/groupreap"
	"github.com/Beamfall/corvint/internal/procgroup"
)

const (
	exitKillHelperEnvironment    = "CORVINT_AHI048_EXIT_HELPER"
	exitKillProcgroupEnvironment = "CORVINT_AHI049_PROCGROUP_PIDS"
)

// AHI-048: exitProcess SIGKILLs a live child group before os.Exit, so neither the abandoned leader
// nor a sleeping member it started outlives the process. Without the kill both would run on for
// minutes, since os.Exit neither cancels contexts nor signals children.
func TestAHI048ExitProcessKillsLiveChildGroups(t *testing.T) {
	requireExitProcessKillsHelperChildren(t, "^TestAHI048ExitProcessHelper$", exitKillHelperEnvironment+"=1")
}

// AHI-049: a non-Git group started by internal/procgroup, the runner of the Claude Code source
// handoff's Git reads, is recorded too, so exitProcess kills it while the abandoned Run still holds
// it. Before V1-1040 procgroup started with plain command.Start and both children outlived the exit.
func TestAHI049ExitProcessKillsProcgroupGroup(t *testing.T) {
	pids := filepath.Join(t.TempDir(), "pids")
	requireExitProcessKillsHelperChildren(t, "^TestAHI049ExitProcessProcgroupHelper$", exitKillProcgroupEnvironment+"="+pids)
}

func requireExitProcessKillsHelperChildren(t *testing.T, run, environment string) {
	t.Helper()
	helper := exec.Command(os.Args[0], "-test.run="+run)
	helper.Env = append(os.Environ(), environment)
	stdout, err := helper.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := helper.Start(); err != nil {
		t.Fatal(err)
	}
	var children []int
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		if pid, err := strconv.Atoi(strings.TrimSpace(scanner.Text())); err == nil {
			children = append(children, pid)
		}
	}
	if err := helper.Wait(); err != nil {
		t.Fatalf("helper did not exit 0: %v", err)
	}
	if len(children) != 2 {
		t.Fatalf("helper reported children %v, want a leader and a member", children)
	}
	// A hang detector, not a budget (decision 0082): unkilled children sleep for 300 s.
	deadline := time.Now().Add(time.Minute)
	for _, pid := range children {
		for failOpenAlive(pid) {
			if time.Now().After(deadline) {
				_ = syscall.Kill(-children[0], syscall.SIGKILL)
				t.Fatalf("child %d outlived exitProcess", pid)
			}
			time.Sleep(5 * time.Millisecond)
		}
	}
}

// TestAHI048ExitProcessHelper is the helper process: it starts a recorded group, prints the
// leader and a member, and exits through exitProcess without waiting for either.
func TestAHI048ExitProcessHelper(t *testing.T) {
	if os.Getenv(exitKillHelperEnvironment) != "1" {
		t.Skip("helper process only")
	}
	command := exec.Command("/bin/sh", "-c", "sleep 300 >/dev/null 2>&1 & echo $!; wait")
	groupreap.Contain(command)
	stdout, err := command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := groupreap.StartLive(command); err != nil {
		t.Fatal(err)
	}
	member, err := bufio.NewReader(stdout).ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	fmt.Printf("%d\n%s", command.Process.Pid, member)
	exitProcess(0)
}

// TestAHI049ExitProcessProcgroupHelper is the helper process: procgroup.Run starts a shell that
// leads its own group and writes its PID and a sleeping member's to a file; the helper prints both
// and exits through exitProcess while Run is still waiting on the shell.
func TestAHI049ExitProcessProcgroupHelper(t *testing.T) {
	pids := os.Getenv(exitKillProcgroupEnvironment)
	if pids == "" {
		t.Skip("helper process only")
	}
	script := `sleep 300 >/dev/null 2>&1 & echo "$$ $!" >"$1.tmp" && mv "$1.tmp" "$1"; wait`
	go procgroup.Run(context.Background(), procgroup.Spec{
		Argv: []string{"/bin/sh", "-c", script, "sh", pids}, Dir: filepath.Dir(pids),
		Env: []string{"PATH=/usr/bin:/bin"}, Timeout: 10 * time.Minute,
	})
	// A hang detector, not a budget (decision 0082).
	deadline := time.Now().Add(time.Minute)
	for {
		data, err := os.ReadFile(pids)
		if err == nil {
			fmt.Println(strings.Join(strings.Fields(string(data)), "\n"))
			exitProcess(0)
		}
		if time.Now().After(deadline) {
			t.Fatal("procgroup child did not report its PIDs")
		}
		time.Sleep(5 * time.Millisecond)
	}
}
