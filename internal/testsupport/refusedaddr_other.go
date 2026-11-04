//go:build !unix || darwin

package testsupport

import (
	"net"
	"testing"
)

// RefusedLoopbackAddr returns a 127.0.0.1 host:port that refuses a TCP dial at
// once (ASD-V0-919). Off Linux-style Unix the address of a listener closed
// before the dial is refused by the local stack. Darwin is here too: its kernel
// drops a SYN to a socket that is bound but not listening without a reset, so
// the bound-socket form in refusedaddr_unix.go waits out the dial timeout there
// (V1-4553).
func RefusedLoopbackAddr(t testing.TB) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("refused loopback listener: %v", err)
	}
	addr := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatalf("refused loopback close: %v", err)
	}
	return addr
}
