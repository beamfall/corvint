//go:build darwin || linux

package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/gitstatus"
)

const snapshotRefreshWorkerEnv = "CORVINT_INTERNAL_INDEX_REFRESH"
const snapshotRefreshPriorityEnv = "CORVINT_INTERNAL_INDEX_REFRESH_PRIORITY"
const snapshotRefreshWallBound = time.Minute

// Tests can substitute only the scheduler syscall in their exec'd helper when
// the test sandbox denies lowering priority. Production never replaces it.
var snapshotRefreshSetpriority = syscall.Setpriority

// Per-invocation injection keeps parallel tests independent and gives tests
// ownership of every real child. Production always executes this same binary.
type snapshotRefreshCommandKey struct{}
type snapshotRefreshCommand func(root string) (*exec.Cmd, error)

func startSnapshotRefresh(ctx context.Context, root string) string {
	if adapterGetenv(ctx, "CORVINT_INDEX_AUTO_REFRESH") == "0" {
		return "disabled"
	}
	lock, err := contextindex.AcquireSnapshotRefreshLock(root)
	if err != nil {
		return "failed to start"
	}
	if lock == nil {
		return "already running"
	}
	defer lock.Close()
	commandFor := func(root string) (*exec.Cmd, error) {
		executable, err := os.Executable()
		if err != nil {
			return nil, err
		}
		return exec.Command(executable, "--root", root, "index", "--if-stale"), nil
	}
	if injected, ok := ctx.Value(snapshotRefreshCommandKey{}).(snapshotRefreshCommand); ok {
		commandFor = injected
	}
	command, err := commandFor(root)
	if err != nil {
		return "failed to start"
	}
	devnull, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err != nil {
		return "failed to start"
	}
	defer devnull.Close()
	command.Stdin, command.Stdout, command.Stderr = devnull, devnull, devnull
	command.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	command.ExtraFiles = []*os.File{lock}
	env := command.Env
	if env == nil {
		env = os.Environ()
	}
	// Do not let inherited internal markers or GOMAXPROCS override the child
	// boundary. It performs no shell lookup and inherits no hook pipe.
	command.Env = make([]string, 0, len(env)+2)
	for _, entry := range env {
		if !strings.HasPrefix(entry, snapshotRefreshWorkerEnv+"=") && !strings.HasPrefix(entry, snapshotRefreshPriorityEnv+"=") && !strings.HasPrefix(entry, "GOMAXPROCS=") {
			command.Env = append(command.Env, entry)
		}
	}
	command.Env = append(command.Env, snapshotRefreshWorkerEnv+"=1", "GOMAXPROCS=1")
	if err := command.Start(); err != nil {
		return "failed to start"
	}
	// Reap without holding up the hook. If the hook exits first, the detached
	// child is adopted by the OS reaper. No pipes or completion wait are retained.
	go func() { _ = command.Wait() }()
	return "started"
}

// runSnapshotRefreshWorker runs only at process startup with an inherited lock
// and a new session, before ordinary CLI dispatch or Git resolution. All Core
// Git subprocesses inherit this owned group, so the hard stop also retires them.
func runSnapshotRefreshWorker(arguments []string, wallBound time.Duration) int {
	if err := gitstatus.EnableOwnedWorker(); err != nil {
		fmt.Fprintln(os.Stderr, "index refresh group:", err)
		return 2
	}
	// Keep the lock held until the group dies. Never trust or signal a PID from
	// stale metadata: the kernel owns lock recovery even after SIGKILL.
	defer syscall.Kill(-os.Getpid(), syscall.SIGKILL)
	deadline := time.Now().Add(wallBound)
	if runtime.GOOS == "linux" {
		if stage := os.Getenv(snapshotRefreshPriorityEnv); stage != "" {
			nanos, err := strconv.ParseInt(stage, 10, 64)
			if err != nil {
				return 2
			}
			deadline = time.Unix(0, nanos)
		}
	}
	if !time.Now().Before(deadline) {
		return 2
	}
	timer := time.AfterFunc(time.Until(deadline), func() { _ = syscall.Kill(-os.Getpid(), syscall.SIGKILL) })
	defer timer.Stop()
	lock := os.NewFile(3, "index-refresh-lock")
	if lock == nil {
		return 2
	}
	invocation, recognized, err := parseIndexInvocation(arguments)
	if err != nil || !recognized || !invocation.ifStale {
		return 2
	}
	if err := contextindex.ValidateSnapshotRefreshLock(invocation.root, lock); err != nil {
		fmt.Fprintln(os.Stderr, "index refresh lock:", err)
		return 2
	}
	// Linux retains fd 3 and the original wall deadline across its self-exec.
	if err := prepareSnapshotRefreshPriority(deadline); err != nil {
		fmt.Fprintln(os.Stderr, "index refresh priority:", err)
		return 2
	}
	// FD 3 belongs only to this worker, never its Git children.
	syscall.CloseOnExec(3)
	if err := lock.Truncate(0); err != nil {
		return 2
	}
	if _, err := fmt.Fprintf(lock, "{\"pid\":%d,\"started\":%q}\n", os.Getpid(), time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
		return 2
	}
	defer lock.Truncate(0)
	runtime.GOMAXPROCS(1)
	interrupted := make(chan os.Signal, 1)
	signal.Notify(interrupted, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(interrupted)
	go func() {
		<-interrupted
		// Publication/eviction can be outside the cancellable build. Signals
		// retire the whole group even there; atomic rename protects readers.
		_ = syscall.Kill(-os.Getpid(), syscall.SIGKILL)
	}()
	ctx, cancel := context.WithDeadline(context.Background(), deadline.Add(-wallBound/12))
	defer cancel()
	return runIndex(ctx, invocation, os.Stdout, os.Stderr)
}

// Linux nice is per-thread. Exec on the lowered, locked thread discards the
// other runtime threads; every thread in the new image inherits its priority.
// Darwin's PRIO_PROCESS already covers the process and needs no second image.
func prepareSnapshotRefreshPriority(deadline time.Time) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	priority, err := syscall.Getpriority(syscall.PRIO_PROCESS, 0)
	if err != nil {
		return err
	}
	if runtime.GOOS == "linux" {
		priority = 20 - priority
		if os.Getenv(snapshotRefreshPriorityEnv) != "" {
			if priority < 10 {
				return fmt.Errorf("exec inherited nice %d, want >=10", priority)
			}
			return nil
		}
	}
	if err := snapshotRefreshSetpriority(syscall.PRIO_PROCESS, 0, max(priority, 10)); err != nil {
		return err
	}
	if runtime.GOOS != "linux" {
		return nil
	}
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	env := make([]string, 0, len(os.Environ())+1)
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, snapshotRefreshPriorityEnv+"=") {
			env = append(env, entry)
		}
	}
	env = append(env, snapshotRefreshPriorityEnv+"="+strconv.FormatInt(deadline.UnixNano(), 10))
	return syscall.Exec(executable, os.Args, env)
}
