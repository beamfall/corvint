// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestOptIn(t *testing.T) {
	if run(nil) != 2 || run([]string{"--experimental", "extra"}) != 2 || run([]string{"--experimental", "--workspace-drift-guard"}) != 2 {
		t.Fatal("missing opt-in accepted")
	}
}

// The subprocess runs the actual signal/stdin ownership path without spawning
// a compiler child; its process must retire even with a partial frame pending.
func TestInterrupt(t *testing.T) {
	if os.Getenv("CORVINT_LSP_TEST_CHILD") == "1" {
		os.Exit(run([]string{"--experimental"}))
	}
	if runtime.GOOS == "windows" {
		t.Skip("experimental transport requires Unix pollable descriptors")
	}
	for _, prefix := range []string{"", "Content-Length: 100\r\n\r\n{"} {
		cmd := exec.Command(os.Args[0], "-test.run=^TestInterrupt$")
		cmd.Env = append(os.Environ(), "CORVINT_LSP_TEST_CHILD=1")
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			t.Fatal(err)
		}
		in, err := cmd.StdinPipe()
		if err != nil {
			t.Fatal(err)
		}
		if err = cmd.Start(); err != nil {
			t.Fatal(err)
		}
		done := make(chan error, 1)
		go func() { done <- cmd.Wait() }()
		t.Cleanup(func() {
			in.Close()
			if cmd.ProcessState == nil {
				cmd.Process.Kill()
			}
		})
		// A real initialization response proves handlers and Serve are active.
		body := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"capabilities":{}}}`
		if _, err = fmt.Fprintf(in, "Content-Length: %d\r\n\r\n%s", len(body), body); err != nil {
			t.Fatal(err)
		}
		ready := make(chan error, 1)
		go func() {
			r := bufio.NewReader(stdout)
			line, e := r.ReadString('\n')
			if e != nil {
				ready <- e
				return
			}
			var length int
			if _, e = fmt.Sscanf(line, "Content-Length: %d", &length); e != nil {
				ready <- e
				return
			}
			if _, e = r.ReadString('\n'); e != nil {
				ready <- e
				return
			}
			_, e = io.CopyN(io.Discard, r, int64(length))
			ready <- e
		}()
		select {
		case err = <-ready:
			if err != nil {
				t.Fatal(err)
			}
		case <-time.After(3 * time.Second):
			cmd.Process.Kill()
			<-done
			t.Fatal("no initialize response")
		}

		if _, err = in.Write([]byte(prefix)); err != nil {
			t.Fatal(err)
		}
		if err = cmd.Process.Signal(syscall.SIGTERM); err != nil {
			t.Fatal(err)
		}
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			cmd.Process.Kill()
			<-done
			t.Fatal("interrupted command failed to retire")
		}
		in.Close()
	}
}

func TestRejectRegularDescriptors(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "regular")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err = pollable(f); err == nil {
		t.Fatal("regular descriptor admitted")
	}
}

// LWB-V0-006 LWB-V0-007 LWB-V0-010: configuration refusals are fixed tokens
// printed before any server starts; other misuse prints the usage line.
func TestBackendRefusals(t *testing.T) {
	if raw := os.Getenv("CORVINT_LSP_TEST_ARGS"); raw != "" {
		var args []string
		json.Unmarshal([]byte(raw), &args)
		os.Exit(run(args))
	}
	dir := t.TempDir()
	server := filepath.Join(dir, "server")
	if err := os.WriteFile(server, []byte("#!/bin/sh\nexit 0\n"), 0700); err != nil {
		t.Fatal(err)
	}
	missing := filepath.Join(dir, "absent-server")
	const usage = "usage: corvint-lsp --experimental [--root ABSOLUTE_ROOT [--gopls ABSOLUTE_BINARY] [--tsserver ABSOLUTE_BINARY] [--ruby-lsp ABSOLUTE_BINARY] [--server SUFFIXES=ABSOLUTE_ARGV]... [--workspace-drift-guard]]\n"
	for _, v := range []struct {
		args []string
		want string
	}{
		{[]string{"--experimental", "--root", dir, "--server", ".py=" + server}, "corvint-lsp: refused: UNSUPPORTED_SUFFIX\n"},
		{[]string{"--experimental", "--root", dir, "--server", ".go=" + server}, "corvint-lsp: refused: UNSUPPORTED_SUFFIX\n"},
		{[]string{"--experimental", "--root", dir, "--tsserver", missing}, "corvint-lsp: refused: SERVER_BINARY_MISSING\n"},
		{[]string{"--experimental", "--root", dir, "--ruby-lsp", server, "--server", ".js=" + missing + " --stdio"}, "corvint-lsp: refused: SERVER_BINARY_MISSING\n"},
		{[]string{"--experimental", "--root", dir, "--gopls", missing}, "corvint-lsp: refused: SERVER_BINARY_MISSING\n"},
		{[]string{"--experimental", "--root", dir, "--ruby-lsp", "ruby-lsp"}, usage},
		{[]string{"--experimental", "--tsserver", server}, usage},
		{[]string{"--experimental", "--root", dir}, usage},
		{[]string{"--experimental", "--root", dir, "--tsserver", server, "--server", ".tsx=" + server}, usage},
		{[]string{"--experimental", "--root", dir, "--server", ".ts,.rb=" + server}, usage},
	} {
		raw, _ := json.Marshal(v.args)
		cmd := exec.Command(os.Args[0], "-test.run=^TestBackendRefusals$")
		cmd.Env = append(os.Environ(), "CORVINT_LSP_TEST_ARGS="+string(raw))
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		err := cmd.Run()
		exit, _ := err.(*exec.ExitError)
		if exit == nil || exit.ExitCode() != 2 || stderr.String() != v.want {
			t.Errorf("%v: %v %q", v.args, err, stderr.String())
		}
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestBackendRefusals$")
	cmd.Env = append(os.Environ(), `CORVINT_LSP_TEST_ARGS=["--help"]`)
	help, err := cmd.Output()
	if err != nil || !strings.HasPrefix(string(help), usage) {
		t.Fatalf("help: %v %q", err, help)
	}
	for _, want := range []string{"-tsserver string", "-ruby-lsp string", "-server value", "SUFFIXES=ABSOLUTE_ARGV", "-gopls string", "-workspace-drift-guard"} {
		if !strings.Contains(string(help), want) {
			t.Errorf("help lacks %q: %s", want, help)
		}
	}
}
