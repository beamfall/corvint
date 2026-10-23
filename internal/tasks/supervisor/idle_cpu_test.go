//go:build darwin || linux

package supervisor

import (
	"context"
	"os"
	"strconv"
	"syscall"
	"testing"
	"time"
)

// TestCALV0137_HostWaitIdleCPU is the opt-in V1-0911 measurement of the CPU a
// supervisor spends waiting on an idle host: CORVINT_SUPERVISOR_IDLE_SECONDS
// seconds (a plain `go test` skips it) for each of a normal host and the
// detached OpenCode host whose escape scan runs every escapeScanEvery
// (CAL-V0-136, CAL-V0-137). The waiting loop runs in this process, so its
// cost is this process's user plus system time over the window, reported per
// minute; the host is `sleep`, which spends none.
func TestCALV0137_HostWaitIdleCPU(t *testing.T) {
	seconds, err := strconv.Atoi(os.Getenv("CORVINT_SUPERVISOR_IDLE_SECONDS"))
	if err != nil || seconds < 1 {
		t.Skip("set CORVINT_SUPERVISOR_IDLE_SECONDS to the window to measure")
	}
	cpu := func() time.Duration {
		var ru syscall.Rusage
		if err := syscall.Getrusage(syscall.RUSAGE_SELF, &ru); err != nil {
			t.Fatal(err)
		}
		return time.Duration(ru.Utime.Nano() + ru.Stime.Nano())
	}
	for _, mode := range []string{"normal", "detached"} {
		t.Run(mode, func(t *testing.T) {
			var c Capsule
			if mode == "detached" {
				c, _ = detachedCapsule(t, detachedServer+" & wait")
			} else {
				c, _ = detachedCapsule(t, "sleep 600")
				c.Host = ""
			}
			ctx, cancel := context.WithTimeout(context.Background(), time.Duration(seconds)*time.Second)
			defer cancel()
			start, before := time.Now(), cpu()
			out, _ := Run(ctx, os.Args[0], t.TempDir(), c, func(string, Boot, *Outcome) error { return nil })
			wall, used := time.Since(start), cpu()-before
			t.Logf("%s host: %s over %v wall: %.3f CPU s per minute (class %s)", mode, used, wall.Round(time.Millisecond), used.Seconds()/wall.Minutes(), out.Class)
		})
	}
}

// BenchmarkCALV0136_ProcRowsOfThisHost is the per-scan cost of the detached
// host escape scan (escapeScanEvery) on this host's process table: the
// reader that CAL-V0-136 selects, which is /proc on Linux and the kernel
// table on Darwin.
func BenchmarkCALV0136_ProcRowsOfThisHost(b *testing.B) {
	rows, err := processRows()
	if err != nil {
		b.Fatal(err)
	}
	b.ReportMetric(float64(len(rows)), "processes")
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := processRows(); err != nil {
			b.Fatal(err)
		}
	}
}
