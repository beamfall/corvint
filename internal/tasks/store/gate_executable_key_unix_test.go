//go:build darwin || linux

package store_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestV13010_ReuseKeyBindsResolvedExecutable: a reusable gate whose bare
// argv[0] two agents' PATHs resolve to different programs passes under one;
// the second agent at the same tree executes its own program instead of
// reusing that PASS, and a third agent resolving the first program reuses
// it (V1-3010, TCD-V0-158). It sets PATH, so it does not run in parallel.
func TestV13010_ReuseKeyBindsResolvedExecutable(t *testing.T) {
	const program = "corvint-v13010-gate"
	gate := obj("gateId", str("pathgate"), "kind", str("COMMAND"), "argv", wire.Strings([]string{program}), "cwd", str("WORKTREE"),
		"env", wire.Strings(nil), "timeoutSeconds", str("30"), "expected", obj("exitCode", str("0"), "reducer", wire.Null()),
		"evidence", wire.Strings(nil), "inputs", wire.Strings(nil), "sharedResource", wire.Null(), "reusable", wire.Bool(true), "required", wire.Bool(true))
	agent := map[string]string{}
	for _, name := range []string{"one", "two"} {
		dir := t.TempDir()
		script := "#!/bin/sh\n# agent " + name + "\nprintf x >> .git/runs-pathgate\nexit 0\n"
		if err := os.WriteFile(filepath.Join(dir, program), []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
		agent[name] = dir + string(os.PathListSeparator) + os.Getenv("PATH")
	}
	s := newLeaseStore(t, gate)
	claims, _, _ := s.sharedTree(t, "src", "lib", "cmd")
	t.Setenv("PATH", agent["one"])
	s.passes(t, "gate-a", gateOf(claims[0], "pathgate"), 2)
	t.Setenv("PATH", agent["two"])
	report, err := s.gate(t, "gate-b", gateOf(claims[1], "pathgate"), 3)
	if err != nil || report.Outcome.Outcome != mutation.OutcomeCompleted || report.GateReusedFrom != nil || s.runs(t, "pathgate") != 2 {
		t.Fatalf("a PASS of another resolved program was reused: %+v %v (%d runs)", report, err, s.runs(t, "pathgate"))
	}
	t.Setenv("PATH", agent["one"])
	report, err = s.gate(t, "gate-c", gateOf(claims[2], "pathgate"), 4)
	if err != nil || report.Outcome.Outcome != mutation.OutcomeCompleted || report.GateReusedFrom == nil || s.runs(t, "pathgate") != 2 {
		t.Fatalf("a PASS of the same resolved program was not reused: %+v %v (%d runs)", report, err, s.runs(t, "pathgate"))
	}
}

// TestV13010_UnhashableExecutableIsNotReused: a reusable gate whose bare
// argv[0] resolves to a program this process can execute but not read has
// no content digest, so its PASS is never reused: two such programs would
// otherwise share one key. The second run at the same tree executes.
func TestV13010_UnhashableExecutableIsNotReused(t *testing.T) {
	const program = "corvint-v13010-unhashable"
	trueBin, err := exec.LookPath("true")
	if err != nil {
		t.Skip("no true binary on PATH")
	}
	body, err := os.ReadFile(trueBin)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	target := filepath.Join(dir, program)
	if err := os.WriteFile(target, body, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(target, 0o111); err != nil {
		t.Fatal(err)
	}
	if f, err := os.Open(target); err == nil {
		f.Close()
		t.Skip("this user can read an execute-only file")
	}
	gate := obj("gateId", str("xonly"), "kind", str("COMMAND"), "argv", wire.Strings([]string{program}), "cwd", str("WORKTREE"),
		"env", wire.Strings(nil), "timeoutSeconds", str("30"), "expected", obj("exitCode", str("0"), "reducer", wire.Null()),
		"evidence", wire.Strings(nil), "inputs", wire.Strings(nil), "sharedResource", wire.Null(), "reusable", wire.Bool(true), "required", wire.Bool(true))
	s := newLeaseStore(t, gate)
	claims, _, _ := s.sharedTree(t, "src", "lib")
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	s.passes(t, "gate-a", gateOf(claims[0], "xonly"), 2)
	report, err := s.gate(t, "gate-b", gateOf(claims[1], "xonly"), 3)
	if err != nil || report.Outcome.Outcome != mutation.OutcomeCompleted || report.GateReusedFrom != nil || report.GateExecuted == nil {
		t.Fatalf("a PASS of an unhashable program was reused: %+v %v", report, err)
	}
}

// TestV13010_ExecutableDigestRefusesNonRegularFile: a FIFO on PATH is
// refused before it is opened, so hashing it cannot block on a writer that
// never comes; the run is then not reusable.
func TestV13010_ExecutableDigestRefusesNonRegularFile(t *testing.T) {
	t.Parallel()
	fifo := filepath.Join(t.TempDir(), "fifo")
	if err := syscall.Mkfifo(fifo, 0o755); err != nil {
		t.Skip("mkfifo:", err)
	}
	done := make(chan error, 1)
	go func() {
		_, err := store.ExecutableSha256ForTest(fifo)
		done <- err
	}()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("a FIFO was hashed")
		}
	case <-time.After(10 * time.Second):
		// Release the blocked reader before failing.
		if w, err := os.OpenFile(fifo, os.O_WRONLY|syscall.O_NONBLOCK, 0); err == nil {
			w.Close()
		}
		t.Fatal("hashing a FIFO blocked")
	}
}
