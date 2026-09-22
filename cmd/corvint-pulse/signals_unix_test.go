//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package main

import (
	"bufio"
	"bytes"
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"
)

const signalHelperEnvironment = "CORVINT_PULSE_SIGNAL_HELPER"

func TestCorvintPulseSignalHelper(t *testing.T) {
	if os.Getenv(signalHelperEnvironment) != "1" {
		return
	}
	os.Args = []string{
		"corvint-pulse", "--root", os.Getenv("CORVINT_PULSE_SIGNAL_ROOT"), "serve", "--stdio",
	}
	main()
}

func TestSIGTERMStopsStdioServer(t *testing.T) {
	command := exec.Command(os.Args[0], "-test.run=^TestCorvintPulseSignalHelper$")
	command.Env = append(os.Environ(),
		signalHelperEnvironment+"=1",
		"CORVINT_PULSE_SIGNAL_ROOT="+t.TempDir(),
	)
	stdin, err := command.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	command.Stderr = &stderr
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- command.Wait() }()
	waited := false
	t.Cleanup(func() {
		_ = stdin.Close()
		if !waited {
			_ = command.Process.Kill()
			<-done
		}
	})

	request := "{\"body\":{\"clientBootId\":\"boot\",\"clientId\":\"client\"},\"clientSeq\":1," +
		"\"protocol\":\"corvint-pulse-snapshot/0\",\"requestId\":\"i1\",\"type\":\"initialize\"}\n"
	if _, err := stdin.Write([]byte(request)); err != nil {
		t.Fatal(err)
	}
	line := make(chan string, 1)
	go func() {
		value, _ := bufio.NewReader(stdout).ReadString('\n')
		line <- value
	}()
	select {
	case value := <-line:
		if !bytes.Contains([]byte(value), []byte(`"type":"initialized"`)) {
			t.Fatalf("initial response = %q", value)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("server did not initialize")
	}

	if err := command.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
		waited = true
	case <-time.After(3 * time.Second):
		t.Fatal("server did not stop after SIGTERM")
	}
}

func TestSIGTERMStopsServerWithUnreadFullStdout(t *testing.T) {
	command := exec.Command(os.Args[0], "-test.run=^TestCorvintPulseSignalHelper$")
	command.Env = append(os.Environ(),
		signalHelperEnvironment+"=1",
		"CORVINT_PULSE_SIGNAL_ROOT="+t.TempDir(),
	)
	command.Stdin = bytes.NewReader(bytes.Repeat([]byte("{}\r\n"), 100_000))
	stdout, err := command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	defer stdout.Close()
	var stderr bytes.Buffer
	command.Stderr = &stderr
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- command.Wait() }()
	select {
	case err := <-done:
		t.Fatalf("server exited before signal: %v; stderr=%q", err, &stderr)
	case <-time.After(300 * time.Millisecond):
	}
	if err := command.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		_ = command.Process.Kill()
		<-done
		t.Fatal("server with unread stdout did not stop after SIGTERM")
	}
}
