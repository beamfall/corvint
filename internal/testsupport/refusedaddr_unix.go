//go:build unix

package testsupport

import (
	"net"
	"strconv"
	"syscall"
	"testing"
)

// RefusedLoopbackAddr returns a 127.0.0.1 host:port that refuses a TCP dial at
// once (ASD-V0-919). It holds a socket bound to the port without listening, so
// the local kernel owns the port and answers a SYN with a reset. A fixed port
// such as 127.0.0.1:1, or a listener closed before the dial, is not enough: on a
// WSL2 host in mirrored networking mode a loopback dial to a port no Linux
// socket holds is routed to Windows, whose firewall drops it, so the dial waits
// out its connect timeout (about 30 s) instead of failing (V1-4083).
func RefusedLoopbackAddr(t testing.TB) string {
	t.Helper()
	fd, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_STREAM, 0)
	if err != nil {
		t.Fatalf("refused loopback socket: %v", err)
	}
	t.Cleanup(func() { _ = syscall.Close(fd) })
	if err := syscall.Bind(fd, &syscall.SockaddrInet4{Addr: [4]byte{127, 0, 0, 1}}); err != nil {
		t.Fatalf("refused loopback bind: %v", err)
	}
	bound, err := syscall.Getsockname(fd)
	if err != nil {
		t.Fatalf("refused loopback name: %v", err)
	}
	inet, ok := bound.(*syscall.SockaddrInet4)
	if !ok || inet.Port == 0 {
		t.Fatalf("refused loopback name: %#v", bound)
	}
	return net.JoinHostPort("127.0.0.1", strconv.Itoa(inet.Port))
}
