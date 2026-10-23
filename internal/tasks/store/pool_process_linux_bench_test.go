//go:build linux

package store

import (
	"context"
	"os"
	"testing"
	"time"
)

// BenchmarkPSRV0204_RunnerIdentity measures one pool runner identity read
// for this process, the call a pool preparation makes (PSR-V0-204, V1-2425).
func BenchmarkPSRV0204_RunnerIdentity(b *testing.B) {
	for i := 0; i < b.N; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		if _, e := poolRunnerIdentityContext(ctx, os.Getpid()); e != nil {
			b.Fatal(e)
		}
		cancel()
	}
}
