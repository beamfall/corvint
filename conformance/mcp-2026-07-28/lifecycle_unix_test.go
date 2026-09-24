//go:build unix

package mcp20260728

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestTerminationSignalsCancelInFlightDescendantGroup(t *testing.T) {
	for _, version := range []string{protocolVersion, "2025-11-25"} {
		t.Run(version, func(t *testing.T) { terminationSignalsCancelInFlightDescendantGroup(t, version) })
	}
}

func terminationSignalsCancelInFlightDescendantGroup(t *testing.T, version string) {
	for _, signal := range []os.Signal{os.Interrupt, syscall.SIGTERM} {
		t.Run(signal.String(), func(t *testing.T) {
			root := fixtureRepository(t)
			fakeDirectory, pidFile := installBlockingFakeGit(t)
			client := startServerWithArguments(t, root, []string{"--protocol-version", version},
				"PATH="+fakeDirectory+string(os.PathListSeparator)+os.Getenv("PATH"),
			)
			t.Cleanup(func() {
				_ = client.stdin.Close()
				if client.command.ProcessState == nil {
					_ = client.command.Process.Kill()
				}
			})
			meta := requestMeta()
			if version == "2025-11-25" {
				initialized := client.call(t, 0, "initialize", map[string]any{"protocolVersion": version, "capabilities": map[string]any{}, "clientInfo": map[string]any{"name": "conformance", "version": "test"}})
				if initialized["error"] != nil {
					t.Fatal(initialized)
				}
				client.sendJSON(t, map[string]any{"jsonrpc": "2.0", "method": "notifications/initialized"})
				meta = map[string]any{}
			}
			client.sendJSON(t, request(90, "tools/call", map[string]any{
				"_meta": meta, "name": "corvint.status", "arguments": map[string]any{},
			}))
			pids := waitForRecordedPIDs(t, pidFile, 3*time.Second)
			t.Cleanup(func() {
				for _, pid := range pids {
					if process, err := os.FindProcess(pid); err == nil {
						_ = process.Kill()
					}
				}
			})
			started := time.Now()
			if err := client.command.Process.Signal(signal); err != nil {
				t.Fatalf("signal %s: %v", signal, err)
			}
			waitForSignalExit(t, client, 3*time.Second)
			_ = client.stdin.Close()
			if elapsed := time.Since(started); elapsed > 3*time.Second {
				t.Fatalf("signal %s exit took %s", signal, elapsed)
			}
			waitForProcessesGone(t, pids, 3*time.Second)
		})
	}
}

// A client that closes the read end of the server's stdout while a tool call
// holds a Git child is a transport failure (MCPV0-011): the next response write
// must fail rather than kill the process by SIGPIPE, cancel the in-flight call,
// reap its descendant group, and exit with the transport-failure status.
func TestClosedStdoutCancelsInFlightDescendantGroup(t *testing.T) {
	t.Run("MCPV0-011 closed stdout cancels descendants", closedStdoutCancelsInFlightDescendantGroup)
}

func closedStdoutCancelsInFlightDescendantGroup(t *testing.T) {
	root := fixtureRepository(t)
	fakeDirectory, pidFile := installBlockingFakeGit(t)
	command := exec.Command(serverBinary, "--root", root)
	command.Env = replaceEnvironment(os.Environ(), "PATH="+fakeDirectory+string(os.PathListSeparator)+os.Getenv("PATH"))
	stdin, err := command.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdoutRead, stdoutWrite, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	client := &stdioClient{command: command, stdin: stdin}
	command.Stdout = stdoutWrite
	command.Stderr = &client.stderr
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	_ = stdoutWrite.Close()
	t.Cleanup(func() {
		_ = stdin.Close()
		if command.ProcessState == nil {
			_ = command.Process.Kill()
		}
	})
	client.sendJSON(t, request(92, "tools/call", map[string]any{
		"_meta": requestMeta(), "name": "corvint.status", "arguments": map[string]any{},
	}))
	pids := waitForRecordedPIDs(t, pidFile, 3*time.Second)
	t.Cleanup(func() {
		for _, pid := range pids {
			if process, err := os.FindProcess(pid); err == nil {
				_ = process.Kill()
			}
		}
	})
	if err := stdoutRead.Close(); err != nil {
		t.Fatal(err)
	}
	client.sendJSON(t, request(93, "server/discover", map[string]any{"_meta": requestMeta()}))
	done := make(chan error, 1)
	go func() { done <- command.Wait() }()
	var waitErr error
	select {
	case waitErr = <-done:
	case <-time.After(3 * time.Second):
		_ = command.Process.Kill()
		<-done
		t.Fatalf("server did not exit after stdout closed; stderr=%q", client.stderr.String())
	}
	var exitErr *exec.ExitError
	if !errors.As(waitErr, &exitErr) || exitErr.ExitCode() != 2 {
		t.Fatalf("closed-stdout exit = %v, want status 2; stderr=%q", waitErr, client.stderr.String())
	}
	if stderr := client.stderr.String(); stderr != "corvint-mcp: transport failed\n" {
		t.Fatalf("closed-stdout stderr = %q", stderr)
	}
	waitForProcessesGone(t, pids, 3*time.Second)
}

// A git planted earlier on PATH after the server starts must never run: the
// server pins one absolute Git executable at start and never looks Git up on
// PATH again (MCPV0-016).
func TestGitPlantedOnPathAfterStartNeverRuns(t *testing.T) {
	t.Run("MCPV0-016 git executable pinned at start", gitPlantedOnPathAfterStartNeverRuns)
	t.Run("MCPV0-016 refuses to start without git", refusesToStartWithoutGit)
}

func refusesToStartWithoutGit(t *testing.T) {
	command := exec.Command(serverBinary, "--root", fixtureRepository(t))
	command.Env = replaceEnvironment(os.Environ(), "PATH="+t.TempDir())
	var stderr strings.Builder
	command.Stderr = &stderr
	var exitErr *exec.ExitError
	if err := command.Run(); !errors.As(err, &exitErr) || exitErr.ExitCode() != 2 || stderr.String() != "corvint-mcp: git unavailable\n" {
		t.Fatalf("start without git: err=%v stderr=%q", err, stderr.String())
	}
}

func gitPlantedOnPathAfterStartNeverRuns(t *testing.T) {
	planted := t.TempDir()
	marker := filepath.Join(t.TempDir(), "planted-git-ran")
	root := fixtureRepository(t)
	client := startServerWithEnv(t, root, "PATH="+planted+string(os.PathListSeparator)+os.Getenv("PATH"))
	defer client.close(t)
	successResult(t, client.call(t, 94, "server/discover", map[string]any{"_meta": requestMeta()}))
	script := "#!/bin/sh\n: > " + shellQuote(marker) + "\nexit 1\n"
	if err := os.WriteFile(filepath.Join(planted, "git"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	status := successResult(t, client.call(t, 95, "tools/call", map[string]any{
		"_meta": requestMeta(), "name": "corvint.status", "arguments": map[string]any{},
	}))
	if status["isError"] == true {
		t.Fatalf("status after planting git=%s", canonicalJSON(status))
	}
	impact := successResult(t, client.call(t, 96, "tools/call", map[string]any{
		"_meta": requestMeta(), "name": "corvint.impact", "arguments": map[string]any{"paths": []any{"pkg/value.go"}, "snapshot": headSnapshot(t, root)},
	}))
	if impact["isError"] == true {
		t.Fatalf("impact with snapshot after planting git=%s", canonicalJSON(impact))
	}
	if _, err := os.Stat(marker); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("git planted on PATH after start ran: %v", err)
	}
}

// Profile /1 (MCPV0-025, MCPV0-026): the CEM seams spawn Git through their
// own runner, which is pinned to the same start-time executable.
func TestTaskReviewCEMReportNeverRunsPlantedGit(t *testing.T) {
	planted := t.TempDir()
	marker := filepath.Join(t.TempDir(), "planted-git-ran")
	root := fixtureRepository(t)
	client := startServerWithArguments(t, root, taskReviewArguments, "PATH="+planted+string(os.PathListSeparator)+os.Getenv("PATH"))
	defer client.close(t)
	successResult(t, client.call(t, 94, "server/discover", map[string]any{"_meta": requestMeta()}))
	script := "#!/bin/sh\n: > " + shellQuote(marker) + "\nexit 1\n"
	if err := os.WriteFile(filepath.Join(planted, "git"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	head := gitOutput(t, root, "rev-parse", "HEAD")
	report := successResult(t, client.call(t, 97, "tools/call", map[string]any{
		"_meta": requestMeta(), "name": "corvint.cem.report", "arguments": map[string]any{"map": "missing.cem.json", "expectedBase": head, "target": head},
	}))
	if code := object(t, report["structuredContent"])["code"]; report["isError"] != true || code != "cem-map-unavailable" {
		t.Fatalf("cem report after planting git=%s", canonicalJSON(report))
	}
	writeCEMMap(t, root, "change.cem.json", head, "pkg/value.go", 1)
	report = successResult(t, client.call(t, 98, "tools/call", map[string]any{
		"_meta": requestMeta(), "name": "corvint.cem.report", "arguments": map[string]any{"map": "change.cem.json", "expectedBase": head, "target": head},
	}))
	if report["isError"] == true {
		t.Fatalf("cem report after planting git=%s", canonicalJSON(report))
	}
	if _, err := os.Stat(marker); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("git planted on PATH after start ran: %v", err)
	}
}

// headSnapshot is a planning-snapshot receipt for a clean HEAD with no changed
// paths; the server validates it with Git on every call that carries it.
func headSnapshot(t *testing.T, root string) map[string]any {
	revision := gitOutput(t, root, "rev-parse", "HEAD")
	emptyPaths := sha256.Sum256([]byte("[]"))
	return map[string]any{
		"schema": "corvint-planning-snapshot/0", "commitRevision": revision, "baseRevision": revision,
		"treeRevision": gitOutput(t, root, "rev-parse", "HEAD^{tree}"),
		"changedPaths": []any{}, "changedPathsSha256": hex.EncodeToString(emptyPaths[:]),
	}
}

// installBlockingFakeGit puts a git on a private PATH directory that records
// its own pid and a background child's pid, then blocks on that child.
func installBlockingFakeGit(t *testing.T) (directory, pidFile string) {
	t.Helper()
	directory = t.TempDir()
	pidFile = filepath.Join(t.TempDir(), "pids")
	script := "#!/bin/sh\n/bin/sleep 60 </dev/null >/dev/null 2>/dev/null &\nchild=$!\nprintf '%s %s\\n' \"$$\" \"$child\" > " + shellQuote(pidFile) + "\nwait \"$child\"\n"
	if err := os.WriteFile(filepath.Join(directory, "git"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return directory, pidFile
}

func waitForSignalExit(t *testing.T, client *stdioClient, timeout time.Duration) {
	t.Helper()
	type drainResult struct {
		body []byte
		err  error
	}
	drained := make(chan drainResult, 1)
	go func() {
		body, err := io.ReadAll(client.stdout)
		drained <- drainResult{body: body, err: err}
	}()
	done := make(chan error, 1)
	go func() { done <- client.command.Wait() }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("server signal exit: %v; stderr=%q", err, client.stderr.String())
		}
		remainder := <-drained
		if remainder.err != nil || len(remainder.body) != 0 {
			t.Fatalf("signal-exit stdout: bytes=%q err=%v", remainder.body, remainder.err)
		}
	case <-time.After(timeout):
		_ = client.command.Process.Signal(syscall.SIGQUIT)
		select {
		case <-done:
		case <-time.After(time.Second):
			_ = client.command.Process.Kill()
			<-done
		}
		<-drained
		t.Fatalf("server did not exit before deadline; SIGQUIT stack=%q", client.stderr.String())
	}
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func TestRecordedPIDsWaitsForEmptyFilePublication(t *testing.T) {
	pidFile := filepath.Join(t.TempDir(), "pids")
	if err := os.WriteFile(pidFile, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	written := make(chan error, 1)
	timer := time.AfterFunc(30*time.Millisecond, func() {
		written <- os.WriteFile(pidFile, []byte("123 456\n"), 0o600)
	})
	t.Cleanup(func() {
		if !timer.Stop() {
			if err := <-written; err != nil {
				t.Error(err)
			}
		}
	})
	pids := waitForRecordedPIDs(t, pidFile, time.Second)
	if len(pids) != 2 || pids[0] != 123 || pids[1] != 456 {
		t.Fatalf("published pids = %v", pids)
	}
}

func waitForRecordedPIDs(t *testing.T, pidFile string, timeout time.Duration) []int {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		raw, err := os.ReadFile(pidFile)
		if err == nil && len(raw) > 0 {
			fields := strings.Fields(string(raw))
			if len(fields) != 2 {
				t.Fatalf("invalid fake Git pid file %q", raw)
			}
			pids := make([]int, 0, len(fields))
			for _, field := range fields {
				pid, parseErr := strconv.Atoi(field)
				if parseErr != nil || pid < 1 {
					t.Fatalf("invalid fake Git pid %q", field)
				}
				pids = append(pids, pid)
			}
			return pids
		}
		if err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("fake Git did not start before %s", timeout)
	return nil
}

func waitForProcessesGone(t *testing.T, pids []int, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		alive := false
		for _, pid := range pids {
			process, err := os.FindProcess(pid)
			if err == nil && process.Signal(syscall.Signal(0)) == nil {
				alive = true
				break
			}
		}
		if !alive {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("descendant processes survived server termination: %v", pids)
}
