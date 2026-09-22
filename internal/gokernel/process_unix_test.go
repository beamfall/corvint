//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package gokernel

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestCancellationKillsAndReapsGitDescendants(t *testing.T) {
	root := testRepository(t)
	bin := t.TempDir()
	started := filepath.Join(bin, "started")
	descendantFile := filepath.Join(bin, "descendant")
	fakeGit := filepath.Join(bin, "git")
	script := "#!/bin/sh\n" +
		"sleep 30 &\n" +
		"child=$!\n" +
		"printf '%s' \"$child\" > '" + descendantFile + "'\n" +
		"printf started > '" + started + "'\n" +
		"wait \"$child\"\n"
	if err := os.WriteFile(fakeGit, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() {
		_, err := ProbeRepositoryContext(ctx, root)
		result <- err
	}()
	deadline := time.Now().Add(30 * time.Second)
	for {
		if _, err := os.Stat(started); err == nil {
			break
		}
		if time.Now().After(deadline) {
			cancel()
			t.Fatalf("fake Git did not start within %s", 30*time.Second)
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	select {
	case err := <-result:
		if kernelCode(err) != "repository-probe-cancelled" {
			t.Fatalf("cancellation = %v", err)
		}
	case <-time.After(30 * time.Second):
		t.Fatalf("cancelled probe did not terminate within %s", 30*time.Second)
	}
	// descendantFile is written by fakeGit strictly before the started marker,
	// so its content should already be visible; under heavy host load reading
	// it here has occasionally raced a not-yet-durable write, so poll instead
	// of reading once.
	readDeadline := time.Now().Add(30 * time.Second)
	var pid int
	for {
		rawPID, readErr := os.ReadFile(descendantFile)
		if readErr == nil {
			if parsed, parseErr := strconv.Atoi(strings.TrimSpace(string(rawPID))); parseErr == nil {
				pid = parsed
				break
			}
		}
		if time.Now().After(readDeadline) {
			t.Fatalf("descendant pid file %s did not contain a parseable pid within %s", descendantFile, 30*time.Second)
		}
		time.Sleep(10 * time.Millisecond)
	}
	deadline = time.Now().Add(30 * time.Second)
	for {
		killErr := syscall.Kill(pid, 0)
		if errors.Is(killErr, syscall.ESRCH) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("Git descendant %d remains alive after %s: %v", pid, 30*time.Second, killErr)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestProbeRetriesAConcurrentCleanToDirtyTransition(t *testing.T) {
	root := testRepository(t)
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	marker := filepath.Join(bin, "first-status-complete")
	wrapper := filepath.Join(bin, "git")
	script := "#!/bin/sh\n" +
		"'" + realGit + "' \"$@\"\n" +
		"code=$?\n" +
		"case \" $* \" in\n" +
		"  *' status '*)\n" +
		"    if [ ! -e '" + marker + "' ]; then\n" +
		"      printf raced > '" + marker + "'\n" +
		"      printf raced > '" + filepath.Join(root, "raced.txt") + "'\n" +
		"    fi\n" +
		"    ;;\n" +
		"esac\n" +
		"exit \"$code\"\n"
	if err := os.WriteFile(wrapper, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	repository, err := ProbeRepository(root)
	if err != nil {
		t.Fatal(err)
	}
	if repository.WorktreeState != "mixed" || repository.DirtyPathCount != 1 {
		t.Fatalf("stale repository snapshot: %#v", repository)
	}
}

// The group is signalled while the exited leader is still unreaped, so its
// PID, and with it the group ID, cannot have been reused by another process.
func TestProcessGroupIsSignalledBeforeLeaderIsReaped(t *testing.T) {
	command := exec.CommandContext(t.Context(), "/bin/sh", "-c", "exit 0")
	configureProcess(command)
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	leader := command.Process.Pid
	observed := errors.New("process group was not signalled")
	previous := signalProcessGroup
	t.Cleanup(func() { signalProcessGroup = previous })
	signalProcessGroup = func(processID int, signal syscall.Signal) error {
		if processID == -leader {
			observed = waitLeaderUnreaped(leader)
		}
		return previous(processID, signal)
	}
	if err := waitGroupLeader(command); err != nil {
		t.Fatal(err)
	}
	if errors.Is(observed, errors.ErrUnsupported) {
		t.Skip("waitid is unavailable on this platform")
	}
	if observed != nil {
		t.Fatalf("group signalled after the leader was reaped: %v", observed)
	}
}
