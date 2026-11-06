//go:build unix

// SPDX-License-Identifier: Apache-2.0

package main

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// GAG-V0-9962: a group hangup must join isolated loose builds before the
// wrapper removes scratch. Checking the wrapper's group alone misses them.
func TestGroupHangupJoinsLooseDescendantsBeforeWrapperCleanup(t *testing.T) {
	scratch, err := os.MkdirTemp(os.TempDir(), "gag-hup-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(scratch)
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: filepath.Join(scratch, "ready"), Net: "unix"})
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	_ = listener.SetDeadline(time.Now().Add(30 * time.Second))
	root := filepath.Join(scratch, "repo")
	for _, path := range []string{filepath.Join(root, "script"), filepath.Join(scratch, "bin"), filepath.Join(scratch, "tmp")} {
		if err := os.MkdirAll(path, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	wrapper, err := os.ReadFile("../../script/go-archive-gate")
	if err != nil {
		t.Fatal(err)
	}
	gate := filepath.Join(root, "script", "go-archive-gate")
	if err := os.WriteFile(gate, wrapper, 0o700); err != nil {
		t.Fatal(err)
	}
	manifest := workerManifest(t)
	for index, legal := range manifest.Legal {
		mustWrite(t, filepath.Join(root, legal.Path), "fixture legal text\n")
		manifest.Legal[index].SHA256 = fmt.Sprintf("%x", sha256.Sum256([]byte("fixture legal text\n")))
	}
	raw, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(root, "manifest.json"), string(raw))
	mustWrite(t, filepath.Join(root, "go.mod"), "module github.com/Beamfall/corvint\n\ngo 1.27.3\n")
	runGitTest(t, root, "init", "-q")
	runGitTest(t, root, "add", ".")
	runGitTest(t, root, "-c", "user.name=Corvint Test", "-c", "user.email=corvint@example.invalid", "commit", "-qm", "fixture")
	quote := func(s string) string { return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'" }
	// Replace compilation only. The launcher waits like go run; the producer
	// calls the real main, preflight, worker pool, buildOnce and group cleanup.
	shim := "#!/bin/sh\ncase $1 in\nenv) case $2 in GOVERSION) echo go1.27.3;; GOTOOLCHAIN) echo local;; esac;;\n" +
		"run) exec " + quote(os.Args[0]) + " -test.run=^TestHangupProcessHelper$ -- launcher;;\n" +
		"build) exec " + quote(os.Args[0]) + " -test.run=^TestHangupProcessHelper$ -- build " + quote(listener.Addr().String()) + ";;\nesac\n"
	if err := os.WriteFile(filepath.Join(scratch, "bin", "go"), []byte(shim), 0o700); err != nil {
		t.Fatal(err)
	}
	command := exec.Command(gate)
	command.Env = append(os.Environ(), "PATH="+filepath.Join(scratch, "bin")+string(os.PathListSeparator)+os.Getenv("PATH"), "TMPDIR="+filepath.Join(scratch, "tmp"), "CORVINT_ARCHIVE_WORKERS=1")
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	log, err := os.Create(filepath.Join(scratch, "wrapper.log"))
	if err != nil {
		t.Fatal(err)
	}
	defer log.Close()
	command.Stdout, command.Stderr = log, log
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- command.Wait() }()
	defer func() { _ = syscall.Kill(-command.Process.Pid, syscall.SIGKILL) }()
	connection, err := listener.AcceptUnix()
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	_ = connection.SetDeadline(time.Now().Add(10 * time.Second))
	var buildPID, childPID int
	var parent string
	if _, err := fmt.Fscanln(connection, &buildPID, &childPID, &parent); err != nil {
		t.Fatal(err)
	}
	// On any assertion failure release the helper and explicitly retire its
	// isolated group, so a negative regression run cannot leak the fixture.
	defer func() {
		_, _ = connection.Write([]byte("x"))
		_ = syscall.Kill(-buildPID, syscall.SIGTERM)
	}()
	if group, err := syscall.Getpgid(childPID); err != nil || group == command.Process.Pid {
		t.Fatalf("descendant must be outside wrapper group: pgid=%d err=%v", group, err)
	}
	if err := syscall.Kill(-command.Process.Pid, syscall.SIGHUP); err != nil {
		t.Fatal(err)
	}
	var acknowledged [1]byte
	if _, err := connection.Read(acknowledged[:]); err != nil || acknowledged[0] != 'c' {
		t.Fatalf("hangup did not cancel the loose build: %q %v", acknowledged, err)
	}
	select {
	case err := <-done:
		t.Fatalf("wrapper exited before worker retirement: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	if _, err := os.Stat(parent); err != nil {
		t.Fatalf("wrapper removed scratch before worker retirement: %v", err)
	}
	_, _ = connection.Write([]byte("x"))
	select {
	case err := <-done:
		var exit *exec.ExitError
		if !errors.As(err, &exit) || exit.ExitCode() != 129 {
			t.Fatalf("wrapper exit=%v, want 129", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("wrapper did not join workers after hangup")
	}
	for _, pid := range []int{buildPID, childPID} {
		if err := syscall.Kill(pid, 0); !errors.Is(err, syscall.ESRCH) {
			t.Errorf("recorded descendant %d survived wrapper cleanup: %v", pid, err)
		}
	}
	if _, err := os.Stat(parent); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("wrapper scratch survived hangup: %v", err)
	}
}

func TestHangupProcessHelper(t *testing.T) {
	separator := -1
	for index, arg := range os.Args {
		if arg == "--" {
			separator = index
			break
		}
	}
	if separator < 0 {
		return
	}
	args := os.Args[separator+1:]
	switch args[0] {
	case "launcher":
		command := exec.Command(os.Args[0], "-test.run=^TestHangupProcessHelper$", "--", "producer")
		command.Stdout, command.Stderr = os.Stdout, os.Stderr
		if err := command.Run(); err != nil {
			os.Exit(1)
		}
		os.Exit(0)
	case "producer":
		os.Args = []string{os.Args[0], "--manifest", "manifest.json", "--output", filepath.Join(filepath.Dir(os.Getenv("HOME")), "loose")}
		main()
	case "build":
		signals := make(chan os.Signal, 1)
		signal.Notify(signals, syscall.SIGTERM)
		child := exec.Command("sleep", "60")
		if err := child.Start(); err != nil {
			os.Exit(3)
		}
		connection, err := net.Dial("unix", args[1])
		if err == nil {
			_, err = fmt.Fprintln(connection, os.Getpid(), child.Process.Pid, filepath.Dir(os.Getenv("HOME")))
		}
		if err == nil {
			select {
			case <-signals:
				_, _ = connection.Write([]byte("c"))
				_ = connection.SetReadDeadline(time.Now().Add(10 * time.Second))
				var release [1]byte
				_, _ = connection.Read(release[:])
			case <-time.After(30 * time.Second):
			}
		}
		_ = child.Process.Kill()
		_ = child.Wait()
		os.Exit(0)
	}
}
