//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"testing"
	"time"
)

func TestCLISignalCancellationBlackBox(t *testing.T) {
	assertSignalCancellation(t, "TestCLIInterruptHelper", "CORVINT_DASHBOARD_INTERRUPT_HELPER")
}

func TestCLISIGTERMCancellationBlackBox(t *testing.T) {
	assertSignalCancellation(t, "TestCLISIGTERMHelper", "CORVINT_DASHBOARD_SIGTERM_HELPER")
}

func TestCLIBrokenStdoutIsContainedBlackBox(t *testing.T) {
	readEnd, writeEnd, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := readEnd.Close(); err != nil {
		t.Fatal(err)
	}
	defer writeEnd.Close()

	command := exec.Command(os.Args[0], "-test.run=TestCLIBrokenStdoutHelper")
	command.Env = append(os.Environ(), "CORVINT_DASHBOARD_BROKEN_STDOUT_HELPER=1")
	command.Stdout = writeEnd
	var stderr bytes.Buffer
	command.Stderr = &stderr
	err = command.Run()
	var exitError *exec.ExitError
	if !errors.As(err, &exitError) || exitError.ExitCode() != 2 {
		t.Fatalf("error=%v stderr=%q", err, stderr.Bytes())
	}
	want := `{"code":"OUTPUT_WRITE_FAILED","profile":"corvint-dashboard-error/0"}` + "\n"
	if stderr.String() != want {
		t.Fatalf("stderr=%q, want=%q", stderr.Bytes(), want)
	}
}

func TestCLIBrokenStdoutHelper(t *testing.T) {
	if os.Getenv("CORVINT_DASHBOARD_BROKEN_STDOUT_HELPER") != "1" {
		return
	}
	body := validSnapshot(t)
	exit := runProcess([]string{"snapshot", "--root", string(os.PathSeparator)}, os.Stdout, os.Stderr,
		func(context.Context, compileRequest) ([]byte, error) { return body, nil })
	os.Exit(exit)
}

func assertSignalCancellation(t *testing.T, helper, environment string) {
	t.Helper()
	command := exec.Command(os.Args[0], "-test.run="+helper)
	command.Env = append(os.Environ(), environment+"=1")
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	err := command.Run()
	var exitError *exec.ExitError
	if !errors.As(err, &exitError) || exitError.ExitCode() != 2 {
		t.Fatalf("error=%v", err)
	}
	want := `{"code":"DASHBOARD_INTERRUPTED","profile":"corvint-dashboard-error/0"}` + "\n"
	if stdout.Len() != 0 || stderr.String() != want {
		t.Fatalf("stdout=%q stderr=%q", stdout.Bytes(), stderr.Bytes())
	}
}

func TestCLIInterruptHelper(t *testing.T) {
	if os.Getenv("CORVINT_DASHBOARD_INTERRUPT_HELPER") != "1" {
		return
	}
	runSignalHelper(t, os.Interrupt)
}

func TestCLISIGTERMHelper(t *testing.T) {
	if os.Getenv("CORVINT_DASHBOARD_SIGTERM_HELPER") != "1" {
		return
	}
	runSignalHelper(t, syscall.SIGTERM)
}

func runSignalHelper(t *testing.T, sent os.Signal) {
	t.Helper()
	ctx, cancel := signal.NotifyContext(context.Background(), terminationSignals()...)
	started := make(chan struct{})
	go func() {
		<-started
		process, err := os.FindProcess(os.Getpid())
		if err != nil {
			return
		}
		_ = process.Signal(sent)
	}()
	exit := runContext(ctx, []string{"snapshot", "--root", string(os.PathSeparator)}, os.Stdout, os.Stderr,
		func(ctx context.Context, _ compileRequest) ([]byte, error) {
			close(started)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(2 * time.Second):
				return nil, &dashboardError{code: errorInternal}
			}
		})
	cancel()
	os.Exit(exit)
}
