//go:build darwin || linux

package testrunner

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/execfile"
)

// scriptRequest pins a shell script as the primary tool, so a test can stand
// in for a device-bound native tool such as adb.
func scriptRequest(t *testing.T, script string) Request {
	t.Helper()
	root, _ := filepath.EvalSymlinks(t.TempDir())
	if err := os.MkdirAll(filepath.Join(root, "bin"), 0700); err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(root, "bin", "adb")
	if err := execfile.WriteFile(exe, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "source"), []byte("source"), 0600); err != nil {
		t.Fatal(err)
	}
	return Request{Runner: "probe", Root: root, Executable: exe, ExecutableSha256: Digest([]byte(script)), InputFiles: map[string]string{"source": Digest([]byte("source"))}, ReportDir: filepath.Join(root, "report"), TimeoutSeconds: 1}
}

// TestDeviceCleanupAdmission binds TRE-V0-087's closed step shape: a step is
// a declared-tool argv or a loopback WebDriver session deletion, never both,
// and a malformed declaration is refused before anything runs.
func TestDeviceCleanupAdmission(t *testing.T) {
	r := scriptRequest(t, "#!/bin/sh\nexit 0\n")
	for name, steps := range map[string][]CleanupStep{
		"unnamed":       {{Tool: "primary", Argv: []string{"x"}}},
		"duplicate":     {{Name: "a", Argv: []string{"x"}}, {Name: "a", Argv: []string{"y"}}},
		"undeclared":    {{Name: "a", Tool: "missing", Argv: []string{"x"}}},
		"ambiguous":     {{Name: "a", Argv: []string{"x"}, WebDriver: "http://127.0.0.1:4723/", SessionFile: "s"}},
		"empty":         {{Name: "a"}},
		"remote":        {{Name: "a", WebDriver: "http://example.invalid:4723/", SessionFile: "s"}},
		"noncanonical":  {{Name: "a", WebDriver: "http://127.0.0.1:04723/", SessionFile: "s"}},
		"escape":        {{Name: "a", WebDriver: "http://127.0.0.1:4723/", SessionFile: "../s"}},
		"expect-on-web": {{Name: "a", WebDriver: "http://127.0.0.1:4723/", SessionFile: "s", Expect: "x"}},
		"bound":         {{Name: "a", Argv: []string{"x"}}, {Name: "b", Argv: []string{"x"}}, {Name: "c", Argv: []string{"x"}}, {Name: "d", Argv: []string{"x"}}, {Name: "e", Argv: []string{"x"}}},
	} {
		inv := Invocation{Argv: []string{"test"}, DeviceCleanup: steps}
		if _, err := Execute(context.Background(), r, inv); err == nil {
			t.Fatalf("%s: malformed device cleanup admitted", name)
		}
		if _, err := os.Stat(r.ReportDir); err == nil {
			t.Fatalf("%s: refusal created the report directory", name)
		}
	}
}

// TestDeviceCleanupIsBoundedAndRecorded binds TRE-V0-087: a timed-out run
// runs every step under its own bound, records each outcome, and anything
// short of clean is an execution problem, so the observation stays
// incomplete. A step that exits 0 without its completion line is not clean.
func TestDeviceCleanupIsBoundedAndRecorded(t *testing.T) {
	previous := deviceCleanupStepTimeout
	deviceCleanupStepTimeout = 300 * time.Millisecond
	t.Cleanup(func() { deviceCleanupStepTimeout = previous })
	r := scriptRequest(t, "#!/bin/sh\ncase \"$1\" in\ntest|hang) exec /bin/sleep 30;;\nfail) echo nope; exit 9;;\nok) echo done;;\nesac\nexit 0\n")
	inv := Invocation{Argv: []string{"test"}, DeviceCleanup: []CleanupStep{
		{Name: "hang", Tool: "primary", Argv: []string{"hang"}},
		{Name: "fail", Tool: "primary", Argv: []string{"fail"}},
		{Name: "ok", Tool: "primary", Argv: []string{"ok"}, Expect: "done"},
		{Name: "quiet", Tool: "primary", Argv: []string{"quiet"}, Expect: "done"},
	}}
	start := time.Now()
	out, err := Execute(context.Background(), r, inv)
	elapsed := time.Since(start)
	if err != nil || !out.Input.TimedOut {
		t.Fatalf("timeout not observed: %+v %v", out.Input, err)
	}
	d := out.DeviceCleanup
	if d == nil || d.Outcome != CleanupIncomplete || len(d.Steps) != 4 {
		t.Fatalf("cleanup not recorded: %+v", d)
	}
	for i, want := range []string{"timed-out", "failed", "ok", "failed"} {
		if d.Steps[i].Status != want {
			t.Fatalf("step %s: %+v, want %s", d.Steps[i].Name, d.Steps[i], want)
		}
	}
	if d.Steps[1].ExitCode != 9 || !strings.Contains(d.Steps[1].Detail, "nope") {
		t.Fatalf("failed step lost its evidence: %+v", d.Steps[1])
	}
	if elapsed > 1*time.Second+4*(deviceCleanupStepTimeout+deviceCleanupWaitDelay) {
		t.Fatalf("cleanup unbounded: %s", elapsed)
	}
	problems := 0
	for _, p := range out.Input.ExecutionProblems {
		if p.Code == "device-cleanup" {
			problems++
			for _, name := range []string{"hang timed-out", "fail failed", "quiet failed"} {
				if !strings.Contains(p.Detail, name) {
					t.Fatalf("problem omits %q: %s", name, p.Detail)
				}
			}
		}
	}
	if problems != 1 {
		t.Fatalf("incomplete cleanup not an execution problem: %+v", out.Input.ExecutionProblems)
	}
	o := Normalize(out.Input, Observation{Complete: true, RetryInformation: NotApplicable})
	if o.Complete {
		t.Fatal("incomplete device cleanup hidden by a complete observation")
	}
}

// TestDeviceCleanupOnlyAfterTimeoutOrInterrupt binds TRE-V0-087's trigger: a
// run that finished on its own needs no device stop, and an interrupted run
// gets one even though the caller's context is already cancelled.
func TestDeviceCleanupOnlyAfterTimeoutOrInterrupt(t *testing.T) {
	script := "#!/bin/sh\ncase \"$1\" in\nslow) exec /bin/sleep 30;;\nstop) echo \"$1\" >> \"$0.log\"; echo stopped;;\nesac\nexit 0\n"
	r := scriptRequest(t, script)
	steps := []CleanupStep{{Name: "stop", Tool: "primary", Argv: []string{"stop"}, Expect: "stopped"}}
	out, err := Execute(context.Background(), r, Invocation{Argv: []string{"fast"}, DeviceCleanup: steps})
	if err != nil || out.DeviceCleanup == nil || out.DeviceCleanup.Outcome != CleanupNotRequired || len(out.DeviceCleanup.Steps) != 0 {
		t.Fatalf("completed run recorded a device stop: %+v %v", out.DeviceCleanup, err)
	}
	if _, err := os.Stat(r.Executable + ".log"); err == nil {
		t.Fatal("completed run invoked device cleanup")
	}
	r = scriptRequest(t, script)
	r.TimeoutSeconds = 30
	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(300*time.Millisecond, cancel)
	out, err = Execute(ctx, r, Invocation{Argv: []string{"slow"}, DeviceCleanup: steps})
	if err != nil || !out.Input.Interrupted || out.DeviceCleanup.Outcome != CleanupClean || out.DeviceCleanup.Steps[0].Detail != "stopped" {
		t.Fatalf("interrupted run not stopped cleanly: %+v %+v %v", out.Input, out.DeviceCleanup, err)
	}
	for _, p := range out.Input.ExecutionProblems {
		if p.Code == "device-cleanup" {
			t.Fatalf("clean stop reported as a problem: %+v", p)
		}
	}
	if b, err := os.ReadFile(r.Executable + ".log"); err != nil || string(b) != "stop\n" {
		t.Fatalf("device cleanup invocation not recorded once: %q %v", b, err)
	}
}

// TestWebDriverSessionDeletionProvesAbsence binds TRE-V0-087's session
// deletion: the record is read without blocking and only when it is a regular
// file of newline-terminated session ids, every recorded session (the first
// and each reload replacement) is deleted, and only a whole W3C null-value
// success or "invalid session id" answer, read before the deadline, is ok.
func TestWebDriverSessionDeletionProvesAbsence(t *testing.T) {
	const a, b = "0f1e2d3c-4b5a-4968-8776-655443322110", "1a2b3c4d-5e6f-4a0b-9c1d-2e3f4a5b6c7d"
	for _, c := range []struct {
		name, body, status string
		record             func(path string) error
		stall              bool
		deleted            []string
	}{
		{name: "reloaded", body: `{"value":null}`, status: "ok", deleted: []string{a, b}, record: lines(a, b)},
		{name: "repeated", body: `{"value":null}`, status: "ok", deleted: []string{a}, record: lines(a, a)},
		{name: "not-envelope", body: `ok`, status: "failed", deleted: []string{a}, record: lines(a)},
		{name: "value-not-null", body: `{"value":{}}`, status: "failed", deleted: []string{a}, record: lines(a)},
		{name: "stalled-body", stall: true, status: "timed-out", deleted: []string{a}, record: lines(a)},
		{name: "unterminated", body: `{"value":null}`, status: "unobserved", record: func(p string) error { return os.WriteFile(p, []byte(a), 0600) }},
		{name: "too-many", body: `{"value":null}`, status: "unobserved", record: lines(a, b, a, b, a, b, a, b, a)},
		{name: "fifo", body: `{"value":null}`, status: "unobserved", record: func(p string) error { return syscall.Mkfifo(p, 0600) }},
		{name: "symlink", body: `{"value":null}`, status: "unobserved", record: func(p string) error {
			if err := lines(a)(p + ".target"); err != nil {
				return err
			}
			return os.Symlink(filepath.Base(p)+".target", p)
		}},
	} {
		t.Run(c.name, func(t *testing.T) {
			release := make(chan struct{})
			var mu sync.Mutex
			var deleted []string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, q *http.Request) {
				mu.Lock()
				deleted = append(deleted, strings.TrimPrefix(q.URL.Path, "/session/"))
				mu.Unlock()
				w.WriteHeader(http.StatusOK)
				if c.stall {
					_, _ = w.Write([]byte(`{"value":`))
					w.(http.Flusher).Flush()
					select {
					case <-release:
					case <-q.Context().Done():
					}
					return
				}
				_, _ = w.Write([]byte(c.body))
			}))
			defer srv.Close()
			defer close(release)
			dir := t.TempDir()
			if err := c.record(filepath.Join(dir, ".appium-session")); err != nil {
				t.Fatal(err)
			}
			root, err := os.OpenRoot(dir)
			if err != nil {
				t.Fatal(err)
			}
			defer root.Close()
			ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
			defer cancel()
			started := time.Now()
			res := deleteWebDriverSession(ctx, CleanupStep{Name: "delete-session", WebDriver: srv.URL + "/", SessionFile: ".appium-session"}, root)
			if res.Status != c.status || time.Since(started) > 5*time.Second {
				t.Fatalf("result %+v after %s, want %s", res, time.Since(started), c.status)
			}
			mu.Lock()
			defer mu.Unlock()
			if strings.Join(deleted, ",") != strings.Join(c.deleted, ",") {
				t.Fatalf("deleted %q, want %q", deleted, c.deleted)
			}
		})
	}
}

func lines(ids ...string) func(string) error {
	return func(path string) error {
		return os.WriteFile(path, []byte(strings.Join(ids, "\n")+"\n"), 0600)
	}
}
