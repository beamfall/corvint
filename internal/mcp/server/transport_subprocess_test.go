package server

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/mcp/protocol"
)

const serveHelperEnvironment = "CORVINT_MCP_SERVE_HELPER"

func TestServeSubprocessHelper(t *testing.T) {
	if os.Getenv(serveHelperEnvironment) != "1" {
		return
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	instance, err := New(Config{
		Name: "corvint-test", Version: "test", Handler: HandlerFunc(
			func(context.Context, protocol.Request, Notifier) (map[string]any, *protocol.RPCError) {
				return nil, protocol.MethodNotFound()
			},
		),
	})
	if err != nil {
		os.Exit(2)
	}
	_, _ = io.WriteString(os.Stderr, "ready\n")
	err = instance.Serve(ctx, os.Stdin, os.Stdout)
	if err != nil && !errors.Is(err, context.Canceled) {
		os.Exit(2)
	}
	os.Exit(0)
}

func TestSignalInterruptsSubprocessStdinPipeRead(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("os.Interrupt delivery is not implemented on Windows")
	}
	command := exec.Command(os.Args[0], "-test.run=^TestServeSubprocessHelper$")
	command.Env = append(os.Environ(), serveHelperEnvironment+"=1")
	stdin, err := command.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stderr, err := command.StderrPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = stdin.Close()
		if command.ProcessState == nil {
			_ = command.Process.Kill()
			_, _ = command.Process.Wait()
		}
	}()
	stderrReader := bufio.NewReader(stderr)
	ready, err := stderrReader.ReadString('\n')
	if err != nil || ready != "ready\n" {
		t.Fatalf("helper ready=%q err=%v", ready, err)
	}
	var stderrRemainder bytes.Buffer
	stderrDone := make(chan error, 1)
	go func() {
		_, err := io.Copy(&stderrRemainder, stderrReader)
		stderrDone <- err
	}()
	// The helper announces after installing signal handling and immediately
	// enters Serve. This delay makes the StdinPipe read durably block.
	time.Sleep(25 * time.Millisecond)
	if err := command.Process.Signal(os.Interrupt); err != nil {
		_ = command.Process.Kill()
		<-stderrDone
		_ = command.Wait()
		t.Fatalf("signal helper: %v", err)
	}
	select {
	case stderrErr := <-stderrDone:
		err := command.Wait()
		if err != nil {
			t.Fatalf("helper exit: %v; stderr=%q; drain=%v", err, stderrRemainder.String(), stderrErr)
		}
		if stderrErr != nil || stderrRemainder.Len() != 0 {
			t.Fatalf("helper stderr=%q drain=%v", stderrRemainder.String(), stderrErr)
		}
		if command.ProcessState == nil || !command.ProcessState.Exited() {
			t.Fatalf("helper was not reaped: state=%v", command.ProcessState)
		}
	case <-time.After(3 * time.Second):
		_ = stdin.Close()
		_ = command.Process.Kill()
		<-stderrDone
		_ = command.Wait()
		t.Fatal("SIGINT did not interrupt subprocess StdinPipe read")
	}
}
