//go:build darwin || linux

package console

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func ownedFixture(t *testing.T, mode string) (string, string) {
	t.Helper()
	dir := t.TempDir()
	pidfile := filepath.Join(dir, "child.pid")
	script := filepath.Join(dir, "tool")
	// The trap is fixture containment; normal-leader-exit deliberately leaves
	// the descendant for the runner's group cleanup regression.
	end := map[string]string{"normal": "trap - EXIT; exit 0", "timeout": "wait", "cancel": "wait", "overflow": "yes x", "stderr": "yes x >&2"}[mode]
	body := fmt.Sprintf("#!/bin/sh\nsleep 60 &\nchild=$!\ntrap 'kill \"$child\" 2>/dev/null; wait \"$child\" 2>/dev/null' EXIT\ntrap 'exit 0' INT TERM\necho $child > '%s'\nprintf '%%s' '{\"profile\":\"taskman-command-result/0\",\"outcome\":\"OK\",\"items\":[]}'\n%s\n", pidfile, end)
	if err := os.WriteFile(script, []byte(body), 0755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if raw, err := os.ReadFile(pidfile); err == nil {
			pid, _ := strconv.Atoi(strings.TrimSpace(string(raw)))
			if pid > 0 {
				_ = syscall.Kill(pid, syscall.SIGKILL)
			}
		}
	})
	return script, pidfile
}

// waitChild is a hang detector, not a performance budget (decision 0082):
// ten seconds bounds a stuck fork, not the ordinary cost of a forked shell
// writing its pidfile under host contention.
func waitChild(t *testing.T, path string) int {
	t.Helper()
	pid, _ := waitChildOrRunner(t, path, nil)
	return pid
}

// waitChildOrRunner returns pid 0 with the runner's error when the runner
// joins before the fixture publishes its descendant, distinguishing a runner
// that ended the fixture during shell startup from a stuck fork.
func waitChildOrRunner(t *testing.T, path string, done <-chan error) (int, error) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		raw, err := os.ReadFile(path)
		if err == nil {
			pid, _ := strconv.Atoi(strings.TrimSpace(string(raw)))
			if pid > 0 {
				return pid, nil
			}
		} else {
			lastErr = err
		}
		select {
		case runErr := <-done:
			return 0, runErr
		case <-time.After(5 * time.Millisecond):
		}
	}
	t.Fatalf("child not started within 10s: last pidfile read error=%v", lastErr)
	return 0, nil
}

// assertChildGone is likewise a hang detector: five seconds bounds a stuck
// descendant, not the ordinary cost of process teardown under contention.
func assertChildGone(t *testing.T, pid int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	var lastStat string
	for time.Now().Before(deadline) {
		if errors.Is(syscall.Kill(pid, 0), syscall.ESRCH) {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		cmd := exec.CommandContext(ctx, "/bin/ps", "-p", strconv.Itoa(pid), "-o", "stat=")
		cmd.WaitDelay = time.Second
		out, err := cmd.Output()
		cancel()
		if err != nil && errors.Is(syscall.Kill(pid, 0), syscall.ESRCH) {
			return
		}
		lastStat = strings.TrimSpace(string(out))
		if strings.HasPrefix(lastStat, "Z") {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("owned descendant %d survived 5s, last observed ps stat=%q", pid, lastStat)
}

// Only the timeout mode ends by the runner's timer. Every other mode ends by
// its own mechanism, so its timer and the join wait are hang detectors
// (decision 0082) that cannot end the fixture before its shell publishes the
// descendant. The timeout mode's timer necessarily races that startup: an
// attempt whose timer fires first proves no descendant cleanup and is repeated
// with a doubled timer, bounded as a hang detector.
func TestConsoleProcessLifecycle(t *testing.T) {
	for _, mode := range []string{"normal", "timeout", "cancel", "overflow", "stderr"} {
		t.Run(mode, func(t *testing.T) {
			timeout := time.Minute
			if mode == "timeout" {
				timeout = 500 * time.Millisecond
			}
			for {
				binary, path := ownedFixture(t, mode)
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				done := make(chan error, 1)
				go func() { _, _, err := runTool(ctx, filepath.Dir(binary), nil, timeout, 1024, binary); done <- err }()
				pid, runErr := waitChildOrRunner(t, path, done)
				if pid == 0 && mode == "timeout" && boundaryFailure(runErr) && timeout < 16*time.Second {
					timeout *= 2
					continue
				}
				if pid == 0 {
					t.Fatalf("runner joined before the fixture published its descendant: %v", runErr)
				}
				if mode == "cancel" {
					if err := syscall.Kill(pid, 0); err != nil {
						t.Fatal("child was not live before cancellation")
					}
					cancel()
				}
				select {
				case err := <-done:
					if mode == "normal" && err != nil {
						t.Fatal(err)
					}
					if mode != "normal" && !boundaryFailure(err) {
						t.Fatalf("success envelope overrode %s: %v", mode, err)
					}
				case <-time.After(30 * time.Second):
					t.Fatal("runner did not join within 30s")
				}
				assertChildGone(t, pid)
				return
			}
		})
	}
}
func TestConsoleOverflowCannotAcceptOKEnvelope(t *testing.T) {
	binary, path := ownedFixture(t, "stderr")
	// The stderr limit ends this fixture; its timeout is a hang detector that
	// cannot fire during the shell's startup under contention (decision 0082).
	env, source := (&Taskman{Binary: binary, Repo: filepath.Dir(binary), Timeout: time.Minute}).Run(context.Background(), "version")
	if env != nil || !strings.Contains(source.Err, "output-limit") {
		t.Fatalf("accepted success after stderr overflow: %+v %s", env, source.Err)
	}
	assertChildGone(t, waitChild(t, path))
}
func TestConsoleHTTPProcessCleanup(t *testing.T) {
	for _, mode := range []string{"request-cancel", "foreground-cancel", "listener-close"} {
		t.Run(mode, func(t *testing.T) {
			binary, path := ownedFixture(t, "cancel")
			s, err := New(Options{Addr: "127.0.0.1:0", Repo: filepath.Dir(binary), Binary: binary, Timeout: 10 * time.Second})
			if err != nil {
				t.Fatal(err)
			}
			ln, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			ctx, stop := context.WithCancel(context.Background())
			defer stop()
			defer ln.Close()
			served := make(chan error, 1)
			go func() { served <- s.Serve(ctx, ln) }()
			requestCtx, cancel := context.WithCancel(context.Background())
			defer cancel()
			r, _ := http.NewRequestWithContext(requestCtx, "GET", "http://"+ln.Addr().String()+"/", nil)
			client := &http.Client{Timeout: 6 * time.Second}
			defer client.CloseIdleConnections()
			read := make(chan struct{})
			go func() {
				defer close(read)
				response, err := client.Do(r)
				if err == nil {
					io.Copy(io.Discard, response.Body)
					response.Body.Close()
				}
			}()
			pid := waitChild(t, path)
			if err := syscall.Kill(pid, 0); err != nil {
				t.Fatal("child not live before cancellation")
			}
			switch mode {
			case "request-cancel":
				cancel()
			case "foreground-cancel":
				stop()
			case "listener-close":
				ln.Close()
			}
			select {
			case <-read:
			case <-time.After(7 * time.Second):
				t.Fatal("HTTP request did not return")
			}
			assertChildGone(t, pid)
			stop()
			select {
			case err := <-served:
				if mode != "listener-close" && err != nil {
					t.Fatal(err)
				}
			case <-time.After(7 * time.Second):
				t.Fatal("Serve did not join shutdown")
			}
			conn, err := net.DialTimeout("tcp", ln.Addr().String(), 100*time.Millisecond)
			if err == nil {
				conn.Close()
				t.Fatal("listener survived Serve")
			}
		})
	}
}

// exitFixture writes a tool that prints one envelope and then ends the way
// mode says: an ordinary exit status or death by signal.
func exitFixture(t *testing.T, envelope, end string) string {
	t.Helper()
	script := filepath.Join(t.TempDir(), "tool")
	body := fmt.Sprintf("#!/bin/sh\nprintf '%%s' '%s'\n%s\n", envelope, end)
	if err := os.WriteFile(script, []byte(body), 0755); err != nil {
		t.Fatal(err)
	}
	return script
}

// TestConsoleExitClassification is the review repair for runTool: an
// observed ordinary non-zero exit is distinct from an infrastructure failure,
// an OK envelope cannot ride on a failing status, a REFUSED envelope still
// can, a signal is a boundary failure, and Git's non-zero exits fail. Every
// fixture below ends by its own exit, signal, or Git's own non-zero status,
// never by the timeout, so each timeout is a hang detector, not a
// performance budget (decision 0082): a forked /bin/sh reaching its first
// command can exceed a one- or two-second budget under host load, which
// would misreport as a boundary timeout instead of the asserted outcome.
func TestConsoleExitClassification(t *testing.T) {
	ok := `{"profile":"taskman-command-result/0","outcome":"OK","items":[]}`
	refused := `{"profile":"taskman-command-result/0","outcome":"REFUSED","codes":["UNINITIALIZED"],"items":[]}`
	run := func(binary string) (*Envelope, Source) {
		return (&Taskman{Binary: binary, Repo: filepath.Dir(binary), Timeout: time.Minute}).Run(context.Background(), "version")
	}
	t.Run("ok-envelope-with-failing-exit", func(t *testing.T) {
		binary := exitFixture(t, ok, "exit 3")
		_, _, err := runTool(context.Background(), filepath.Dir(binary), nil, time.Minute, 1024, binary)
		var exit *ExitError
		if !errors.As(err, &exit) || exit.Status != 3 || boundaryFailure(err) {
			t.Fatalf("ordinary exit 3 reported as %v", err)
		}
		env, source := run(binary)
		if env != nil || !strings.Contains(source.Err, "status 3") {
			t.Fatalf("OK envelope accepted on exit 3: %+v %q", env, source.Err)
		}
	})
	t.Run("refused-envelope-with-failing-exit", func(t *testing.T) {
		env, source := run(exitFixture(t, refused, "exit 1"))
		if env == nil || !env.Refused() || source.Codes[0] != "UNINITIALIZED" {
			t.Fatalf("valid refusal lost: %+v %q", env, source.Err)
		}
	})
	t.Run("ok-envelope-on-stderr-with-zero-exit", func(t *testing.T) {
		binary := filepath.Join(t.TempDir(), "tool")
		if err := os.WriteFile(binary, []byte("#!/bin/sh\nprintf '%s' '"+ok+"' >&2\nexit 0\n"), 0o755); err != nil {
			t.Fatal(err)
		}
		if env, source := run(binary); env != nil || source.Err == "" {
			t.Fatalf("OK envelope admitted from stderr on exit 0: %+v %q", env, source.Err)
		}
	})
	t.Run("signal-is-boundary", func(t *testing.T) {
		binary := exitFixture(t, ok, "kill -9 $$")
		_, _, err := runTool(context.Background(), filepath.Dir(binary), nil, time.Minute, 1024, binary)
		if !boundaryFailure(err) || !strings.Contains(err.Error(), "signalled") {
			t.Fatalf("signal death was not a boundary failure: %v", err)
		}
		if env, source := run(binary); env != nil || !strings.Contains(source.Err, "signalled") {
			t.Fatalf("OK envelope accepted after signal: %+v %q", env, source.Err)
		}
	})
	t.Run("git-nonzero-exit-fails", func(t *testing.T) {
		root := t.TempDir()
		init := exec.Command("git", "-C", root, "init", "-q")
		init.Env = gitEnvironment()
		if out, err := init.CombinedOutput(); err != nil {
			t.Fatalf("git init: %s %v", out, err)
		}
		out, err := (Worktree{Root: root, Timeout: time.Minute}).git(context.Background(), "rev-parse", "--verify", "--quiet", "refs/heads/absent")
		if err == nil {
			t.Fatalf("git non-zero exit became success with output %q", out)
		}
		if boundaryFailure(err) {
			t.Fatalf("ordinary git failure classified as boundary: %v", err)
		}
	})
}
