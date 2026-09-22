//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package server

import (
	"context"
	"io"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/mcp/protocol"
)

func descriptorNonblocking(t *testing.T, fd int) bool {
	t.Helper()
	flags, _, errno := syscall.Syscall(syscall.SYS_FCNTL, uintptr(fd), syscall.F_GETFL, 0)
	if errno != 0 {
		t.Fatal(errno)
	}
	return flags&syscall.O_NONBLOCK != 0
}

// Inherited stdio shares its open file description with the parent, so the
// blocking mode the transport needs while serving must not outlive Serve.
func TestServeRestoresInheritedDescriptorBlockingMode(t *testing.T) {
	server := newTestServer(t, HandlerFunc(func(context.Context, protocol.Request, Notifier) (map[string]any, *protocol.RPCError) {
		return map[string]any{}, nil
	}))
	var inPipe, outPipe [2]int
	if err := syscall.Pipe(inPipe[:]); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Pipe(outPipe[:]); err != nil {
		t.Fatal(err)
	}
	input, clientInput := os.NewFile(uintptr(inPipe[0]), "stdin"), os.NewFile(uintptr(inPipe[1]), "client-stdin")
	clientOutput, output := os.NewFile(uintptr(outPipe[0]), "client-stdout"), os.NewFile(uintptr(outPipe[1]), "stdout")
	for _, file := range []*os.File{input, clientInput, clientOutput, output} {
		t.Cleanup(func() { _ = file.Close() })
	}
	go func() { _, _ = io.Copy(io.Discard, clientOutput) }()
	_, _ = io.WriteString(clientInput, `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{`+requestMeta+`}}`+"\n")
	_ = clientInput.Close()
	done := make(chan error, 1)
	go func() { done <- server.Serve(context.Background(), input, output) }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("serve did not return at EOF")
	}
	if descriptorNonblocking(t, inPipe[0]) || descriptorNonblocking(t, outPipe[1]) {
		t.Fatalf("serve left inherited descriptors nonblocking: stdin=%v stdout=%v", descriptorNonblocking(t, inPipe[0]), descriptorNonblocking(t, outPipe[1]))
	}
}
