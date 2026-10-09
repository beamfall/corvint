//go:build darwin || linux

package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/groupreap"
)

const exitKillHelperEnvironment = "CORVINT_AHI048_EXIT_HELPER"

// AHI-048: exitProcess SIGKILLs a live child group before os.Exit, so neither the abandoned leader
// nor a sleeping member it started outlives the process. Without the kill both would run on for
// minutes, since os.Exit neither cancels contexts nor signals children.
func TestAHI048ExitProcessKillsLiveChildGroups(t *testing.T) {
	helper := exec.Command(os.Args[0], "-test.run=^TestAHI048ExitProcessHelper$")
	helper.Env = append(os.Environ(), exitKillHelperEnvironment+"=1")
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
