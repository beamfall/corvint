//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"

	"github.com/Beamfall/corvint/internal/groupreap"
)

// recordWrapper installs a wrapperHost that records each group signal, probe
// and the reap around the real host (PGO-V0-019, V1-1387).
func recordWrapper(t *testing.T, killErr error) func() []string {
	t.Helper()
	if !groupreap.OwnerAvailable() {
		if code := run([]string{"--", "true"}, launchHooks{}); code != unsupportedExitCode {
			t.Fatalf("run without an owner = %d, want %d", code, unsupportedExitCode)
		}
		t.Skip("no process-group owner on this platform")
	}
	var mu sync.Mutex
	var calls []string
	add := func(call string) { mu.Lock(); calls = append(calls, call); mu.Unlock() }
	old := wrapperHost
	wrapperHost = func() groupreap.Primitives {
		host := old()
		kill, signalGroup, probe := host.KillGroup, host.SignalGroup, host.ProbeGroup
		host.KillGroup = func(leader int) error {
			add("kill")
			if killErr != nil {
				return killErr
			}
			return kill(leader)
		}
		host.SignalGroup = func(leader int, signal syscall.Signal) error {
			add("signal-" + signal.String())
			return signalGroup(leader, signal)
		}
		host.ProbeGroup = func(leader int) (groupreap.Probe, error) { add("probe"); return probe(leader) }
		host.Reap = func(command *exec.Cmd) error { add("reap"); return command.Wait() }
		return host
	}
	t.Cleanup(func() { wrapperHost = old })
	return func() []string { mu.Lock(); defer mu.Unlock(); return slices.Clone(calls) }
}

// TestPGOV0019WrapperRetiresResidueBeforeTheReap runs a command that exits
// leaving a member: SIGTERM and SIGKILL reach the group while the leader is
// unreaped, the member is gone, and only probes follow the reap.
func TestPGOV0019WrapperRetiresResidueBeforeTheReap(t *testing.T) {
	seen := recordWrapper(t, nil)
	pidFile := filepath.Join(t.TempDir(), "pid")
	code := run([]string{"--", "/bin/sh", "-c", "sleep 300 >/dev/null 2>&1 & echo $! > \"$1\"", "sh", pidFile}, launchHooks{})
	raw, err := os.ReadFile(pidFile)
	if err != nil {
		t.Fatal(err)
	}
	member, _ := strconv.Atoi(strings.TrimSpace(string(raw)))
	t.Cleanup(func() { _ = syscall.Kill(member, syscall.SIGKILL) })
	if code != 0 {
		t.Fatalf("run = %d, want 0", code)
	}
	if err := syscall.Kill(member, 0); !errors.Is(err, syscall.ESRCH) {
		t.Fatalf("member %d survived: %v", member, err)
	}
	calls := seen()
	reap := slices.Index(calls, "reap")
	if reap < 0 || slices.Contains(calls[reap+1:], "reap") {
		t.Fatalf("calls = %v, want exactly one reap", calls)
	}
	for _, want := range []string{"signal-terminated", "kill"} {
		if i := slices.Index(calls, want); i < 0 || i > reap {
			t.Fatalf("calls = %v, want %s before the reap", calls, want)
		}
	}
	for _, call := range calls[reap+1:] {
		if call != "probe" {
			t.Fatalf("calls = %v: %s after the reap", calls, call)
		}
	}
}

// TestPGOV0019WrapperUnprovedRetirementIsHold fails the group SIGKILL: the
// wrapper reports holdExitCode, never success, and does not reap.
func TestPGOV0019WrapperUnprovedRetirementIsHold(t *testing.T) {
	seen := recordWrapper(t, errors.New("injected kill failure"))
	if code := run([]string{"--", "/bin/sh", "-c", "exit 0"}, launchHooks{}); code != holdExitCode {
		t.Fatalf("run = %d, want %d", code, holdExitCode)
	}
	if slices.Contains(seen(), "reap") {
		t.Fatalf("calls = %v: a HOLD must not reap", seen())
	}
}

func TestEarlySignalsPreventOrReapEveryLaunchBoundary(t *testing.T) {
	for _, stage := range []string{"after-install", "before-start", "after-start"} {
		for _, signal := range []string{"TERM", "INT", "HUP"} {
			t.Run(stage+"-"+signal, func(t *testing.T) {
				marker := filepath.Join(t.TempDir(), "started")
				command := exec.Command(os.Args[0], "-test.run=^TestEarlySignalHelper$", "--", stage, signal, marker)
				command.Env = append(os.Environ(), "SHADOW_WRAPPER_EARLY_SIGNAL_HELPER=1")
				err := command.Run()
				var exitError *exec.ExitError
				if !errors.As(err, &exitError) || exitError.ExitCode() != 128 {
					t.Fatalf("stage=%s signal=%s err=%v", stage, signal, err)
				}
				if stage != "after-start" {
					if _, err := os.Stat(marker); !os.IsNotExist(err) {
						t.Fatalf("stage=%s launched child before signal handling: %v", stage, err)
					}
				}
			})
		}
	}
}

func TestEarlySignalHelper(t *testing.T) {
	if os.Getenv("SHADOW_WRAPPER_EARLY_SIGNAL_HELPER") != "1" {
		return
	}
	arguments := os.Args
	separator := 0
	for i, value := range arguments {
		if value == "--" {
			separator = i
			break
		}
	}
	if separator == 0 || len(arguments) != separator+4 {
		os.Exit(2)
	}
	stage, signalName, marker := arguments[separator+1], arguments[separator+2], arguments[separator+3]
	signal := map[string]syscall.Signal{"TERM": syscall.SIGTERM, "INT": syscall.SIGINT, "HUP": syscall.SIGHUP}[signalName]
	if signal == 0 {
		os.Exit(2)
	}
	send := func() { _ = syscall.Kill(os.Getpid(), signal) }
	hooks := launchHooks{}
	switch stage {
	case "after-install":
		hooks.afterSignalInstall = send
	case "before-start":
		hooks.beforeStart = send
	case "after-start":
		hooks.afterStart = send
	default:
		os.Exit(2)
	}
	code := run([]string{"--", "/bin/sh", "-c", "printf started > \"$1\"; sleep 30 & wait", "shadow", marker}, hooks)
	os.Exit(code)
}
