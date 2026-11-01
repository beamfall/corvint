//go:build !unix

package testsupport

import (
	"net"
	"testing"
)

// RefusedLoopbackAddr returns a 127.0.0.1 host:port that refuses a TCP dial at
// once (ASD-V0-908). Off Unix the address of a listener closed before the dial
// is refused by the local stack.
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
