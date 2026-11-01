package testsupport

import (
	"net"
	"testing"
	"time"
)

// TestASDV0908RefusedLoopbackAddrRefusesAtOnce: a dial to the address fails
// well inside one second, never by its own timeout (V1-4083).
func TestASDV0908RefusedLoopbackAddrRefusesAtOnce(t *testing.T) {
	addr := RefusedLoopbackAddr(t)
	start := time.Now()
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err == nil {
		conn.Close()
		t.Fatalf("dial to %s connected", addr)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("dial to %s took %s: %v", addr, elapsed, err)
	}
	if ne, ok := err.(net.Error); ok && ne.Timeout() {
		t.Fatalf("dial to %s timed out: %v", addr, err)
	}
}
