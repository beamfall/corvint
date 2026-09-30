// SPDX-License-Identifier: AGPL-3.0-or-later
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"syscall"
	"testing"
	"time"
)

func TestOptIn(t *testing.T) {
	if run(nil) != 2 || run([]string{"--experimental", "extra"}) != 2 {
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
